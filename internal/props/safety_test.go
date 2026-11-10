package props

import (
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"testing"
	"time"
)

func instant(v string) time.Time { t, _ := time.Parse(time.RFC3339, v); return t }
func TestExecutableRejectsLiveStaleAndUnknownQuotes(t *testing.T) {
	now := instant("2026-11-10T19:00:00Z")
	g := GameInfo{Start: "2026-11-10T20:00:00Z"}
	good := Line{Period: "game", Line: 6.5, QuoteVerified: true, OverOdds: -110, UnderOdds: -110, CapturedAt: "2026-11-10T18:55:00Z", UpdatedAt: "2026-11-10T18:50:00Z"}
	if !Executable(good, g, now, time.Hour, 15*time.Minute) {
		t.Fatal("valid quote rejected")
	}
	for _, change := range []func(*Line){
		func(l *Line) { l.UpdatedAt = "2026-11-10T16:00:00Z" },
		func(l *Line) { l.CapturedAt = "2026-11-10T18:00:00Z" },
		func(l *Line) { l.CapturedAt = "2026-11-10T19:01:00Z" },
		func(l *Line) { l.Period = "1q" },
		func(l *Line) { l.UnderOdds = 0 },
		func(l *Line) { l.QuoteVerified = false },
	} {
		l := good
		change(&l)
		if Executable(l, g, now, time.Hour, 15*time.Minute) {
			t.Fatalf("unsafe quote accepted %+v", l)
		}
	}
	if Executable(good, g, instant(g.Start), time.Hour, time.Hour) {
		t.Fatal("accepted at tip")
	}
	if Executable(good, GameInfo{}, now, time.Hour, time.Hour) {
		t.Fatal("unknown start accepted")
	}
}
func TestConsensusCountsBookOnceAcrossFeeds(t *testing.T) {
	a := Line{GameID: 1, PlayerID: 2, Vendor: "draftkings", Stat: data.Points, Line: 20.5, UpdatedAt: "2026-11-10T12:00:00Z"}
	b := a
	b.Vendor = "sgo/draftkings"
	b.Line = 21.5
	b.UpdatedAt = "2026-11-10T13:00:00Z"
	c := a
	c.Vendor = "fanduel"
	got := UniqueBooks([]Line{a, b, c})
	if len(got) != 2 || got[0].Line != 21.5 {
		t.Fatalf("consensus inflated: %+v", got)
	}
}
func TestHistoricalSnapshotKeepsLatestPreTipInTimeOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "props.jsonl")
	row := func(at string, line float64) string {
		return fmt.Sprintf(`{"captured_at":"%s","game_id":9,"game":{"id":9,"datetime":"2026-11-10T20:00:00Z","home_team":{"id":1},"visitor_team":{"id":2}},"response":{"data":[{"player_id":7,"vendor":"dk","prop_type":"points","line_value":%v,"updated_at":"%s","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}]}}`, at, line, at) + "\n"
	}
	// A merge put an older capture last, after a live capture.
	body := row("2026-11-10T19:55:00Z", 24.5) + row("2026-11-10T20:05:00Z", 15.5) + row("2026-11-10T18:00:00Z", 23.5)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPregameSnapshot(path, false, time.Time{}, time.Hour)
	if err != nil || len(got) != 1 || got[0].Line != 24.5 {
		t.Fatalf("wrong historical line: %+v %v", got, err)
	}
}
