package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"propsmodel/internal/gameboard"
	"testing"
	"time"
)

func TestGamePacePublishHelper(t *testing.T) {
	if os.Getenv("GAMEPACE_TEST_HELPER") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("gamepace", flag.ExitOnError)
	dir := os.Getenv("GAMEPACE_TEST_DIR")
	os.Args = []string{"gamepace", "-data", dir, "-out", filepath.Join(dir, "evidence")}
	main()
	os.Exit(0)
}

func TestCommandPublishesSeparateSnapshotWithoutReplacingBaseline(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()
	loc, _ := time.LoadLocation("America/New_York")
	date := now.In(loc).Format("2006-01-02")
	day, _ := time.Parse("2006-01-02", date)
	f, err := os.Create(filepath.Join(dir, "stats_2026.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for d := 400; d >= 1; d-- {
		for team := 1; team <= 2; team++ {
			for p := 1; p <= 5; p++ {
				pts := 20
				if team == 2 {
					pts = 18
				}
				r := map[string]any{"min": "48", "pts": pts, "reb": 4, "ast": 3, "fg3m": 2, "blk": 1, "fga": 18, "fta": 4, "oreb": 2, "turnover": 1, "team": map[string]any{"id": team}, "player": map[string]any{"id": team*100 + p, "first_name": "Test", "last_name": fmt.Sprint(team*100 + p), "position": "G"}, "game": map[string]any{"id": d, "date": day.AddDate(0, 0, -d).Format("2006-01-02"), "status": "Final", "home_team_id": 1, "visitor_team_id": 2, "home_team_score": 100, "visitor_team_score": 90}}
				if err = enc.Encode(r); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	f.Close()
	tip := now.Add(2 * time.Hour).Format(time.RFC3339Nano)
	at := now.Add(-time.Minute).Format(time.RFC3339Nano)
	quotes := []gameboard.Quote{}
	for _, mk := range []string{"moneyline", "spread", "total"} {
		line := 0.
		labels := [2]string{"HOME", "AWAY"}
		if mk == "spread" {
			line = -4.5
		}
		if mk == "total" {
			line = 215.5
			labels = [2]string{"Over", "Under"}
		}
		quotes = append(quotes, gameboard.Quote{Market: mk, Book: "fixture", Line: line, Tip: tip, QuoteAt: at, SourceUpdatedAt: at, BookCount: 3, Sides: gameboard.Pair(labels, [2]int{-110, -110}, [2]float64{.6, .4}, 0)})
	}
	baseline := gameboard.Snapshot{Policy: "game-v2", Date: date, At: at, Games: []gameboard.Game{{ID: 9999, HomeID: 1, AwayID: 2, Home: "HOME", Away: "AWAY", Tip: tip, Quotes: quotes}}}
	if err = baseline.Save(dir); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(dir, "gameboard.json"))
	cmd := exec.Command(os.Args[0], "-test.run=^TestGamePacePublishHelper$")
	cmd.Env = append(os.Environ(), "GAMEPACE_TEST_HELPER=1", "GAMEPACE_TEST_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	saved, _ := os.ReadFile(filepath.Join(dir, "gameboard.json"))
	if string(saved) != string(original) {
		t.Fatal("baseline replaced")
	}
	shadow, err := gameboard.LoadNamed(dir, date, "gamechallenger.json")
	if err != nil || shadow == nil || len(shadow.Games) != 1 || len(shadow.Games[0].Quotes) != 3 || shadow.Games[0].PaceContext == nil {
		t.Fatal(shadow, err)
	}
	if shadow.Games[0].PaceContext.LineupApplied {
		t.Fatal("missing lineup evidence invented an adjustment")
	}
	rows, err := os.ReadFile(filepath.Join(dir, "gamepace_picklog.jsonl"))
	if err != nil || len(rows) == 0 {
		t.Fatal("separate decisions not frozen", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "v2_gamepicklog.jsonl")); !os.IsNotExist(err) {
		t.Fatal("baseline record changed")
	}
}
