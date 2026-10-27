// Command bias checks whether the model projects too low or too high this season.
//
//	go run ./cmd/bias                 (the current season; use -season 2025 for last season)
//
// Every player-game of the season is projected with a model trained only on earlier months, and the
// projection is compared with what happened. Nothing is selected, so unlike the review of posted
// picks it cannot be fooled by the model choosing the plays where it happened to project low. If
// the actual numbers sit well above the projections for everyone, the one-sided "all unders"
// boards are a bias in the model, not a read on the games.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/pipeline"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	season := flag.Int("season", 0, "season start year (default: the current season)")
	noExitFix := flag.Bool("noexitfix", false, "leave out the injury-exit correction to the minutes averages (to measure what it adds)")
	noBlowout := flag.Bool("noblowout", false, "leave out the blowout correction to the minutes averages (to see whether it is what moves the bias)")
	lineup := flag.Bool("lineup", false, "check the model with the teammates-out and opponent-out features (use the same setting you trained with)")
	write := flag.Bool("write", false, "save a conservative per-stat correction to data/biasfix.json, which predict applies (only a large, statistically clear bias is used)")
	flag.Parse()
	feat.BlowoutAdjust = !*noBlowout
	feat.ShortExitAdjust = !*noExitFix
	if *season == 0 {
		now := time.Now()
		*season = now.Year()
		if now.Month() < 8 {
			*season--
		}
	}
	d, err := pipeline.Load(*dir, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("Projecting every eligible player-game of the %d season with a model trained on earlier months only...\n\n", *season)
	samples, err := eval.BiasCheck(d.Rows, *season, pipeline.DisabledFor(*lineup))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(samples) == 0 {
		fmt.Println("No games to check for that season (need at least 500 earlier rows to train on).")
		return
	}
	table("ALL GAMES", samples, func(eval.BiasSample) bool { return true })
	table("FIRST THREE WEEKS OF THE SEASON", samples, func(s eval.BiasSample) bool { return s.EarlySeason })
	table("AFTER THE FIRST THREE WEEKS", samples, func(s eval.BiasSample) bool { return !s.EarlySeason })
	blendTable(samples)
	spreadTable(samples)
	coverageTable(samples)
	woTable(samples)
	if *write {
		wf := eval.FitWoFix(samples)
		wf.Season, wf.Updated = *season, time.Now().Format("2006-01-02")
		wpath := filepath.Join(*dir, "wofix.json")
		if err := wf.Save(wpath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Printf("Saved %s. ", wpath)
		if wf.Active() {
			fmt.Println("predict applies it.")
		} else {
			fmt.Println("Nothing is clear enough yet, so predict ignores it.")
		}
		mf := eval.FitMinFix(samples)
		mf.Season, mf.Updated = *season, time.Now().Format("2006-01-02")
		mpath := filepath.Join(*dir, "minfix.json")
		if err := mf.Save(mpath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Printf("Saved %s: %.2f minutes per spread point (raw %.2f, z %+.1f) from %d games with a spread. ", mpath, mf.Slope, mf.Raw, mf.Z, mf.Samples)
		if mf.Active() {
			fmt.Println("predict applies it.")
		} else {
			fmt.Println("Not clear enough yet, so predict ignores it.")
		}
		bf := eval.FitBiasFix(samples)
		bf.Season, bf.Updated = *season, time.Now().Format("2006-01-02")
		path := filepath.Join(*dir, "biasfix.json")
		if err := bf.Save(path); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Printf("Saved %s from %d player-games.\n", path, bf.Samples)
		for _, st := range data.Stats {
			fmt.Printf("  %-9s raw x%.3f (z %+.1f) -> applied x%.3f\n", st, bf.Raw[string(st)], bf.Z[string(st)], bf.For(st))
		}
		if !bf.Active() {
			fmt.Println("  Nothing is large and clear enough yet, so predict will not change anything.")
		}
		fmt.Println()
	}
	fmt.Println("How to read this: bias is actual minus projected, so a positive number means the model projected too low.")
	fmt.Println("'above projection' is the share of games where the player beat the projection; about 45 to 50% is normal")
	fmt.Println("for low counts, and 60% or more means the projections are systematically low (and the unders it posts too many).")
	fmt.Println("z is the bias divided by its own noise: under about 2 in size it could be luck, above 3 it is probably real.")
}

func table(title string, samples []eval.BiasSample, keep func(eval.BiasSample) bool) {
	var n float64
	var pm, am float64
	var ps, as, above, sq [data.NStats]float64
	var msq float64
	for _, s := range samples {
		if !keep(s) {
			continue
		}
		n++
		pm += s.PredMin
		am += s.ActualMin
		msq += (s.ActualMin - s.PredMin) * (s.ActualMin - s.PredMin)
		for k := range ps {
			ps[k] += s.Pred[k]
			as[k] += s.Actual[k]
			if s.Actual[k] > s.Pred[k] {
				above[k]++
			}
			sq[k] += (s.Actual[k] - s.Pred[k]) * (s.Actual[k] - s.Pred[k])
		}
	}
	if n == 0 {
		return
	}
	fmt.Printf("%s (%d player-games)\n", title, int(n))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "what\tprojected\tactual\tbias\tbias %\tabove projection\tz")
	z := func(sum, sumSq float64) float64 {
		mean := sum / n
		v := sumSq/n - mean*mean
		if v <= 0 || n < 2 {
			return 0
		}
		return mean / math.Sqrt(v/n)
	}
	fmt.Fprintf(tw, "minutes\t%.1f\t%.1f\t%+.2f\t%+.1f%%\t\t%+.1f\n", pm/n, am/n, (am-pm)/n, 100*(am-pm)/pm, z(am-pm, msq))
	for _, st := range data.Stats {
		k := data.StatIndex(st)
		fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%+.2f\t%+.1f%%\t%.0f%%\t%+.1f\n", st, ps[k]/n, as[k]/n, (as[k]-ps[k])/n, 100*(as[k]-ps[k])/ps[k], 100*above[k]/n, z(as[k]-ps[k], sq[k]))
	}
	tw.Flush()
	fmt.Println()
}

// blendTable tests the idea that the model's minutes cuts below a player's normal are partly noise.
// For each share w, the projection is moved w of the way to his normal minutes, and the result is
// scored on every game. The best w is the one with the smallest minutes error.
func blendTable(samples []eval.BiasSample) {
	var use []eval.BiasSample
	for _, s := range samples {
		if s.Typical > 0 && !math.IsNaN(s.Typical) {
			use = append(use, s)
		}
	}
	if len(use) < 100 {
		return
	}
	fmt.Printf("MINUTES CUT FROM NORMAL (%d player-games): projected minutes minus his normal minutes\n", len(use))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "cut\tgames\tprojected\tnormal\tactual\tactual - projected")
	for _, b := range []struct {
		name   string
		lo, hi float64
	}{{"-6 or more", -99, -6}, {"-6 to -3", -6, -3}, {"-3 to 0", -3, 0}, {"0 to +3", 0, 3}, {"+3 or more", 3, 99}} {
		var n, p, t, a float64
		for _, s := range use {
			if d := s.PredMin - s.Typical; d >= b.lo && d < b.hi {
				n++
				p += s.PredMin
				t += s.Typical
				a += s.ActualMin
			}
		}
		if n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%.1f\t%+.2f\n", b.name, int(n), p/n, t/n, a/n, (a-p)/n)
		}
	}
	tw.Flush()
	fmt.Println()

	fmt.Println("BLEND TOWARD NORMAL MINUTES: move the projection this share of the way to his normal minutes")
	tw = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "share\tminutes error (RMSE)\tminutes bias\tpoints error (MAE)\tpoints bias")
	best, bestE := 0.0, math.Inf(1)
	for _, w := range []float64{0, 0.25, 0.5, 0.75, 1} {
		var se, bias, pae, pbias float64
		for _, s := range use {
			p := s.Pred0.BlendMinutes(s.Typical, w)
			e := s.ActualMin - p.Minutes
			se += e * e
			bias += e
			pm := p.Mean(data.Points)
			pae += math.Abs(s.Actual[0] - pm)
			pbias += s.Actual[0] - pm
		}
		n := float64(len(use))
		rmse := math.Sqrt(se / n)
		if rmse < bestE {
			best, bestE = w, rmse
		}
		fmt.Fprintf(tw, "%.2f\t%.2f\t%+.2f\t%.2f\t%+.2f\n", w, rmse, bias/n, pae/n, pbias/n)
	}
	tw.Flush()
	fmt.Printf("\nLowest minutes error at a share of %.2f. Use it with: go run ./cmd/predict -minblend %.2f\n", best, best)
	fmt.Println("A share above 0 means the model's cuts from his normal minutes are partly noise. Trust it more with more games.")
	fmt.Println()
}

