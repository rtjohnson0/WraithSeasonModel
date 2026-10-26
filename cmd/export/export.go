package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"propsmodel/internal/bets"
	"propsmodel/internal/picklog"
	"propsmodel/internal/runlog"
)

// Play is one pick on the board.
type Play struct {
	Player   string  `json:"player"`
	Team     string  `json:"team"`
	Stat     string  `json:"stat"`
	Side     string  `json:"side"`
	Line     float64 `json:"line"`
	Prob     float64 `json:"prob"`
	Edge     float64 `json:"edge"`
	Grade    string  `json:"grade"`
	Tier     string  `json:"tier"`
	Odds     int     `json:"odds"`
	Book     string  `json:"book"`
	ProjStat float64 `json:"projStat"`
	ProjMin  float64 `json:"projMin"`
	Units    float64 `json:"units"`
	Why      string  `json:"why"`

	// Form is how he has done against this line recently: the last five games, his average and how
	// often this side would have hit. Empty when the stats are not available to the export.
	// Runs is how many of today's runs (9 AM, 3 PM, 6 PM, 8 PM) had this play on the board, and
	// SeenIn names them. Multi is true when it showed up in more than one: the model kept choosing it.
	Runs   int      `json:"runs,omitempty"`
	SeenIn []string `json:"seenIn,omitempty"`
	Multi  bool     `json:"multi,omitempty"`

	Label  string  `json:"label"`  // Max play, Great play, Risky play or Lean
	Badges []Badge `json:"badges"` // quick-read signals: weak defense, hot streak, back-to-back and so on

	Last5     []Game5 `json:"last5,omitempty"`
	L5Avg     float64 `json:"l5Avg"`
	L5Hits    int     `json:"l5Hits"`
	SeasonAvg float64 `json:"seasonAvg"`
	SeasonGP  int     `json:"seasonGames"`
	MinAvg5   float64 `json:"minAvg5"`
}

// Badge is one quick-read signal about a play. Tone is from the point of view of the pick.
type Badge struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Tone  string `json:"tone"` // good, bad or note
}

// Game5 is one of his recent games.
type Game5 struct {
	Date    string  `json:"date"`
	Opp     string  `json:"opp"`
	Home    bool    `json:"home"`
	Value   float64 `json:"value"`
	Minutes float64 `json:"minutes"`
}

// Form looks up a player's recent games for a stat before a date, newest last. Nil is allowed.
type Form func(playerID int, stat, before string) (games []Game5, seasonAvg float64, seasonGames int)

// Game groups one matchup's plays.
type Game struct {
	ID      int    `json:"id"`
	Matchup string `json:"matchup"`
	Plays   []Play `json:"plays"`
}

// Board is one day's plays.
type Board struct {
	Date   string `json:"date"`
	Staked int    `json:"staked"`
	Plays  int    `json:"plays"`
	Games  []Game `json:"games"`

	// RunsToday lists the day's runs in order, each with what it dropped from the run before it.
	RunsToday []RunSummary `json:"runsToday,omitempty"`
}

// RunSummary is one run of the day and how its board differed from the previous run's.
type RunSummary struct {
	Label   string        `json:"label"`
	Clock   string        `json:"clock"`
	Prev    string        `json:"prev,omitempty"`
	Plays   int           `json:"plays"`
	New     int           `json:"new"`
	Dropped []DroppedPlay `json:"dropped,omitempty"`
	Changed []TierMove    `json:"changed,omitempty"`
}

// DroppedPlay is a play that left the board between runs, with the reason.
type DroppedPlay struct {
	Player  string  `json:"player"`
	Team    string  `json:"team,omitempty"`
	Matchup string  `json:"matchup,omitempty"`
	Stat    string  `json:"stat"`
	Side    string  `json:"side"`
	Line    float64 `json:"line"`
	Tier    string  `json:"tier"`
	Prob    float64 `json:"prob"`
	Reason  string  `json:"reason"`
}

// TierMove is a play that stayed on the board but changed tier between runs.
type TierMove struct {
	Player string `json:"player"`
	Stat   string `json:"stat"`
	Side   string `json:"side"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// Row is one line of a record table.
type Row struct {
	Name      string  `json:"name"`
	Picks     int     `json:"picks"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	Pushes    int     `json:"pushes"`
	HitRate   float64 `json:"hitRate"`
	Predicted float64 `json:"predicted"`
	Units     float64 `json:"units"`
	AvgCLV    float64 `json:"avgClv"`
	CLVN      int     `json:"clvN"`
}

