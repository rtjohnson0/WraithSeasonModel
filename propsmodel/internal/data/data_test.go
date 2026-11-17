package data

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseMinutes(t *testing.T) {
	cases := map[string]float64{
		`"34"`:    34,
		`34`:      34,
		`"34:30"`: 34.5,
		`"00"`:    0,
		`""`:      0,
		`null`:    0,
		`"abc"`:   0,
	}
	for in, want := range cases {
		if got := ParseMinutes(json.RawMessage(in)); math.Abs(got-want) > 1e-9 {
			t.Errorf("ParseMinutes(%s) = %v, want %v", in, got, want)
		}
	}
}

func TestSeasonOf(t *testing.T) {
	cases := map[string]int{
		"2025-10-21": 2025,
		"2026-01-15": 2025,
		"2026-06-10": 2025,
		"2026-10-20": 2026,
	}
	for in, want := range cases {
		if got := SeasonOf(in); got != want {
			t.Errorf("SeasonOf(%s) = %d, want %d", in, got, want)
		}
	}
}

func TestLoadStatsDir(t *testing.T) {
	dir := t.TempDir()
	rows := `{"min":"34:12","pts":25,"reb":5,"ast":7,"player":{"id":1,"first_name":"A","last_name":"B","team_id":9},"team":{"id":9},"game":{"id":100,"date":"2025-10-21","season":2025}}
{"min":"00","pts":0,"reb":0,"ast":0,"player":{"id":2,"first_name":"C","last_name":"D"},"team":{"id":9},"game":{"id":100,"date":"2025-10-21","season":2025}}
{"min":"30","pts":10,"reb":2,"ast":1,"player":{"id":1,"first_name":"A","last_name":"B"},"team":{"id":9},"game":{"id":101,"date":"2025-10-23T23:30:00.000Z","season":2025}}
{"min":"34:12","pts":25,"reb":5,"ast":7,"player":{"id":1,"first_name":"A","last_name":"B","team_id":9},"team":{"id":9},"game":{"id":100,"date":"2025-10-21","season":2025}}
`
	if err := os.WriteFile(filepath.Join(dir, "stats_2025.jsonl"), []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadStatsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 played, de-duplicated rows, got %d: %+v", len(got), got)
	}
	if got[0].GameID != 100 || got[1].GameID != 101 {
		t.Errorf("rows should be sorted by date: %+v", got)
	}
	if got[1].Date != "2025-10-23" {
		t.Errorf("timestamp should be trimmed to a date, got %q", got[1].Date)
	}
	if math.Abs(got[0].Minutes-34.2) > 1e-9 || got[0].Name != "A B" {
		t.Errorf("unexpected first row: %+v", got[0])
	}
	if _, err := LoadStatsDir(t.TempDir()); err == nil {
		t.Error("expected an error when no stats files exist")
	}
}

func TestComboValuesAddUp(t *testing.T) {
	g := PlayerGame{Pts: 24, Reb: 9, Ast: 7, Fg3m: 4}
	want := map[Stat]float64{PRA: 40, PR: 33, PA: 31, RA: 16, Points: 24, Threes: 4}
	for s, w := range want {
		if got := g.Value(s); got != w {
			t.Errorf("%s = %v, want %v", s, got, w)
		}
	}
	if Points.IsCombo() || !PRA.IsCombo() || len(PRA.Parts()) != 3 || len(RA.Parts()) != 2 {
		t.Error("IsCombo/Parts wrong")
	}
}
