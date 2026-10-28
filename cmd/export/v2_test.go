package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"propsmodel/internal/picklog"
)

func TestBuildV2(t *testing.T) {
	dir := t.TempDir()
	if buildV2(dir, nil) != nil {
		t.Fatal("no v2.json means no V2 section")
	}
	f := v2File{
		Date: "2026-10-28",
		Plays: []picklog.Entry{
			{Date: "2026-10-28", GameID: 1, PlayerID: 5, Player: "A", Team: "ATL", Matchup: "MIA @ ATL", Stat: "points", Side: "OVER", Line: 20.5, Odds: -110, Prob: 0.57, Edge: 0.04, Tier: "PAPER"},
		},
		Combos: []picklog.Entry{
			{Date: "2026-10-28", GameID: 1, PlayerID: 6, Player: "B", Team: "MIA", Matchup: "MIA @ ATL", Stat: "pra", Side: "UNDER", Line: 30.5, Odds: -110, Prob: 0.54, Edge: 0.02, Tier: "COMBO"},
		},
		Info: &V2Info{Weight: 0.5, MinShift: 0.4},
	}
	b, _ := json.Marshal(f)
	os.WriteFile(filepath.Join(dir, "v2.json"), b, 0o644)
	// One graded single and one graded combo; each lands in its own record.
	settled := []picklog.Settled{
		picklog.Grade(f.Plays[0], 24, 0, true),
		picklog.Grade(f.Combos[0], 25, 0, true),
	}
	if _, err := picklog.AppendSettled(filepath.Join(dir, "v2settled.jsonl"), settled); err != nil {
		t.Fatal(err)
	}
	v := buildV2(dir, nil)
	if v == nil {
		t.Fatal("V2 section missing")
	}
	if v.Board.Date != "2026-10-28" || v.Board.Plays != 1 || len(v.Board.Games) != 1 || len(v.Board.Combos) != 1 {
		t.Errorf("board wrong: %+v", v.Board)
	}
	if v.Board.Games[0].Plays == nil {
		t.Error("plays must be a list, never null")
	}
	if v.Record.Days != 1 || len(v.Record.Recent) != 1 {
		t.Errorf("the single should be in the V2 record and the combo should not: %+v", v.Record)
	}
	if v.Combos.Days != 1 || len(v.Combos.Recent) != 1 {
		t.Errorf("the combo should be in the combo record: %+v", v.Combos)
	}
	if v.Info == nil || v.Info.MinShift != 0.4 {
		t.Error("info should pass through")
	}
}

func TestBuildV2TiersOversAndRecordRows(t *testing.T) {
	dir := t.TempDir()
	strong := picklog.Entry{Date: "2026-10-28", GameID: 1, PlayerID: 5, Player: "A", Team: "ATL", Matchup: "MIA @ ATL", Stat: "rebounds", Side: "UNDER", Line: 8.5, Odds: -110, Prob: 0.60, Edge: 0.06, Tier: "STRONG"}
	lean := picklog.Entry{Date: "2026-10-28", GameID: 1, PlayerID: 7, Player: "C", Team: "ATL", Matchup: "MIA @ ATL", Stat: "points", Side: "OVER", Line: 18.5, Odds: -110, Prob: 0.56, Edge: 0.03, Tier: "V2LEAN"}
	f := v2File{Date: "2026-10-28", Plays: []picklog.Entry{lean, strong},
		Overs: []OverWatch{{GameID: 1, Player: "D", Stat: "assists", Line: 5.5, Prob: 0.53, Status: "model gives it 53%; a pick needs 55%"}},
		Info:  &V2Info{Strong: 1, Lean: 1}}
	b, _ := json.Marshal(f)
	os.WriteFile(filepath.Join(dir, "v2.json"), b, 0o644)
	if _, err := picklog.AppendSettled(filepath.Join(dir, "v2settled.jsonl"), []picklog.Settled{picklog.Grade(strong, 6, 0, true), picklog.Grade(lean, 22, 0, true)}); err != nil {
		t.Fatal(err)
	}
	v := buildV2(dir, nil)
	if v == nil {
		t.Fatal("V2 section missing")
	}
	labels := map[string]string{}
	for _, p := range v.Board.Games[0].Plays {
		labels[p.Player] = p.Label
	}
	if labels["A"] != "Strong play" || labels["C"] != "Lean" {
		t.Errorf("tiers should read Strong play and Lean: %v", labels)
	}
	if v.Board.Games[0].Plays[0].Player != "A" {
		t.Error("the strong play sorts first")
	}
	if len(v.Board.Overs) != 1 || v.Board.Overs[0].Player != "D" {
		t.Errorf("the Overs watch should pass through: %+v", v.Board.Overs)
	}
	if v.Info.Gates == nil {
		t.Error("gates must be a list, never null")
	}
	names := map[string]int{}
	for _, r := range v.Record.Tiers {
		names[r.Name] = r.Picks
	}
	if names["V2 strong plays (the real model agrees, stat and side proven)"] != 1 || names["V2 lean plays (posted, not yet backed by the real model or a proven record)"] != 1 {
		t.Errorf("record should split strong and lean: %v", names)
	}
}