// Bucket compares predicted and actual hit rates.
type Bucket struct {
	Label     string  `json:"label"`
	Picks     int     `json:"picks"`
	Predicted float64 `json:"predicted"`
	Actual    float64 `json:"actual"`
}

// Result is one settled play.
type Result struct {
	Date    string  `json:"date"`
	Player  string  `json:"player"`
	Team    string  `json:"team"`
	Matchup string  `json:"matchup"`
	Stat    string  `json:"stat"`
	Side    string  `json:"side"`
	Line    float64 `json:"line"`
	Actual  float64 `json:"actual"`
	Result  string  `json:"result"`
	Tier    string  `json:"tier"`
	Prob    float64 `json:"prob"`
	Grade   string  `json:"grade"`
	Label   string  `json:"label"`
}

// Day is one day of results with running totals, for the charts.
type Day struct {
	Date        string  `json:"date"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	StakedUnits float64 `json:"stakedUnits"` // cumulative, at the stakes used
	FlatUnits   float64 `json:"flatUnits"`   // cumulative, 1 unit on every tracked play
	StakedW     int     `json:"stakedWins"`
	StakedL     int     `json:"stakedLosses"`
}

// Record is the live record.
type Record struct {
	Since   string   `json:"since"`
	Until   string   `json:"until"`
	Days    int      `json:"days"`
	Dups    int      `json:"duplicatesCollapsed"`
	Tiers   []Row    `json:"tiers"`
	Stats   []Row    `json:"stats"`
	Sides   []Row    `json:"sides"`
	Buckets []Bucket `json:"buckets"`
	Daily   []Day    `json:"daily"`
	Recent  []Result `json:"recent"`
}

// Site is everything the website reads.
type Site struct {
	GeneratedAt string `json:"generatedAt"`
	Board       Board  `json:"board"`
	Record      Record `json:"record"`
}

// grade turns the model's calibrated probability into a letter. Only staked picks can earn an A:
// plays on stats the backtest did not support, early-season plays and plays that need a look are
// capped at B+, so a letter never claims more than the evidence does.
func grade(prob float64, tier string) string {
	g := "D"
	switch {
	case prob >= 0.66:
		g = "A+"
	case prob >= 0.63:
		g = "A"
	case prob >= 0.60:
		g = "A-"
	case prob >= 0.58:
		g = "B+"
	case prob >= 0.56:
		g = "B"
	case prob >= 0.55:
		g = "B-"
	case prob >= 0.53:
		g = "C+"
	case prob >= 0.52:
		g = "C"
	}
	if tier != "" && tier != "PICK" {
		order := []string{"D", "C", "C+", "B-", "B", "B+", "A-", "A", "A+"}
		rank := map[string]int{}
		for i, x := range order {
			rank[x] = i
		}
		if rank[g] > rank["B+"] {
			g = "B+"
		}
	}
	return g
}

// label is the friendly name for a play. It describes the evidence, not just the number:
// only staked picks are Max plays, and a strong number on an unproven stat is a Great play,
// never a Max play.
func label(tier string, prob float64) string {
	switch normTier(tier) {
	case "PICK":
		return "Max play"
	case "CHECK":
		return "Risky play"
	case "PAPER":
		if prob >= 0.58 {
			return "Great play"
		}
	}
	return "Lean"
}

// badgesFor reads the model's notes and the recent games into short signals. Tone is from the
// point of view of the pick: a weak defense helps an over and hurts an under.
func badgesFor(p Play) []Badge {
	var out []Badge
	add := func(key, label string, good bool) {
		tone := "bad"
		if good {
			tone = "good"
		}
		out = append(out, Badge{key, label, tone})
	}
	note := func(key, label string) { out = append(out, Badge{key, label, "note"}) }
	over := p.Side == "OVER"
	why := strings.ToLower(p.Why)

	if strings.Contains(why, "soft defense") {
		add("weak-defense", "Weak defense against his position", over)
	}
	if strings.Contains(why, "tough defense") {
		add("tough-defense", "Tough defense against his position", !over)
	}
	if len(p.Last5) >= 4 && p.SeasonGP >= 6 && p.SeasonAvg > 0 {
		r := p.L5Avg / p.SeasonAvg
		switch {
		case r >= 1.2 && p.L5Avg-p.SeasonAvg >= 0.8:
			add("hot", "Hot streak: last games well above his season average", over)
		case r <= 0.8 && p.SeasonAvg-p.L5Avg >= 0.8:
			add("cold", "Cold streak: last games well below his season average", !over)
		}
	}
	if n := len(p.Last5); n >= 4 {
		switch {
		case p.L5Hits >= n-1:
			add("trend-for", fmt.Sprintf("The %s hit in %d of his last %d", strings.ToLower(p.Side), p.L5Hits, n), true)
		case p.L5Hits <= 1:
			add("trend-against", fmt.Sprintf("The %s hit in only %d of his last %d", strings.ToLower(p.Side), p.L5Hits, n), false)
		}
	}
	if len(p.Last5) >= 4 && p.MinAvg5 > 0 {
		d := p.ProjMin - p.MinAvg5
		if d >= 3 {
			add("minutes-up", fmt.Sprintf("Minutes up: projected %.0f vs %.0f lately", p.ProjMin, p.MinAvg5), over)
		} else if d <= -3 {
			add("minutes-down", fmt.Sprintf("Minutes down: projected %.0f vs %.0f lately", p.ProjMin, p.MinAvg5), !over)
		}
	}
	if strings.Contains(why, "back-to-back") {
		add("back-to-back", "Second game in two nights", !over)
	}
	if strings.Contains(why, "out:") {
		note("teammates-out", "Teammates out: roles and usage shift")
	}
	if strings.Contains(why, "opponent out:") {
		note("opponent-out", "Opponent missing rotation players: their defense and rebounding change")
	}
	if strings.Contains(why, "uncertain") || strings.Contains(why, "his status") {
		note("status", "Injury status to check before tip-off")
	}
	if strings.Contains(why, "market line moved with the pick") {
		add("line-move", "The market line has moved toward this pick since the first snapshot", true)
	} else if strings.Contains(why, "market line moved against the pick") {
		add("line-move", "The market line has moved away from this pick since the first snapshot", false)
	} else if strings.Contains(why, "market line moved") {
		note("line-move", "The market line has moved since the first snapshot")
	}
	if strings.Contains(why, "off the market, in your favor") {
		add("odd-line", "Soft line: this book is off the market in your favor", true)
	} else if strings.Contains(why, "off the market") {
		note("odd-line", "Off the market: this book's line differs from the rest")
	}
	hasOdd := false
	for _, x := range out {
		if x.Key == "odd-line" {
			hasOdd = true
		}
	}
	if !hasOdd && strings.Contains(why, "far from projection") {
		note("odd-line", "Odd line: confirm it in your book")
	}
	return out
}

func normTier(t string) string {
	if t == "" {
		return "PICK"
	}
	return t
}

func edgeOf(e picklog.Entry) float64 {
	if e.Edge != 0 {
		return e.Edge
	}
	if e.Odds != 0 {
		return bets.Edge(e.Prob, e.Odds)
	}
	return 0
}

func matchupOf(e picklog.Entry) string {
	if e.Matchup != "" {
		return e.Matchup
	}
	return fmt.Sprintf("Game %d", e.GameID)
}

// buildBoard collapses repeats of the same player, stat and side (keeping the most trusted tier,
// then the best edge) and groups the plays by game.
func buildBoard(entries []picklog.Entry, date string, form Form, day []runlog.Run) Board {
	best := map[string]picklog.Entry{}
	var order []string
	// With run history, the board is the latest run's plays. A play from an earlier run that the
	// latest run dropped is listed under that run's changes, with the reason, not shown as live.
	var live map[string]bool
	var count map[string]int
	var seenIn map[string][]string
	if len(day) > 0 {
		live = map[string]bool{}
		for _, p := range day[len(day)-1].Plays {
			live[p.Key()] = true
		}
		count, seenIn = runlog.Appearances(day)
	}
	for _, e := range entries {
		if e.Date != date {
			continue
		}
		k := fmt.Sprintf("%d|%s|%s", e.PlayerID, e.Stat, e.Side)
		if live != nil && !live[k] {
			continue
		}
		cur, ok := best[k]
		if !ok {
			order = append(order, k)
			best[k] = e
			continue
		}
		if picklog.TierRank(e.Tier) < picklog.TierRank(cur.Tier) ||
			(picklog.TierRank(e.Tier) == picklog.TierRank(cur.Tier) && edgeOf(e) > edgeOf(cur)) {
			best[k] = e
		}
	}
	byGame := map[int]*Game{}
	var ids []int
	b := Board{Date: date}
	for _, k := range order {
		e := best[k]
		g := byGame[e.GameID]
		if g == nil {
			g = &Game{ID: e.GameID, Matchup: matchupOf(e)}
			byGame[e.GameID] = g
			ids = append(ids, e.GameID)
		}
		t := normTier(e.Tier)
		p := Play{
			Player: e.Player, Team: e.Team, Stat: e.Stat, Side: e.Side, Line: e.Line, Prob: e.Prob,
			Edge: edgeOf(e), Grade: grade(e.Prob, t), Tier: t, Odds: e.Odds, Book: e.Book,
			ProjStat: e.ProjMean, ProjMin: e.ProjMin, Units: e.Units, Why: e.Why,
		}
		if form != nil {
			games, sAvg, sGP := form(e.PlayerID, e.Stat, date)
			p.Last5, p.SeasonAvg, p.SeasonGP = games, sAvg, sGP
			var sum, mins float64
			for _, x := range games {
				sum += x.Value
				mins += x.Minutes
				if (e.Side == "OVER" && x.Value > e.Line) || (e.Side == "UNDER" && x.Value < e.Line) {
					p.L5Hits++
				}
			}
			if n := float64(len(games)); n > 0 {
				p.L5Avg, p.MinAvg5 = sum/n, mins/n
			}
		}
		if n := count[k]; n > 0 {
			p.Runs, p.SeenIn, p.Multi = n, seenIn[k], n >= 2
		}
		p.Label = label(t, e.Prob)
		p.Badges = badgesFor(p)
		g.Plays = append(g.Plays, p)
		b.Plays++
		if t == "PICK" && e.Units > 0 {
			b.Staked++
		}
	}
	for _, r := range day {
		rs := RunSummary{Label: r.Label, Clock: r.Clock, Prev: r.Prev, Plays: len(r.Plays)}
		for _, d := range r.Dropped {
			rs.Dropped = append(rs.Dropped, DroppedPlay{Player: d.Player, Team: d.Team, Matchup: d.Matchup, Stat: d.Stat,
				Side: d.Side, Line: d.Line, Tier: normTier(d.Tier), Prob: d.Prob, Reason: d.Reason})
		}
		for _, c := range r.Changed {
			rs.Changed = append(rs.Changed, TierMove{Player: c.Player, Stat: c.Stat, Side: c.Side, From: normTier(c.From), To: normTier(c.Tier)})
		}
		b.RunsToday = append(b.RunsToday, rs)
	}
	for i := range b.RunsToday {
		// New plays: on this run's board but not the previous run's.
		if i == 0 {
			b.RunsToday[i].New = b.RunsToday[i].Plays
			continue
		}
		_, added, _ := runlog.Diff(day[i-1].Plays, day[i].Plays)
		b.RunsToday[i].New = len(added)
	}
	sort.Ints(ids)
	for _, id := range ids {
		g := byGame[id]
		sort.SliceStable(g.Plays, func(i, j int) bool {
			a, c := g.Plays[i], g.Plays[j]
			if picklog.TierRank(a.Tier) != picklog.TierRank(c.Tier) {
				return picklog.TierRank(a.Tier) < picklog.TierRank(c.Tier)
			}
			return a.Prob > c.Prob
		})
		b.Games = append(b.Games, *g)
	}
	return b
}

func row(name string, rs []picklog.Settled) Row {
	s := picklog.Summarize(rs)
	r := Row{Name: name, Picks: s.Picks - s.Voids, Wins: s.Wins, Losses: s.Losses, Pushes: s.Pushes,
		HitRate: s.HitRate(), Predicted: s.AvgProb, CLVN: s.CLVN, AvgCLV: s.AvgCLV}
	// Staked picks report their real profit; everything else is a flat 1u comparison.
	if len(rs) > 0 && rs[0].Units > 0 {
		r.Units = s.Profit
	} else {
		r.Units = s.Flat
	}
	return r
}

// buildRecord summarizes settled plays. The same play at several books counts once.
func buildRecord(settled []picklog.Settled, recentN int) Record {
	every, dups := picklog.DedupeSettled(settled)
	rec := Record{Dups: dups}
	if len(every) == 0 {
		return rec
	}
	days := map[string]bool{}
	for _, r := range every {
		days[r.Date] = true
		if rec.Since == "" || r.Date < rec.Since {
			rec.Since = r.Date
		}
		if r.Date > rec.Until {
			rec.Until = r.Date
		}
	}
	rec.Days = len(days)

	staked := func(r picklog.Settled) bool { return r.Tier == "" || r.Tier == "PICK" }
	var st []picklog.Settled
	byTier := map[string][]picklog.Settled{}
	for _, r := range every {
		if staked(r) {
			st = append(st, r)
		} else {
			byTier[r.Tier] = append(byTier[r.Tier], r)
		}
	}
	// The staked-pick row reports real profit at the logged stake, even before any pick has
	// units on it (an empty slice means no row at all).
	add := func(name string, rs []picklog.Settled, stake bool) {
		if len(rs) == 0 {
			return
		}
		r := row(name, rs)
		if stake {
			r.Units = picklog.Summarize(rs).Profit
		} else {
			r.Units = picklog.Summarize(rs).Flat
		}
		rec.Tiers = append(rec.Tiers, r)
	}
	add("Max plays (staked)", st, true)
	add("Not staked: unproven stat or early season", byTier["PAPER"], false)
	add("Not staked: needed a check", byTier["CHECK"], false)
	add("Not staked: fill-in leans", byTier["LEAN"], false)
	add("Everything tracked", every, false)

	for _, stat := range []string{"points", "rebounds", "assists", "threes"} {
		var sub []picklog.Settled
		for _, r := range every {
			if r.Stat == stat {
				sub = append(sub, r)
			}
		}
		if len(sub) > 0 {
			x := row(stat, sub)
			x.Units = picklog.Summarize(sub).Flat
			rec.Stats = append(rec.Stats, x)
		}
	}
	for _, side := range []string{"OVER", "UNDER"} {
		var sub []picklog.Settled
		for _, r := range every {
			if r.Side == side {
				sub = append(sub, r)
			}
		}
		if len(sub) > 0 {
			x := row(side, sub)
			x.Units = picklog.Summarize(sub).Flat
			rec.Sides = append(rec.Sides, x)
		}
	}
	for _, b := range []struct {
		label  string
		lo, hi float64
	}{{"50-55%", 0, 0.55}, {"55-58%", 0.55, 0.58}, {"58-62%", 0.58, 0.62}, {"62% and up", 0.62, 2}} {
		var w, l int
		var p float64
		for _, r := range every {
			if r.Prob < b.lo || r.Prob >= b.hi || (r.Result != "WIN" && r.Result != "LOSS") {
				continue
			}
			if r.Result == "WIN" {
				w++
			} else {
				l++
			}
			p += r.Prob
		}
		if n := w + l; n > 0 {
			rec.Buckets = append(rec.Buckets, Bucket{Label: b.label, Picks: n, Predicted: p / float64(n), Actual: float64(w) / float64(n)})
		}
	}

	byDay := map[string]*Day{}
	var dayKeys []string
	for _, r := range every {
		d := byDay[r.Date]
		if d == nil {
			d = &Day{Date: r.Date}
			byDay[r.Date] = d
			dayKeys = append(dayKeys, r.Date)
		}
		switch r.Result {
		case "WIN":
			d.Wins++
		case "LOSS":
			d.Losses++
		}
		d.FlatUnits += r.Flat
		if staked(r) {
			d.StakedUnits += r.Profit
			switch r.Result {
			case "WIN":
				d.StakedW++
			case "LOSS":
				d.StakedL++
			}
		}
	}
	sort.Strings(dayKeys)
	var runStaked, runFlat float64
	for _, k := range dayKeys {
		d := *byDay[k]
		runStaked += d.StakedUnits
		runFlat += d.FlatUnits
		d.StakedUnits, d.FlatUnits = runStaked, runFlat
		rec.Daily = append(rec.Daily, d)
	}

	sorted := append([]picklog.Settled(nil), every...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i], sorted[j]
		if a.Date != c.Date {
			return a.Date > c.Date
		}
		if picklog.TierRank(a.Tier) != picklog.TierRank(c.Tier) {
			return picklog.TierRank(a.Tier) < picklog.TierRank(c.Tier)
		}
		return a.Prob > c.Prob
	})
	if len(sorted) > recentN {
		sorted = sorted[:recentN]
	}
	for _, r := range sorted {
		t := normTier(r.Tier)
		actual := r.Actual
		if r.Result == "VOID" {
			actual = math.NaN()
		}
		res := Result{Date: r.Date, Player: r.Player, Team: r.Team, Matchup: matchupOf(r.Entry), Stat: r.Stat, Side: r.Side,
			Line: r.Line, Result: r.Result, Tier: t, Prob: r.Prob, Grade: grade(r.Prob, t), Label: label(t, r.Prob)}
		if !math.IsNaN(actual) {
			res.Actual = actual
		}
		rec.Recent = append(rec.Recent, res)
	}
	return rec
}

func latestDate(entries []picklog.Entry) string {
	d := ""
	for _, e := range entries {
		if e.Date > d {
			d = e.Date
		}
	}
	return d
}
