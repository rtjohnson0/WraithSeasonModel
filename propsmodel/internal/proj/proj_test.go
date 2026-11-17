package proj

import (
	"math"
	"testing"

	"propsmodel/internal/data"
)

// steadyPlayer returns n games for one player, one per day, with fixed minutes and stats.
func steadyPlayer(id, n int, mins, pts, reb, ast float64) []data.PlayerGame {
	var out []data.PlayerGame
	for i := 0; i < n; i++ {
		out = append(out, data.PlayerGame{
			PlayerID: id, Name: "P", GameID: 1000 + i, Date: day(i), Season: 2025,
			Minutes: mins, Pts: pts, Reb: reb, Ast: ast,
		})
	}
	return out
}

func day(i int) string {
	// 2025-11-01 plus i days, kept simple: 28 days per month is enough for tests.
	m := 11 + i/28
	d := 1 + i%28
	y := 2025
	if m > 12 {
		m -= 12
		y++
	}
	return data.NormalizeDate(sprintfDate(y, m, d))
}

func sprintfDate(y, m, d int) string {
	return string([]byte{
		byte('0' + y/1000), byte('0' + y/100%10), byte('0' + y/10%10), byte('0' + y%10), '-',
		byte('0' + m/10), byte('0' + m%10), '-',
		byte('0' + d/10), byte('0' + d%10),
	})
}

func TestProjectUsesOnlyPriorGames(t *testing.T) {
	games := steadyPlayer(1, 30, 30, 20, 5, 4)
	// A huge game ON the as-of date must be ignored.
	asOf := day(30)
	games = append(games, data.PlayerGame{PlayerID: 1, GameID: 5000, Date: asOf, Season: 2025, Minutes: 40, Pts: 90, Reb: 30, Ast: 30})
	ix := NewIndex(games)

	p, ok := ix.Project(1, asOf)
	if !ok {
		t.Fatal("expected a projection")
	}
	// 30 minutes at 20 points per 30 minutes = 20 points. League rate is the same player,
	// so shrinkage barely moves it; the 90-point game must have no effect.
	if math.Abs(p.Mean[data.Points]-20) > 1.0 {
		t.Errorf("projected points %v, want about 20 (future game must not leak in)", p.Mean[data.Points])
	}
	if math.Abs(p.ProjMinutes-30) > 0.01 {
		t.Errorf("projected minutes %v, want 30", p.ProjMinutes)
	}
}

func TestNotEnoughHistory(t *testing.T) {
	ix := NewIndex(steadyPlayer(1, 5, 30, 20, 5, 4))
	if _, ok := ix.Project(1, day(10)); ok {
		t.Error("a player with 5 prior games should not be projected")
	}
	if _, ok := ix.Project(999, day(10)); ok {
		t.Error("an unknown player should not be projected")
	}
}

func TestRecentMinutesWeighMore(t *testing.T) {
	var games []data.PlayerGame
	for i := 0; i < 20; i++ {
		mins := 20.0
		if i >= 15 { // role grows in the last five games
			mins = 34
		}
		games = append(games, data.PlayerGame{PlayerID: 1, GameID: 1 + i, Date: day(i), Season: 2025, Minutes: mins, Pts: mins * 0.6, Reb: 3, Ast: 3})
	}
	p, ok := NewIndex(games).Project(1, day(20))
	if !ok {
		t.Fatal("expected a projection")
	}
	if p.ProjMinutes <= p.AvgMinutes {
		t.Errorf("projected minutes %v should exceed the 15-game average %v after a role increase", p.ProjMinutes, p.AvgMinutes)
	}
	if p.ProjMinutes < 25 || p.ProjMinutes > 34 {
		t.Errorf("projected minutes %v outside the sensible range", p.ProjMinutes)
	}
}

func TestVarianceFloorAndPositivity(t *testing.T) {
	p, ok := NewIndex(steadyPlayer(1, 20, 30, 20, 5, 4)).Project(1, day(20))
	if !ok {
		t.Fatal("expected a projection")
	}
	for _, s := range data.Stats {
		if p.Var[s] < p.Mean[s]*1.05-1e-9 {
			t.Errorf("%s variance %v is below the floor for mean %v", s, p.Var[s], p.Mean[s])
		}
	}
}
