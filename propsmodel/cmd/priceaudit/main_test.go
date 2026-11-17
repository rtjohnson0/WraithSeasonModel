package main

import (
	"os"
	"path/filepath"
	"propsmodel/internal/replay"
	"strings"
	"testing"
)

func TestPriceChecksUseExactSidePrice(t *testing.T) {
	if !strings.Contains(reason(.60, -150), "price edge") {
		t.Fatal("60% at -150 is break-even, not a 1pp edge")
	}
	if !strings.Contains(reason(.62, -150), "clears") {
		t.Fatal("62% at -150 should clear fixed bars")
	}
	if !strings.Contains(reason(.42, 150), "opposite side") {
		t.Fatal("positive forecast value must not silently bypass confidence selection")
	}
	if !strings.Contains(reason(.55, -120), "price edge") {
		t.Fatal("55% is insufficient at -120")
	}
}
func TestCaptureChoiceKeepsChronologyAndCurrentLatest(t *testing.T) {
	old := replay.Record{Date: "2026-11-12", At: "2026-11-12T12:00:00.9Z"}
	r := replay.Record{Date: old.Date, At: "2026-11-12T12:00:00Z"}
	if !choose(old, r, "2026-11-13") {
		t.Fatal("completed cohort must use earliest eligible question")
	}
	if choose(old, r, "2026-11-12") {
		t.Fatal("current cohort must use latest question")
	}
}
func TestBothSidesGetSameNumericalRulesAndSeparatePriceGroups(t *testing.T) {
	qs := []Question{{Cohort: "test", Stat: "points", Side: "OVER", Odds: -150, Raw: .6, Final: .6, Edge: 0, Result: "WIN"}, {Cohort: "test", Stat: "points", Side: "UNDER", Odds: 150, Raw: .4, Final: .4, Edge: 0, Result: "LOSS"}}
	gs := summarize(qs)
	if len(gs) != 2 {
		t.Fatal(gs)
	}
	for _, g := range gs {
		if g.N != 1 || g.BothBars != 0 {
			t.Fatal(g)
		}
		if g.Side == "OVER" && (g.Wins != 1 || g.ProbabilityBar != 1) {
			t.Fatal(g)
		}
		if g.Side == "UNDER" && (g.Losses != 1 || g.ProbabilityBar != 0) {
			t.Fatal(g)
		}
	}
}
func TestHistoricalResultsAreRecheckedAndInputsRemainUnchanged(t *testing.T) {
	dir := t.TempDir()
	stats := `{"_stats_fetched_at":"2026-11-11T04:00:00Z","_stats_endpoint":"/nba/v1/stats","min":"30","pts":20,"reb":5,"ast":4,"fg3m":2,"player":{"id":1,"first_name":"Test","last_name":"Player"},"team":{"id":1,"abbreviation":"BOS"},"game":{"id":7,"date":"2026-11-10","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	history := `{"date":"2026-11-10","logged_at":"2026-11-10T18:00:00Z","tip":"2026-11-10T23:00:00Z","period":"game","game_id":7,"player_id":1,"player":"Test Player","team":"BOS","stat":"rebounds","side":"OVER","line":6.5,"odds":-110,"prob":0.6,"prob_raw":0.62,"tier":"PAPER","result":"WIN","actual":9}` + "\n"
	for name, s := range map[string]string{"stats_2026.jsonl": stats, "settled.jsonl": history} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte(s), 0644); e != nil {
			t.Fatal(e)
		}
	}
	r, e := collect(dir, "2026-11-13", false)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Questions) != 1 || r.Questions[0].Result != "LOSS" {
		t.Fatal(r)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "settled.jsonl"))
	if string(b) != history {
		t.Fatal("audit rewrote source results")
	}
}
