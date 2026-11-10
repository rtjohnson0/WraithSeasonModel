package main

import (
	"propsmodel/internal/accuracy"
	"testing"
)

func TestEvidenceGateKeepsVisibleResearchPicks(t *testing.T) {
	p := []scored{{Tier: "PICK", Units: .5}, {Tier: "CHECK"}}
	out := evidenceGate(p, accuracy.Report{}, "2026-11-10", "pregame", 4)
	if len(out) != 2 || out[0].Tier != "PAPER" || out[0].Units != 0 || out[1].Tier != "CHECK" {
		t.Fatal(out)
	}
}
func TestAccuracyRankingIsNotLargestPriceEdge(t *testing.T) {
	p := []scored{{PlayerID: 1, GameID: 1, Prob: .56, Edge: .1}, {PlayerID: 2, GameID: 1, Prob: .65, Edge: .03}}
	p = capCorrelated(p, 1, 1)
	if len(p) != 1 || p[0].PlayerID != 2 {
		t.Fatal(p)
	}
}
