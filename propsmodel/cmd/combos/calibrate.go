package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/settlement"
	"time"
)

func calibrateCombos(dir, before string) {
	report := map[string]any{"before": before, "at": time.Now().UTC().Format(time.RFC3339), "status": "More forward data needed", "eligibleRows": 0, "trainingRows": 0, "evaluationRows": 0, "evaluationDates": 0, "qualifiedStats": []string{}}
	defer func() {
		b, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "combohealth.json"), b, 0644); e != nil {
			fatal(e)
		}
	}()
	model, err := predictor.Load(filepath.Join(dir, "model.json"))
	if err != nil {
		fatal(err)
	}
	config := pipeline.ConfigName(model.Disabled)
	rows, err := lineslog.LoadComboMarkets(dir)
	if err != nil {
		fatal(err)
	}
	ix, err := settlement.Load(dir)
	if err != nil {
		fatal(err)
	}
	report["loadedRows"] = len(rows)
	active := []string{}
	if c, ok := eval.LoadCalibration(filepath.Join(dir, "combocalibration.json")); ok && c.ModelVersion == model.Version && c.Config == config && c.TrainedBefore != "" && c.TrainedBefore < before {
		for _, st := range data.Combos {
			if _, ok := c.ByStat[string(st)]; ok {
				active = append(active, string(st))
			}
		}
	}
	report["activeStats"] = active
	rejected := map[string]int{}
	report["excluded"] = rejected
	var rs []backtest.Result
	for _, r := range rows {
		if r.TrainedBefore == "" || r.TrainedBefore >= r.Date || r.Date >= before || r.ModelVersion != model.Version || r.Config != config || r.AuditVersion < 2 || r.RawOver <= 0 || r.RawOver >= 1 || r.MarketOver <= 0 || r.MarketOver >= 1 {
			rejected["model/date/provenance mismatch"]++
			continue
		}
		at, ea := time.Parse(time.RFC3339Nano, r.At)
		q, eq := time.Parse(time.RFC3339Nano, r.QuoteAt)
		u, eu := time.Parse(time.RFC3339Nano, r.UpdatedAt)
		if ea != nil || eq != nil || eu != nil || !props.BeforeTip(r.Tip, at) || q.After(at) || u.After(at) || at.Sub(q) > 30*time.Minute || at.Sub(u) > 6*time.Hour || r.Books < 3 || r.OverOdds == 0 || r.UnderOdds == 0 {
			rejected["quote timing or book checks"]++
			continue
		}
		actual := ix.Grade(picklog.Entry{Date: r.Date, LoggedAt: r.At, Tip: r.Tip, Period: "game", GameID: r.GameID, PlayerID: r.PlayerID, Player: r.Player, Team: r.Team, Stat: r.Stat, Side: "OVER", Line: r.Line}, 0)
		if actual.Result == "REVIEW" {
			rejected["final actuals pending or identity review"]++
			continue
		}
		stat := data.Stat(r.Stat)
		if !stat.IsCombo() {
			rejected["not a combo"]++
			continue
		}
		rs = append(rs, backtest.Result{Date: r.Date, GameID: r.GameID, PlayerID: r.PlayerID, Stat: stat, Line: r.Line, POver: r.RawOver, Market: r.MarketOver, Actual: actual.Actual, OverOdds: r.OverOdds, UnderOdds: r.UnderOdds})
	}
	backtest.SortChronologically(rs)
	report["eligibleRows"] = len(rs)
	first, test := backtest.Split(rs)
	report["trainingRows"], report["evaluationRows"] = len(first), len(test)
	allDates := map[string]bool{}
	for _, r := range rs {
		allDates[r.Date] = true
	}
	report["eligibleDates"] = len(allDates)
	if len(first) < 300 || len(test) < 100 {
		fmt.Println("Need at least 300 earlier calibration rows and 100 later evaluation rows on separate dates. No calibration written.")
		return
	}
	trial := eval.FitCalibrationStats(first, data.Combos, 300)
	corrected := trial.Results(test)
	baseline := append([]backtest.Result(nil), test...)
	for i := range baseline {
		baseline[i].POver = baseline[i].Market
	}
	gain, z := eval.PairedGain(corrected, baseline)
	dates := map[string]bool{}
	for _, r := range test {
		dates[r.Date] = true
	}
	report["evaluationDates"], report["gainVsBook"], report["zVsBook"] = len(dates), gain, z
	rawScore, calScore := eval.Score(test), eval.Score(corrected)
	fmt.Printf("Untouched later-date evaluation: %d rows, raw Brier %.5f calibrated %.5f; raw log loss %.5f calibrated %.5f\n", calScore.N, rawScore.Brier, calScore.Brier, rawScore.LogLoss, calScore.LogLoss)
	report["rawLogLoss"], report["calibratedLogLoss"] = rawScore.LogLoss, calScore.LogLoss
	if len(dates) < 30 || gain <= 0 || z < 2 || calScore.N == 0 || calScore.LogLoss >= rawScore.LogLoss {
		fmt.Println("Direct calibration did not clear 30-date evidence and improvement over raw and sportsbook probabilities. Keeping existing file.")
		return
	}
	// Only promote stat fits that individually improved on held-out dates.
	final := eval.FitCalibrationStats(rs, data.Combos, 300)
	for _, st := range data.Combos {
		var a, b, book []backtest.Result
		for i, r := range test {
			if r.Stat == st {
				a = append(a, r)
				b = append(b, corrected[i])
				book = append(book, baseline[i])
			}
		}
		g, z := eval.PairedGain(b, book)
		dates := map[string]bool{}
		for _, r := range a {
			dates[r.Date] = true
		}
		report["rawLogLoss"], report["calibratedLogLoss"] = rawScore.LogLoss, calScore.LogLoss
		if len(dates) < 30 || len(a) < 200 || g <= 0 || z < 2 || eval.Score(b).LogLoss >= eval.Score(a).LogLoss {
			delete(final.ByStat, string(st))
		}
	}
	if len(final.ByStat) == 0 {
		fmt.Println("No individual combo cleared the held-out check. No calibration written.")
		return
	}
	qualified := []string{}
	for _, st := range data.Combos {
		if _, ok := final.ByStat[string(st)]; ok {
			qualified = append(qualified, string(st))
		}
	}
	report["status"], report["qualifiedStats"] = "Direct calibration fitted; next dates are the forward test", qualified
	final.Config = pipeline.ConfigName(model.Disabled)
	final.ModelVersion = model.Version
	if err := final.Save(filepath.Join(dir, "combocalibration.json")); err != nil {
		fatal(err)
	}
	fmt.Printf("Saved direct fits for %d combo types, using dates strictly before %s. Subsequent dates remain the forward test.\n", len(final.ByStat), before)
}
