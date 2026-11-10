package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"propsmodel/internal/bdl"
	"strings"
	"testing"
)

func TestFailedStatsDownloadPreservesCurrentFile(t *testing.T) {
	for _, appendMode := range []bool{false, true} {
		dir := t.TempDir()
		path := filepath.Join(dir, "stats_2026.jsonl")
		original := []byte("{\"saved\":true}\n")
		os.WriteFile(path, original, 0600)
		d, e := beginStatsDownload(dir, "stats_2026.jsonl", appendMode)
		if e != nil {
			t.Fatal(e)
		}
		d.writeRow([]byte(`{"new":true}`))
		d.Abort()
		b, _ := os.ReadFile(path)
		if string(b) != string(original) {
			t.Fatal("failed refresh changed saved inputs")
		}
		tmp, _ := filepath.Glob(filepath.Join(dir, "*.download"))
		if len(tmp) != 0 {
			t.Fatal("partial download left behind")
		}
	}
}
func TestSuccessfulStatsRefreshReplacesOnlyOnCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stats_2026.jsonl")
	os.WriteFile(path, []byte("{\"saved\":true}\n"), 0600)
	d, e := beginStatsDownload(dir, "stats_2026.jsonl", true)
	if e != nil {
		t.Fatal(e)
	}
	d.writeRow([]byte(`{"new":true}`))
	before, _ := os.ReadFile(path)
	if string(before) != "{\"saved\":true}\n" {
		t.Fatal("updated before commit")
	}
	if e = d.Commit(); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(path)
	if string(after) != "{\"saved\":true}\n{\"new\":true}\n" {
		t.Fatal("missing appended row")
	}
}

type emptyStatsTransport struct{}

func (emptyStatsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[],"meta":{}}`)), Header: make(http.Header)}, nil
}
func TestEmptyFullRefreshDoesNotReplaceExistingStats(t *testing.T) {
	previous, _ := os.Getwd()
	root := t.TempDir()
	if e := os.Chdir(root); e != nil {
		t.Fatal(e)
	}
	defer os.Chdir(previous)
	os.Mkdir("data", 0755)
	path := filepath.Join("data", "stats_2026.jsonl")
	original := []byte("{\"old\":true}\n")
	os.WriteFile(path, original, 0600)
	c := bdl.New("test")
	c.HTTP = &http.Client{Transport: emptyStatsTransport{}}
	c.Sleep = 0
	if e := runStats(c, []string{"-seasons", "2026"}); e == nil {
		t.Fatal("empty response accepted as replacement")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("empty refresh discarded last usable file")
	}
}
