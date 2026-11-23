package main

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"testing"
)

func guardedFixture() (replay.Record, predictor.Pred) {
	base := predictor.Pred{Minutes: 30}
	base.Rate[data.StatIndex(data.Threes)] = .1
	for i := range base.MinNodes {
		base.MinNodes[i] = 30
	}
	scale := [data.NStats]float64{1, 1, 1, 1.1}
	r := replay.Record{SelectionPolicy: rotation.GuardedPolicy, Date: "2026-11-22", At: "2026-11-22T18:00:00Z", Policy: predictor.Adjustments{Scale: scale}, Rotation: &rotation.Context{Policy: rotation.GuardedPolicy, UsedMinutes: 30, RateScale: scale, Opportunities: []opportunity.Signal{{Stat: data.Threes, Through: "2026-11-20", Available: true, Games: 10, Chances: 100, Minutes: 200, Conversion: .4, Rate: .2}}}}
	return r, base
}
func TestGuardedCaptureReconstructsActualThreesAdjustment(t *testing.T) {
	r, base := guardedFixture()
	if !validGuardedCapture(r, base) {
		t.Fatal("valid V5 forecast excluded")
	}
	if validProductionCapture(r) {
		t.Fatal("fixture no longer demonstrates V4 contract mismatch")
	}
	bad, bp := guardedFixture()
	bad.Rotation.Opportunities[0].Chances = math.NaN()
	if validGuardedCapture(bad, bp) {
		t.Fatal("NaN evidence accepted")
	}
	r.Policy.Scale[3] = 1.09
	if validGuardedCapture(r, base) {
		t.Fatal("tampered threes scale accepted")
	}
	r, base = guardedFixture()
	r.Rotation.Opportunities[0].Through = r.Date
	if validGuardedCapture(r, base) {
		t.Fatal("same-day attempts accepted")
	}
	r, base = guardedFixture()
	r.Rotation.Opportunities[0].Rate = .9
	if validGuardedCapture(r, base) {
		t.Fatal("incoherent attempt signal accepted")
	}
}
func TestGuardedMinutesCapAndUncertainStatusAreChecked(t *testing.T) {
	r, base := guardedFixture()
	target := 32.5
	r.Policy.TargetMinutes = &target
	r.Rotation.AllocationApplied = true
	r.Rotation.Supported = true
	r.Rotation.MatchedGames = 3
	r.Rotation.RelevantOut = []int{9}
	r.Rotation.ConfirmedOut = []rotation.Status{{PlayerID: 9, Status: "Out", At: "2026-11-22T17:50:00Z"}}
	r.Rotation.AllocationMinutes = 39
	r.Rotation.UsedMinutes = 32.5
	if !validGuardedCapture(r, base) {
		t.Fatal("supported capped forecast excluded")
	}
	target = 39
	if validGuardedCapture(r, base) {
		t.Fatal("uncapped workload accepted")
	}
	target = 32.5
	r.Rotation.OwnStatus = "Questionable"
	if validGuardedCapture(r, base) {
		t.Fatal("uncertain own workload boost accepted")
	}
}
