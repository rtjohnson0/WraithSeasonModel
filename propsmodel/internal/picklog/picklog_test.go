package picklog

import (
	"path/filepath"
	"testing"
)

func TestAppendDedupes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.jsonl")
	e := Entry{Date: "2026-10-20", PlayerID: 1, Player: "A", Stat: "points", Side: "OVER", Line: 20.5, Book: "dk", Prob: 0.58}
	n, err := Append(path, []Entry{e, e})
	if err != nil || n != 1 {
		t.Fatalf("first append: n=%d err=%v", n, err)
	}
	n, _ = Append(path, []Entry{e})
	if n != 0 {
		t.Errorf("repeat append should add nothing, added %d", n)
	}
	e2 := e
	e2.Line = 21.5
	n, _ = Append(path, []Entry{e2})
	if n != 1 {
		t.Errorf("a different line is a different pick, added %d", n)
	}
	all, _ := Load(path)
	if len(all) != 2 || all[0].LoggedAt == "" {
		t.Errorf("load: %+v", all)
	}
}

func TestGradeAndSummarize(t *testing.T) {
	over := Entry{Date: "d", PlayerID: 1, Stat: "points", Side: "OVER", Line: 20.5, Odds: -110, Units: 1, Prob: 0.6}
	under := Entry{Date: "d", PlayerID: 2, Stat: "points", Side: "UNDER", Line: 20.5, Odds: -110, Units: 2, Prob: 0.6}
	rows := []Settled{
		Grade(over, 25, 21.5, true),  // win, line moved up: beat the close by 1.0
		Grade(under, 25, 21.5, true), // loss, closing line is higher, so UNDER got a worse number: CLV -1
		Grade(over, 20.5, 0, true),   // push (cannot really happen on .5, but handles whole lines)
		Grade(over, 0, 0, false),     // void
	}
	rows[2].Line = 20.5
	if rows[0].Result != "WIN" || rows[1].Result != "LOSS" || rows[3].Result != "VOID" {
		t.Fatalf("results: %v %v %v", rows[0].Result, rows[1].Result, rows[3].Result)
	}
	if rows[0].CLV != 1 || rows[1].CLV != -1 {
		t.Errorf("CLV: %v %v", rows[0].CLV, rows[1].CLV)
	}
	s := Summarize(rows)
	if s.Wins != 1 || s.Losses != 1 || s.Voids != 1 {
		t.Errorf("summary counts: %+v", s)
	}
	want := Payout(-110) - 2 // +0.909 on 1u, -2 on 2u
	if d := s.Profit - want; d > 1e-9 || d < -1e-9 {
		t.Errorf("profit %v, want %v", s.Profit, want)
	}
	if s.CLVN != 2 || s.AvgCLV != 0 {
		t.Errorf("CLV summary: %+v", s)
	}
}

func TestFlags(t *testing.T) {
	got := Flags("out: A. Duren; opponent out: R. Gobert (starter), N. Reid; back-to-back")
	want := []string{"teammates-out", "opponent-out", "opponent-starter-out", "back-to-back"}
	if len(got) != len(want) {
		t.Fatalf("flags = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flags = %v, want %v", got, want)
		}
	}
	if HasFlag("opponent out: N. Reid", "teammates-out") {
		t.Error("an opponent absence must not count as a teammate absence")
	}
	if len(Flags("")) != 0 {
		t.Error("an empty note has no flags")
	}
}