// spreadTable shows the minutes miss by the game's pre-game spread. If close games run over
// projection and blowouts run under, the model is under-reacting to the spread.
func spreadTable(samples []eval.BiasSample) {
	var use []eval.BiasSample
	for _, s := range samples {
		if !math.IsNaN(s.AbsSpread) && s.AbsSpread > 0 {
			use = append(use, s)
		}
	}
	if len(use) < 100 {
		fmt.Printf("SPREAD CHECK: only %d player-games had a pre-game spread, too few to check.\n\n", len(use))
		return
	}
	fmt.Printf("MINUTES MISS BY PRE-GAME SPREAD (%d player-games)\n", len(use))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "spread\tgames\tprojected\tactual\tactual - projected\tafter correction")
	mf := eval.FitMinFix(samples)
	for _, b := range []struct {
		name   string
		lo, hi float64
	}{{"0 to 3", 0, 3}, {"3 to 6", 3, 6}, {"6 to 9", 6, 9}, {"9 to 12", 9, 12}, {"12 or more", 12, 99}} {
		var n, p, a, adj float64
		for _, s := range use {
			if s.AbsSpread >= b.lo && s.AbsSpread < b.hi {
				n++
				p += s.PredMin
				a += s.ActualMin
				adj += s.ActualMin - (s.PredMin + mf.Shift(s.AbsSpread, s.PredMin))
			}
		}
		if n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%+.2f\t%+.2f\n", b.name, int(n), p/n, a/n, (a-p)/n, adj/n)
		}
	}
	tw.Flush()
	fmt.Printf("\nFitted slope %.2f minutes per spread point at 30 projected minutes (z %+.1f). ", mf.Raw, mf.Z)
	if mf.Active() {
		fmt.Println("Clear enough to use; run with -write to save it for predict.")
	} else {
		fmt.Println("Not clear enough to use yet.")
	}
	fmt.Println()
}

