package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTodaysForecastIsPendingAndInputsRemainUntouched(t *testing.T) {
	dir := t.TempDir()
	stats := `{"_stats_fetched_at":"2026-11-11T04:00:00Z","_stats_endpoint":"/nba/v1/stats","min":"30","pts":20,"reb":5,"ast":4,"fg3m":2,"player":{"id":1,"first_name":"Test","last_name":"Player"},"team":{"id":1,"abbreviation":"BOS"},"game":{"id":7,"date":"2026-11-10","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	if e := os.WriteFile(filepath.Join(dir, "stats_2026.jsonl"), []byte(stats), 0644); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "v2_rotation", "replay")
	os.MkdirAll(path, 0755)
	capture := `{"date":"2026-11-13","selection_policy":"rotation-v1"}` + "\n"
	file := filepath.Join(path, "decisions_2026-11-13.jsonl")
	os.WriteFile(file, []byte(capture), 0644)
	r, e := collect(dir, "2026-11-13")
	if e != nil {
		t.Fatal(e)
	}
	if r.PlayerGames != 0 || r.Questions != 0 || r.Excluded["date not completed"] != 1 {
		t.Fatal(r)
	}
	b, _ := os.ReadFile(file)
	if string(b) != capture {
		t.Fatal("audit changed original forecasts")
	}
}
