package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompactAdvancedDedupesAndKeepsLoaderFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "advanced_2026.jsonl")
	body := `{"period":0,"usage_percentage":0.2,"pace":99,"extra":"x","player":{"id":1},"game":{"id":10}}
{"period":1,"usage_percentage":0.9,"player":{"id":1},"game":{"id":10}}
{"period":0,"usage_percentage":0.25,"pace":100,"player":{"id":1},"game":{"id":10}}
{"usage_percentage":0.3,"player":{"id":2},"game":{"id":10}}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	before, after, err := CompactAdvanced(p)
	if err != nil || before != 4 || after != 2 {
		t.Fatalf("want 4 rows down to 2, got %d to %d err %v", before, after, err)
	}
	m, err := LoadAdvancedDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if m[AdvKey{1, 10}].Usage != 0.25 || m[AdvKey{1, 10}].Pace != 100 || m[AdvKey{2, 10}].Usage != 0.3 {
		t.Errorf("the latest whole-game row should survive: %+v", m)
	}
}

func TestCompactStatsKeepsLatestLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stats_2026.jsonl")
	body := `{"pts":10,"player":{"id":1},"game":{"id":10}}
{"pts":12,"player":{"id":1},"game":{"id":10}}
{"pts":5,"player":{"id":2},"game":{"id":10}}
`
	os.WriteFile(p, []byte(body), 0o644)
	before, after, err := CompactStats(p)
	if err != nil || before != 3 || after != 2 {
		t.Fatalf("got %d to %d err %v", before, after, err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "{\"pts\":12,\"player\":{\"id\":1},\"game\":{\"id\":10}}\n{\"pts\":5,\"player\":{\"id\":2},\"game\":{\"id\":10}}\n" {
		t.Errorf("unexpected file: %q", b)
	}
}
