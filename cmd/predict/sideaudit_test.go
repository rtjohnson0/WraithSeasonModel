package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
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
