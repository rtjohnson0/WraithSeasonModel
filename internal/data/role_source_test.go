package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchivedBoxScoreCompletenessNeverCertifiesForwardProvenance(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stats.jsonl")
	full := `{"min":"30","pts":10,"reb":5,"ast":2,"fg3m":1,"blk":0,"player":{"id":1},"team":{"id":1},"game":{"id":1,"date":"2025-11-01","status":"Final","home_team_id":1,"visitor_team_id":2,"home_team_score":100,"visitor_team_score":90}}`
	partial := strings.Replace(full, `"blk":0,`, ``, 1)
	os.WriteFile(p, []byte(full+"\n"+partial+"\n"), 0600)
	rows, e := LoadStatsFile(p)
	if e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
	if !rows[0].BoxScoreComplete || rows[0].AnalyticsReady || rows[1].BoxScoreComplete {
		t.Fatal("missing fields or capture provenance silently filled", rows)
	}
}
