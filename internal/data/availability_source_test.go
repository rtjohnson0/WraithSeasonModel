package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAvailabilityRequiresExplicitFinalAndMatchingGameTeam(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.jsonl")
	body := `{"min":"30:00","player":{"id":1},"team":{"id":1},"game":{"id":1,"date":"2026-11-01","status":"Final","home_team_id":1,"visitor_team_id":2}}
{"min":"30:00","player":{"id":2},"team":{"id":1},"game":{"id":1,"date":"2026-11-01","home_team_id":1,"visitor_team_id":2}}
{"min":"30:00","player":{"id":3},"team":{"id":3},"game":{"id":1,"date":"2026-11-01","status":"Final","home_team_id":1,"visitor_team_id":2}}
`
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	rows, e := LoadStatsFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 3 || !rows[0].SourceFinal || rows[1].SourceFinal || rows[2].SourceFinal {
		t.Fatalf("source-final flags %+v", rows)
	}
}
