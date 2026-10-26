// Command backtest tests the full model against a past season's opening sportsbook lines and
// saves the probability calibration for the predict command.
//
//	go run ./cmd/backtest -season 2025
//	go run ./cmd/backtest -season 2025 -ablate        (also measure what each feature group is worth)
//	go run ./cmd/backtest -season 2025 -lineup        (include the teammates-out features)
//
// Needs data/stats_*.jsonl (go run ./cmd/pull stats -seasons 2022,2023,2024,2025),
// data/opening_props_2025.jsonl (go run ./cmd/pull props -season 2025) and, for the usage
// features, data/advanced_*.jsonl (go run ./cmd/pull advanced -seasons 2022,2023,2024,2025).
//
// The model is retrained every month using only games before that month (walk-forward), so no
// prediction ever sees its own future.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
)

func main() {
	season := flag.Int("season", 2025, "season whose opening lines to test against")
	dir := flag.String("data", "data", "data directory")
	noVol := flag.Bool("novol", false, "leave out the per-player minutes-volatility scaling (to measure what it adds)")
	noExitFix := flag.Bool("noexitfix", false, "leave out the injury-exit correction to the minutes averages (to measure what it adds)")
	noBlowout := flag.Bool("noblowout", false, "leave out the blowout correction to the minutes averages (to measure what it adds)")
	lineup := flag.Bool("lineup", false, "include teammates-out features (flattered by opening lines; see README)")
	minEdge := flag.Float64("minedge", 0.01, "also require the probability to beat the offered odds break-even by this much")
	live := flag.Bool("live", false, "also score this season's saved daily snapshots (data/snapshots/props_*.jsonl) once their games are final, so the calibration keeps learning from current lines")
	ablate := flag.Bool("ablate", false, "retrain once per feature group with it removed (slow)")
	flag.Parse()
	predictor.UseVolatility = !*noVol
	feat.BlowoutAdjust = !*noBlowout
	feat.ShortExitAdjust = !*noExitFix

	d, err := pipeline.Load(*dir, true)
	if err != nil {
		fatal(err)
	}
	path := filepath.Join(*dir, fmt.Sprintf("opening_props_%d.jsonl", *season))
	lines, err := props.LoadOpening(path)
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/pull props -season %d)", err, *season))
	}
	seasons := []int{*season}
	if *live {
		extra, liveSeasons := loadSnapshotLines(*dir)
		lines = append(lines, extra...)
		for _, s := range liveSeasons {
			if s != *season {
				seasons = append(seasons, s)
			}
		}
		fmt.Printf("live snapshots: %d lines from %d season(s) added\n", len(extra), len(liveSeasons))
	}
	disabled := pipeline.DisabledFor(*lineup)
	config := pipeline.ConfigName(disabled)
	fmt.Printf("%d player-games of history, %d feature rows, %d advanced rows, %d prop lines read\n",
		len(d.Games), len(d.Rows), len(d.Adv), len(lines))
	fmt.Printf("feature set: %s (disabled groups: %v)\n", config, disabled)
	if len(d.Adv) == 0 {
		fmt.Println("note: no data/advanced_*.jsonl found, so the usage features are empty")
	}
	fmt.Println("Retraining each month on earlier games only. This takes a minute or two...")
	fmt.Println()

	v2, minSamples, err := eval.WalkForwardMinutes(d.Rows, lines, seasons, disabled)
	if err != nil {
		fatal(err)
	}
	v1 := backtest.Run(d.Builder.Index(), lines)
	v2, v1 = eval.Intersect(v2, v1)
	if len(v2) < 400 {
		fmt.Printf("Only %d lines could be scored. Check that the stats files cover the seasons before %d\n", len(v2), *season)
		fmt.Println("and that the opening props file has points/rebounds/assists over-under lines.")
		return
	}
	fmt.Printf("%d lines scored by both the baseline and the full model (eligible, played, enough history)\n\n", len(v2))

	minutesTable(os.Stdout, minSamples)

	raw2, cal2, calib := eval.Holdout(v2)
	raw1, cal1, _ := eval.Holdout(v1)
	fmt.Printf("Calibration is fitted on the first half and everything below is judged on the second half\n")
	fmt.Printf("(%d lines the calibration never saw).\n", len(cal2))
	fmt.Printf("Final probability = intercept %.2f + %.2f x logit(model) + %.2f x logit(sportsbook price).\n", calib.Global.Intercept, calib.Global.Slope, calib.Global.Market)
	fmt.Println("Model weight near 0 = it adds nothing beyond the book; sportsbook weight near 1 = the book's prices already carry it.")
	if c := calib.Global.Intercept; c > 0.1 || c < -0.1 {
		fmt.Printf("Intercept %.2f means overs hit more or less than half the time on these lines: part of any gain\n", c)
		fmt.Println("below may be a constant over/under bias in the books' lines rather than player knowledge. Check it holds across months.")
	}
	fmt.Println()

	// The honest comparator: the sportsbook's own prices with no model at all.
	priceOnly := make([]backtest.Result, len(v2))
	for i, r := range v2 {
		r.POver = 0.5
		priceOnly[i] = r
	}
	_, calPrice, _ := eval.Holdout(priceOnly)

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Println("SCORECARD (lower log loss is better; always saying 50% scores 0.6931)")
	fmt.Fprintln(tw, "model\tlines\tlog loss\tBrier\tgain vs coin flip\tz")
	for _, row := range []struct {
		name string
		rs   []backtest.Result
	}{{"baseline, raw", raw1}, {"baseline, calibrated", cal1}, {"full model, raw", raw2}, {"sportsbook price only, no model", calPrice}, {"full model + price, calibrated", cal2}} {
		m := eval.Score(row.rs)
		gain, z := eval.SkillVsCoinFlip(row.rs)
		fmt.Fprintf(tw, "%s\t%d\t%.4f\t%.4f\t%+.4f\t%+.1f\n", row.name, m.N, m.LogLoss, m.Brier, gain, z)
	}
	tw.Flush()
	pg, pz := eval.PairedGain(cal2, calPrice)
	fmt.Printf("\nWhat the model adds beyond the sportsbook's price alone: %+.4f log loss per line, z = %+.1f\n", pg, pz)
	fmt.Println("This is the number that matters. 'gain vs coin flip' above credits the book's own prices too.")
	gain, z := eval.PairedGain(cal2, cal1)
	fmt.Printf("Full model vs baseline (both calibrated, same lines): %+.4f log loss per line, z = %+.1f\n", gain, z)
	fmt.Println("z above about 2 is unlikely to be luck. 'gain vs coin flip' is the real test: lines sit near 50/50,")
	fmt.Println("so a model only beats the coin flip if it knows something the sportsbook's line does not.")
	fmt.Println()

	calibrationTable(tw, "FULL MODEL, RAW probabilities (second half)", raw2)
	calibrationTable(tw, "FULL MODEL, CALIBRATED probabilities (second half)", cal2)
	thresholdTable(tw, "PICKS, FULL MODEL, CALIBRATED (second half) - this is what predict posts", cal2, *minEdge)
	thresholdTable(tw, "PICKS, BASELINE, CALIBRATED (second half), for comparison", cal1, *minEdge)

	fmt.Println("BY STAT, full model, calibrated, t = 0.55 (second half)")
	fmt.Fprintln(tw, "stat\tpicks\twins\tlosses\thit rate\tbreak-even\tunits")
	by := backtest.EvaluateByStat(cal2, 0.55, *minEdge)
	for _, s := range data.Stats {
		st := by[s]
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1f%%\t%.1f%%\t%+.1f\n", s, st.Picks, st.Wins, st.Losses, 100*st.HitRate(), 100*st.BreakEven(), st.Units)
	}
	tw.Flush()

	if *ablate {
		fmt.Println("\nABLATION: retraining with each feature group removed (second half, calibrated)")
		var groups []string
		for _, g := range feat.OptionalGroups {
			if !contains(disabled, g) {
				groups = append(groups, g)
			}
		}
		full := eval.Score(cal2)
		rows, err := eval.Ablate(d.Rows, lines, *season, groups, full, disabled)
		if err != nil {
			fatal(err)
		}
		fmt.Fprintln(tw, "group removed\tlog loss\tchange\tverdict")
		for _, r := range rows {
			verdict := "no measurable effect"
			switch {
			case r.Delta > 0.0005:
				verdict = "HELPS (keep)"
			case r.Delta < -0.0005:
				verdict = "HURTS (consider removing)"
			}
			fmt.Fprintf(tw, "%s\t%.5f\t%+.5f\t%s\n", r.Group, r.LogLoss, r.Delta, verdict)
		}
		tw.Flush()
		fmt.Printf("(full model log loss %.5f; a positive change means removing the group made it worse)\n", full.LogLoss)
	}

	// Final calibration for live use is fitted on every scored line.
	if *noBlowout || *noVol || *noExitFix {
		fmt.Println("\nExperiment run (-noblowout, -noexitfix or -novol): calibration.json was NOT saved, so the live calibration still matches the real model.")
		return
	}
	final := eval.FitCalibration(v2)
	final.Config = config
	out := filepath.Join(*dir, "calibration.json")
	if err := final.Save(out); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved the calibration fitted on all %d lines to %s (config %q).\n", len(v2), out, config)
	fmt.Println("\nHow to read this: break-even at -110 is a 52.4% hit rate. If the calibrated 0.55 row beats that")
	fmt.Println("over a few hundred picks, with the hit rate above the break-even column and 'gain vs coin flip' z above 2, the model has found an edge worth testing live.")
	fmt.Println("If it posts almost nothing or sits below 52.4%, it has not, and you should not bet it.")
	fmt.Println("Even a good backtest only earns the right to paper-trade live; the closing-line-value check in")
	fmt.Println("`go run ./cmd/settle` is the next test.")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func calibrationTable(tw *tabwriter.Writer, title string, rs []backtest.Result) {
	fmt.Println(title)
	fmt.Fprintln(tw, "bin\tn\tpredicted\tactual")
	for _, r := range backtest.Calibration(rs) {
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%.3f\n", r.Bin, r.N, r.Predicted, r.Actual)
	}
	tw.Flush()
	fmt.Println()
}

