package main

import (
	"fmt"
	"math"
)

// GameContext is a game's betting market in plain terms: who is favoured by how much, the total,
// the score each team is expected to put up, and what that means for minutes and scoring chances.
type GameContext struct {
	Home       string   `json:"home"`
	Visitor    string   `json:"visitor"`
	Favorite   string   `json:"favorite"` // abbreviation, empty for a pick'em
	Spread     float64  `json:"spread"`   // size of the favourite's margin (always positive)
	Total      float64  `json:"total"`
	HomePts    float64  `json:"homePts"` // expected score from the spread and total
	VisitorPts float64  `json:"visitorPts"`
	SlateAvg   float64  `json:"slateAvg"`  // average total across today's games
	Closeness  string   `json:"closeness"` // tight, competitive, lopsided or blowout
	Pace       string   `json:"pace"`      // fast, slow or middle
	Reads      []string `json:"reads"`
}

type ctxIn struct {
	GameID     int     `json:"game_id"`
	Home       string  `json:"home"`
	Visitor    string  `json:"visitor"`
	SpreadHome float64 `json:"spread_home"`
	Total      float64 `json:"total"`
}

func closenessOf(spread float64) string {
	switch {
	case spread <= 3.5:
		return "tight"
	case spread <= 7:
		return "competitive"
	case spread < 10:
		return "lopsided"
	}
	return "blowout"
}

// buildContext turns one game's spread and total into the reads shown on the board. slateAvg is the
// average total of the day's games, the yardstick for "fast" and "slow".
func buildContext(c ctxIn, slateAvg float64) *GameContext {
	g := &GameContext{Home: c.Home, Visitor: c.Visitor, Total: c.Total, SlateAvg: slateAvg}
	g.Spread = math.Abs(c.SpreadHome)
	switch {
	case c.SpreadHome < 0:
		g.Favorite = c.Home
	case c.SpreadHome > 0:
		g.Favorite = c.Visitor
	}
	g.HomePts = (c.Total - c.SpreadHome) / 2
	g.VisitorPts = (c.Total + c.SpreadHome) / 2
	g.Closeness = closenessOf(g.Spread)
	switch d := c.Total - slateAvg; {
	case d >= 4:
		g.Pace = "fast"
	case d <= -4:
		g.Pace = "slow"
	default:
		g.Pace = "middle"
	}
	favTxt := "a pick'em"
	if g.Favorite != "" {
		favTxt = fmt.Sprintf("%s favoured by %.1f", g.Favorite, g.Spread)
	}
	switch g.Closeness {
	case "tight":
		g.Reads = append(g.Reads, fmt.Sprintf("Close game expected (%s). Starters rarely sit early in tight games, so minutes should hold or run long.", favTxt))
	case "competitive":
		g.Reads = append(g.Reads, fmt.Sprintf("Competitive game (%s). Minutes should be close to normal.", favTxt))
	case "lopsided":
		g.Reads = append(g.Reads, fmt.Sprintf("Lopsided game (%s). Some blowout risk: starters on the favoured side may lose late minutes.", favTxt))
	default:
		g.Reads = append(g.Reads, fmt.Sprintf("Blowout risk (%s). Starters could sit the fourth quarter, which hurts overs on counting stats and helps bench minutes.", favTxt))
	}
	diff := c.Total - slateAvg
	switch g.Pace {
	case "fast":
		g.Reads = append(g.Reads, fmt.Sprintf("High total (%.1f, %.1f above today's average %.1f): more possessions and shots to go around.", c.Total, diff, slateAvg))
	case "slow":
		g.Reads = append(g.Reads, fmt.Sprintf("Low total (%.1f, %.1f below today's average %.1f): fewer possessions, so fewer chances to score.", c.Total, -diff, slateAvg))
	default:
		g.Reads = append(g.Reads, fmt.Sprintf("Total %.1f is near today's average (%.1f).", c.Total, slateAvg))
	}
	return g
}

// attachContext adds each game's market read from data/more.json. Games without a market keep none.
func attachContext(b *Board, mf moreFile) {
	if mf.Date != b.Date || len(mf.Games) == 0 {
		return
	}
	var sum float64
	for _, c := range mf.Games {
		sum += c.Total
	}
	avg := sum / float64(len(mf.Games))
	byID := map[int]ctxIn{}
	for _, c := range mf.Games {
		byID[c.GameID] = c
	}
	for i := range b.Games {
		if c, ok := byID[b.Games[i].ID]; ok {
			b.Games[i].Ctx = buildContext(c, avg)
		}
	}
}

// contextNotes are the points a game's market adds for or against one play.
func contextNotes(p Play, g *GameContext) (forNotes, againstNotes []string) {
	if g == nil {
		return
	}
	over := p.Side == "OVER"
	starter := p.ProjMin >= 26
	onFavorite := g.Favorite != "" && g.Favorite == p.Team
	scoring := p.Stat == "points" || p.Stat == "threes" || p.Stat == "assists"
	add := func(supports bool, s string) {
		if supports {
			forNotes = append(forNotes, s)
		} else {
			againstNotes = append(againstNotes, s)
		}
	}
	switch g.Pace {
	case "fast":
		if scoring || p.Stat == "rebounds" {
			add(over, fmt.Sprintf("The game total is %.1f, above today's average (%.1f): more possessions and misses to go around.", g.Total, g.SlateAvg))
		}
	case "slow":
		if scoring || p.Stat == "rebounds" {
			add(!over, fmt.Sprintf("The game total is %.1f, below today's average (%.1f): fewer possessions.", g.Total, g.SlateAvg))
		}
	}
	switch g.Closeness {
	case "tight":
		if starter {
			add(over, fmt.Sprintf("Tight game (spread %.1f): starters tend to keep their minutes.", g.Spread))
		}
	case "lopsided", "blowout":
		if starter {
			who := "starters"
			if onFavorite {
				who = fmt.Sprintf("starters on the favoured %s", g.Favorite)
			}
			add(!over, fmt.Sprintf("Spread of %.1f raises blowout risk: %s may lose fourth-quarter minutes.", g.Spread, who))
		}
	}
	return
}
