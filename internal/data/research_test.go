package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResearchLoaderRetainsOnlyExplicitZeroAndLeavesTrainingAlone(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "stats_2026.jsonl")
	raw := `{"min":"0:00","player":{"id":1},"team":{"id":1},"game":{"id":9,"date":"2026-10-01","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n" +
		`{"min":null,"player":{"id":2},"team":{"id":1},"game":{"id":9,"date":"2026-10-01","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n" +
		`{"min":"DNP","player":{"id":3},"team":{"id":1},"game":{"id":9,"date":"2026-10-01","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	if e := os.WriteFile(p, []byte(raw), 0644); e != nil {
		t.Fatal(e)
	}
	normal, e := LoadStatsDir(dir)
	if e != nil || len(normal) != 0 {
		t.Fatal(normal, e)
	}
	research, e := LoadStatsResearchDir(dir)
	if e != nil || len(research) != 1 || research[0].PlayerID != 1 || !research[0].SourceFinal {
		t.Fatal(research, e)
	}
	for _, raw := range []string{`null`, `""`, `"DNP"`, `"NaN"`, `"0:bad"`, `"-1"`, `"0:00:00"`} {
		if explicitZero([]byte(raw)) {
			t.Fatal(raw)
		}
	}
}
func TestResearchLoaderRejectsConflictingZeroAndPlayedDuplicates(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "stats_2026.jsonl")
	raw := `{"min":0,"player":{"id":1},"team":{"id":1},"game":{"id":9,"date":"2026-10-01","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n" +
		`{"min":30,"player":{"id":1},"team":{"id":1},"game":{"id":9,"date":"2026-10-01","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	os.WriteFile(p, []byte(raw), 0644)
	research, e := LoadStatsResearchDir(dir)
	if e != nil || len(research) != 0 {
		t.Fatal(research, e)
	}
	normal, e := LoadStatsDir(dir)
	if e != nil || len(normal) != 1 || normal[0].Minutes != 30 {
		t.Fatal(normal, e)
	}
}
