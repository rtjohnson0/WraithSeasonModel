// Command livecheck compares what the model projected for EVERY player it scored (data/projlog.jsonl,
// written by predict) with what happened and with the books' lines. Unlike the review of posted
// picks, nothing here is selected, so it shows whether the live projections are biased, and where.
//
//	go run ./cmd/livecheck
//
// Bias is actual minus projected: positive means the model projected too low.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"propsmodel/internal/data"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/projlog"
)

type obs struct {
	e       projlog.Entry
	minutes float64
	actual  map[string]float64
}

type acc struct {
	n         int
	sum, sq   float64
	over      int // actual above the number compared
	sumProj   float64
	sumActual float64
}

func (a *acc) add(actual, proj float64) {
	d := actual - proj
	a.n++
	a.sum += d
	a.sq += d * d
	a.sumProj += proj
	a.sumActual += actual
	if actual > proj {
		a.over++
	}
}

func (a acc) z() float64 {
	if a.n < 2 {
		return 0
	}
	m := a.sum / float64(a.n)
	v := a.sq/float64(a.n) - m*m
	if v <= 0 {
		return 0
	}
	return m / math.Sqrt(v/float64(a.n))
}

func main() {
	dir := flag.String("data", "data", "data directory")
	since := flag.String("since", "", "only days on or after this date (YYYY-MM-DD)")
	flag.Parse()
	entries, err := projlog.Load(filepath.Join(*dir, "projlog.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("data/projlog.jsonl is empty. Run `go run ./cmd/predict` on a game day (not with -nolog), then check again after the games are in your box scores.")
		return
	}
	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	ix := d.Builder.Index()
	var obss []obs
	days := map[string]bool{}
	for _, e := range entries {
		if *since != "" && e.Date < *since {
			continue
		}
		g, ok := ix.Actual(e.GameID, e.PlayerID)
		if !ok || g.Minutes <= 0 {
			continue
		}
		o := obs{e: e, minutes: g.Minutes, actual: map[string]float64{}}
		for _, st := range data.Stats {
			o.actual[string(st)] = g.Value(st)
		}
		obss = append(obss, o)
		days[e.Date] = true
	}
	if len(obss) == 0 {
		fmt.Printf("%d players are logged, but none has a box score yet. Pull stats first: go run ./cmd/pull stats -seasons 2026 -new\n", len(entries))
		return
	}
	var dl []string
	for k := range days {
		dl = append(dl, k)
	}
	sort.Strings(dl)
	fmt.Printf("LIVE PROJECTIONS VS RESULTS: %d player-games across %d day(s), %s to %s. Every player the model scored, picked or not.\n\n",
		len(obss), len(dl), dl[0], dl[len(dl)-1])

	groups := []struct {
		name string
		keep func(o obs) bool
	}{
		{"everyone", func(o obs) bool { return true }},
		{"back-to-back", func(o obs) bool { return o.e.B2B }},
		{"not back-to-back", func(o obs) bool { return !o.e.B2B }},
		{"teammates out", func(o obs) bool { return o.e.TeamOut > 0 }},
		{"no teammates out", func(o obs) bool { return o.e.TeamOut == 0 }},
		{"opponent players out", func(o obs) bool { return o.e.OppOut > 0 }},
		{"his own status uncertain", func(o obs) bool { return o.e.Maybe != "" }},
		{"close game (spread 3.5 or less)", func(o obs) bool { return o.e.Total > 0 && o.e.Spread <= 3.5 }},
		{"competitive (3.5 to 7)", func(o obs) bool { return o.e.Total > 0 && o.e.Spread > 3.5 && o.e.Spread <= 7 }},
		{"lopsided (7 to 10)", func(o obs) bool { return o.e.Total > 0 && o.e.Spread > 7 && o.e.Spread < 10 }},
		{"blowout risk (10 or more)", func(o obs) bool { return o.e.Total > 0 && o.e.Spread >= 10 }},
		{"high total (232 or more)", func(o obs) bool { return o.e.Total >= 232 }},
		{"low total (under 224)", func(o obs) bool { return o.e.Total > 0 && o.e.Total < 224 }},
		{"projected under 20 min", func(o obs) bool { return o.e.ProjMin < 20 }},
		{"projected 20-26 min", func(o obs) bool { return o.e.ProjMin >= 20 && o.e.ProjMin < 26 }},
		{"projected 26-32 min", func(o obs) bool { return o.e.ProjMin >= 26 && o.e.ProjMin < 32 }},
		{"projected 32+ min", func(o obs) bool { return o.e.ProjMin >= 32 }},
		{"cut 4+ below his normal", func(o obs) bool { return o.e.TypMin > 0 && o.e.ProjMin-o.e.TypMin <= -4 }},
		{"within 4 of his normal", func(o obs) bool { return o.e.TypMin > 0 && math.Abs(o.e.ProjMin-o.e.TypMin) < 4 }},
		{"raised 4+ above his normal", func(o obs) bool { return o.e.TypMin > 0 && o.e.ProjMin-o.e.TypMin >= 4 }},
	}

	fmt.Println("MINUTES (actual minus projected; positive = the model projected too few)")
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "group\tplayers\tprojected\tactual\tbias\tz")
	for _, g := range groups {
		var a acc
		for _, o := range obss {
			if g.keep(o) {
				a.add(o.minutes, o.e.ProjMin)
			}
		}
		if a.n < 8 {
			continue
		}
		fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%+.2f\t%+.1f\n", g.name, a.n, a.sumProj/float64(a.n), a.sumActual/float64(a.n), a.sum/float64(a.n), a.z())
	}
	tw.Flush()
	fmt.Println()

	for _, st := range data.Stats {
		k := string(st)
		fmt.Printf("%s: actual minus the model's raw projection, minus the projection used, and minus the books' median line\n", k)
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "group\tplayers\tvs raw\tvs used\tvs line\tbeat line\tz (vs raw)")
		for _, g := range groups {
			var raw, used, line acc
			for _, o := range obss {
				sp, ok := o.e.Stats[k]
				if !ok || !g.keep(o) {
					continue
				}
				raw.add(o.actual[k], sp.Raw)
				used.add(o.actual[k], sp.Proj)
				if sp.Line > 0 {
					line.add(o.actual[k], sp.Line)
				}
			}
			if raw.n < 8 {
				continue
			}
			beat := "n/a"
			lb := "n/a"
			if line.n > 0 {
				beat = fmt.Sprintf("%.0f%%", 100*float64(line.over)/float64(line.n))
				lb = fmt.Sprintf("%+.2f", line.sum/float64(line.n))
			}
			fmt.Fprintf(tw, "%s\t%d\t%+.2f\t%+.2f\t%s\t%s\t%+.1f\n", g.name, raw.n, raw.sum/float64(raw.n), used.sum/float64(used.n), lb, beat, raw.z())
		}
		tw.Flush()
		fmt.Println()
	}
	fmt.Println("How to read this: a bias near 0 with |z| under 2 is fine. A group with a large, clear bias is where the model is off.")
	fmt.Println("'vs line' shows whether the books were right for the same players; the model only has an edge where it beats the line, not just the raw number.")
	fmt.Println("Players are logged from the latest run each day, so run predict (without -nolog) before the games for a clean check.")
}
