package main

import (
	"fmt"
	"propsmodel/internal/boardstate"
	"propsmodel/internal/props"
	"sort"
)

func slateGames(games map[int]props.GameInfo) []boardstate.Game {
	out := make([]boardstate.Game, 0, len(games))
	for _, g := range games {
		match := g.VisitorAbbr + " @ " + g.HomeAbbr
		if g.HomeAbbr == "" || g.VisitorAbbr == "" {
			match = fmt.Sprintf("Game %d", g.ID)
		}
		out = append(out, boardstate.Game{ID: g.ID, Matchup: match, Tip: g.Start})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tip == out[j].Tip {
			return out[i].ID < out[j].ID
		}
		if out[i].Tip == "" {
			return false
		}
		if out[j].Tip == "" {
			return true
		}
		return out[i].Tip < out[j].Tip
	})
	return out
}