func thresholdTable(tw *tabwriter.Writer, title string, rs []backtest.Result, minEdge float64) {
	fmt.Printf("%s - OVER at >= t, UNDER at <= 1-t, and the price must beat break-even by %.0f%%\n", title, 100*minEdge)
	fmt.Fprintln(tw, "t\tpicks\twins\tlosses\tpushes\thit rate\tbreak-even\tunits\tROI")
	for _, t := range []float64{0.52, 0.55, 0.58, 0.60} {
		st := backtest.EvaluateEdge(rs, t, minEdge)
		fmt.Fprintf(tw, "%.2f\t%d\t%d\t%d\t%d\t%.1f%%\t%.1f%%\t%+.1f\t%+.1f%%\n",
			t, st.Picks, st.Wins, st.Losses, st.Pushes, 100*st.HitRate(), 100*st.BreakEven(), st.Units, 100*st.ROI())
	}
	tw.Flush()
	fmt.Println()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// loadSnapshotLines reads every saved daily props snapshot. It returns the lines and the seasons
// they belong to. Lines for games that are not final yet are ignored later (no box score).
func loadSnapshotLines(dir string) ([]props.Line, []int) {
	files, _ := filepath.Glob(filepath.Join(dir, "snapshots", "props_*.jsonl"))
	var out []props.Line
	seen := map[int]bool{}
	var seasons []int
	for _, f := range files {
		ls, err := props.LoadSnapshot(f)
		if err != nil {
			continue
		}
		out = append(out, ls...)
		// props_YYYY-MM-DD.jsonl
		base := filepath.Base(f)
		if len(base) >= 16 {
			if s := data.SeasonOf(base[6:16]); s != 0 && !seen[s] {
				seen[s] = true
				seasons = append(seasons, s)
			}
		}
	}
	return out, seasons
}

// minutesTable prints how well the minutes projection did out of sample, against the plain
// recent average, because wrong minutes are the most common reason a pick misses.
func minutesTable(w io.Writer, samples []eval.MinSample) {
	if len(samples) == 0 {
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "MINUTES CHECK (every player-game in the test months, projected before tip-off)")
	fmt.Fprintln(w, "error is in minutes; bias is actual minus projected, so positive means the projection was too low")
	fmt.Fprintln(tw, "slice\tgames\tmodel error\trecent-average error\tmodel bias\trecent-average bias")
	for _, row := range []struct {
		name string
		keep func(eval.MinSample) bool
	}{
		{"all eligible players", func(x eval.MinSample) bool { return x.Eligible }},
		{"projected under 15 min", func(x eval.MinSample) bool { return x.Eligible && x.Pred < 15 }},
		{"projected 15 to 25 min", func(x eval.MinSample) bool { return x.Eligible && x.Pred >= 15 && x.Pred < 25 }},
		{"projected 25 to 32 min", func(x eval.MinSample) bool { return x.Eligible && x.Pred >= 25 && x.Pred < 32 }},
		{"projected 32+ min (starters)", func(x eval.MinSample) bool { return x.Eligible && x.Pred >= 32 }},
		{"first three weeks of a season", func(x eval.MinSample) bool { return x.Eligible && x.EarlySeason }},
	} {
		s := eval.SummarizeMinutes(samples, row.keep)
		if s.N == 0 {
			continue
		}
		fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.2f\t%+.2f\t%+.2f\n", row.name, s.N, s.ModelMAE, s.NaiveMAE, s.ModelBias, s.NaiveBias)
	}
	tw.Flush()
	fmt.Fprintln(w)
}
