package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/quarter"
	"testing"
	"time"
)

func TestOfflinePipelineComparesAndGradesOnlyQuarterActuals(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)
	date := "2026-11-10"
	if e := os.MkdirAll(filepath.Join(dir, "snapshots"), 0755); e != nil {
		t.Fatal(e)
	}
	stats, err := os.Create(filepath.Join(dir, "stats_2026.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(stats)
	for id := 1; id <= 8; id++ {
		d := time.Date(2026, 10, id+20, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		enc.Encode(map[string]any{"player": map[string]any{"id": 7, "first_name": "A", "last_name": "Player"}, "team": map[string]any{"id": 1}, "game": map[string]any{"id": id, "date": d, "season": 2026, "status": "Final", "home_team_id": 1, "visitor_team_id": 2}, "pts": 40, "min": "30"})
		quarter.Save(filepath.Join(dir, "quarter_games", string(rune('0'+id))+".json"), quarter.GameData{GameID: id, Date: d, Version: 1, Rows: []quarter.Observation{{GameID: id, PlayerID: 7, Date: d, Period: "1q", Points: 2, Participated: true}}})
	}
	stats.Close()
	snapshot := filepath.Join(dir, "snapshots", "props_"+date+".jsonl")
	f, _ := os.Create(snapshot)
	j := json.NewEncoder(f)
	for _, book := range []string{"draftkings", "fanduel", "betmgm"} {
		j.Encode(map[string]any{"captured_at": "2026-11-10T16:59:00Z", "game_id": 99, "game": map[string]any{"id": 99, "date": date, "datetime": "2026-11-10T23:00:00Z", "home_team_id": 1, "visitor_team_id": 2}, "response": map[string]any{"data": []any{map[string]any{"game_id": 99, "player_id": 7, "vendor": book, "prop_type": "points_1q", "line_value": "3.5", "updated_at": "2026-11-10T16:58:00Z", "market": map[string]any{"type": "over_under", "over_odds": -110, "under_odds": -110}}}}})
	}
	f.Close()
	if e := run(dir, date, now, true); e != nil {
		t.Fatal(e)
	}
	b, e := quarter.LoadBoard(dir, date)
	if e != nil || b == nil || len(b.Cards) != 3 {
		t.Fatalf("missing board: %+v %v", b, e)
	}
	c := b.Cards[0]
	if c.Period != "1q" || c.Comparison.N != 8 || *c.Comparison.Mean != 2 || c.Side != "UNDER" {
		t.Fatalf("full-game 40 points leaked into Q1: %+v", c)
	}
	logs, e := quarter.LoadLog(filepath.Join(dir, "quarterlog.jsonl"))
	if e != nil || len(logs) != 3 {
		t.Fatal("quarter paper log missing")
	}
	quarter.Save(filepath.Join(dir, "quarter_games", "99.json"), quarter.GameData{GameID: 99, Date: date, Version: 1, Rows: []quarter.Observation{{GameID: 99, PlayerID: 7, Date: date, Period: "1q", Points: 2, Participated: true}}})
	next := "2026-11-11"
	os.WriteFile(filepath.Join(dir, "snapshots", "props_"+next+".jsonl"), nil, 0600)
	if e := run(dir, next, now.Add(24*time.Hour), true); e != nil {
		t.Fatal(e)
	}
	logs, e = quarter.LoadLog(filepath.Join(dir, "quarterlog.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range logs {
		if r.Result != "WIN" || r.Actual == nil || *r.Actual != 2 {
			t.Fatalf("not graded using 1Q: %+v", r)
		}
	}
	b, _ = quarter.LoadBoard(dir, next)
	if len(b.Cards) != 0 {
		t.Fatal("old quarter board survived empty run")
	}
}
