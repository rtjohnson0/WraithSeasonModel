package picklog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPartialMarketsExcludedFromSettlementAndRecord(t *testing.T) {
	full := Entry{Stat: "points", Side: "OVER", Line: 24.5, Units: 1}
	partial := Entry{Stat: "points", Period: "1q", Side: "OVER", Line: 6.5, Units: 1}
	bad := Grade(partial, 30, 7.5, true)
	if bad.Result != "EXCLUDED" || bad.Profit != 0 || bad.Flat != 0 || bad.Actual != 0 {
		t.Fatalf("graded partial: %+v", bad)
	}
	sum := Summarize([]Settled{Grade(full, 30, 25.5, true), {Entry: partial, Result: "WIN", Profit: 99, Flat: 99}})
	if sum.Wins != 1 || sum.Picks != 1 || sum.Profit >= 1 {
		t.Fatalf("polluted record: %+v", sum)
	}
	path := filepath.Join(t.TempDir(), "settled.jsonl")
	if err := os.WriteFile(path, []byte("{\"stat\":\"points\",\"result\":\"WIN\"}\n{\"stat\":\"points\",\"period\":\"1q\",\"result\":\"WIN\"}\n{\"stat\":\"points_1q\",\"result\":\"WIN\"}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadSettled(path)
	if err != nil || len(rows) != 1 {
		t.Fatalf("load: %+v %v", rows, err)
	}
}

func TestNewPicksPersistFullGamePeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "picks.jsonl")
	if _, err := Append(path, []Entry{{Stat: "points"}}); err != nil {
		t.Fatal(err)
	}
	rows, err := Load(path)
	if err != nil || len(rows) != 1 || rows[0].Period != "game" {
		t.Fatalf("load: %+v %v", rows, err)
	}
	if _, err := Append(path, []Entry{{Stat: "points", Period: "1q"}}); err == nil {
		t.Fatal("accepted partial pick")
	}
}
