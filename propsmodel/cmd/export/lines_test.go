package main

import (
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/lineslog"
)

func TestSummariseLines(t *testing.T) {
	act := func(g, p int) (data.PlayerGame, bool) {
		if p == 1 {
			return data.PlayerGame{Pts: 25}, true
		}
		return data.PlayerGame{}, false
	}
	mk := []lineslog.Market{
		{Date: "2026-10-27", GameID: 1, PlayerID: 1, Stat: "points", Line: 20.5, POver: 0.52, Books: 8},
		{Date: "2026-10-27", GameID: 1, PlayerID: 2, Stat: "points", Line: 20.5, POver: 0.52, Books: 8}, // did not play
	}
	off := []lineslog.Off{{Date: "2026-10-27", GameID: 1, PlayerID: 1, Stat: "points", Side: "OVER", Line: 18.5, Median: 20.5, Books: 8, Odds: -110, PSide: 0.6}}
	rec := summariseLines(mk, off, act)
	if rec.Logged != 2 || rec.Graded != 1 || rec.Days != 1 || rec.Since != "2026-10-27" {
		t.Fatalf("counts wrong: %+v", rec)
	}
	if len(rec.Bins) != 1 || rec.Bins[0].Actual != 1 {
		t.Errorf("bins wrong: %+v", rec.Bins)
	}
	if rec.OffCount != 1 || rec.Off[0].Wins != 1 || rec.Off[0].Units <= 0.9 {
		t.Errorf("off-market wrong: %+v", rec.Off)
	}
}
