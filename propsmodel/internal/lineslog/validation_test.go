package lineslog

import (
	"math"
	"testing"
)

func TestBaselineNeverUsesTodaysOutcomes(t *testing.T) {
	gm := []GradedMarket{
		{Market: Market{Date: "2026-11-09", Stat: "points", POver: 0.5}, Over: true},
		{Market: Market{Date: "2026-11-10", Stat: "points", POver: 0.5}, Over: false},
	}
	before := priorRates(gm)[[2]string{"2026-11-10", "points"}]
	gm[1].Over = true
	after := priorRates(gm)[[2]string{"2026-11-10", "points"}]
	if before != after || math.Abs(before-21./41) > 1e-12 {
		t.Fatalf("lookahead baseline %v %v", before, after)
	}
}
func TestModelComparisonUsesOnlySameQuestions(t *testing.T) {
	gm := []GradedMarket{
		{Market: Market{Date: "a", POver: 0.7, P2: 0.8}, Over: true},
		{Market: Market{Date: "b", POver: 0.8, P2: 0.9}, Over: false},
		{Market: Market{Date: "c", POver: 0.99}, Over: true},
	}
	c := CompareModels(gm)
	if c.N != 2 || c.Days != 2 || math.Abs(c.Brier1-(0.09+0.64)/2) > 1e-10 {
		t.Fatalf("unmatched data included %+v", c)
	}
}
