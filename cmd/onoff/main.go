// Command onoff shows who gains minutes and production when a teammate is out, from the box scores
// already in data/. For every player ruled out today it lists the teammates whose minutes and
// stats were higher in games without him, with how many games each number rests on.
//
//	go run ./cmd/onoff                    (everyone ruled out in today's injuries snapshot)
//	go run ./cmd/onoff -player "Duren"    (what happens when that player sits, whoever's out today)
//
// These are raw averages from small samples, so read the game counts: three games without him is a
// hint, not a rule.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/props"
)

type avg struct {
	n                      int
	min, pts, reb, ast, tp float64
}

func (a *avg) add(g data.PlayerGame) {
	a.n++
	a.min += g.Minutes
	a.pts += g.Pts
	a.reb += g.Reb
	a.ast += g.Ast
	a.tp += g.Fg3m
}

func (a avg) mean() avg {
	if a.n == 0 {
		return a
	}
	f := float64(a.n)
	return avg{a.n, a.min / f, a.pts / f, a.reb / f, a.ast / f, a.tp / f}
}

type effect struct {
	name       string
	with, wout avg
}

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	player := flag.String("player", "", "show the effect of this player being out (part of his name)")
	minOut := flag.Int("minout", 2, "fewest games without him a teammate needs to be listed")
	minWith := flag.Int("minwith", 3, "fewest games together a teammate needs")
	top := flag.Int("top", 6, "teammates to list per absent player")
	all := flag.Bool("all", false, "include teams that are not playing on that date")
	doubtful := flag.Bool("doubtful", false, "treat doubtful players as out")
	flag.Parse()
	if *date == "" {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			loc = time.UTC
		}
		*date = time.Now().In(loc).Format("2006-01-02")
	}
	games, err := data.LoadStatsDir(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	today := data.DayNumber(*date)
	// two seasons of games before the date
	var window []data.PlayerGame
	for _, g := range games {
		if g.Day < today && today-g.Day <= 560 && g.Minutes > 0 {
			window = append(window, g)
		}
	}
	names := map[int]string{}
	lastTeam := map[int]int{}
	lastDay := map[int]int{}
	recent := map[int][]float64{} // minutes of his last ten games, however long ago
	for _, g := range window {
		if g.Name != "" {
			names[g.PlayerID] = g.Name
		}
		if g.Day >= lastDay[g.PlayerID] {
			lastDay[g.PlayerID], lastTeam[g.PlayerID] = g.Day, g.TeamID
		}
	}
	for _, g := range window {
		r := append(recent[g.PlayerID], g.Minutes)
		if len(r) > 10 {
			r = r[1:]
		}
		recent[g.PlayerID] = r
	}
	recentAvg := func(id int) (float64, int) {
		r := recent[id]
		var s float64
		for _, m := range r {
			s += m
		}
		if len(r) == 0 {
			return 0, 0
		}
		return s / float64(len(r)), len(r)
	}

	type tg struct{ team, game int }
	roster := map[tg][]data.PlayerGame{}
	for _, g := range window {
		k := tg{g.TeamID, g.GameID}
		roster[k] = append(roster[k], g)
	}

	var absent []int
	label := map[int]string{}
	if *player != "" {
		for id, n := range names {
			if strings.Contains(strings.ToLower(n), strings.ToLower(*player)) {
				absent = append(absent, id)
				label[id] = "out (asked for)"
			}
		}
		if len(absent) == 0 {
			fmt.Println("no player in the box scores matches", *player)
			return
		}
	} else {
		injPath := filepath.Join(*dir, "snapshots", fmt.Sprintf("injuries_%s.jsonl", *date))
		out, err := props.LoadOut(injPath, *doubtful)
		if err != nil {
			fmt.Fprintln(os.Stderr, "no injuries snapshot for", *date, "- run: go run ./cmd/pull snapshot")
			os.Exit(1)
		}
		for id := range out {
			absent = append(absent, id)
			label[id] = "out"
		}
		if st, err := props.LoadStatuses(injPath); err == nil {
			for id, s := range st {
				if _, isOut := out[id]; !isOut && props.IsUncertain(s) {
					absent = append(absent, id)
					label[id] = strings.ToLower(s) + " (if he sits)"
				}
			}
		}
	}
	sort.Ints(absent)

	// Keep only teams playing on the date (from the props snapshot) unless -all or -player is used.
	if !*all && *player == "" {
		if gm, err := props.LoadGames(filepath.Join(*dir, "snapshots", fmt.Sprintf("props_%s.jsonl", *date))); err == nil && len(gm) > 0 {
			playing := map[int]bool{}
			for _, g := range gm {
				playing[g.Home], playing[g.Visitor] = true, true
			}
			kept := absent[:0]
			for _, a := range absent {
				if playing[lastTeam[a]] {
					kept = append(kept, a)
				}
			}
			absent = kept
		} else {
			fmt.Println("(no props snapshot for", *date+", so teams that are not playing are included)")
		}
	}

	shown := 0
	for _, a := range absent {
		team := lastTeam[a]
		if m, n := recentAvg(a); team == 0 || n < 3 || m < 15 {
			continue // not a rotation player in his last ten games, however long ago they were
		}
		// the first game this season-or-last he played for this team
		join := 1 << 30
		for _, g := range window {
			if g.PlayerID == a && g.TeamID == team && g.Day < join {
				join = g.Day
			}
		}
		with := map[int]*avg{}
		wout := map[int]*avg{}
		nWith, nWout := 0, 0
		for k, rs := range roster {
			if k.team != team || rs[0].Day < join {
				continue
			}
			played := false
			for _, g := range rs {
				if g.PlayerID == a {
					played = true
				}
			}
			if played {
				nWith++
			} else {
				nWout++
			}
			for _, g := range rs {
				if g.PlayerID == a {
					continue
				}
				m := wout
				if played {
					m = with
				}
				if m[g.PlayerID] == nil {
					m[g.PlayerID] = &avg{}
				}
				m[g.PlayerID].add(g)
			}
		}
		var rows []effect
		for pid, w := range with {
			o := wout[pid]
			if o == nil || lastTeam[pid] != team || w.n < *minWith || o.n < *minOut {
				continue
			}
			rows = append(rows, effect{names[pid], w.mean(), o.mean()})
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].wout.min-rows[i].with.min > rows[j].wout.min-rows[j].with.min
		})
		fmt.Printf("%s is %s. His team played %d games with him and %d without him since he joined.\n", names[a], label[a], nWith, nWout)
		if len(rows) == 0 {
			fmt.Println("  not enough games without him to say anything about his teammates.")
			fmt.Println()
			continue
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  teammate\tgames with/without\tminutes with > without\tpoints\trebounds\tassists\tthrees\t")
		for i, r := range rows {
			if i == *top {
				break
			}
			thin := ""
			if r.wout.n < 3 {
				thin = "thin"
			}
			fmt.Fprintf(tw, "  %s\t%d / %d\t%.1f > %.1f (%+.1f)\t%+.1f\t%+.1f\t%+.1f\t%+.1f\t%s\n", r.name, r.with.n, r.wout.n,
				r.with.min, r.wout.min, r.wout.min-r.with.min,
				r.wout.pts-r.with.pts, r.wout.reb-r.with.reb, r.wout.ast-r.with.ast, r.wout.tp-r.with.tp, thin)
		}
		tw.Flush()
		fmt.Println()
		shown++
	}
	if shown == 0 {
		fmt.Println("no rotation players ruled out for", *date, "(or none with enough recent games).")
	}
	fmt.Println("Per-game averages in games without him minus games with him. Raw numbers from small samples: read the game counts.")
}
