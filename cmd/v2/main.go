// Command v2 grades the V2 challenger, which predict runs beside the real model.
//
//	go run ./cmd/v2 settle     grade data/v2log.jsonl into data/v2settled.jsonl and print V2 against V1
//
// V2's plays are tracked, never staked, and kept out of the pick record. This is where its results are
// compared with the real model's over the same days.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "settle" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/v2 settle [-data data]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("settle", flag.ExitOnError)
	dir := fs.String("data", "data", "data directory")
	fs.Parse(os.Args[2:])
	settle(*dir)
}

func settle(dir string) {
	entries, err := picklog.Load(filepath.Join(dir, "v2log.jsonl"))
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fmt.Println("data/v2log.jsonl is empty. Run `go run ./cmd/predict` first; it logs the V2 plays.")
		return
	}
	settledPath := filepath.Join(dir, "v2settled.jsonl")
	done, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	key := func(date string, pid int, stat, side, book string, line float64) string {
		return fmt.Sprintf("%s|%d|%s|%s|%s|%.1f", date, pid, stat, side, book, line)
	}
	have := map[string]bool{}
	for _, s := range done {
		have[key(s.Date, s.PlayerID, s.Stat, s.Side, s.Book, s.Line)] = true
	}
	d, err := pipeline.Load(dir, false)
	if err != nil {
		fatal(err)
	}
	ix := d.Builder.Index()
	var fresh []picklog.Settled
	pending := 0
	for _, e := range entries {
		if have[key(e.Date, e.PlayerID, e.Stat, e.Side, e.Book, e.Line)] {
			continue
		}
		if _, known := ix.GameDate(e.GameID); !known {
			pending++
			continue
		}
		actual, played := ix.Actual(e.GameID, e.PlayerID)
		fresh = append(fresh, picklog.Grade(e, actual.Value(data.Stat(e.Stat)), 0, played))
	}
	n, err := picklog.AppendSettled(settledPath, fresh)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("settled %d new V2 plays (%d still waiting for final stats)\n\n", n, pending)

	all, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	v2, _ := picklog.DedupeSettled(all)
	if len(v2) == 0 {
		return
	}
	v1all, _ := picklog.LoadSettled(filepath.Join(dir, "settled.jsonl"))
	v1, _ := picklog.DedupeSettled(v1all)
	days := map[string]bool{}
	for _, r := range v2 {
		days[r.Date] = true
	}
	var v1same []picklog.Settled
	for _, r := range v1 {
		if days[r.Date] {
			v1same = append(v1same, r)
		}
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "slice\tplays\tW-L-P\thit rate\tpredicted\tflat 1u\tROI at the odds")
	row := func(name string, rs []picklog.Settled) {
		s := picklog.Summarize(rs)
		if s.Picks-s.Voids == 0 {
			return
		}
		roi := 0.0
		if s.Wins+s.Losses > 0 {
			roi = s.Flat / float64(s.Wins+s.Losses)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d-%d-%d\t%.1f%%\t%.1f%%\t%+.1f\t%+.1f%%\n", name, s.Picks-s.Voids, s.Wins, s.Losses, s.Pushes, 100*s.HitRate(), 100*s.AvgProb, s.Flat, 100*roi)
	}
	pick := func(rs []picklog.Settled, keep func(picklog.Settled) bool) []picklog.Settled {
		var out []picklog.Settled
		for _, r := range rs {
			if keep(r) {
				out = append(out, r)
			}
		}
		return out
	}
	isCombo := func(r picklog.Settled) bool { return data.Stat(r.Stat).IsCombo() }
	singles := pick(v2, func(r picklog.Settled) bool { return !isCombo(r) })
	fmt.Printf("V2 against V1 over the same %d day(s). Both are graded at the odds offered; V1 includes unstaked plays so the two are comparable.\n", len(days))
	row("V1 all plays, same days", v1same)
	row("V2 singles", singles)
	row("V2 combos", pick(v2, isCombo))
	for _, st := range data.Stats {
		stat := string(st)
		row("V2 "+stat, pick(singles, func(r picklog.Settled) bool { return r.Stat == stat }))
		row("V1 "+stat, pick(v1same, func(r picklog.Settled) bool { return r.Stat == stat }))
	}
	for _, side := range []string{"OVER", "UNDER"} {
		sd := side
		row("V2 "+strings.ToLower(sd)+"s", pick(singles, func(r picklog.Settled) bool { return r.Side == sd }))
		row("V1 "+strings.ToLower(sd)+"s", pick(v1same, func(r picklog.Settled) bool { return r.Side == sd }))
	}
	tw.Flush()
	fmt.Println("\nA few days of plays cannot separate skill from luck. Look for V2 beating V1 on the same days over a few hundred plays before reading anything into it.")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
