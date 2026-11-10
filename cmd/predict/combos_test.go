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
