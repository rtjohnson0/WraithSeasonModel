package props

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTipUsesEasternSlateDateAtUTCMidnight(t *testing.T) {
	for _, tc := range []struct{ name, date, datetime, status, want string }{
		{"winter 7 PM", "2026-11-12", "2026-11-13T00:00:00Z", "", "2026-11-13T00:00:00Z"},
		{"summer 8 PM", "2026-06-12", "2026-06-13T00:00:00Z", "", "2026-06-13T00:00:00Z"},
		{"explicit offset", "2026-11-12", "2026-11-12T19:00:00-05:00", "", "2026-11-13T00:00:00Z"},
		{"status fallback", "2026-11-12", "2026-11-12T00:00:00Z", "2026-11-13T00:00:00Z", "2026-11-13T00:00:00Z"},
		{"date placeholder", "2026-11-12", "2026-11-12T00:00:00Z", "Final", ""},
		{"bare date", "2026-11-12", "2026-11-12", "7:00 PM", ""},
		{"wrong day", "2026-11-12", "2026-11-14T01:00:00Z", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := startTime(tc.datetime, tc.status, tc.date); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// This is the failure path behind a scheduled game with a blank tip and no props:
// a real midnight-UTC start was lost before both the snapshot loader and safety gate.
func TestSevenPMPropsSurviveCaptureAndPregameChecks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "props_2026-11-12.jsonl")
	row := func(at string, line float64) string {
		return fmt.Sprintf(`{"captured_at":%q,"game_id":9,"game":{"id":9,"date":"2026-11-12","datetime":"2026-11-13T00:00:00Z","home_team":{"id":1},"visitor_team":{"id":2}},"response":{"data":[{"player_id":7,"vendor":"draftkings","prop_type":"points","line_value":%v,"updated_at":%q,"market":{"type":"over_under","over_odds":-110,"under_odds":-110}}]}}`, at, line, at) + "\n"
	}
	if err := os.WriteFile(path, []byte(row("2026-11-12T23:55:00Z", 24.5)+row("2026-11-13T00:01:00Z", 15.5)), 0600); err != nil {
		t.Fatal(err)
	}
	games, err := LoadGames(path)
	if err != nil || games[9].Start != "2026-11-13T00:00:00Z" {
		t.Fatalf("lost tip: %+v %v", games, err)
	}
	now := instant("2026-11-12T23:58:00Z")
	lines, err := LoadPregameSnapshot(path, false, now, time.Hour)
	if err != nil || len(lines) != 1 || lines[0].Line != 24.5 {
		t.Fatalf("lost pregame quote: %+v %v", lines, err)
	}
	if got := ExecutableLines(lines, games, now, time.Hour, time.Hour); len(got) != 1 {
		t.Fatal("valid 7 PM quote rejected")
	}
	if got := ExecutableLines(lines, games, instant(games[9].Start), time.Hour, time.Hour); len(got) != 0 {
		t.Fatal("live quote accepted")
	}
}
