package availability

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatusesNeedFreshTimestampsAndUseLatestObservation(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-12T19:00:00Z")
	path := filepath.Join(t.TempDir(), "injuries.jsonl")
	body := `{"captured_at":"2026-11-12T18:55:00Z","row":{"player":{"id":1},"status":"Available"}}
{"captured_at":"2026-11-12T18:40:00Z","row":{"player":{"id":1},"status":"Out"}}
{"captured_at":"2026-11-12T17:00:00Z","row":{"player":{"id":2},"status":"Out"}}
{"row":{"player":{"id":3},"status":"Out"}}
{"captured_at":"2026-11-12T19:05:00Z","row":{"player":{"id":4},"status":"Out"}}
{"captured_at":"2026-11-12T18:55:00Z","row":{"player":{"id":5},"status":"Questionable"}}
{"captured_at":"2026-11-12T18:55:00Z","row":{"player":{"id":5},"status":"Out"}}
`
	if e := os.WriteFile(path, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	got := FreshStatuses(path, now, 30*time.Minute)
	if len(got) != 2 || got[1].Status != "Available" || got[5].Status != "conflicting reports" {
		t.Fatalf("stale, future, conflicting or untimed report accepted: %+v", got)
	}
}
