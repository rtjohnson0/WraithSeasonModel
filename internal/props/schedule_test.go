package props

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func scheduleRows() []json.RawMessage {
	var rows []json.RawMessage
	for id := 1; id <= 4; id++ {
		tip := "2026-11-13T00:00:00Z"
		if id == 3 {
			tip = ""
		}
		if id == 4 {
			tip = "2026-11-12T15:00:00Z"
		}
		rows = append(rows, json.RawMessage(fmt.Sprintf(`{"id":%d,"date":"2026-11-12","datetime":%q,"home_team":{"id":%d,"abbreviation":"H"},"visitor_team":{"id":%d,"abbreviation":"A"}}`, id, tip, id*2, id*2+1)))
	}
	return rows
}
func TestScheduleKeepsFourGamesWhenOnlyTwoHaveProps(t *testing.T) {
	dir := t.TempDir()
	rows := scheduleRows()
	at := "2026-11-12T16:00:00Z"
	now, _ := time.Parse(time.RFC3339, at)
	if e := SaveSchedule(dir, "2026-11-12", at, rows); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "props_2026-11-12.jsonl")
	body := ""
	for i := 0; i < 2; i++ {
		body += fmt.Sprintf(`{"game_id":%d,"game":%s,"response":{"data":[]}}`, i+1, rows[i]) + "\n"
	}
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	games, complete, captured, e := LoadSlateGames(path, "2026-11-12", now)
	if e != nil || len(games) != 4 || !complete || captured != at || games[3].Start != "" || games[4].Start == "" {
		t.Fatalf("incomplete slate: %d %v %v", len(games), complete, e)
	}
	// No prop requests succeeding must still preserve the independent schedule.
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	games, complete, _, e = LoadSlateGames(path, "2026-11-12", now)
	if e != nil || !complete || len(games) != 4 {
		t.Fatal("missing prop file removed scheduled games")
	}
	// A newer authoritative zero-game response must clear old quote-only games.
	if e = SaveSchedule(dir, "2026-11-12", at, nil); e != nil {
		t.Fatal(e)
	}
	games, complete, _, e = LoadSlateGames(path, "2026-11-12", now)
	if e != nil || !complete || len(games) != 0 {
		t.Fatal("zero-game schedule not authoritative")
	}
}
func TestScheduleFallbackAndFutureCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "props_2026-11-12.jsonl")
	now, _ := time.Parse(time.RFC3339, "2026-11-12T16:00:00Z")
	games, complete, _, e := LoadSlateGames(path, "2026-11-12", now)
	if e != nil || complete || len(games) != 0 {
		t.Fatal("missing schedule claimed complete")
	}
	if e = SaveSchedule(dir, "2026-11-12", "2026-11-12T17:00:00Z", scheduleRows()); e != nil {
		t.Fatal(e)
	}
	if _, _, _, e = LoadSlateGames(path, "2026-11-12", now); e == nil {
		t.Fatal("future schedule used")
	}
}
