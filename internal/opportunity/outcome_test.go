package opportunity

import (
	"propsmodel/internal/data"
	"testing"
	"time"
)

func TestOutcomeTrackingRequiresMatchingFinalAndCaptureButCanUseSameGame(t *testing.T) {
	v := 9.
	g := data.PlayerGame{PlayerID: 1, GameID: 2, TeamID: 3, Date: "2026-11-19", Minutes: 30, Ast: 4, SourceFinal: true, BoxScoreComplete: true}
	k := Key{Player: 1, Game: 2, Type: "passing"}
	r := Observation{Player: 1, Game: 2, Team: 3, Date: g.Date, Captured: "2026-11-20T08:00:00Z", Stats: map[string]*float64{"potential_ast": &v}}
	ix := &Index{AsOf: time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC), Rows: map[Key]Observation{k: r}}
	if n, _, _, ok := ix.OutcomeVolume(g, data.Assists); !ok || n != 9 {
		t.Fatal("valid final tracking missing")
	}
	for _, change := range []func(*Observation){func(r *Observation) { r.Team = 4 }, func(r *Observation) { r.Date = "2026-11-18" }, func(r *Observation) { r.Captured = "2026-11-21T08:00:00Z" }, func(r *Observation) { r.Captured = "2026-11-18T08:00:00Z" }, func(r *Observation) { r.Stats = map[string]*float64{"potential_ast": nil} }} {
		bad := r
		change(&bad)
		ix.Rows[k] = bad
		if _, _, _, ok := ix.OutcomeVolume(g, data.Assists); ok {
			t.Fatal("bad outcome accepted", bad)
		}
	}
	ix.Rows[k] = r
	g.SourceFinal = false
	if _, _, _, ok := ix.OutcomeVolume(g, data.Assists); ok {
		t.Fatal("non-final accepted")
	}
}
