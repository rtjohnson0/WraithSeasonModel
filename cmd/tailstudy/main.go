// tailstudy is a development-only study. It excludes the previously examined
// March 14-June 13 test period, writes a separate candidate and never promotes it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/backtest"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"sort"
	"time"
)

type key struct{ game, player int }
type forecast struct {
	Raw       [2]backtest.Result `json:"raw"`
	Mean      [2]float64         `json:"mean"`
	Min       [2]float64         `json:"min"`
	ActualMin float64            `json:"actualMin"`
}
type row struct {
	Name     string       `json:"name"`
	Stat     string       `json:"stat"`
	Metrics  eval.Metrics `json:"metrics"`
	Picks    int          `json:"picks"`
	Overs    int          `json:"overs"`
	Unders   int          `json:"unders"`
	WinRate  float64      `json:"winRate"`
	ROI      float64      `json:"roi"`
	MeanProb float64      `json:"meanProb"`
	StatMAE  float64      `json:"statMAE"`
	MinMAE   float64      `json:"minMAE"`
}

func main() {
	dir := flag.String("data", "data", "data directory")
	out := flag.String("out", "research/tails", "isolated output root")
	cache := flag.String("cache", "", "reuse this study's saved development questions; does not create a new test")
	write := flag.Bool("writecandidate", false, "write separate model_tail.json and calibration_tail.json in data; never replace primary")
	flag.Parse()
	fitBefore := "2026-02-04"
	devBefore := "2026-03-14"
	run := filepath.Join(*out, time.Now().UTC().Format("20060102T150405.000000000Z"))
	if e := os.MkdirAll(run, 0755); e != nil {
		fatal(e)
	}
	var qs []forecast
	var d *pipeline.Data
	var e error
	if *cache != "" {
		raw, e := os.ReadFile(*cache)
		if e != nil {
			fatal(e)
		}
		if e = json.Unmarshal(raw, &qs); e != nil {
			fatal(e)
		}
		for _, q := range qs {
			if q.Raw[0].Date >= devBefore {
				fatal(fmt.Errorf("cache includes the previously seen final period"))
			}
		}
	} else {
		fmt.Println("Loading historical development data; the previously seen final-test period is excluded.")
		pipeline.Missed = false
		d, e = pipeline.Load(*dir, true)
		if e != nil {
			fatal(e)
		}
		lines, e := props.LoadOpening(filepath.Join(*dir, "opening_props_2025.jsonl"))
		if e != nil {
			fatal(e)
		}
		ix := map[key]*feat.Row{}
		for i := range d.Rows {
			r := &d.Rows[i]
			ix[key{r.G.GameID, r.G.PlayerID}] = r
		}
		months := map[string][]props.Line{}
		for _, l := range backtest.MedianLines(lines) {
			r := ix[key{l.GameID, l.PlayerID}]
			if r == nil || r.G.Season != 2025 || r.G.Date >= devBefore || !props.ValidOdds(l.OverOdds) || !props.ValidOdds(l.UnderOdds) {
				continue
			}
			months[r.G.Date[:7]] = append(months[r.G.Date[:7]], l)
		}
		var names []string
		for m := range months {
			names = append(names, m)
		}
		sort.Strings(names)
		for _, month := range names {
			cut := sort.Search(len(d.Rows), func(i int) bool { return d.Rows[i].G.Date >= month+"-01" })
			if cut < 1000 {
				continue
			}
			fmt.Println("Training reference and tail uncertainty before", month, "on", cut, "rows")
			base, e := predictor.Train(d.Rows[:cut], pipeline.DisabledFor(false))
			if e != nil {
				fatal(e)
			}
			tail, e := predictor.FitTailUncertainty(base, d.Rows[:cut])
			if e != nil {
				fatal(e)
			}
			for _, l := range months[month] {
				r := ix[key{l.GameID, l.PlayerID}]
				a, b := base.Predict(r), tail.Predict(r)
				if !model.Eligible(r.AvgMin15, a.Minutes) {
					continue
				}
				q := forecast{ActualMin: r.G.Minutes}
				for i, p := range []predictor.Pred{a, b} {
					pr, e := p.ProbOver(l.Stat, l.Line)
					if e != nil {
						fatal(e)
					}
					q.Raw[i] = backtest.Result{Date: r.G.Date, GameID: l.GameID, PlayerID: l.PlayerID, Stat: l.Stat, Line: l.Line, POver: pr.Over, Actual: r.G.Value(l.Stat), OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, Market: bets.MarketOver(l.OverOdds, l.UnderOdds)}
					q.Mean[i] = p.Mean(l.Stat)
					q.Min[i] = p.Minutes
				}
				qs = append(qs, q)
			}
		}
	}
	sort.Slice(qs, func(i, j int) bool {
		a, b := qs[i].Raw[0], qs[j].Raw[0]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.GameID != b.GameID {
			return a.GameID < b.GameID
		}
		if a.PlayerID != b.PlayerID {
			return a.PlayerID < b.PlayerID
		}
		return a.Stat < b.Stat
	})
	var fit, dev []forecast
	for _, q := range qs {
		if q.Raw[0].Date < fitBefore {
			fit = append(fit, q)
		} else {
			dev = append(dev, q)
		}
	}
	if len(fit) < 1000 || len(dev) < 500 {
		fatal(fmt.Errorf("insufficient development split"))
	}
	raw := func(qs []forecast, k int) []backtest.Result {
		out := make([]backtest.Result, len(qs))
		for i, q := range qs {
			out[i] = q.Raw[k]
		}
		return out
	}
	cals := []eval.Calibration{eval.FitCalibration(raw(fit, 0)), eval.FitCalibration(raw(fit, 1)), eval.FitResidualCalibration(raw(fit, 1))}
	labels := []string{"reference_v4", "tail_v5_standard", "tail_v5_symmetric_residual"}
	var scores []row
	for k, label := range labels {
		variant := 0
		if k > 0 {
			variant = 1
		}
		scored := cals[k].Results(raw(dev, variant))
		for _, st := range []string{"all", "points", "rebounds", "assists", "threes"} {
			scores = append(scores, score(label, st, dev, scored, variant))
		}
	}
	candidate := cals[1]
	chosen := map[string]string{}
	for _, st := range data.Stats {
		a, b := cals[1].Results(raw(dev, 1)), cals[2].Results(raw(dev, 1))
		var aa, bb []backtest.Result
		for i, r := range a {
			if r.Stat == st {
				aa = append(aa, r)
				bb = append(bb, b[i])
			}
		}
		if eval.Score(bb).LogLoss < eval.Score(aa).LogLoss {
			candidate.ByStat[string(st)] = cals[2].ByStat[string(st)]
			chosen[string(st)] = labels[2]
		} else {
			chosen[string(st)] = labels[1]
		}
	}
	combined := candidate.Results(raw(dev, 1))
	for _, st := range []string{"all", "points", "rebounds", "assists", "threes"} {
		scores = append(scores, score("tail_v5_selected_development", st, dev, combined, 1))
		scores = append(scores, scoreValue(st, combined))
	}
	candidate.ModelVersion = predictor.TailVersion
	candidate.Config = pipeline.ConfigName(pipeline.DisabledFor(false))
	report := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "fitBefore": fitBefore, "developmentBefore": devBefore, "fitN": len(fit), "developmentN": len(dev), "scores": scores, "selectedCalibration": chosen, "note": "Development study only. Policies were designed after earlier model diagnosis, so this is not an untouched performance test. Previously seen March 14-June 13 final period is excluded. Monthly forecasts train strictly earlier; each calibration fits only through February 3. Calibration family is chosen on development dates; it needs new future validation. Candidate files are separate and paper-only. Cached historical source/quote provenance is not independently certified. No production replacement or launch hit-rate claim."}
	save(filepath.Join(run, "study.json"), report)
	save(filepath.Join(run, "calibration_tail.json"), candidate)
	save(filepath.Join(run, "questions.json"), qs)
	for _, r := range scores {
		fmt.Printf("%-28s %-8s N=%d logloss=%.6f Brier=%.6f picks=%d O/U=%d/%d ROI=%+.1f%%\n", r.Name, r.Stat, r.Metrics.N, r.Metrics.LogLoss, r.Metrics.Brier, r.Picks, r.Overs, r.Unders, 100*r.ROI)
	}
	if *write {
		if d == nil {
			d, e = pipeline.Load(*dir, true)
			if e != nil {
				fatal(e)
			}
		}
		fmt.Println("Fitting separate tail candidate on current available history...")
		m, e := predictor.TrainTails(d.Rows, pipeline.DisabledFor(false))
		if e != nil {
			fatal(e)
		}
		if e = m.Save(filepath.Join(*dir, "model_tail.json")); e != nil {
			fatal(e)
		}
		if e = candidate.Save(filepath.Join(*dir, "calibration_tail.json")); e != nil {
			fatal(e)
		}
		b, _ := os.ReadFile(filepath.Join(*dir, "model_tail.json"))
		sum := sha256.Sum256(b)
		report["candidateModelSHA256"] = hex.EncodeToString(sum[:])
		report["candidateTrainedThrough"] = m.TrainedBefore
		save(filepath.Join(run, "study.json"), report)
	}
	fmt.Println("Development report:", run, "; primary model unchanged.")
}
func score(name, stat string, qs []forecast, rs []backtest.Result, v int) row {
	out := row{Name: name, Stat: stat}
	var sub []backtest.Result
	n := 0
	for i, r := range rs {
		if stat != "all" && string(r.Stat) != stat {
			continue
		}
		sub = append(sub, r)
		out.StatMAE += abs(qs[i].Mean[v] - r.Actual)
		out.MinMAE += abs(qs[i].Min[v] - qs[i].ActualMin)
		n++
		p, o, over := r.POver, r.OverOdds, true
		if p < .5 {
			p = 1 - p
			o = r.UnderOdds
			over = false
		}
		if r.Actual == r.Line || p < .55 || bets.Edge(p, o) < .01 {
			continue
		}
		out.Picks++
		out.MeanProb += p
		if over {
			out.Overs++
		} else {
			out.Unders++
		}
		if (r.Actual > r.Line) == over {
			out.WinRate++
			if o > 0 {
				out.ROI += float64(o) / 100
			} else {
				out.ROI += 100 / float64(-o)
			}
		} else {
			out.ROI--
		}
	}
	out.Metrics = eval.Score(sub)
	if n > 0 {
		out.StatMAE /= float64(n)
		out.MinMAE /= float64(n)
	}
	if out.Picks > 0 {
		out.WinRate /= float64(out.Picks)
		out.MeanProb /= float64(out.Picks)
		out.ROI /= float64(out.Picks)
	}
	return out
}
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func save(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		fatal(e)
	}
	if e = os.WriteFile(path, b, 0644); e != nil {
		fatal(e)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }

func scoreValue(stat string, rs []backtest.Result) row {
	out := row{Name: "tail_v5_price_research", Stat: stat}
	var sub []backtest.Result
	for _, r := range rs {
		if stat != "all" && string(r.Stat) != stat {
			continue
		}
		sub = append(sub, r)
		side, p, o := bets.ValueSide(r.POver, r.OverOdds, r.UnderOdds)
		if r.Actual == r.Line || p < .40 || bets.Edge(p, o) < .02 || bets.ExpectedReturn(p, o) < .04 {
			continue
		}
		out.Picks++
		out.MeanProb += p
		if side == "OVER" {
			out.Overs++
		} else {
			out.Unders++
		}
		if (r.Actual > r.Line) == (side == "OVER") {
			out.WinRate++
			if o > 0 {
				out.ROI += float64(o) / 100
			} else {
				out.ROI += 100 / float64(-o)
			}
		} else {
			out.ROI--
		}
	}
	out.Metrics = eval.Score(sub)
	if out.Picks > 0 {
		out.WinRate /= float64(out.Picks)
		out.MeanProb /= float64(out.Picks)
		out.ROI /= float64(out.Picks)
	}
	return out
}
