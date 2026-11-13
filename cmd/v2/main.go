// Command v2 grades the V2 challenger, which predict runs beside the real model.
//
//	go run ./cmd/v2 settle     grade data/v2log.jsonl into data/v2settled.jsonl and print V2 against V1
//
// V2's plays are tracked, never staked, and kept out of the pick record. This is where its results are
// compared with the real model's over the same days.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/rotation"
	"propsmodel/internal/settlement"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "settle" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/v2 settle [-data data]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("settle", flag.ExitOnError)
	dir := fs.String("data", "data", "data directory")
	legacy := fs.Bool("legacy", false, "include unverified legacy quotes for forensic comparison")
	fs.Parse(os.Args[2:])
	settle(*dir, *legacy)
}

func settle(dir string, legacy bool) {
	logName, settledName := "v2log.jsonl", "v2settled.jsonl"
	if b, e := os.ReadFile(filepath.Join(dir, "v2.json")); e == nil {
		var f struct {
			Info struct {
				Policy string `json:"policy"`
			} `json:"info"`
		}
		if json.Unmarshal(b, &f) == nil && f.Info.Policy == rotation.Policy {
			logName, settledName = rotation.LogFile, rotation.SettledFile
		}
	}
	entries, err := picklog.Load(filepath.Join(dir, logName))
	if err != nil {
		fatal(err)
	}
	if !legacy {
		entries = picklog.VerifiedEntries(entries)
	}
	if len(entries) == 0 {
		fmt.Printf("%s is empty. Run `go run ./cmd/predict` first; it logs the V2 plays.\n", filepath.Join(dir, logName))
	}
	settledPath := filepath.Join(dir, settledName)

	report, err := settlement.Reconcile(dir, settledName, entries, nil)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Audited %d results: %d confirmed, %d for review.\n", report.Checked, report.Confirmed, report.Review)

	all, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	if !legacy {
		all = picklog.VerifiedSettled(all)
	}
	v2, _ := picklog.DedupeSettled(all)
	if len(v2) == 0 {
		return
	}
	v1all, _ := picklog.LoadSettled(filepath.Join(dir, "settled.jsonl"))
	if !legacy {
		v1all = picklog.VerifiedSettled(v1all)
	}
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
	fmt.Printf("V2 against V1 over the same %d day(s). These are different selected sets, not a matched model comparison. Use cmd/lines for identical questions.\n", len(days))
	row("V1 all plays, same days", v1same)
	row("V2 singles", singles)
	row("V2 combos", pick(v2, isCombo))
	for _, t := range []struct{ tier, name string }{
		{"STRONG", "V2 Strong plays (V1 agrees, stat and side proven)"},
		{"V2LEAN", "V2 Lean plays (posted, not yet backed)"},
		{"CHECK", "V2 Check plays (big gap to the book, or uncertain status)"},
	} {
		tier := t.tier
		row(t.name, pick(singles, func(r picklog.Settled) bool { return r.Tier == tier }))
	}
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
	fmt.Println("\nA few days of plays cannot separate skill from luck. Compare V1 and V2 on identical market questions and separate future dates before promotion.")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
