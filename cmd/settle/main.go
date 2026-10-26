// Command settle grades the picks in data/picklog.jsonl once games are final, measures closing
// line value (CLV), and prints your live record.
//
//	go run ./cmd/pull stats -seasons 2026      (so last night's results are in)
//	go run ./cmd/settle                         (settles every logged pick whose game is final)
//
// CLV is the most trustworthy early signal. If the line moves toward your side after you pick
// (you took OVER 24.5 and it closes at 25.5), the market agreed with you, and that predicts
// long-run profit far better than a hundred wins and losses do. The "closing" line here is the
// latest line in your snapshots for that day, so run `go run ./cmd/pull snapshot` shortly
// before tip-off (or on a schedule) for it to mean anything.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/props"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	flag.Parse()

	entries, err := picklog.Load(filepath.Join(*dir, "picklog.jsonl"))
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fmt.Println("data/picklog.jsonl is empty. Run `go run ./cmd/predict` first; it logs every posted pick.")
		return
	}
	settledPath := filepath.Join(*dir, "settled.jsonl")
	done, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	have := map[string]bool{}
	for _, s := range done {
		have[fmt.Sprintf("%s|%d|%s|%s|%s|%.1f", s.Date, s.PlayerID, s.Stat, s.Side, s.Book, s.Line)] = true
	}

	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fatal(err)
	}
	ix := d.Builder.Index()

	closing := map[string]map[closeKey]float64{} // date -> closing median line
	var fresh []picklog.Settled
	pending := 0
	for _, e := range entries {
		if have[fmt.Sprintf("%s|%d|%s|%s|%s|%.1f", e.Date, e.PlayerID, e.Stat, e.Side, e.Book, e.Line)] {
			continue
		}
		if _, known := ix.GameDate(e.GameID); !known {
			pending++ // the game is not in the stats yet
			continue
		}
		actual, played := ix.Actual(e.GameID, e.PlayerID)
		cl, ok := closing[e.Date]
		if !ok {
			cl = closingLines(*dir, e.Date)
			closing[e.Date] = cl
		}
		fresh = append(fresh, picklog.Grade(e, actual.Value(data.Stat(e.Stat)), cl[closeKey{e.GameID, e.PlayerID, e.Stat}], played))
	}
	if n, err := picklog.AppendSettled(settledPath, fresh); err != nil {
		fatal(err)
	} else {
		fmt.Printf("settled %d new picks (%d still waiting for final stats)\n\n", n, pending)
	}

	everyRaw, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	if len(everyRaw) == 0 {
		return
	}
	// The same play logged twice (another book, or a re-run) is one result, not two.
	every, dups := picklog.DedupeSettled(everyRaw)
	if dups > 0 {
		fmt.Printf("(%d duplicate log lines collapsed: the same player, stat and side on one day counts once)\n\n", dups)
	}
	// The headline record is real picks only. CHECK and LEAN plays are tracked beside it so you
	// can see whether the extra caution and the 55% bar are doing any work.
	var all []picklog.Settled
	byTier := map[string][]picklog.Settled{}
	for _, r := range every {
		if r.Tier == "" || r.Tier == "PICK" {
			all = append(all, r)
		} else {
			byTier[r.Tier] = append(byTier[r.Tier], r)
		}
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Println("LIVE RECORD")
	fmt.Fprintln(tw, "slice\tpicks\tW-L-P\thit rate\tpredicted\tunits\tROI\tflat 1u\tavg CLV\tCLV>0")
	row := func(name string, rs []picklog.Settled) {
		s := picklog.Summarize(rs)
		if s.Picks == 0 {
			return
		}
		clv, pos := "n/a", "n/a"
		if s.CLVN > 0 {
			clv, pos = fmt.Sprintf("%+.2f", s.AvgCLV), fmt.Sprintf("%.0f%%", 100*s.CLVPositive)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d-%d-%d\t%.1f%%\t%.1f%%\t%+.1f\t%+.1f%%\t%+.1f\t%s\t%s\n",
			name, s.Picks-s.Voids, s.Wins, s.Losses, s.Pushes, 100*s.HitRate(), 100*s.AvgProb, s.Profit, 100*s.ROI(), s.Flat, clv, pos)
	}
	row("all", all)
	for _, st := range data.Stats {
		var sub []picklog.Settled
		for _, r := range all {
			if r.Stat == string(st) {
				sub = append(sub, r)
			}
		}
		row(string(st), sub)
	}
	for _, side := range []string{"OVER", "UNDER"} {
		var sub []picklog.Settled
		for _, r := range all {
			if r.Side == side {
				sub = append(sub, r)
			}
		}
		row(side, sub)
	}
	for _, t := range []string{"PAPER", "CHECK", "LEAN"} {
		row(t+" (not staked)", byTier[t])
	}
	tw.Flush()

	// Situations, across every tier (most plays are PAPER right now, so this is where the sample is).
	// CLV as a share of the line is comparable across stats; stat points are not.
	fmt.Println("\nSITUATIONS (every tier, one row per player/stat/side/day)")
	fmt.Fprintln(tw, "situation\tplays\tW-L\thit rate\tpredicted\tavg CLV\tCLV % of line\tCLV>0")
	sit := func(name string, keep func(picklog.Settled) bool) {
		var sub []picklog.Settled
		var pct float64
		var pn int
		for _, r := range every {
			if !keep(r) {
				continue
			}
			sub = append(sub, r)
			if r.CloseLine > 0 && r.Line > 0 {
				pct += r.CLV / r.Line
				pn++
			}
		}
		s := picklog.Summarize(sub)
		if s.Picks-s.Voids == 0 {
			return
		}
		clv, share, pos := "n/a", "n/a", "n/a"
		if s.CLVN > 0 {
			clv, pos = fmt.Sprintf("%+.2f", s.AvgCLV), fmt.Sprintf("%.0f%%", 100*s.CLVPositive)
		}
		if pn > 0 {
			share = fmt.Sprintf("%+.1f%%", 100*pct/float64(pn))
		}
		fmt.Fprintf(tw, "%s\t%d\t%d-%d\t%.1f%%\t%.1f%%\t%s\t%s\t%s\n",
			name, s.Picks-s.Voids, s.Wins, s.Losses, 100*s.HitRate(), 100*s.AvgProb, clv, share, pos)
	}
	sit("every play", func(picklog.Settled) bool { return true })
	for _, f := range []string{"teammates-out", "opponent-out", "opponent-starter-out", "back-to-back"} {
		f := f
		sit(f, func(r picklog.Settled) bool { return picklog.HasFlag(r.Why, f) })
		sit("no "+f, func(r picklog.Settled) bool { return !picklog.HasFlag(r.Why, f) })
	}
	for _, side := range []string{"OVER", "UNDER"} {
		side := side
		sit(side, func(r picklog.Settled) bool { return r.Side == side })
	}
	tw.Flush()
	fmt.Println("A situation helps only if its plays beat the 'no ...' row on CLV and hit rate across a few hundred plays.")

	// Calibration of live picks: does a 60% pick win 60% of the time?
	fmt.Println("\nDO THE PROBABILITIES HOLD UP? (graded picks)")
	fmt.Fprintln(tw, "predicted\tpicks\tactual hit rate")
	for _, b := range [][2]float64{{0.55, 0.58}, {0.58, 0.62}, {0.62, 1.01}} {
		var sub []picklog.Settled
		for _, r := range all {
			if r.Prob >= b[0] && r.Prob < b[1] {
				sub = append(sub, r)
			}
		}
		s := picklog.Summarize(sub)
		if s.Wins+s.Losses > 0 {
			fmt.Fprintf(tw, "%.0f%%-%.0f%%\t%d\t%.1f%%\n", 100*b[0], 100*min(b[1], 1), s.Wins+s.Losses, 100*s.HitRate())
		}
	}
	tw.Flush()

	g := picklog.Summarize(all)
	fmt.Println()
	if len(all) == 0 {
		fmt.Println("No settled full picks yet.")
		return
	}
	switch {
	case g.Wins+g.Losses < 100:
		fmt.Printf("Only %d graded picks. Win/loss over that few says almost nothing (luck swings a 55%% bettor by\n", g.Wins+g.Losses)
		fmt.Println("several units either way). Watch average CLV: it is meaningful much sooner.")
	case g.CLVN >= 50 && g.AvgCLV <= 0:
		fmt.Println("Closing line value is not positive: the market is not moving toward your picks. Do not scale up.")
	case g.CLVN >= 50:
		fmt.Println("Closing line value is positive, which is the best early sign of a real edge. Keep staking small.")
	}
	days := map[string]bool{}
	for _, r := range all {
		days[r.Date] = true
	}
	var ds []string
	for d := range days {
		ds = append(ds, d)
	}
	sort.Strings(ds)
	fmt.Printf("%d picks across %d days (%s to %s).\n", len(all), len(ds), ds[0], ds[len(ds)-1])
	fmt.Println("PAPER, CHECK and LEAN rows were never staked; their units column is a flat 1u for comparison only.")
}

type closeKey struct {
	game, player int
	stat         string
}

// closingLines returns the median line across books in the latest snapshot for a date.
func closingLines(dir, date string) map[closeKey]float64 {
	out := map[closeKey]float64{}
	lines, err := props.LoadSnapshot(filepath.Join(dir, "snapshots", fmt.Sprintf("props_%s.jsonl", date)))
	if err != nil {
		return out
	}
	groups := map[closeKey][]float64{}
	for _, l := range lines {
		k := closeKey{l.GameID, l.PlayerID, string(l.Stat)}
		groups[k] = append(groups[k], l.Line)
	}
	for k, v := range groups {
		sort.Float64s(v)
		out[k] = v[len(v)/2]
	}
	return out
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
