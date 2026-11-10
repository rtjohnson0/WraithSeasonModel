package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"testing"
)

func TestSideAuditDedupesBooksAndRevealsCalibrationFlip(t *testing.T) {
	dir := t.TempDir()
	s := scored{GameID: 9, PlayerID: 7, Stat: data.Points, Side: "UNDER", Prob: .6, ProbRaw: .4, Edge: .05, Line: 20.5, MarketMedian: 20.5, ProjMean: 18, Shift: -5}
	if e := saveSideAudit(dir, "2026-11-10", []scored{s, s}, []scored{s}, .55, .01); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "side_audit.json"))
	var a SideAudit
	json.Unmarshal(b, &a)
	r := a.Rows[0]
	if r.Markets != 1 || r.RawOver != 1 || r.FinalUnder != 1 || r.UnderEdgeBar != 1 || r.PostedUnder != 1 || r.MinutesDrivenUnders != 1 {
		t.Fatalf("wrong audit %+v", r)
	}
}

func TestDecisionCaptureBindsQuoteGameWithoutChangingFeatures(t *testing.T) {
	r := &feat.Row{G: data.PlayerGame{PlayerID: 7, GameID: 0, Date: "2026-11-10"}, X: []float64{1, 2}}
	got := decisionIdentity(r, 99, "2026-11-10")
	if got.G.GameID != 99 || r.G.GameID != 0 || *got.X[0] != 1 {
		t.Fatal("decision identity repair changed features or original row")
	}
}

func TestSelectedSideGuardDoesNotManufactureOvers(t *testing.T) {
	picks := []scored{}
	for i := 0; i < 10; i++ {
		side := "UNDER"
		if i == 9 {
			side = "OVER"
		}
		picks = append(picks, scored{GameID: 9, PlayerID: i, Stat: data.Points, Side: side, Tier: "PICK", Units: .5, Prob: .6})
	}
	before := append([]scored(nil), picks...)
	got, reason := guardSelectedSides(picks, .8)
	if reason == "" {
		t.Fatal("concentration missed")
	}
	for i, s := range got {
		if s.Side != before[i].Side || s.Prob != .6 {
			t.Fatal("guard changed a forecast")
		}
		if i < 9 && (s.Tier != "CHECK" || s.Units != 0) {
			t.Fatal("dominant-side stake not withheld")
		}
		if i == 9 && s.Side != "OVER" {
			t.Fatal("minority side changed")
		}
	}
}
