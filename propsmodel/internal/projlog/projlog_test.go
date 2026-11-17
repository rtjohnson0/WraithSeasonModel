package projlog

import (
	"path/filepath"
	"testing"
)

func TestSaveReplacesSamePlayerAndDay(t *testing.T) {
	p := filepath.Join(t.TempDir(), "projlog.jsonl")
	e := func(date string, pid int, pm float64) Entry {
		return Entry{Date: date, PlayerID: pid, ProjMin: pm, Stats: map[string]StatProj{"points": {Raw: 10, Proj: 10}}}
	}
	if err := Save(p, []Entry{e("2026-10-26", 1, 20), e("2026-10-26", 2, 30)}); err != nil {
		t.Fatal(err)
	}
	if err := Save(p, []Entry{e("2026-10-26", 1, 25), e("2026-10-27", 1, 22)}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || len(got) != 3 {
		t.Fatalf("want 3 entries (one replaced), got %d %v", len(got), err)
	}
	for _, g := range got {
		if g.Date == "2026-10-26" && g.PlayerID == 1 && g.ProjMin != 25 {
			t.Errorf("the later run should replace the earlier one, got %v", g.ProjMin)
		}
	}
}
