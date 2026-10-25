package main

import (
	"testing"

	"propsmodel/internal/team"
)

func row(v string, sp float64, tot float64, mlh, mla int, stamp string) team.Odds {
	return team.Odds{Vendor: v, SpreadHome: sp, SpreadAway: -sp, SpreadHomeOdds: -110, SpreadAwayOdds: -110, HasSpread: true,
		Total: tot, Over: -110, Under: -110, HasTotal: true, MLHome: mlh, MLAway: mla, Stamp: stamp}
}

func TestCleanRowsDropsLinesThatLookWrong(t *testing.T) {
	good := []team.Odds{
		row("dk", -5.5, 224.5, -220, 180, "2026-10-22T20:00:00Z"),
		row("fd", -5.5, 224.5, -225, 185, "2026-10-22T20:01:00Z"),
		row("mgm", -6.5, 225.5, -240, 195, "2026-10-22T20:02:00Z"),
		row("cz", -6.5, 225.5, -235, 190, "2026-10-22T20:03:00Z"),
	}
	c := team.ConsensusOf(good)[0]
	if _, d := cleanRows(good, c); d != 0 {
		t.Fatalf("clean rows were dropped: %d", d)
	}

	bad := append([]team.Odds(nil), good...)
	bad = append(bad,
		row("odd", -5.25, 224.5, -220, 180, "2026-10-22T20:00:00Z"),  // off the half-point grid
		row("far", -9.5, 224.5, -220, 180, "2026-10-22T20:00:00Z"),   // 3+ points from the market
		row("lone", -4.5, 224.5, -220, 180, "2026-10-22T20:00:00Z"),  // nobody else posts -4.5
		row("flip", -5.5, 224.5, 180, -220, "2026-10-22T20:00:00Z"),  // moneyline favourite is the spread underdog
		row("stale", -5.5, 224.5, -220, 180, "2026-10-22T10:00:00Z"), // 10 hours old
		row("sharp", -5.5, 238.5, -220, 180, "2026-10-22T20:00:00Z"), // total 14 points off
	)
	kept, dropped := cleanRows(bad, c)
	if dropped < 6 {
		t.Fatalf("dropped %d, want at least 6", dropped)
	}
	for _, o := range kept {
		switch o.Vendor {
		case "stale":
			t.Error("stale row survived")
		case "odd", "far", "lone":
			if o.HasSpread {
				t.Errorf("%s spread survived", o.Vendor)
			}
		case "flip":
			if o.MLHome != 0 {
				t.Error("flipped moneyline survived")
			}
		case "sharp":
			if o.HasTotal {
				t.Error("off total survived")
			}
		}
	}
}
