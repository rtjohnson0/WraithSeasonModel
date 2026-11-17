package main

import (
	"propsmodel/internal/picklog"
	"testing"
)

func TestOverComparisonsUseExactQuoteAndFullAnalytics(t *testing.T) {
	e := picklog.Entry{Date: "2026-11-16", GameID: 9, PlayerID: 1, Player: "A", Team: "CHI", Matchup: "CHI @ MIL", Stat: "points", Side: "OVER", Line: 12.5, Odds: 110, Prob: .54, ProjMean: 14, ProjMin: 28, Period: "game", AuditVersion: 2, Tip: "2026-11-17T00:00:00Z", LoggedAt: "2026-11-16T14:00:00Z", QuoteAt: "2026-11-16T13:59:00Z", Dist: []float64{.1, .2, .7}}
	form := func(int, string, string) ([]Game5, float64, int) { return []Game5{{Value: 15, Minutes: 30}}, 14, 4 }
	b := Board{Date: e.Date, Overs: []OverWatch{{Forecast: &e, Line: 99, Prob: .99}}}
	enrichOvers(&b, form)
	if len(b.Overs) != 1 {
		t.Fatal("positive comparison missing")
	}
	w := b.Overs[0]
	if w.Line != 12.5 || w.Analysis == nil || len(w.Analysis.Last5) != 1 || len(w.Analysis.Dist) != 3 || w.Analysis.QuoteAt != e.QuoteAt || !w.Analysis.Research {
		t.Fatalf("incomplete/wrong analytics: %+v", w)
	}
	b.Games = []Game{{ID: 9, Plays: []Play{{PlayerID: 1, Stat: "points", Side: "UNDER", Line: 18.5}}}}
	b.Overs = []OverWatch{{Forecast: &e}}
	enrichOvers(&b, form)
	if len(b.Overs) != 0 {
		t.Fatal("comparison conflicts with selected opposite side")
	}
	bad := e
	bad.Prob = .4
	b = Board{Date: e.Date, Overs: []OverWatch{{Forecast: &bad}, {Player: "legacy partial", Prob: .9}}}
	enrichOvers(&b, form)
	if len(b.Overs) != 0 {
		t.Fatal("negative-edge or partial row presented as comparison")
	}
}