// coverageTable checks the width of the minutes distribution. The model gives a range of
// outcomes, not one number; if its 10th-to-90th percentile band holds the actual minutes about 80%
// of the time it is the right width. Much less means the plays built on it are overconfident.
func coverageTable(samples []eval.BiasSample) {
	fmt.Println("MINUTES RANGE CHECK: how often actual minutes fall inside the model's 10th-90th percentile band (about 80% is right)")
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "projected minutes\tgames\tinside\tbelow\tabove\taverage band")
	for _, b := range []struct {
		name   string
		lo, hi float64
	}{{"under 20", 0, 20}, {"20 to 28", 20, 28}, {"28 or more", 28, 99}, {"everyone", 0, 99}} {
		var n, in, below, above, width float64
		for _, s := range samples {
			if s.PredMin < b.lo || s.PredMin >= b.hi {
				continue
			}
			nodes := append([]float64(nil), s.Pred0.MinNodes[:]...)
			sort.Float64s(nodes)
			lo, hi := nodes[1], nodes[len(nodes)-2]
			n++
			width += hi - lo
			switch {
			case s.ActualMin < lo:
				below++
			case s.ActualMin > hi:
				above++
			default:
				in++
			}
		}
		if n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%.0f%%\t%.0f%%\t%.0f%%\t%.1f min\n", b.name, int(n), 100*in/n, 100*below/n, 100*above/n, width/n)
		}
	}
	tw.Flush()
	fmt.Println("Inside well under 80% means the band is too narrow (too confident). Below and above should each be near 10%.")
	fmt.Println()
}

// woTable tests whether a player's own history of minutes and output without today's absent
// teammates helps. Every game is projected without that history, then grouped by what the
// history says; if the bigger groups beat their projection by more, the history carries signal.
func woTable(samples []eval.BiasSample) {
	fmt.Println("WITH-AND-WITHOUT HISTORY: minutes miss (actual - projected) by what his history says about today's absent teammates")
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "history says\tgames\tprojected\tactual\tminutes miss\tpoints miss\tassists miss\trebounds miss")
	for _, b := range []struct {
		name   string
		lo, hi float64
	}{{"minutes down 1+", -99, -1}, {"about the same", -1, 1}, {"minutes up 1 to 3", 1, 3}, {"minutes up 3 to 6", 3, 6}, {"minutes up 6+", 6, 99}} {
		var n, p, a, dp, da, dr float64
		for _, s := range samples {
			if s.WoMin < b.lo || s.WoMin >= b.hi {
				continue
			}
			n++
			p += s.PredMin
			a += s.ActualMin
			dp += s.Actual[0] - s.Pred[0]
			dr += s.Actual[1] - s.Pred[1]
			da += s.Actual[2] - s.Pred[2]
		}
		if n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%+.2f\t%+.2f\t%+.2f\t%+.2f\n", b.name, int(n), p/n, a/n, (a-p)/n, dp/n, da/n, dr/n)
		}
	}
	tw.Flush()
	wf := eval.FitWoFix(samples)
	fmt.Printf("\nMinutes: a miss of %+.2f per minute the history predicts (z %+.1f)", wf.MinRaw, wf.MinZ)
	if wf.Minutes > 0 {
		fmt.Printf(" -> %.2f applied.\n", wf.Minutes)
	} else {
		fmt.Println(" -> not clear enough, not applied.")
	}
	for _, st := range data.Stats {
		fmt.Printf("%-9s per-minute output: slope %+.2f (z %+.1f)", st, wf.RateRaw[string(st)], wf.RateZ[string(st)])
		if wf.Rate[string(st)] > 0 {
			fmt.Printf(" -> %.2f applied.\n", wf.Rate[string(st)])
		} else {
			fmt.Println(" -> not applied.")
		}
	}
	fmt.Println("A slope near 1 would mean the history is right as it stands; near 0, it is noise. Run with -write to save it for predict.")
	fmt.Println()
}
