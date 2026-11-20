package main

import (
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/gameboard"
	"propsmodel/internal/rotation"
	"testing"
	"time"
)

func TestFreshQuoteAndRotationBoundaries(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-19T18:00:00Z")
	tip := now.Add(time.Hour)
	q := gameboard.Quote{QuoteAt: now.Add(-time.Minute).Format(time.RFC3339), SourceUpdatedAt: now.Add(-time.Minute).Format(time.RFC3339), BookCount: 3, Tip: tip.Format(time.RFC3339)}
	if !fresh(q, now, tip) {
		t.Fatal("fresh quote rejected")
	}
	q.QuoteAt = now.Add(time.Second).Format(time.RFC3339)
	if fresh(q, now, tip) {
		t.Fatal("future capture accepted")
	}
	q.QuoteAt = now.Add(-time.Hour).Format(time.RFC3339)
	if fresh(q, now, tip) {
		t.Fatal("stale capture accepted")
	}
	a := rotation.Allocation{Team: 1, Date: "2026-11-19", Valid: true, RoleProjected: true, Minutes: map[int]float64{1: 48, 2: 48, 3: 48, 4: 48, 5: 48}}
	roster := map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}
	if validAllocation(a, 1, a.Date, roster) == nil {
		t.Fatal("valid allocation rejected")
	}
	a.Minutes[1] = math.NaN()
	if validAllocation(a, 1, a.Date, roster) != nil {
		t.Fatal("NaN workload accepted")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, rotation.EvidenceDirectory(rotation.ContextPolicy))
	os.MkdirAll(root, 0755)
	os.WriteFile(filepath.Join(root, "rotation_2026-11-19.json"), []byte(`{"Policy":"rotation-v4","Date":"2026-11-19","At":"2026-11-19T19:00:00Z"}`), 0600)
	if loadEvidence(dir, a.Date, now).Policy != "" {
		t.Fatal("future lineup capture accepted")
	}
}
