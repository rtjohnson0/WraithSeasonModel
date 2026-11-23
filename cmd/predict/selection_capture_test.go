package main

import (
	"propsmodel/internal/data"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"testing"
)

func TestCaptureSelectionKeepsExactQuotesAndAlternateLinesSeparate(t *testing.T) {
	s := play(1, data.Rebounds, "OVER", .65)
	s.Line = 6.5
	s.Vendor = "fixture"
	s.QuoteAt = "2026-11-23T18:00:00Z"
	r := replay.Record{Stat: s.Stat, Line: s.Line, Book: s.Vendor, QuoteAt: s.QuoteAt, Row: replay.Row{G: data.PlayerGame{PlayerID: s.PlayerID, GameID: s.GameID}}}
	rules := v2Rules{Hi: .55, MinEdge: .01, MaxGame: 4, MaxPlayer: 2}
	for _, tc := range []struct {
		posted []scored
		status string
	}{{[]scored{s}, "posted"}, {nil, "excluded"}} {
		records := []replay.Record{r}
		captureSelection(records, []scored{s}, tc.posted, rules)
		if records[0].Selection == nil || records[0].Selection.Status != tc.status {
			t.Fatal(records)
		}
	}
	alt := s
	alt.Line = 5.5
	records := []replay.Record{r}
	captureSelection(records, []scored{s}, []scored{alt}, rules)
	if records[0].Selection.Status != "alternate-quote" {
		t.Fatal("different line called rejected", records)
	}
	s.Prob = .51
	records = []replay.Record{r}
	captureSelection(records, []scored{s}, nil, rules)
	if records[0].Selection.Reason != "below probability or edge rule" {
		t.Fatal(records)
	}
	records = []replay.Record{r}
	captureSelection(records, nil, nil, rules)
	if records[0].Selection != nil {
		t.Fatal("missing capture invented")
	}
}

func TestGuardedV5WorkloadReversalIsLeanNotRemoved(t *testing.T) {
	for _, side := range []string{"OVER", "UNDER"} {
		s := play(1, data.Rebounds, side, .65)
		s.Rotation = &rotation.Context{Policy: rotation.GuardedPolicy, Supported: true, WorkloadReview: &rotation.WorkloadReview{Policy: "minutes-sensitivity-v1", DirectionChanges: true, BaselineMinutes: 30, BaselineProbability: .45}}
		picks, _ := v2Select([]scored{s}, v2Rules{Hi: .55, MinEdge: .01, MaxGame: 4, MaxPlayer: 2, Gates: provenGates(data.Rebounds, side)})
		if len(picks) != 1 || picks[0].Tier != tierLean || picks[0].Prob != s.Prob {
			t.Fatal(picks)
		}
	}
}
