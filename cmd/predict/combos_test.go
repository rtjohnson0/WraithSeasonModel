package main

import (
	"strings"
	"testing"

	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/predictor"
)

func TestComboEntriesAreTheirOwnTier(t *testing.T) {
	s := scored{PlayerID: 7, GameID: 9, Player: "A B", Stat: data.PRA, Side: "UNDER", Line: 38.5, Prob: 0.58, Odds: -115, Units: 1.2, Tier: comboTier}
	es := comboEntries("2026-10-27", []scored{s})
	if len(es) != 1 || es[0].Tier != "COMBO" || es[0].Units != 0 || es[0].Stat != "pra" {
		t.Fatalf("combo entries must be tier COMBO, never staked: %+v", es)
	}
}

func TestComponentNote(t *testing.T) {
	var p predictor.Pred
	for i := range p.MinNodes {
		p.MinNodes[i] = 30
	}
	p.Minutes = 30
	p.Rate[data.StatIndex(data.Points)] = 0.8
	p.Rate[data.StatIndex(data.Rebounds)] = 0.3
	got := componentNote(data.PR, p)
	if !strings.Contains(got, "24.0 pts + 9.0 reb") {
		t.Errorf("got %q", got)
	}
}

func TestComboDistTrimsTail(t *testing.T) {
	var p predictor.Pred
	for i := range p.MinNodes {
		p.MinNodes[i] = 30
	}
	p.Minutes = 30
	p.Rate[data.StatIndex(data.Points)] = 0.5
	p.Rate[data.StatIndex(data.Rebounds)] = 0.2
	p.Rate[data.StatIndex(data.Assists)] = 0.1
	p.Alpha = [data.NStats]float64{0.1, 0.1, 0.1, 0.1}
	d := comboDist(combo.Simulate(p, combo.Default, 500, 1), data.PRA)
	if len(d) < 15 || len(d) > 80 {
		t.Errorf("unexpected curve length %d", len(d))
	}
	var sum float64
	for _, v := range d {
		sum += v
	}
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("curve should add to about 1, got %.3f", sum)
	}
}

func TestComboCalibrationUsesItsOwnMarket(t *testing.T) {
	var p predictor.Pred
	singles := eval.Calibration{ByStat: map[string]eval.Cal{"points": {Slope: 1}, "rebounds": {Slope: 1}}}
	d := comboDeps{cal: singles, calOK: true, weight: -1}
	if got := d.calibrate(data.PR, p, 0.2, 0.5); got != 0.5 {
		t.Fatalf("borrowed component calibration: %v", got)
	}
	d.cal.ByStat["pr"] = eval.Cal{Slope: 1}
	if got := d.calibrate(data.PR, p, 0.2, 0.5); got < 0.199 || got > 0.201 {
		t.Fatalf("direct PR fit ignored: %v", got)
	}
	d.weight = 0.5
	if got := d.calibrate(data.PR, p, 0.2, 0.5); got < 0.349 || got > 0.351 {
		t.Fatalf("explicit experimental blend ignored: %v", got)
	}
}

func TestComboWatchDoesNotCreateQualifiedTracking(t *testing.T) {
	watch := scored{GameID: 9, PlayerID: 7, Stat: data.PRA, Tier: "COMBOWATCH", Prob: .5, Edge: -.02}
	rows := comboDisplay(nil, []scored{watch})
	if len(rows) != 1 || comboEntries("2026-11-10", rows)[0].Tier != "COMBOWATCH" {
		t.Fatal("watch comparison lost")
	}
	picked := watch
	picked.Tier = comboTier
	picked.Edge = .03
	rows = comboDisplay([]scored{picked}, []scored{watch})
	if len(rows) != 1 || rows[0].Tier != comboTier {
		t.Fatal("watch duplicated a qualified pick")
	}
}

func TestWatchEstimateDoesNotReuseMarketCoinFlip(t *testing.T) {
	side, p, raw, odds := comboEstimate(.73, -115, -105)
	if side != "OVER" || p != .73 || raw != .73 || odds != -115 {
		t.Fatal(side, p, raw, odds)
	}
	side, p, raw, odds = comboEstimate(.25, -115, -105)
	if side != "UNDER" || p != .75 || raw != .75 || odds != -105 {
		t.Fatal(side, p, raw, odds)
	}
}

func TestExpandedComboDisplayRetainsTrackingAndSpreadsComparisons(t *testing.T) {
	picked := scored{PlayerID: 999, GameID: 1, Stat: data.PRA, Tier: "COMBO", Prob: .57, Odds: -110}
	var watch []scored
	for pid := 1; pid <= 40; pid++ {
		game := 1
		if pid > 30 {
			game = 2
		}
		for _, st := range []data.Stat{data.PRA, data.PR, data.PA, data.RA} {
			watch = append(watch, scored{PlayerID: pid, GameID: game, Stat: st, Tier: "COMBOWATCH", Prob: .6, Odds: -110, Edge: .08})
		}
	}
	got := comboDisplay([]scored{picked}, watch)
	if len(got) != 60 || got[0].PlayerID != 999 || got[0].Prob != .57 || got[0].Tier != "COMBO" {
		t.Fatal("display changed tracked pick or failed expanded capacity")
	}
	byPlayer, byGame := map[int]int{}, map[int]int{}
	for _, s := range got {
		byPlayer[s.PlayerID]++
		byGame[s.GameID]++
		if s.PlayerID != 999 && s.Tier != "COMBOWATCH" {
			t.Fatal("comparison promoted into pick")
		}
	}
	for _, n := range byPlayer {
		if n > 2 {
			t.Fatal("one player crowded out comparisons")
		}
	}
	if byGame[2] == 0 {
		t.Fatal("smaller game's comparisons vanished")
	}
}
