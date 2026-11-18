package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"propsmodel/internal/bdl"
	"testing"
	"time"
)

func TestContextBudgetResumesSavedCursorAndDoesNotResetCompletedWindow(t *testing.T) {
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(t.TempDir())
	requests := 0
	sawResume := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("cursor") == "1" {
			sawResume = true
			w.Write([]byte(`{"data":[{"id":2}],"meta":{}}`))
		} else {
			time.Sleep(3 * time.Millisecond)
			w.Write([]byte(`{"data":[{"id":1}],"meta":{"next_cursor":1}}`))
		}
	}))
	defer server.Close()
	c := bdl.New("test")
	c.BaseURL = server.URL
	c.Sleep = 0
	now := time.Now()
	feeds := []contextFeed{{"passing", "/nba/v1/stats/tracking", "passing"}}
	if e := pullContextPages(c, []int{dataSeasonForContext(now)}, feeds, time.Millisecond, now); e != nil {
		t.Fatal(e)
	}
	body, e := os.ReadFile(filepath.Join("data", "context", "progress.json"))
	if e != nil {
		t.Fatal(e)
	}
	state := map[string]contextCursor{}
	json.Unmarshal(body, &state)
	cursorSaved := false
	for _, s := range state {
		cursorSaved = cursorSaved || s.Cursor == "1"
	}
	if !cursorSaved {
		t.Fatal("progress lost", state)
	}
	if e := pullContextPages(c, []int{dataSeasonForContext(now)}, feeds, 0, now); e != nil {
		t.Fatal(e)
	}
	if !sawResume {
		t.Fatal("did not resume cursor")
	}
	before := requests
	if e := pullContextPages(c, []int{dataSeasonForContext(now)}, feeds, 0, now); e != nil {
		t.Fatal(e)
	}
	if requests != before {
		t.Fatal("completed windows re-downloaded")
	}
}
func dataSeasonForContext(now time.Time) int {
	y := now.Year()
	if now.Month() < 8 {
		y--
	}
	return y
}
