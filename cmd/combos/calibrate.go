package main

import (
	"fmt"
	"path/filepath"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
)

func calibrateCombos(dir, before string) {
	model, err := predictor.Load(filepath.Join(dir, "model.json"))
	if err != nil {
		fatal(err)
	}
	config := pipeline.ConfigName(model.Disabled)
	rows, err := lineslog.LoadComboMarkets(dir)
	if err != nil {
		fatal(err)
	}
	d, err := pipeline.Load(dir, false)
	if err != nil {
		fatal(err)
	}
	var rs []backtest.Result
	for _, r := range rows {
		if r.Date >= before || r.ModelVersion != model.Version || r.Config != config || r.AuditVersion < 2 || r.RawOver <= 0 || r.MarketOver <= 0 {
			continue
		}
		actual, played := d.Builder.Index().Actual(r.GameID, r.PlayerID)
		if !played {
			continue
		}
		stat := data.Stat(r.Stat)
		if !stat.IsCombo() {
			continue
		}
		rs = append(rs, backtest.Result{Date: r.Date, GameID: r.GameID, PlayerID: r.PlayerID, Stat: stat, Line: r.Line, POver: r.RawOver, Market: r.MarketOver, Actual: actual.Value(stat)})
	}
	backtest.SortChronologically(rs)
	first, test := backtest.Split(rs)
	if len(first) < 300 || len(test) < 100 {
		fmt.Println("Need at least 300 earlier calibration rows and 100 later evaluation rows on separate dates. No calibration written.")
		return
	}
	trial := eval.FitCalibrationStats(first, data.Combos, 300)
	corrected := trial.Results(test)
	rawScore, calScore := eval.Score(test), eval.Score(corrected)
	fmt.Printf("Untouched later-date evaluation: %d rows, raw Brier %.5f calibrated %.5f; raw log loss %.5f calibrated %.5f\n", calScore.N, rawScore.Brier, calScore.Brier, rawScore.LogLoss, calScore.LogLoss)
	if calScore.N == 0 || calScore.LogLoss >= rawScore.LogLoss {
		fmt.Println("Direct calibration did not improve later-date log loss. Keeping existing file.")
		return
	}
	// Only promote stat fits that individually improved on held-out dates.
	final := eval.FitCalibrationStats(rs, data.Combos, 300)
	for _, st := range data.Combos {
		var a, b []backtest.Result
		for i, r := range test {
			if r.Stat == st {
				a = append(a, r)
				b = append(b, corrected[i])
			}
		}
		if len(a) < 100 || eval.Score(b).LogLoss >= eval.Score(a).LogLoss {
			delete(final.ByStat, string(st))
		}
	}
	if len(final.ByStat) == 0 {
		fmt.Println("No individual combo cleared the held-out check. No calibration written.")
		return
	}
	final.Config = pipeline.ConfigName(model.Disabled)
	final.ModelVersion = model.Version
	if err := final.Save(filepath.Join(dir, "combocalibration.json")); err != nil {
		fatal(err)
	}
	fmt.Printf("Saved direct fits for %d combo types, using dates strictly before %s. Subsequent dates remain the forward test.\n", len(final.ByStat), before)
}
