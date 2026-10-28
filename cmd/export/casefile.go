package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"propsmodel/internal/picklog"
)

// Case is the full argument for one play, built from numbers the model already produced and the
// player's own recent games. Nothing here is a new prediction: it lays the evidence out so a
// reader can check the pick instead of taking it on faith.
type Case struct {
	// The arithmetic: minutes times his per-minute rate is the projection, compared with the line.
	Minutes   float64 `json:"minutes"`
	PerMin    float64 `json:"perMin"`
	Projected float64 `json:"projected"`
	Line      float64 `json:"line"`
	Gap       float64 `json:"gap"`    // in the pick's direction: positive means the projection is on our side
	GapPct    float64 `json:"gapPct"` // gap as a share of the line

	// Three probabilities that answer "why this much confidence".
	ModelProb float64 `json:"modelProb"` // the model alone
	BookProb  float64 `json:"bookProb"`  // what the offered odds imply (break-even)
	FinalProb float64 `json:"finalProb"` // the blend that is shown as the play's percentage
	Edge      float64 `json:"edge"`

	For     []string `json:"for"`
	Against []string `json:"against"`

	// Track is the model's live record on this kind of play, and Trust says what to make of it.
	Track []string `json:"track,omitempty"`
	Trust string   `json:"trust"`
}

// breakEven is the win probability at which American odds pay nothing.
func breakEven(odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	if odds < 0 {
		return float64(-odds) / float64(-odds+100)
	}
	return 100 / float64(odds+100)
}

// trackRecords keys the live record by "stat" and "stat|SIDE", counting each graded play once.
func trackRecords(settled []picklog.Settled) map[string]Row {
	every, _ := picklog.DedupeSettled(settled)
	groups := map[string][]picklog.Settled{}
	for _, r := range every {
		groups[r.Stat] = append(groups[r.Stat], r)
		groups[r.Stat+"|"+r.Side] = append(groups[r.Stat+"|"+r.Side], r)
	}
	out := map[string]Row{}
	for k, rs := range groups {
		out[k] = row(k, rs)
	}
	return out
}

func attachCases(b *Board, track map[string]Row) {
	for gi := range b.Games {
		for pi := range b.Games[gi].Plays {
			p := &b.Games[gi].Plays[pi]
			p.Case = buildCase(*p, track, b.Games[gi].Ctx)
		}
		for pi := range b.Games[gi].More {
			p := &b.Games[gi].More[pi]
			p.Case = buildCase(*p, track, b.Games[gi].Ctx)
		}
	}
}

func pct(x float64) string { return fmt.Sprintf("%.0f%%", 100*x) }

