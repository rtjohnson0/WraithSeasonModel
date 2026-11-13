package main

import (
	"propsmodel/internal/data"
	"propsmodel/internal/matchup"
	"strings"
)

func enrichMatchups(site *Site, dir string) {
	rows, err := data.LoadStatsDir(dir)
	if err != nil {
		return
	}
	enrich := func(b *Board) {
		ix := matchup.Build(rows, b.Date)
		attach := func(p *Play, match string) {
			// Never describe full-game team stats as quarter defense.
			if p.Period != "" && p.Period != "game" {
				return
			}
			pair := strings.Split(match, " @ ")
			if len(pair) != 2 {
				return
			}
			opp := pair[0]
			if p.Team == pair[0] {
				opp = pair[1]
			} else if p.Team != pair[1] {
				return
			}
			p.MatchupContext = ix.ForPlayer(p.PlayerID, opp)
		}
		for gi := range b.Games {
			g := &b.Games[gi]
			for i := range g.Plays {
				attach(&g.Plays[i], g.Matchup)
			}
			for i := range g.More {
				attach(&g.More[i], g.Matchup)
			}
		}
		for i := range b.Combos {
			attach(&b.Combos[i].Play, b.Combos[i].Matchup)
		}
	}
	if site.GameMarkets != nil {
		ix := matchup.Build(rows, site.GameMarkets.Date)
		for i := range site.GameMarkets.Games {
			g := &site.GameMarkets.Games[i]
			if t, ok := ix.Teams[ix.Abbr[g.Home]]; ok {
				g.HomeContext = &t
			}
			if t, ok := ix.Teams[ix.Abbr[g.Away]]; ok {
				g.AwayContext = &t
			}
		}
	}
	enrich(&site.Board)
	if site.V2 != nil {
		enrich(&site.V2.Board)
	}
}
