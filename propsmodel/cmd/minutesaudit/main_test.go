package main

import (
	"os"
	"path/filepath"
	"propsmodel/internal/minutesaudit"
	"strings"
	"testing"
)

func TestFinalTimeMetadataDoesNotInventPregameProvenance(t *testing.T) {
	if inWindow("2026-11-13", "2026-11-01", "2026-11-13") || inWindow("bad", "2026-11-01", "2026-11-13") {
		t.Fatal("current or malformed date accepted")
	}
}
func TestReportEscapesPlayerNamesAndLabelsLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	r := Report{LegacyQuestions: 1, Observations: []minutesaudit.Observation{{Cohort: "legacy projection diagnostic", Player: "<script>bad</script>", Date: "2026-11-01", GameID: 7, PlayerID: 1}}}
	if e := writeHTML(path, r); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	if strings.Contains(s, "<script>bad</script>") || !strings.Contains(s, "&lt;script&gt;") {
		t.Fatal("unsafe report HTML")
	}
}

func TestLegacyDiagnosticsRemainSeparateAndInputsAreReadOnly(t *testing.T) {
	dir := t.TempDir()
	stats := `{"_stats_fetched_at":"2026-11-11T04:00:00Z","_stats_endpoint":"/nba/v1/stats","min":"30","pts":20,"reb":7,"ast":4,"fg3m":2,"player":{"id":1,"first_name":"Test","last_name":"Player"},"team":{"id":1,"abbreviation":"BOS"},"game":{"id":7,"date":"2026-11-10","datetime":"2026-11-10T23:00:00Z","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	projection := `{"date":"2026-11-10","at":"2026-11-10T18:00:00Z","game_id":7,"player_id":1,"player":"Test Player","team":"BOS","proj_min":20,"typ_min":28,"stats":{"rebounds":{"raw":4,"proj":5,"line":6.5}}}` + "\n"
	os.WriteFile(filepath.Join(dir, "stats_2026.jsonl"), []byte(stats), 0644)
	os.WriteFile(filepath.Join(dir, "projlog.jsonl"), []byte(projection), 0644)
	os.WriteFile(filepath.Join(dir, "model.json"), []byte("unchanged model"), 0644)
	r, e := collect(dir, "2026-11-11", "2026-11-01", true)
	if e != nil {
		t.Fatal(e)
	}
	if r.VerifiedQuestions != 0 || r.LegacyQuestions != 1 || r.Stats[0].Bias != 2 {
		t.Fatal(r)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "model.json"))
	if string(b) != "unchanged model" {
		t.Fatal("modified model")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "projlog.jsonl"))
	if string(b) != projection {
		t.Fatal("modified input log")
	}
	r, e = collect(dir, "2026-11-11", "2026-11-01", false)
	if e != nil || len(r.Observations) != 0 {
		t.Fatal("legacy data entered strict cohort", r, e)
	}
}
