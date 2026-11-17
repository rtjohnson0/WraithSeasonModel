package main

import (
	"math"
	"propsmodel/internal/bets"
)

// enrichOvers reuses the normal prop analytics. Legacy partial watch records are
// not reconstructed from a different quote or passed off as recommendations.
func enrichOvers(b *Board, form Form) {
	var out []OverWatch
	for _, w := range b.Overs {
		e := w.Forecast
		if e == nil || e.GameID == 0 || e.PlayerID == 0 || !e.FullGame() || !e.Verified() ||
			e.Side != "OVER" || e.ProjMin <= 0 || e.Tip == "" || e.QuoteAt == "" ||
			e.Prob < .5 || math.IsNaN(e.Prob) || math.IsInf(e.Prob, 0) || bets.Edge(e.Prob, e.Odds) <= 0 {
			continue
		}
		blocked := false
		for _, g := range b.Games {
			if g.ID != e.GameID {
				continue
			}
			for _, p := range append(append([]Play{}, g.Plays...), g.More...) {
				if p.PlayerID == e.PlayerID && p.Stat == e.Stat {
					blocked = true
				}
			}
		}
		if blocked {
			continue
		}
		e.Date, e.Period, e.Tier, e.Units = b.Date, "game", "OVERWATCH", 0
		p := makePlay(*e, b.Date, form, 0, nil)
		if w.Experimental {
			p.ProbabilityBasis = "estimate"
		}
		p.Label, p.Grade, p.Research = "Research comparison", "—", true
		w.GameID, w.PlayerID, w.Player, w.Team, w.Matchup = e.GameID, e.PlayerID, e.Player, e.Team, e.Matchup
		w.Stat, w.Line, w.Odds, w.Book, w.Prob, w.Edge = e.Stat, e.Line, e.Odds, e.Book, e.Prob, bets.Edge(e.Prob, e.Odds)
		w.Proj, w.ProjMin, w.Tip, w.Analysis = e.ProjMean, e.ProjMin, e.Tip, &p
		w.Forecast = nil // The public payload uses the shared Play schema.
		out = append(out, w)
	}
	b.Overs = out
}
