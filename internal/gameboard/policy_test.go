package gameboard

import (
	"os"
	"path/filepath"
	"propsmodel/internal/team"
	"testing"
)

func TestGameV2DoesNotInheritLegacyRecord(t *testing.T) {
	dir := t.TempDir()
	old := Result{Decision: Decision{GameID: 7}, Result: "WIN"}
	if err := writeJSONL(filepath.Join(dir, "gamesettled.jsonl"), []Result{old}); err != nil {
		t.Fatal(err)
	}
	if err := (Snapshot{Policy: team.CurrentGamePolicy}).Save(dir); err != nil {
		t.Fatal(err)
	}
	rs, err := LoadResults(dir)
	if err != nil || len(rs) != 0 {
		t.Fatal("legacy results leaked", rs, err)
	}
	archive, err := LoadPreviousResults(dir)
	if err != nil || len(archive) != 1 {
		t.Fatal(archive, err)
	}
	_, name := recordFiles(team.CurrentGamePolicy)
	if err := writeJSONL(filepath.Join(dir, name), []Result{{Decision: Decision{Policy: team.CurrentGamePolicy, GameID: 8}, Result: "LOSS"}}); err != nil {
		t.Fatal(err)
	}
	rs, err = LoadResults(dir)
	if err != nil || len(rs) != 1 || rs[0].GameID != 8 {
		t.Fatal(rs, err)
	}
	if _, err = os.Stat(filepath.Join(dir, "gamesettled.jsonl")); err != nil {
		t.Fatal("legacy record removed")
	}
}
