package main

import (
	"propsmodel/internal/data"
	"testing"
)

func TestDirectionsPreserveSelectedTierAndSeparateGames(t *testing.T) {
	a := scored{GameID: 1, PlayerID: 2, Stat: data.Points, Side: "UNDER", Tier: "PICK", Prob: .6, Odds: -110}
	b := a
	b.Side, b.Tier, b.Prob = "OVER", "LEAN", .9
	c := b
	c.GameID = 3
	got := oneDirection([]scored{b, a, c})
	if len(got) != 2 || got[0].Side != "UNDER" || got[1].GameID != 3 {
		t.Fatalf("conflicting selections: %+v", got)
	}
}
