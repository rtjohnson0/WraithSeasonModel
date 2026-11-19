package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPositionCorrectionRequiresIdentityAndPreservesRaw(t *testing.T) {
	for _, c := range PositionCorrections() {
		if c.Source == "" || c.VerifiedAt == "" || c.Reason == "" {
			t.Fatal("unsourced correction", c)
		}
	}
	if p, _ := CorrectPosition(1, "Charles Bassey", "G"); p != "G" {
		t.Fatal("name alone changed position")
	}
	if p, _ := CorrectPosition(17895854, "Other Player", "G"); p != "G" {
		t.Fatal("ID collision changed position")
	}
	dir := t.TempDir()
	raw := `{"min":"20","player":{"id":17895854,"first_name":"Charles","last_name":"Bassey","position":"G"},"team":{"id":10},"game":{"id":1,"date":"2026-11-18"}}`
	path := filepath.Join(dir, "stats_2026.jsonl")
	os.WriteFile(path, []byte(raw), 0600)
	rows, err := LoadStatsDir(dir)
	if err != nil || len(rows) != 1 || rows[0].Pos != "F-C" || rows[0].RawPosition != "G" || rows[0].PositionSource == "" {
		t.Fatal(rows, err)
	}
	saved, _ := os.ReadFile(path)
	if string(saved) != raw {
		t.Fatal("raw data rewritten")
	}
}
