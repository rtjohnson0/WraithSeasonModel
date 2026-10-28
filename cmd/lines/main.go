// Command lines grades the market log predict writes (data/lines): every line the model scored, not
// only the plays it posted.
//
//	go run ./cmd/lines             calibration at the median line, situation slices, off-market record
//	go run ./cmd/lines -since 2026-10-27   only days from this date on
//
// About 400 lines a day are logged against about 50 posted plays, so this answers "is the model
// right?" in days instead of weeks. Nothing here changes a pick.
package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"propsmodel/internal/lineslog"
	"propsmodel/internal/pipeline"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	since := flag.String("since", "", "only days on or after this date (YYYY-MM-DD)")
	flag.Parse()

	mk, err := lineslog.LoadMarkets(*dir)
	if err != nil {
		fatal(err)
	}
	off, err := lineslog.LoadOff(*dir)
	if err != nil {
		fatal(err)
	}
	if *since != "" {
		mk = filter(mk, func(m lineslog.Market) bool { return m.Date >= *since })
		off = filter(off, func(o lineslog.Off) bool { return o.Date >= *since })
	}
	if len(mk) == 0 && len(off) == 0 {
		fmt.Println("data/lines is empty. Run `go run ./cmd/predict` (without -nolog) first; it logs every line it scores.")
		return
	}
	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fatal(err)
	}
	act := lineslog.Actualer(d.Builder.Index().Actual)

	gm := lineslog.GradeMarkets(mk, act)
	fmt.Printf("ALL LINES: %d graded of %d logged (the rest are waiting on final stats or the player did not play)\n\n", len(gm), len(mk))
	if len(gm) > 0 {
		fmt.Println("Calibration at the market's median line: when the model said X% Over, how often did it go Over?")
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "model P(over)\tlines\tmodel said\twent over\tdifference")
		for _, b := range lineslog.CalBins(gm) {
			fmt.Fprintf(tw, "%s\t%d\t%.1f%%\t%.1f%%\t%+.1f pts\n", b.Label, b.N, 100*b.Predicted, 100*b.Actual, 100*(b.Actual-b.Predicted))
		}
		tw.Flush()
		model, base, n := lineslog.Skill(gm)
		fmt.Printf("\nBrier score %.4f for the model against %.4f for always saying the day's base rate (%d lines; lower is better).\n", model, base, n)
		if model < base {
			fmt.Println("The model's probabilities carry information beyond the base rate.")
		} else {
			fmt.Println("The model's probabilities are not yet beating the base rate: its leans at the median line are not predictive on this sample.")
		}
		fmt.Println("\nBy situation: share that went Over against the model's average P(over). A gap of 5+ points on 100+ lines is worth a look.")
		tw = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "situation\tlines\twent over\tmodel said\tgap")
		for _, s := range lineslog.Slices(gm) {
			fmt.Fprintf(tw, "%s\t%d\t%.1f%%\t%.1f%%\t%+.1f pts\n", s.Name, s.N, 100*s.OverRate, 100*s.ModelOver, 100*(s.OverRate-s.ModelOver))
		}
		tw.Flush()
	}

	go2 := lineslog.GradeOff(off, act)
	fmt.Printf("\nOFF-MARKET LINES: %d graded (one per player, stat and side per day, at the best price)\n", len(go2))
	if len(go2) > 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "group\tlines\tW-L-P\thit rate\tmodel said\tflat units\troi")
		for _, r := range lineslog.OffSummary(go2) {
			fmt.Fprintf(tw, "%s\t%d\t%d-%d-%d\t%.1f%%\t%.1f%%\t%+.1f\t%+.1f%%\n", r.Name, r.N, r.Wins, r.Losses, r.Pushes, 100*r.HitRate, 100*r.Predicted, r.Units, 100*r.ROI())
		}
		tw.Flush()
		fmt.Println("\nUnits are one unit per line at the offered odds. A line that sits off the market is judged against its own price, so the hit rate to beat is the break-even at those odds (about 52.4% at -110).")
	}
}

func filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
