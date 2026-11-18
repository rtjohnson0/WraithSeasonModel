package main

import (
	"propsmodel/internal/lineup"
	"propsmodel/internal/rotation"
	"testing"
)

func TestFallbackEvidenceCannotBorrowJointCountsOrStackPlayers(t *testing.T) {
	impact := lineup.Impact{Missing: []lineup.Profile{{PlayerID: 1}, {PlayerID: 2}}}
	e := lineup.Effect{Applied: true, Method: "individual-mixture", Scale: 1.01, Evidence: []lineup.Component{{PlayerID: 1, Without: 5, With: 10, Weight: .5, Scale: 1.02}}}
	if !validEffectEvidence(e, impact, rotation.FallbackPolicy) {
		t.Fatal("valid mixture rejected")
	}
	if validEffectEvidence(e, impact, rotation.StrictPolicy) {
		t.Fatal("new evidence accepted in old policy")
	}
	e.Evidence = append(e.Evidence, e.Evidence[0])
	if validEffectEvidence(e, impact, rotation.FallbackPolicy) {
		t.Fatal("duplicate evidence accepted")
	}
	e.Evidence = e.Evidence[:1]
	e.Evidence[0].Without = 4
	if validEffectEvidence(e, impact, rotation.FallbackPolicy) {
		t.Fatal("thin history accepted")
	}
}

func TestPooledRoleCaptureChecksCompositionAndSeparatePolicy(t *testing.T) {
	role := &lineup.RoleEvidence{Before: "2026-11-18", Games: 1000, Days: 50, Features: make([]float64, 14), Coefficients: make([]float64, 14), Baseline: make([]float64, 7), Projected: make([]float64, 7)}
	role.Baseline[1] = 40
	role.Projected[1] = 36
	role.Features[8] = -.1
	role.Coefficients[8] = -.2
	e := lineup.Effect{Method: "pooled-lineup", Applied: true, Scale: 1.02, Role: role}
	impact := lineup.Impact{Kind: "opponent-allowance"}
	if !validEffectEvidence(e, impact, rotation.FallbackPolicy) {
		t.Fatal("valid league-pooled capture rejected")
	}
	if validEffectEvidence(e, impact, rotation.StrictPolicy) {
		t.Fatal("new policy mixed with old")
	}
	role.Projected[1] = 40
	if validEffectEvidence(e, impact, rotation.FallbackPolicy) {
		t.Fatal("composition tampering accepted")
	}
}
