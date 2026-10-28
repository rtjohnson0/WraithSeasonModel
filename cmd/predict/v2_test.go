package main

import (
	"math"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/predictor"
	"propsmodel/internal/projlog"
)

func TestFitV2UsesOnlyEarlierDaysAndShrinks(t *testing.T) {
	var entries []projlog.Entry
	act := map[[2]int]data.PlayerGame{}
	id := 0
	add := func(date string, projMin, actMin, projPts, actPts float64) {
		id++
		entries = append(entries, projlog.Entry{Date: date, GameID: id, PlayerID: id, ProjMin: projMin,
			Stats: map[string]projlog.StatProj{"points": {Proj: projPts, Line: projPts}}})
		act[[2]int{id, id}] = data.PlayerGame{Minutes: actMin, Pts: actPts}
	}
	for i := 0; i < 12; i++ {
		add("2026-10-27", 33, 35, 20, 23) // starters run 2 minutes over, 15% over on points
	}
	add("2026-10-28", 33, 50, 20, 60) // today: must be ignored (no look-ahead)
	f := fitV2(entries, "2026-10-28", func(g, p int) (data.PlayerGame, bool) { pg, ok := act[[2]int{g, p}]; return pg, ok })
	if f.Days != 1 || f.Players != 12 || f.MinN != 12 {
		t.Fatalf("only the earlier day counts: %+v", f)
	}
	// 12 player-games of +2: shrunk to 2*12/(12+40) = 0.46, far less than the raw miss.
	if f.MinShift < 0.3 || f.MinShift > 0.6 {
		t.Errorf("minutes shift should be shrunk hard from +2, got %v", f.MinShift)
	}
	if got := f.Ratio[data.Points]; got <= 1 || got > 1.03 {
		t.Errorf("points multiplier should move a little toward 1.15, got %v", got)
	}
	if orOne(f.Ratio[data.Rebounds]) != 1 {
		t.Errorf("a stat with no data stays at 1, got %v", f.Ratio[data.Rebounds])
	}
	empty := fitV2(nil, "2026-10-28", nil)
	if empty.MinShift != 0 || empty.statFactor(data.Points) != 1 {
		t.Errorf("no history must mean no correction: %+v", empty)
	}
}

func TestFitV2IsCapped(t *testing.T) {
	var entries []projlog.Entry
	act := map[[2]int]data.PlayerGame{}
	for i := 1; i <= 2000; i++ {
		entries = append(entries, projlog.Entry{Date: "2026-10-01", GameID: i, PlayerID: i, ProjMin: 34, Stats: map[string]projlog.StatProj{"points": {Proj: 10, Line: 10}}})
		act[[2]int{i, i}] = data.PlayerGame{Minutes: 44, Pts: 30}
	}
	f := fitV2(entries, "2026-10-28", func(g, p int) (data.PlayerGame, bool) { pg, ok := act[[2]int{g, p}]; return pg, ok })
	if f.MinShift > v2MinShiftCap+1e-9 || f.Ratio[data.Points] > 1+v2StatCap+1e-9 {
		t.Errorf("caps must hold: %v %v", f.MinShift, f.Ratio[data.Points])
	}
}

func TestShiftMinutesOnlyForStars(t *testing.T) {
	f := newV2Fix()
	f.MinShift = 1.5
	var star, bench predictor.Pred
	for k := range star.MinNodes {
		star.MinNodes[k], bench.MinNodes[k] = 34, 24
	}
	star.Minutes, bench.Minutes = 34, 24
	if got := f.shiftMinutes(star).Minutes; math.Abs(got-35.5) > 1e-6 {
		t.Errorf("star minutes should rise 1.5, got %v", got)
	}
	if got := f.shiftMinutes(bench).Minutes; got != 24 {
		t.Errorf("bench minutes must not change, got %v", got)
	}
}

func TestBlendAndSelect(t *testing.T) {
	if got := blend(0.60, 0.50, 0.5); math.Abs(got-0.55) > 1e-9 {
		t.Errorf("50/50 blend of 60 and 50 should be 55, got %v", got)
	}
	all := []scored{
		{PlayerID: 1, GameID: 1, Stat: data.Points, Side: "OVER", Line: 20.5, Prob: 0.57, Edge: 0.03, Odds: -110},
		{PlayerID: 2, GameID: 1, Stat: data.Points, Side: "UNDER", Line: 10.5, Prob: 0.54, Edge: 0.02, Odds: -110},  // under the bar
		{PlayerID: 3, GameID: 1, Stat: data.Rebounds, Side: "UNDER", Line: 6.5, Prob: 0.58, Edge: 0.00, Odds: -110}, // no edge
		{PlayerID: 4, GameID: 2, Stat: data.Assists, Side: "UNDER", Line: 4.5, Prob: 0.60, Edge: 0.05, Odds: -105},
	}
	got := v2Select(all, 0.55, 0.01, 4, 2)
	if len(got) != 2 || got[0].PlayerID != 1 || got[1].PlayerID != 4 {
		t.Errorf("only plays above the bar with an edge are posted: %+v", got)
	}
	for _, s := range got {
		if s.Units != 0 {
			t.Error("V2 is never staked")
		}
	}
}

func TestAnchorAndFactor(t *testing.T) {
	if got := v2Anchor(23, 22); math.Abs(got-23.0/22) > 1e-9 {
		t.Errorf("anchor should be line/projection, got %v", got)
	}
	if got := v2Anchor(100, 10); got != v2AnchorHi {
		t.Errorf("anchor must be capped, got %v", got)
	}
	if got := v2Anchor(10, 100); got != v2AnchorLo {
		t.Errorf("anchor must be floored, got %v", got)
	}
	if v2Anchor(0, 5) != 1 || v2Anchor(5, 0) != 1 {
		t.Error("nothing to anchor to means no change")
	}
	f := newV2Fix()
	f.Anchor[data.Points], f.Ratio[data.Points] = 1.05, 1.02
	if got := f.statFactor(data.Points); math.Abs(got-1.071) > 1e-9 {
		t.Errorf("factor is anchor times live miss, got %v", got)
	}
	if f.statFactor(data.Threes) != 1 {
		t.Error("unset stats stay at 1")
	}
}
