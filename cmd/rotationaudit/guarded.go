package main

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/lineup"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
)

// Validate V5's actual frozen math, not V4's rate-scale contract. Shared
// evidence checks are retained; no rejected forecast is accepted by policy name alone.
func validGuardedCapture(r replay.Record, base predictor.Pred) bool {
	if r.SelectionPolicy != rotation.GuardedPolicy || r.Rotation == nil || r.Rotation.Policy != rotation.GuardedPolicy {
		return false
	}
	meta := r
	meta.SelectionPolicy = rotation.ContextPolicy
	meta.Policy.Scale = lineup.ScaleContext(r.Rotation.Production)
	if !validProductionCapture(meta) {
		return false
	}
	impacts := append([]lineup.Impact(nil), r.Rotation.Production...)
	for i := range impacts {
		if impacts[i].Kind != "opportunity-production" {
			continue
		}
		impacts[i].Effects = append([]lineup.Effect(nil), impacts[i].Effects...)
		for j := range impacts[i].Effects {
			if impacts[i].Effects[j].Stat == data.Threes {
				impacts[i].Effects[j].Applied = false
			}
		}
	}
	expected := lineup.ScaleContext(impacts)
	k := data.StatIndex(data.Threes)
	seen := false
	for _, s := range r.Rotation.Opportunities {
		if s.Stat != data.Threes {
			continue
		}
		if seen {
			return false
		}
		seen = true
		if s.Available && (data.DayNumber(s.Through) == 0 || math.IsNaN(s.Chances) || math.IsInf(s.Chances, 0) || math.IsNaN(s.Minutes) || math.IsInf(s.Minutes, 0) || math.IsNaN(s.Conversion) || math.IsInf(s.Conversion, 0) || s.Conversion < 0 || s.Conversion > 1 || s.Through == "" || s.Through >= r.Date || s.Minutes <= 0 || math.IsNaN(s.Rate) || math.IsInf(s.Rate, 0) || math.Abs(s.Rate-s.Chances/s.Minutes*s.Conversion) > 1e-9) {
			return false
		}
		expected[k] *= opportunity.ThreeAttemptFactor(base.Rate[k], s)
	}
	expected[k] = math.Max(.88, math.Min(1.12, expected[k]))
	for i, x := range expected {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(r.Policy.Scale[i]) || math.Abs(r.Policy.Scale[i]-x) > 1e-9 || math.IsNaN(r.Rotation.RateScale[i]) || math.Abs(r.Rotation.RateScale[i]-x) > 1e-9 {
			return false
		}
	}
	if r.Policy.TargetMinutes != nil {
		c := r.Rotation
		if math.IsNaN(c.AllocationMinutes) || math.IsInf(c.AllocationMinutes, 0) || !c.AllocationApplied || !c.Supported || c.MatchedGames < 3 || len(c.RelevantOut) == 0 || len(c.ConfirmedOut) == 0 || isUncertainStatus(c.OwnStatus) {
			return false
		}
		target := math.Max(base.Minutes-2.5, math.Min(base.Minutes+2.5, c.AllocationMinutes))
		if math.IsNaN(*r.Policy.TargetMinutes) || math.IsInf(*r.Policy.TargetMinutes, 0) || math.Abs(*r.Policy.TargetMinutes-target) > 1e-9 {
			return false
		}
	} else if r.Rotation.AllocationApplied {
		return false
	}
	used := r.Policy.Apply(base).Minutes
	return !math.IsNaN(r.Rotation.UsedMinutes) && math.Abs(r.Rotation.UsedMinutes-used) < 1e-8
}
