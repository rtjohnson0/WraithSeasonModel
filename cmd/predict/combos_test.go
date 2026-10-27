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

func TestComboCalibrationBorrowsFromTheParts(t *testing.T) {
	var p predictor.Pred
	for i := range p.MinNodes {
		p.MinNodes[i] = 30
	}
	p.Minutes = 30
	p.Rate[data.StatIndex(data.Points)] = 0.8
	p.Rate[data.StatIndex(data.Rebounds)] = 0.3
	p.Rate[data.StatIndex(data.Assists)] = 0.2
	cal := eval.Calibration{ByStat: map[string]eval.Cal{
		"points":   {Slope: 0.03, Market: 0.85},
		"rebounds": {Slope: 0.24, Market: 0.67},
		"assists":  {Slope: 0.18, Market: 0.78},
	}}
	d := comboDeps{cal: cal, calOK: true, weight: -1}
	// The model says 20% over, the book 50%: a model nobody has earned trust in barely moves a combo off the book.
	got := d.calibrate(data.PRA, p, 0.20, 0.50)
	if got < 0.45 || got > 0.50 {
		t.Errorf("a points-heavy combo should stay close to the book, got %.3f", got)
	}
	// A rebounds + assists combo borrows more trust in the model, so it moves further.
	ra := d.calibrate(data.RA, p, 0.20, 0.50)
	if ra >= got {
		t.Errorf("RA (rebounds and assists earned more model weight) should move further than PRA: %.3f vs %.3f", ra, got)
	}
	// An explicit blend overrides the calibration.
	d.weight = 0.5
	if b := d.calibrate(data.PRA, p, 0.20, 0.50); b < 0.349 || b > 0.351 {
		t.Errorf("a 50/50 blend of 0.20 and 0.50 is 0.35, got %.3f", b)
	}
	// No calibration file: a modest default share from the model.
	d = comboDeps{weight: -1}
	if b := d.calibrate(data.PRA, p, 0.20, 0.50); b < 0.42 || b > 0.43 {
		t.Errorf("default fallback is 25%% model, want about 0.425, got %.3f", b)
	}
}
