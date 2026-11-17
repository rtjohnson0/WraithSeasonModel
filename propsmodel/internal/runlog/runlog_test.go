package runlog

import (
	"path/filepath"
	"testing"
	"time"
)

func pl(id int, side string, tier string) Play {
	return Play{PlayerID: id, Player: "P", Stat: "points", Side: side, Tier: tier, Line: 20.5}
}

func TestDiff(t *testing.T) {
	prev := []Play{pl(1, "OVER", "PICK"), pl(2, "UNDER", "PICK"), pl(3, "OVER", "PAPER")}
	cur := []Play{pl(1, "OVER", "CHECK"), pl(3, "UNDER", "PAPER"), pl(4, "OVER", "PICK")}
	kept, added, dropped := Diff(prev, cur)
	if len(kept) != 1 || kept[0].PlayerID != 1 {
		t.Errorf("kept = %v", kept)
	}
	if len(added) != 2 {
		t.Errorf("added = %v (a flipped side is a new play)", added)
	}
	if len(dropped) != 2 {
		t.Errorf("dropped = %v, want players 2 and 3 (OVER)", dropped)
	}
}

func TestAppearancesAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	r1 := Run{Date: "2026-10-26", Label: "9 AM", Plays: []Play{pl(1, "OVER", "PICK"), pl(2, "OVER", "PICK")}}
	r2 := Run{Date: "2026-10-26", Label: "3 PM", Plays: []Play{pl(1, "OVER", "PICK")}}
	r3 := Run{Date: "2026-10-27", Label: "9 AM", Plays: []Play{pl(1, "OVER", "PICK")}}
	for _, r := range []Run{r1, r2, r3} {
		if err := Append(path, r); err != nil {
			t.Fatal(err)
		}
	}
	all, err := Load(path)
	if err != nil || len(all) != 3 {
		t.Fatalf("load: %v %d", err, len(all))
	}
	day := ForDate(all, "2026-10-26")
	count, seen := Appearances(day)
	if count[pl(1, "OVER", "").Key()] != 2 || count[pl(2, "OVER", "").Key()] != 1 {
		t.Errorf("counts = %v", count)
	}
	if got := seen[pl(1, "OVER", "").Key()]; len(got) != 2 || got[0] != "9 AM" || got[1] != "3 PM" {
		t.Errorf("seenIn = %v", got)
	}
}

func TestLabelFor(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tz database")
	}
	for _, c := range []struct {
		h, m int
		want string
	}{{9, 17, "9 AM"}, {8, 17, "9 AM"}, {15, 17, "3 PM"}, {14, 17, "3 PM"}, {18, 17, "6 PM"}, {17, 17, "6 PM"}, {20, 17, "8 PM"}, {19, 17, "8 PM"}} {
		got, _ := LabelFor(time.Date(2026, 10, 26, c.h, c.m, 0, 0, loc))
		if got != c.want {
			t.Errorf("%d:%02d -> %s, want %s", c.h, c.m, got, c.want)
		}
	}
}

func TestAppearancesCountsEachScheduledRunOnce(t *testing.T) {
	// The 3 PM slot ran twice (a manual re-run); the play is on both but counts once for 3 PM.
	runs := []Run{
		{Label: "3 PM", Plays: []Play{pl(1, "OVER", "PICK"), pl(2, "OVER", "PICK")}},
		{Label: "3 PM", Plays: []Play{pl(1, "OVER", "PICK")}},
	}
	count, seen := Appearances(runs)
	k1, k2 := pl(1, "OVER", "").Key(), pl(2, "OVER", "").Key()
	if count[k1] != 1 || len(seen[k1]) != 1 {
		t.Errorf("play 1: count %d seenIn %v, want one 3 PM appearance", count[k1], seen[k1])
	}
	if count[k2] != 0 {
		t.Errorf("play 2 was dropped by the later 3 PM run, got count %d", count[k2])
	}
}
