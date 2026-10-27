// Command export writes the website's data file from your pick logs.
//
//	go run ./cmd/export                       (latest logged day, out: site/public/data/picks.json)
//	go run ./cmd/export -date 2026-10-24
//
// The site reads this one file. Nothing private goes in it: no API keys, no bankroll, only the
// plays you would show a visitor (player, line, pick, grade, tier, result).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/runlog"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	out := flag.String("out", filepath.Join("site", "public", "data", "picks.json"), "file to write")
	date := flag.String("date", "", "board day YYYY-MM-DD (default: the latest day in the pick log)")
	recent := flag.Int("recent", 600, "how many recent settled plays to include")
	flag.Parse()

	entries, err := picklog.Load(filepath.Join(*dir, "picklog.jsonl"))
	if err != nil {
		fatal(err)
	}
	settled, err := picklog.LoadSettled(filepath.Join(*dir, "settled.jsonl"))
	if err != nil {
		fatal(err)
	}
	runs, err := runlog.Load(filepath.Join(*dir, "runs.jsonl"))
	if err != nil {
		fatal(err)
	}
	if *date == "" {
		*date = latestDate(entries)
	}
	form, abbr := loadForm(*dir)
	res := newMatchupResolver(*dir, abbr)
	fixed := res.fill(entries)
	for i := range settled {
		m := []picklog.Entry{settled[i].Entry}
		fixed += res.fill(m)
		settled[i].Entry = m[0]
	}
	if fixed > 0 {
		fmt.Printf("Filled in team names for %d plays that were logged with only a game number.\n", fixed)
	}
	comboSettled, err := picklog.LoadSettled(filepath.Join(*dir, "combosettled.jsonl"))
	if err != nil {
		fatal(err)
	}
	site := Site{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Board:       buildBoard(entries, *date, form, runlog.ForDate(runs, *date)),
		Record:      buildRecord(settled, *recent),
		Combos:      buildComboRecord(comboSettled, 120),
	}
	var mf moreFile
	if raw, err := os.ReadFile(filepath.Join(*dir, "more.json")); err == nil {
		_ = json.Unmarshal(raw, &mf)
	}
	attachMore(&site.Board, mf, form)
	attachContext(&site.Board, mf)
	attachNews(&site.Board, mf)
	if mf.Date == site.Board.Date {
		site.Board.Overs = mf.Overs
	}
	attachCombos(&site.Board, mf, form)
	attachCases(&site.Board, trackRecords(settled))
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fatal(err)
	}
	b, err := json.MarshalIndent(site, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("Wrote %s: board for %s (%d plays in %d games, %d staked), record of %d graded plays across %d days.\n",
		*out, site.Board.Date, site.Board.Plays, len(site.Board.Games), site.Board.Staked, len(site.Record.Recent), site.Record.Days)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// loadForm reads the stats history so each play can show the player's last five games. If the
// stats are not there the board still exports, just without the form charts.
func loadForm(dir string) (Form, map[int]string) {
	d, err := pipeline.Load(dir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "note: no stats history, so the form charts are left out:", err)
		return nil, nil
	}
	ix := d.Builder.Index()
	abbr := map[int]string{}
	for _, g := range d.Games {
		if g.TeamAbbr != "" {
			abbr[g.TeamID] = g.TeamAbbr
		}
	}
	return func(pid int, stat, before string) ([]Game5, float64, int) {
		hist := ix.History(pid)
		var prior []data.PlayerGame
		for _, g := range hist {
			if g.Date < before {
				prior = append(prior, g)
			}
		}
		if len(prior) == 0 {
			return nil, 0, 0
		}
		season := prior[len(prior)-1].Season
		var sum float64
		var n int
		for _, g := range prior {
			if g.Season == season {
				sum += g.Value(data.Stat(stat))
				n++
			}
		}
		start := len(prior) - 5
		if start < 0 {
			start = 0
		}
		var out []Game5
		for _, g := range prior[start:] {
			opp := g.VisitorTeamID
			home := g.TeamID == g.HomeTeamID
			if !home {
				opp = g.HomeTeamID
			}
			out = append(out, Game5{Date: g.Date, Opp: abbr[opp], Home: home, Value: g.Value(data.Stat(stat)), Minutes: g.Minutes, Prior: g.Season != season})
		}
		avg := 0.0
		if n > 0 {
			avg = sum / float64(n)
		}
		return out, avg, n
	}, abbr
}
