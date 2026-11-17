package data

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOnlyFinalFullGameStatsCanBecomeResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.jsonl")
	row := func(pid, period int, status string) string {
		return fmt.Sprintf(`{"period":%d,"min":"30","pts":12,"player":{"id":%d},"game":{"id":9,"date":"2026-11-10","status":"%s"}}`, period, pid, status) + "\n"
	}
	if err := os.WriteFile(path, []byte(row(1, 0, "Final")+row(2, 0, "3rd Qtr")+row(3, 1, "Final")), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadStatsFile(path)
	if err != nil || len(got) != 1 || got[0].PlayerID != 1 {
		t.Fatalf("partial/live results leaked: %+v %v", got, err)
	}
}
