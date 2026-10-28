package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
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
	ProbRaw  float64 `json:"probRaw"`
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

	// Defense is where the opponent ranks against his position in this stat, read from the model's note.
	Defense *DefenseInfo `json:"defense,omitempty"`

	// Case is the full argument for the play: the numbers, what supports it, what could go wrong
	// and how much the model's record on this kind of play deserves trust.
	Case *Case `json:"case,omitempty"`

	PlayerID int `json:"-"`
	// Dist is the model's probability of finishing with 0, 1, 2... of the stat (the outcome curve).
	Dist []float64 `json:"dist,omitempty"`
	// Moves is how the market's median line moved through the day.
	Moves []MovePt `json:"moves,omitempty"`

	Label  string  `json:"label"`  // Max play, Great play, Risky play or Lean
	Badges []Badge `json:"badges"` // quick-read signals: weak defense, hot streak, back-to-back and so on

	Last5     []Game5 `json:"last5,omitempty"`
	L5Avg     float64 `json:"l5Avg"`
	L5Hits    int     `json:"l5Hits"`
	SeasonAvg float64 `json:"seasonAvg"`
	SeasonGP  int     `json:"seasonGames"`
	MinAvg5   float64 `json:"minAvg5"`
	MinTyp    float64 `json:"minTypical"` // median minutes of the last five, cut-short nights left out
}

// DefenseInfo is the opponent's defense against the player's position, for example "3rd-most
// rebounds allowed to centers, +9%". Helps is true when that is good news for this pick.
type DefenseInfo struct {
	Text  string  `json:"text"`
	Rank  int     `json:"rank"` // 1 = allows the most, counted from the nearer end
	Of    int     `json:"of"`
	Kind  string  `json:"kind"` // most or fewest
	Pos   string  `json:"pos"`
	Pct   float64 `json:"pct"`
	Early bool    `json:"early,omitempty"`
	Helps bool    `json:"helps"`
}

var defenseRe = regexp.MustCompile(`defense vs ([a-z ]+): (\d+)(?:st|nd|rd|th)-(most|fewest) ([a-z0-9-]+) allowed \(([+-]\d+)%, of (\d+) teams(, early season)?\)`)

// parseDefense reads the defense note out of the model's reasoning, or returns nil.
func parseDefense(why, side string) *DefenseInfo {
	m := defenseRe.FindStringSubmatch(why)
	if m == nil {
		return nil
	}
	rank, _ := strconv.Atoi(m[2])
	pct, _ := strconv.Atoi(m[5])
	of, _ := strconv.Atoi(m[6])
	// A matchup within 5% of average is no signal: "15th-fewest (-1%)" is the middle of the league, and
	// colouring it as a help or a hurt reads as a reason when there is none.
	if pct > -5 && pct < 5 {
		return nil
	}
	d := &DefenseInfo{Rank: rank, Of: of, Kind: m[3], Pos: m[1], Pct: float64(pct), Early: m[7] != ""}
	d.Helps = (side == "OVER") == (d.Kind == "most")
	d.Text = fmt.Sprintf("Allows the %s %s to %s (%+d%%)", ordinalOf(rank, m[3]), m[4], m[1], pct)
	return d
}

func ordinalOf(n int, kind string) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s-%s", n, suffix, kind)
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
	// Short marks a night well under his normal minutes (an injury exit, foul trouble, a blowout).
	// It is left out of the typical-minutes figure so one cut-short game does not look like a role change.
	Short bool `json:"short,omitempty"`
	// Prior marks a game from last season (for example a playoff game in May): shown, but it is not
	// part of this season's form.
	Prior bool `json:"prior,omitempty"`
}

// Form looks up a player's recent games for a stat before a date, newest last. Nil is allowed.
type Form func(playerID int, stat, before string) (games []Game5, seasonAvg float64, seasonGames int)

// Game groups one matchup's plays.
type Game struct {
	ID      int    `json:"id"`
	Matchup string `json:"matchup"`
	Tip     string `json:"tip,omitempty"` // scheduled tip-off, RFC3339 UTC
	Plays   []Play `json:"plays"`
	// More is the next-best lines in the game that did not make the board, behind the "More plays" button.
	More []Play `json:"more,omitempty"`
	// Ctx is the game's betting market read: spread, total, expected scores and what they mean for minutes.
	Ctx *GameContext `json:"ctx,omitempty"`
	// News is who is out or uncertain on each side.
	News []NewsItem `json:"news,omitempty"`
}

