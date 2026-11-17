package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPropsBudgetPausesWithoutLosingCachedGames(t *testing.T) {
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	if err = os.Mkdir(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	stats := `{"game":{"id":1,"season":2025}}` + "\n" + `{"game":{"id":2,"season":2025}}` + "\n"
	cached := `{"_game_id":1,"response":{"data":[]}}` + "\n"
	os.WriteFile(filepath.Join(dataDir, "stats_2025.jsonl"), []byte(stats), 0644)
	path := filepath.Join(dataDir, "opening_props_2025.jsonl")
	os.WriteFile(path, []byte(cached), 0644)
	// An expired budget must return before requesting the next game (nil client).
	if err := runProps(nil, []string{"-season", "2025", "-maxminutes", "0.000000000001"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != cached {
		t.Fatal("cached progress changed", string(got), err)
	}
	if !alreadyPulled(path)[1] {
		t.Fatal("cached game cannot resume")
	}
}
