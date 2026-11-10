// accuracy grades immutable pregame decisions against identity-checked final
// stats and tests a minutes challenger without changing production parameters.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/accuracy"
	"propsmodel/internal/backtest"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"propsmodel/internal/settlement"
	"sort"
	"time"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	before := flag.String("before", "", "evaluate dates strictly before YYYY-MM-DD; defaults today Eastern")
	flag.Parse()
	loc, _ := time.LoadLocation("America/New_York")
	if *before == "" {
		*before = time.Now().In(loc).Format("2006-01-02")
	}
	if _, e := time.Parse("2006-01-02", *before); e != nil {
		fatal(e)
	}
	current, e := predictor.Load(filepath.Join(*dir, "model.json"))
	if e != nil {
		fatal(e)
	}
	report := accuracy.Report{At: time.Now().UTC().Format(time.RFC3339), Config: pipeline.ConfigName(current.Disabled), ModelVersion: current.Version, Excluded: map[string]int{}, Note: "Fixed policy: probability >=55%, edge >=1pp at recorded odds. One earliest pregame question per player/game/stat; pushes excluded from binary scores. No legacy results, postgame features, or unproven calibration dates. Minutes trial is experimental and never automatically applied. Evidence criteria are provisional, not a profitability guarantee."}
	models := map[string]*predictor.Model{}
	chosen := map[string]replay.Record{}
	files, e := filepath.Glob(filepath.Join(*dir, "replay", "decisions_*.jsonl"))
	if e != nil {
		fatal(e)
	}
	for _, path := range files {
		f, e := os.Open(path)
		if e != nil {
			fatal(e)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 65536), 1<<24)
		for sc.Scan() {
			var r replay.Record
			if e = json.Unmarshal(sc.Bytes(), &r); e != nil {
				fatal(e)
			}
			skip := func(s string) { report.Excluded[s]++ }
			if r.Date >= *before {
				skip("not_completed_date")
				continue
			}
			day, e := time.Parse("2006-01-02", r.Date)
			boundary, _ := time.Parse("2006-01-02", *before)
			if e != nil || boundary.Sub(day) > 90*24*time.Hour {
				skip("outside_90_day_window")
				continue
			}
			if r.Row.G.GameID <= 0 || r.Row.G.PlayerID <= 0 || data.StatIndex(r.Stat) < 0 {
				skip("identity_or_stat_missing")
				continue
			}
			at, ea := time.Parse(time.RFC3339Nano, r.At)
			tip, et := time.Parse(time.RFC3339Nano, r.Tip)
			q, eq := time.Parse(time.RFC3339Nano, r.QuoteAt)
			u, eu := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			if ea != nil || et != nil || eq != nil || eu != nil || !at.Before(tip) || q.After(at) || u.After(at) || at.Sub(q) > 30*time.Minute || at.Sub(u) > 6*time.Hour {
				skip("timing_or_quote_invalid")
				continue
			}
			if r.Period != "game" || r.BookCount < 3 {
				skip("market_period_or_consensus_unproven")
				continue
			}
			experimental := r.Policy.MinutesShift != 0 || r.Policy.Blend != 0 || r.Policy.BigCut != 0
			for _, factor := range r.Policy.Scale {
				if factor != 0 && math.Abs(factor-1) > 1e-9 {
					experimental = true
				}
			}
			if experimental {
				skip("experimental_policy_needs_separate_validation")
				continue
			}
			if r.CalibrationBefore == "" || r.CalibrationBefore >= r.Date {
				skip("calibration_training_cutoff_unproven")
				continue
			}
			if len(r.ModelID) != 64 {
				skip("checkpoint_missing")
				continue
			}
			if _, e = hex.DecodeString(r.ModelID); e != nil {
				skip("checkpoint_id_invalid")
				continue
			}
			m := models[r.ModelID]
			if m == nil {
				b, e := os.ReadFile(filepath.Join(*dir, "replay", "model_"+r.ModelID+".json"))
				if e != nil {
					skip("checkpoint_missing")
					continue
				}
				h := sha256.Sum256(b)
				if hex.EncodeToString(h[:]) != r.ModelID {
					fatal(fmt.Errorf("checkpoint hash mismatch"))
				}
				m, e = predictor.Load(filepath.Join(*dir, "replay", "model_"+r.ModelID+".json"))
				if e != nil {
					fatal(e)
				}
				models[r.ModelID] = m
			}
			if m.TrainedBefore == "" || m.TrainedBefore >= r.Date || m.Version != current.Version || pipeline.ConfigName(m.Disabled) != report.Config {
				skip("training_cutoff_or_configuration_invalid")
				continue
			}
			p, e := replay.Recompute(m, r)
			if e != nil || math.Abs(p-r.P1) > 1e-9 || math.IsNaN(p) {
				skip("not_reproducible")
				continue
			}
			if r.Market <= 0 || r.Market >= 1 || r.OverOdds == 0 || r.UnderOdds == 0 || math.Abs(r.Market-bets.MarketOver(r.OverOdds, r.UnderOdds)) > 1e-9 {
				skip("market_price_missing")
				continue
			}
			key := fmt.Sprintf("%s/%d/%d/%s", r.Date, r.Row.G.GameID, r.Row.G.PlayerID, r.Stat)
			old, ok := chosen[key]
			if !ok || r.At < old.At || (r.At == old.At && r.Book < old.Book) {
				chosen[key] = r
			}
		}
		if e = sc.Err(); e != nil {
			fatal(e)
		}
		f.Close()
	}
	ix, ixErr := settlement.Load(*dir)
	var obs []accuracy.Observation
	for _, r := range chosen {
		if ixErr != nil {
			report.Excluded["final_stats_unavailable"]++
			continue
		}
		entry := picklog.Entry{Date: r.Date, LoggedAt: r.At, Tip: r.Tip, Period: "game", Player: r.Row.G.Name, Team: r.Row.G.TeamAbbr, GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Stat: string(r.Stat), Side: "OVER", Line: r.Line}
		grade := ix.Grade(entry, 0)
		mins, ok := ix.ActualMinutes(entry)
		if !ok || grade.Result == "REVIEW" {
			report.Excluded["final_identity_or_source_unverified"]++
			continue
		}
		p := models[r.ModelID].Forecast(r.Row.Features(), r.Policy)
		raw, _ := p.ProbOver(r.Stat, r.Line)
		o := accuracy.Observation{Raw: raw.Over, Result: backtest.Result{Date: r.Date, GameID: entry.GameID, PlayerID: entry.PlayerID, Stat: r.Stat, Line: r.Line, POver: r.P1, Actual: grade.Actual, OverOdds: r.OverOdds, UnderOdds: r.UnderOdds, Market: r.Market}, Pred: p.Mean(r.Stat), Actual: grade.Actual, Minutes: p.Minutes, ActualMinutes: mins, Typical: r.Row.Features().TypicalMinutes()}
		obs = append(obs, o)
		if r.Date > report.Through {
			report.Through = r.Date
		}
	}
	sort.Slice(obs, func(i, j int) bool {
		a, b := obs[i].Result, obs[j].Result
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
	report.Accepted = len(obs)
	for _, st := range []string{"all", "points", "rebounds", "assists", "threes"} {
		for _, side := range []string{"ALL", "OVER", "UNDER"} {
			report.Slices = append(report.Slices, accuracy.Summary(obs, st, side))
		}
	}
	report.Minutes = accuracy.MinutesExperiment(obs)
	report.Calibration = accuracy.CalibrationExperiment(obs)
	if e = os.MkdirAll(*dir, 0755); e != nil {
		fatal(e)
	}
	b, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		fatal(e)
	}
	path := filepath.Join(*dir, "accuracy.json")
	if e = os.WriteFile(path+".tmp", b, 0644); e != nil {
		fatal(e)
	}
	if e = os.Rename(path+".tmp", path); e != nil {
		fatal(e)
	}
	fmt.Printf("Forward evaluation: %d verified questions; %d exclusion categories. Report saved to %s.\n", report.Accepted, len(report.Excluded), path)
	for _, s := range report.Slices {
		if s.Side != "ALL" {
			fmt.Printf("%-9s %-5s N=%d days=%d accuracy=%.1f%% market gain=%+.5f supported=%t\n", s.Stat, s.Side, s.N, s.Days, 100*s.HitRate, s.Gain, s.Supported)
		}
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
