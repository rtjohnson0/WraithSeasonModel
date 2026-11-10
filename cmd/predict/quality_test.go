package main

import (
	"propsmodel/internal/accuracy"
	"propsmodel/internal/props"
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

func TestGameLeansRequirePositivePriceAndNoBadLine(t *testing.T) {
	all := []scored{{GameID: 1, PlayerID: 1, Stat: "points", Edge: .02, Far: true}, {GameID: 1, PlayerID: 2, Stat: "points", Edge: 0}, {GameID: 1, PlayerID: 3, Stat: "points", Edge: .01, Prob: .54}, {GameID: 1, PlayerID: 4, Stat: "points", Edge: .005, Prob: .53}}
	out := fillLeans(nil, all, map[int]props.GameInfo{1: {}}, 2, 2)
	if len(out) != 2 || out[0].Tier != "LEAN" || out[0].Units != 0 {
		t.Fatal(out)
	}
	for _, s := range out {
		if s.Far || s.Edge <= 0 {
			t.Fatal(s)
		}
	}
}
