package main

import (
	"strings"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/runlog"
)

func TestExplainDrop(t *testing.T) {
	prior := runlog.Play{PlayerID: 7, Stat: "points", Side: "OVER", Line: 22.5, Tier: "PICK", ProjMean: 25, ProjMin: 32}
	hi, minEdge := 0.55, 0.01

	if got := explainDrop(prior, nil, nil, map[int]string{7: "Out"}, hi, minEdge); !strings.Contains(got, "ruled out") {
		t.Errorf("out: %q", got)
	}
	if got := explainDrop(prior, nil, nil, nil, hi, minEdge); !strings.Contains(got, "no longer offered") {
		t.Errorf("gone: %q", got)
	}
	moved := []scored{{PlayerID: 7, Stat: data.Points, Side: "OVER", Line: 24.5, Prob: 0.52, Edge: -0.01, ProjMean: 24.8, ProjMin: 31}}
	got := explainDrop(prior, moved, nil, nil, hi, minEdge)
	for _, want := range []string{"line moved 22.5 to 24.5", "probability fell to 52%"} {
		if !strings.Contains(got, want) {
			t.Errorf("moved: %q missing %q", got, want)
		}
	}
	flipped := []scored{{PlayerID: 7, Stat: data.Points, Side: "UNDER", Line: 22.5, Prob: 0.57, Edge: 0.03, ProjMean: 22.0, ProjMin: 26}}
	got = explainDrop(prior, flipped, nil, nil, hi, minEdge)
	for _, want := range []string{"now leans UNDER", "projection 25.0 to 22.0", "minutes 32 to 26"} {
		if !strings.Contains(got, want) {
			t.Errorf("flipped: %q missing %q", got, want)
		}
	}
	capped := []scored{{PlayerID: 7, Stat: data.Points, Side: "OVER", Line: 22.5, Prob: 0.60, Edge: 0.05, ProjMean: 25, ProjMin: 32}}
	if got := explainDrop(prior, capped, nil, nil, hi, minEdge); !strings.Contains(got, "lost its place") {
		t.Errorf("capped: %q", got)
	}
}
