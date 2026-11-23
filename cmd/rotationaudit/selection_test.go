package main

import (
	"propsmodel/internal/data"
	"propsmodel/internal/replay"
	"testing"
)

func TestSelectionAuditDoesNotInventHistoricalSelection(t *testing.T) {
	a := selectionAudit{}
	r := replay.Record{Date: "2026-11-22", Stat: data.Assists, P1: .35, Line: 5.5}
	a.observe(r, 7, 30, 4, 25)
	rows := a.snapshot()
	if len(rows) != 1 || rows[0].Status != "unknown" || rows[0].Side != "UNDER" || rows[0].Wins != 0 {
		t.Fatal(rows)
	}
	r.Selection = &replay.Selection{Revision: "exact-quote-selection-v1", Status: "excluded", Side: "UNDER", Reason: "thin"}
	a.observe(r, 4, 25, 4, 25)
	rows = a.snapshot()
	if len(rows) != 2 {
		t.Fatal(rows)
	}
	for _, s := range rows {
		if s.Status == "excluded" && (s.Wins != 1 || s.Predicted != .65) {
			t.Fatal(s)
		}
	}
	r.Selection.Side = "OVER"
	a.observe(r, 4, 25, 4, 25)
	for _, s := range a.snapshot() {
		if s.Status == "unknown" && s.N != 2 {
			t.Fatal("different side borrowed selection", s)
		}
	}
}
