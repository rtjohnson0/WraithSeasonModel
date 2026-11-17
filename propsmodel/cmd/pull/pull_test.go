package main

import (
	"os"
	"testing"
)

func TestIncrementalHelpers(t *testing.T) {
	if got := backDays("2026-10-21", 2); got != "2026-10-19" {
		t.Fatal(got)
	}
	if got := backDays("2026-03-01", 2); got != "2026-02-27" {
		t.Fatal(got)
	}
	f := t.TempDir() + "/s.jsonl"
	if err := os.WriteFile(f, []byte("{\"game\":{\"date\":\"2026-10-20T23:30:00.000Z\"}}\n{\"game\":{\"date\":\"2026-10-21T23:30:00.000Z\"}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := maxGameDate(f); got != "2026-10-21" {
		t.Fatal(got)
	}
	if maxGameDate("/nonexistent/file") != "" {
		t.Fatal("missing file should give empty date")
	}
}
