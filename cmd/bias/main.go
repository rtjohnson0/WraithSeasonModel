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
	noBlowout := flag.Bool("noblowout", false, "leave out the blowout correction to the minutes averages (to see whether it is what moves the bias)")
	lineup := flag.Bool("lineup", false, "check the model with the teammates-out and opponent-out features (use the same setting you trained with)")
	write := flag.Bool("write", false, "save a conservative per-stat correction to data/biasfix.json, which predict applies (only a large, statistically clear bias is used)")
	flag.Parse()
	feat.BlowoutAdjust = !*noBlowout
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
	if *write {
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
