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
