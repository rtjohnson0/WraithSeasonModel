package main

import (
	"strings"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/feat"
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

func TestDefenseNote(t *testing.T) {
	got := defenseNote(feat.DefRank{Rank: 3, Of: 30, Pct: 0.093, Pos: 4}, data.Rebounds)
	if got != "defense vs centers: 3rd-most rebounds allowed (+9%, of 30 teams)" {
		t.Errorf("most: %q", got)
	}
	got = defenseNote(feat.DefRank{Rank: 29, Of: 30, Pct: -0.07, Pos: 0, Early: true}, data.Threes)
	if got != "defense vs guards: 2nd-fewest 3-pointers allowed (-7%, of 30 teams, early season)" {
		t.Errorf("fewest: %q", got)
	}
	if ordinal(11) != "11th" || ordinal(22) != "22nd" || ordinal(1) != "1st" {
		t.Error("ordinals")
	}
}

func TestMoreCandidates(t *testing.T) {
	mk := func(pid, gid int, stat data.Stat, side string, edge float64) scored {
		return scored{PlayerID: pid, GameID: gid, Stat: stat, Side: side, Edge: edge, Prob: 0.56}
	}
	posted := []scored{mk(1, 10, data.Points, "UNDER", 0.05)}
	all := []scored{
		mk(1, 10, data.Points, "UNDER", 0.05), // already on the board
		mk(1, 10, data.Rebounds, "OVER", 0.04),
		mk(2, 10, data.Points, "OVER", 0.03),
		mk(3, 10, data.Assists, "UNDER", -0.01), // no edge
		mk(4, 10, data.Threes, "UNDER", 0.02),
	}
	got := moreCandidates(posted, all, 2, 2)
	if len(got) != 2 || got[0].PlayerID != 1 || got[0].Stat != data.Rebounds || got[1].PlayerID != 2 {
		t.Fatalf("want the two best new lines in edge order, got %+v", got)
	}
}

func TestOverWatchRows(t *testing.T) {
	all := []scored{
		{PlayerID: 1, GameID: 9, Player: "A", Stat: "threes", Side: "OVER", Line: 1.5, Odds: -157, Prob: 0.587, Edge: -0.024},
		{PlayerID: 2, GameID: 9, Player: "B", Stat: "assists", Side: "OVER", Line: 3.5, Odds: 101, Prob: 0.502, Edge: 0.004},
		{PlayerID: 3, GameID: 9, Player: "C", Stat: "points", Side: "UNDER", Line: 10.5, Odds: -120, Prob: 0.6, Edge: 0.03},
		{PlayerID: 4, GameID: 9, Player: "D", Stat: "points", Side: "OVER", Line: 12.5, Odds: -110, Prob: 0.56, Edge: 0.03},
	}
	picks := []scored{{PlayerID: 4, Stat: "points", Side: "OVER"}}
	off := []offRow{{PlayerID: 2, Player: "B", Stat: "assists", Side: "OVER", Line: 2.5, Median: 3.5, Books: 8, Odds: -105, Prob: 0.53, Edge: 0.01}}
	got := overWatchRows(all, picks, off, 0.55, 0.01, 10)
	if len(got) != 2 {
		t.Fatalf("want 2 rows (B off-market, A by edge), got %d: %+v", len(got), got)
	}
	if got[0].Player != "B" || !got[0].LowLine || got[0].Line != 2.5 {
		t.Fatalf("off-market Over should come first with the low line: %+v", got[0])
	}
	if got[1].Player != "A" || got[1].Status == "" {
		t.Fatalf("second row should be A with a status: %+v", got[1])
	}
	for _, r := range got {
		if r.Player == "C" || r.Player == "D" {
			t.Fatalf("Unders and plays already on the board must not be listed: %+v", r)
		}
	}
}