func buildCase(p Play, track map[string]Row, ctx *GameContext) *Case {
	over := p.Side == "OVER"
	dir := 1.0
	if !over {
		dir = -1
	}
	c := &Case{Minutes: p.ProjMin, Projected: p.ProjStat, Line: p.Line, ModelProb: p.ProbRaw,
		BookProb: breakEven(p.Odds), FinalProb: p.Prob, Edge: p.Edge}
	if p.ProjMin > 0 {
		c.PerMin = p.ProjStat / p.ProjMin
	}
	c.Gap = dir * (p.ProjStat - p.Line)
	if p.Line > 0 {
		c.GapPct = c.Gap / p.Line
	}
	stat := statWord(p.Stat)
	word := "under"
	if over {
		word = "over"
	}

	// Evidence for and against.
	switch {
	case c.Gap >= 0.5:
		c.For = append(c.For, fmt.Sprintf("The projection (%.1f) sits %.1f %s the line, a %.0f%% gap.", p.ProjStat, c.Gap, relWord(over, true), 100*math.Abs(c.GapPct)))
	case c.Gap <= -0.5:
		c.Against = append(c.Against, fmt.Sprintf("The projection (%.1f) is actually %.1f on the wrong side of the line; the edge comes from the price, not the projection.", p.ProjStat, -c.Gap))
	default:
		c.Against = append(c.Against, fmt.Sprintf("The projection (%.1f) is almost exactly the line, so there is little cushion.", p.ProjStat))
	}
	if n := len(p.Last5); n > 0 {
		hit := fmt.Sprintf("He went %s %.1f in %d of his last %d (average %.1f).", word, p.Line, p.L5Hits, n, p.L5Avg)
		switch {
		case p.L5Hits*2 > n:
			c.For = append(c.For, hit)
		case p.L5Hits*2 < n:
			c.Against = append(c.Against, hit+" Recent form disagrees with the pick.")
		}
	}
	if p.SeasonGP >= 3 {
		d := dir * (p.SeasonAvg - p.Line)
		txt := fmt.Sprintf("His season average is %.1f over %d games, %.1f %s the line.", p.SeasonAvg, p.SeasonGP, math.Abs(d), relWord(over, d >= 0))
		if d >= 0.5 {
			c.For = append(c.For, txt)
		} else if d <= -0.5 {
			c.Against = append(c.Against, txt)
		}
	}
	if p.SeasonGP > 0 && p.SeasonGP < 10 {
		c.Against = append(c.Against, fmt.Sprintf("Only %d games this season, so averages and the defense ranking still lean on last year.", p.SeasonGP))
	}
	normal := p.MinTyp
	if normal == 0 {
		normal = p.MinAvg5
	}
	if normal > 0 && p.ProjMin > 0 {
		d := p.ProjMin - normal
		if math.Abs(d) >= 2.5 {
			txt := fmt.Sprintf("Projected minutes (%.0f) are %.0f %s his normal (%.0f).", p.ProjMin, math.Abs(d), map[bool]string{true: "above", false: "below"}[d > 0], normal)
			if (d > 0) == over {
				c.For = append(c.For, txt)
			} else {
				c.Against = append(c.Against, txt+" If minutes land near normal, this pick loses its edge.")
			}
		}
	}
	if p.Defense != nil {
		txt := p.Defense.Text + fmt.Sprintf(", out of %d teams.", p.Defense.Of)
		if p.Defense.Helps {
			c.For = append(c.For, "Matchup: "+txt)
		} else {
			c.Against = append(c.Against, "Matchup: "+txt)
		}
	}
	cf, ca := contextNotes(p, ctx)
	c.For = append(c.For, cf...)
	c.Against = append(c.Against, ca...)
	for _, note := range strings.Split(p.Why, ";") {
		n := strings.TrimSpace(note)
		l := strings.ToLower(n)
		up := strings.ToUpper(n[:min(1, len(n))]) + n[min(1, len(n)):]
		switch {
		case n == "" || strings.HasPrefix(l, "defense vs ") || strings.HasPrefix(l, "minutes "):
		case strings.HasPrefix(l, "out:"):
			c.add(over && (p.Stat == "points" || p.Stat == "assists" || p.Stat == "rebounds"), up+" (teammates out usually push shots and touches toward the rest of the rotation).")
		case strings.HasPrefix(l, "opponent out:"):
			c.add(over, up+" (their defense is weaker without them).")
		case l == "back-to-back":
			c.add(!over, "Back-to-back: legs and minutes tend to drop.")
		case strings.Contains(l, "in your favor"):
			c.For = append(c.For, "The line is off the market in your favor: "+n+".")
		case strings.Contains(l, "off the market") || strings.Contains(l, "far from projection") || strings.Contains(l, "moved"):
			c.Against = append(c.Against, up+".")
		}
	}
	if d := math.Abs(c.ModelProb - c.BookProb); c.ModelProb > 0 && d >= 0.12 {
		c.Against = append(c.Against, fmt.Sprintf("The model alone says %s but the odds only imply %s. That is a big disagreement with the market, so the final %s leans mostly on the book's price.", pct(c.ModelProb), pct(c.BookProb), pct(c.FinalProb)))
	}
	switch p.Tier {
	case "PAPER":
		c.Against = append(c.Against, fmt.Sprintf("%s has not been proven profitable in backtests, so this play is tracked rather than staked.", stat))
	case "CHECK":
		c.Against = append(c.Against, "The minutes projection needs a human look before trusting this one.")
	case "LEAN":
		c.Against = append(c.Against, "A fill-in play: it did not clear the model's normal bar.")
	}

	// Live track record.
	var trust []string
	describe := func(label string, r Row) {
		if r.Picks == 0 {
			return
		}
		c.Track = append(c.Track, fmt.Sprintf("%s: %d-%d (%s) over %d graded plays.", label, r.Wins, r.Losses, pct(r.HitRate), r.Picks))
	}
	sideRow, sOK := track[p.Stat+"|"+p.Side]
	statRow, tOK := track[p.Stat]
	if tOK {
		describe("All "+stat+" plays", statRow)
	}
	if sOK {
		describe(strings.Title(strings.ToLower(p.Side))+"s on "+stat, sideRow)
	}
	n := 0
	if sOK {
		n = sideRow.Picks
	}
	switch {
	case n < 30:
		trust = append(trust, fmt.Sprintf("Too few graded plays on %s %ss (%d) to say whether the model is good at them yet; treat the percentage as the model's opinion, not a proven hit rate.", stat, word, n))
	case sideRow.HitRate < 0.5:
		trust = append(trust, fmt.Sprintf("Live record on %s %ss is below 50%% (%s over %d plays). Give this one less weight.", stat, word, pct(sideRow.HitRate), n))
	default:
		trust = append(trust, fmt.Sprintf("Live record on %s %ss is %s over %d plays, but a record this size still has a lot of luck in it.", stat, word, pct(sideRow.HitRate), n))
	}
	trust = append(trust, "In backtests a higher percentage did not mean a higher hit rate, so use it to rank plays, not as a promise.")
	c.Trust = strings.Join(trust, " ")
	return c
}

func (c *Case) add(supports bool, text string) {
	if supports {
		c.For = append(c.For, text)
	} else {
		c.Against = append(c.Against, text)
	}
}

