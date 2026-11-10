package main

import (
	"propsmodel/internal/team"
	"testing"
)

func TestBoardPricesBelongToExactBookAndThreshold(t *testing.T) {
	rows := []team.Odds{}
	for i, b := range []string{"draftkings", "fanduel", "betmgm"} {
		rows = append(rows, team.Odds{Vendor: b, HasTotal: true, Total: 220.5 + float64(i), Over: -110, Under: -110, MLHome: -150, MLAway: 130, CapturedAt: "capture", Stamp: "update"})
	}
	g := makeGameBoard(1, "BOS", "NYK", "tip", 3, 221, 15, 12, rows)
	if len(g.Quotes) != 6 || g.HomeScore+g.AwayScore != g.Total {
		t.Fatalf("bad game: %+v", g)
	}
	q0, q1 := g.Quotes[0], g.Quotes[2]
	if q0.Book != "draftkings" || q0.Line != 220.5 || q1.Line != 221.5 || q0.Sides[0].Conditional <= q1.Sides[0].Conditional {
		t.Fatal("total probability detached from exact quote")
	}
	if q0.QuoteAt != "capture" || q0.SourceUpdatedAt != "update" {
		t.Fatal("missing quote times")
	}
	thin := makeGameBoard(1, "BOS", "NYK", "tip", 3, 221, 15, 12, rows[:2])
	if len(thin.Quotes) != 0 {
		t.Fatal("thin market published")
	}
}
