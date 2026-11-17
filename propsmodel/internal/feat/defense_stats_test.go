package feat

import (
	"math"
	"propsmodel/internal/data"
	"testing"
)

func TestAllModeledStatsReachOpponentAndPositionFeatures(t *testing.T) {
	games := miniLeagueN(100)
	for i := range games {
		games[i].Pos = "F"
		games[i].Fg3m = 2
		if games[i].TeamID == 2 {
			games[i].Fg3m = 3
		}
	}
	b := NewBuilder(games, nil)
	three := data.StatIndex(data.Threes)
	for _, g := range b.team[1] {
		if g.For[three] != 10 || g.Against[three] != 15 || g.PosAgainst[2][three] != 15 {
			t.Fatalf("nonzero made threes lost in team/position totals: %+v", g)
		}
	}
	r := b.Live(21, 2, 1, false, "2026-06-01", nil)
	if r == nil {
		t.Fatal("missing live row")
	}
	for _, name := range []string{"opp_allow", "pos5_allow"} {
		v := r.S[three][statIdx[three][name]]
		if math.IsNaN(v) || v <= 0 {
			t.Fatalf("generous three-point opponent must produce a positive %s ratio, got %v", name, v)
		}
	}
	if rank, ok := b.DefenseRank(21, 1, data.Threes, "2026-06-01"); !ok || rank.Rank != 1 || rank.Pct <= 0 {
		t.Fatalf("missing/incorrect threes defense rank: %+v %v", rank, ok)
	}
}