func relWord(over, good bool) string {
	if over == good {
		return "above"
	}
	return "below"
}

func statWord(s string) string { return s }

// moreFile mirrors data/more.json, written by predict.
type moreFile struct {
	Date  string              `json:"date"`
	Plays []picklog.Entry     `json:"plays"`
	Games []ctxIn             `json:"games"`
	Moves map[string][]MovePt `json:"moves"`
	News  []newsIn            `json:"news"`
	Overs []OverWatch         `json:"overs"`
	// Combos mirrors the combined-market plays predict tracks (tier COMBO).
	Combos    []picklog.Entry `json:"combos"`
	ComboInfo *ComboInfo      `json:"combo_info"`
}

// ComboInfo mirrors the summary predict writes about the day's combo pricing.
type ComboInfo struct {
	Priced  int     `json:"priced"`
	Over    int     `json:"over"`
	Under   int     `json:"under"`
	Cleared int     `json:"cleared"`
	Tracked int     `json:"tracked"`
	Bar     float64 `json:"bar"`
	Basis   string  `json:"basis"`
	At      string  `json:"at"`
}

// ComboPlay is a combined-market play (points + rebounds + assists and so on) shown in its own site
// section. It is tracked, not staked, and is not part of the pick record.
type ComboPlay struct {
	Play
	Matchup string `json:"matchup"`
	Tip     string `json:"tip,omitempty"`
}

// attachCombos adds the day's combined-market plays to the board.
func attachCombos(b *Board, mf moreFile, form Form) {
	if mf.Date != b.Date {
		return
	}
	b.ComboInfo = mf.ComboInfo
	for _, e := range mf.Combos {
		b.Combos = append(b.Combos, ComboPlay{Play: makePlay(e, mf.Date, form, 0, nil), Matchup: matchupOf(e), Tip: e.Tip})
	}
}

// OverWatch is an Over listed on the site's "Overs watch" section: not a pick.
type OverWatch struct {
	GameID  int     `json:"game_id"`
	Player  string  `json:"player"`
	Team    string  `json:"team"`
	Matchup string  `json:"matchup"`
	Stat    string  `json:"stat"`
	Line    float64 `json:"line"`
	Odds    int     `json:"odds"`
	Book    string  `json:"book"`
	Prob    float64 `json:"prob"`
	Edge    float64 `json:"edge"`
	Proj    float64 `json:"proj"`
	ProjMin float64 `json:"proj_min"`
	Tip     string  `json:"tip,omitempty"`
	LowLine bool    `json:"low_line,omitempty"`
	Median  float64 `json:"median,omitempty"`
	Books   int     `json:"books,omitempty"`
	Status  string  `json:"status"`
}

// MovePt is the market's median line at one capture.
type MovePt struct {
	At   string  `json:"at"`
	Line float64 `json:"line"`
}

type newsIn struct {
	GameID int      `json:"game_id"`
	Team   string   `json:"team"`
	Out    []string `json:"out"`
	Maybe  []string `json:"maybe"`
}

// NewsItem is one team's injury news for a game.
type NewsItem struct {
	Team  string   `json:"team"`
	Out   []string `json:"out,omitempty"`
	Maybe []string `json:"maybe,omitempty"`
}

// attachNews adds line history to the plays and injury news to the games.
func attachNews(b *Board, mf moreFile) {
	if mf.Date != b.Date {
		return
	}
	for gi := range b.Games {
		g := &b.Games[gi]
		for _, list := range [][]Play{g.Plays, g.More} {
			for i := range list {
				list[i].Moves = mf.Moves[fmt.Sprintf("%d|%s", list[i].PlayerID, list[i].Stat)]
			}
		}
		for _, n := range mf.News {
			if n.GameID == g.ID {
				g.News = append(g.News, NewsItem{Team: n.Team, Out: n.Out, Maybe: n.Maybe})
			}
		}
	}
}

// attachMore puts each game's extra plays behind its "More" button. A game with no board plays gets
// created from the extras, so it still shows up.
func attachMore(b *Board, mf moreFile, form Form) {
	if mf.Date != b.Date || len(mf.Plays) == 0 {
		return
	}
	idx := map[int]int{}
	for i, g := range b.Games {
		idx[g.ID] = i
	}
	for _, e := range mf.Plays {
		i, ok := idx[e.GameID]
		if !ok {
			b.Games = append(b.Games, Game{ID: e.GameID, Matchup: matchupOf(e), Tip: e.Tip, Plays: []Play{}})
			i = len(b.Games) - 1
			idx[e.GameID] = i
		}
		g := &b.Games[i]
		if g.Tip == "" {
			g.Tip = e.Tip
		}
		g.More = append(g.More, makePlay(e, mf.Date, form, 0, nil))
	}
	// Earliest tip-off first, as in buildBoard.
	sort.SliceStable(b.Games, func(i, j int) bool {
		a, c := b.Games[i].Tip, b.Games[j].Tip
		if (a == "") != (c == "") {
			return a != ""
		}
		return a < c
	})
}
