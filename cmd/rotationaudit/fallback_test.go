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
