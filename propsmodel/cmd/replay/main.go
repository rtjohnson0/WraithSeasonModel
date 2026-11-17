// Command replay reproduces saved pregame decisions using their original feature
// vectors and model checkpoints, then scores matched V1/V2 market questions.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/backtest"
	"propsmodel/internal/eval"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"regexp"
	"sort"
	"time"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	fitcal := flag.Bool("fitcal", false, "fit production calibration from timestamped raw decisions on completed earlier dates")
	flag.Parse()
	files, err := filepath.Glob(filepath.Join(*dir, "replay", "decisions_*.jsonl"))
	if err != nil {
		fatal(err)
	}
	if len(files) == 0 {
		fmt.Println("No decision captures yet. Run predict; replay evidence is retained in workflow artifacts.")
		return
	}
	models := map[string]*predictor.Model{}
	latest := map[string]replay.Record{}
	checked := 0
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			fatal(err)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 65536), 1<<24)
		for sc.Scan() {
			var r replay.Record
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				fatal(err)
			}
			if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(r.ModelID) {
				fatal(fmt.Errorf("invalid checkpoint id"))
			}
			at, err := time.Parse(time.RFC3339, r.At)
			if err != nil || !props.BeforeTip(r.Tip, at) {
				fatal(fmt.Errorf("non-pregame replay record"))
			}
			m := models[r.ModelID]
			if m == nil {
				m, err = predictor.Load(filepath.Join(*dir, "replay", "model_"+r.ModelID+".json"))
				if err != nil {
					fatal(err)
				}
				models[r.ModelID] = m
			}
			p, err := replay.Recompute(m, r)
			if err != nil {
				fatal(err)
			}
			if math.Abs(p-r.P1) > 1e-9 {
				fatal(fmt.Errorf("replay differs from logged prediction: game %d player %d %s", r.Row.G.GameID, r.Row.G.PlayerID, r.Stat))
			}
			checked++
			key := fmt.Sprintf("%s|%d|%d|%s", r.Date, r.Row.G.GameID, r.Row.G.PlayerID, r.Stat)
			if prev, ok := latest[key]; !ok || r.At > prev.At {
				latest[key] = r
			}
		}
		if err := sc.Err(); err != nil {
			fatal(err)
		}
		f.Close()
	}
	fmt.Printf("Reproduced %d decisions exactly from %d model checkpoints.\n", checked, len(models))
	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fatal(err)
	}
	var markets []lineslog.Market
	for _, r := range latest {
		markets = append(markets, lineslog.Market{Date: r.Date, At: r.At, Tip: r.Tip, GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Stat: string(r.Stat), Line: r.Line, POver: r.P1, P2: r.P2, AuditVersion: props.QuoteVersion})
	}
	sort.Slice(markets, func(i, j int) bool { return markets[i].Date < markets[j].Date })
	if *fitcal {
		current, err := predictor.Load(filepath.Join(*dir, "model.json"))
		if err != nil {
			fatal(err)
		}
		config := pipeline.ConfigName(current.Disabled)
		var rs []backtest.Result
		for _, r := range latest {
			old := models[r.ModelID]
			if old.Version != current.Version || pipeline.ConfigName(old.Disabled) != config {
				continue
			}
			// Default policy only: experimental post-fit overlays require their own calibration.
			if r.Policy.MinutesShift != 0 || r.Policy.BigCut != 0 || r.Policy.Blend != 0 {
				continue
			}
			changed := false
			for _, f := range r.Policy.Scale {
				if f != 0 && math.Abs(f-1) > 1e-9 {
					changed = true
				}
			}
			if changed {
				continue
			}
			actual, played := d.Builder.Index().Actual(r.Row.G.GameID, r.Row.G.PlayerID)
			if !played {
				continue
			}
			raw, err := old.Forecast(r.Row.Features(), r.Policy).ProbOver(r.Stat, r.Line)
			if err != nil {
				fatal(err)
			}
			rs = append(rs, backtest.Result{Date: r.Date, GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Stat: r.Stat, Line: r.Line, POver: raw.Over, Market: r.Market, Actual: actual.Value(r.Stat), OverOdds: r.OverOdds, UnderOdds: r.UnderOdds})
		}
		backtest.SortChronologically(rs)
		first, test := backtest.Split(rs)
		dates := map[string]bool{}
		for _, r := range test {
			dates[r.Date] = true
		}
		if len(first) < 200 || len(test) < 100 || len(dates) < 10 {
			fmt.Println("Not enough timestamped decisions: need 200 earlier calibration rows and 100 later rows across 10 evaluation dates. No file written.")
		} else {
			trial := eval.FitCalibration(first)
			scored := trial.Results(test)
			baseline := append([]backtest.Result(nil), test...)
			for i := range baseline {
				baseline[i].POver = baseline[i].Market
			}
			improvement, z := eval.PairedGain(scored, baseline)
			fmt.Printf("Timestamped holdout: %d rows; calibrated log loss %.5f, market log loss %.5f; date-clustered gain %+.5f z %.2f\n", len(test), eval.Score(scored).LogLoss, eval.Score(baseline).LogLoss, improvement, z)
			if improvement > 0 && z >= 2 {
				final := eval.FitCalibration(rs)
				final.Config = config
				final.ModelVersion = current.Version
				if err := final.Save(filepath.Join(*dir, "calibration.json")); err != nil {
					fatal(err)
				}
				fmt.Println("Saved calibration from prior timestamped decisions. Future dates are the next validation.")
			} else {
				fmt.Println("No clear holdout improvement over the market; production calibration unchanged.")
			}
		}
	}
	graded := lineslog.GradeMarkets(markets, lineslog.Actualer(d.Builder.Index().Actual))
	model, base, n := lineslog.Skill(graded)
	fmt.Printf("%d final questions: Brier model %.5f, earlier-date baseline %.5f\n", n, model, base)
	c := lineslog.CompareModels(graded)
	fmt.Printf("Matched questions %d over %d dates: V1 %.5f V2 %.5f; V2 gain %+.5f, date-clustered z %.2f\n", c.N, c.Days, c.Brier1, c.Brier2, c.Gain, c.Z)
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