// Board is one day's plays.
type Board struct {
	Date   string `json:"date"`
	Staked int    `json:"staked"`
	Plays  int    `json:"plays"`
	Games  []Game `json:"games"`
	// Overs is the day's Overs closest to qualifying, and Overs at off-market lines. Not picks.
	Overs []OverWatch `json:"overs,omitempty"`
	// Combos is the day's combined-market plays (PRA, PR, PA, RA). Tracked, not staked, not picks.
	Combos    []ComboPlay `json:"combos,omitempty"`
	ComboInfo *ComboInfo  `json:"comboInfo,omitempty"`

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
	// Now is where the player and stat stand on the run that dropped the play.
	Now *runlog.Now `json:"now,omitempty"`
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
	// Proj is what the model projected for the stat, shown next to what he finished with.
	Proj float64 `json:"proj,omitempty"`
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
	GeneratedAt string      `json:"generatedAt"`
	Board       Board       `json:"board"`
	Record      Record      `json:"record"`
	Combos      ComboRecord `json:"combos"`
	Lines       LinesRecord `json:"lines"`
	// V2 is the challenger model's board and record, tracked apart from everything above.
	V2 *V2Site `json:"v2,omitempty"`
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
	case "STRONG":
		return "Strong play"
	case "CHECK":
		return "Risky play"
	case "PAPER":
		if prob >= 0.58 {
			return "Great play"
		}
	case "COMBO":
		return "Tracking"
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

	if d := p.Defense; d != nil {
		// Only the ends of the ranking get a badge; the text line shows the rest.
		if d.Rank <= 8 {
			if d.Kind == "most" {
				add("weak-defense", d.Text, over)
			} else {
				add("tough-defense", d.Text, !over)
			}
		}
	} else if strings.Contains(why, "soft defense") { // picks logged before defense ranks existed
		add("weak-defense", "Weak defense against his position", over)
	} else if strings.Contains(why, "tough defense") {
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
	if len(p.Last5) >= 4 && minutesRef(p) > 0 {
		ref := minutesRef(p)
		d := p.ProjMin - ref
		if d >= 3 {
			add("minutes-up", fmt.Sprintf("Minutes up: projected %.0f vs his normal %.0f", p.ProjMin, ref), over)
		} else if d <= -3 {
			add("minutes-down", fmt.Sprintf("Minutes down: projected %.0f vs his normal %.0f", p.ProjMin, ref), !over)
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

// makePlay turns a logged entry into a board play: grade, recent games, defense, label and badges.
// runs and seenIn say how many of today's runs had it (0 for none).
func makePlay(e picklog.Entry, date string, form Form, runs int, seenIn []string) Play {
	t := normTier(e.Tier)
	p := Play{
		Player: e.Player, Team: e.Team, Stat: e.Stat, Side: e.Side, Line: e.Line, Prob: e.Prob,
		ProbRaw: e.ProbRaw, Edge: edgeOf(e), Grade: grade(e.Prob, t), Tier: t, Odds: e.Odds, Book: e.Book,
		ProjStat: e.ProjMean, ProjMin: e.ProjMin, Units: e.Units, Why: e.Why, PlayerID: e.PlayerID, Dist: e.Dist,
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
		p.MinTyp = markShort(p.Last5)
	}
	if runs > 0 {
		p.Runs, p.SeenIn, p.Multi = runs, seenIn, runs >= 2
	}
	p.Defense = parseDefense(e.Why, e.Side)
	p.Label = label(t, e.Prob)
	p.Badges = badgesFor(p)
	return p
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
			g = &Game{ID: e.GameID, Matchup: matchupOf(e), Tip: e.Tip, Plays: []Play{}}
			byGame[e.GameID] = g
			ids = append(ids, e.GameID)
		}
		if g.Tip == "" {
			g.Tip = e.Tip
		}
		t := normTier(e.Tier)
		p := makePlay(e, date, form, count[k], seenIn[k])
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
				Side: d.Side, Line: d.Line, Tier: normTier(d.Tier), Prob: d.Prob, Reason: d.Reason, Now: d.Now})
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
	// Earliest tip-off first; games without a time keep game-id order after the timed ones.
	sort.SliceStable(ids, func(i, j int) bool {
		a, c := byGame[ids[i]].Tip, byGame[ids[j]].Tip
		if (a == "") != (c == "") {
			return a != ""
		}
		return a < c
	})
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
	add("V2 strong plays (the real model agrees, stat and side proven)", byTier["STRONG"], false)
	add("V2 lean plays (posted, not yet backed by the real model or a proven record)", byTier["V2LEAN"], false)
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
			Line: r.Line, Result: r.Result, Tier: t, Prob: r.Prob, Grade: grade(r.Prob, t), Label: label(t, r.Prob), Proj: r.ProjMean}
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

// minutesRef is the minutes figure a projection is compared with: his typical minutes when known.
func minutesRef(p Play) float64 {
	if p.MinTyp > 0 {
		return p.MinTyp
	}
	return p.MinAvg5
}

// markShort flags games well under his normal minutes and returns the typical minutes of the rest.
// Normal is the median of the five games. With two or more short nights it is a lasting change
// (a benching, a new role), so nothing is flagged and the median speaks for itself.
func markShort(gs []Game5) float64 {
	if len(gs) == 0 {
		return 0
	}
	med := func(xs []float64) float64 {
		c := append([]float64(nil), xs...)
		sort.Float64s(c)
		n := len(c)
		if n%2 == 0 {
			return (c[n/2-1] + c[n/2]) / 2
		}
		return c[n/2]
	}
	all := make([]float64, len(gs))
	for i, g := range gs {
		all[i] = g.Minutes
	}
	normal := med(all)
	if len(gs) >= 4 && normal >= 15 {
		var idx []int
		for i, g := range gs {
			if g.Minutes < 0.55*normal {
				idx = append(idx, i)
			}
		}
		if len(idx) == 1 {
			gs[idx[0]].Short = true
		}
	}
	var keep []float64
	for _, g := range gs {
		if !g.Short {
			keep = append(keep, g.Minutes)
		}
	}
	return med(keep)
}
