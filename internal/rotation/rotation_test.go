package rotation

import (
	"fmt"
	"math"
	"propsmodel/internal/data"
	"testing"
)

func fixture(team int) ([]data.PlayerGame, map[int]int) {
	var rows []data.PlayerGame
	roster := map[int]int{}
	for day := 1; day <= 10; day++ {
		for p := 1; p <= 10; p++ {
			pid := team*100 + p
			roster[pid] = team
			m := 24.
			if day <= 3 {
				if p == 1 {
					m = 0
				}
				if p == 2 || p == 3 {
					m = 36
				}
			}
			rows = append(rows, data.PlayerGame{SourceFinal: true, TeamID: team, GameID: team*1000 + day, PlayerID: pid, Date: fmt.Sprintf("2026-11-%02d", day), Minutes: m})
		}
	}
	return rows, roster
}
func TestJointAbsenceRedistributionAcrossAllTeams(t *testing.T) {
	for team := 1; team <= 30; team++ {
		rows, roster := fixture(team)
		a := New(rows).Allocate(team, "2026-11-11", roster, map[int]bool{team*100 + 1: true})
		if !a.Valid || !a.Supported || a.MatchedGames != 3 {
			t.Fatal(team, a)
		}
		sum := 0.
		for pid, m := range a.Minutes {
			if roster[pid] != team || m < 0 || m > 48 {
				t.Fatal(pid, m)
			}
			sum += m
		}
		if math.Abs(sum-240) > 1e-7 || a.Minutes[team*100+1] != 0 {
			t.Fatal(a)
		}
		if a.Minutes[team*100+2] <= a.Minutes[team*100+4] {
			t.Fatal("observed replacement role was lost", a)
		}
	}
}
func TestFutureGamesCannotChangePastAllocation(t *testing.T) {
	rows, roster := fixture(1)
	ix := New(rows)
	a := ix.Allocate(1, "2026-11-08", roster, map[int]bool{101: true})
	future := append([]data.PlayerGame{}, rows...)
	for _, r := range rows {
		if r.Date == "2026-11-10" {
			r.Date = "2026-11-20"
			r.GameID += 10000
			future = append(future, r)
		}
	}
	b := New(future).Allocate(1, "2026-11-08", roster, map[int]bool{101: true})
	for pid, m := range a.Minutes {
		if math.Abs(m-b.Minutes[pid]) > 1e-9 {
			t.Fatal("future leakage")
		}
	}
}
func TestPartialBoxesAndMissingPlayerDoNotEstablishAbsence(t *testing.T) {
	rows, roster := fixture(1)
	filtered := []data.PlayerGame{}
	for _, r := range rows {
		if r.PlayerID == 101 && r.Minutes == 0 {
			continue
		}
		filtered = append(filtered, r)
	}
	a := New(filtered).Allocate(1, "2026-11-11", roster, map[int]bool{101: true})
	if !a.Valid || a.MatchedGames != 0 || a.Supported {
		t.Fatal("missing row was interpreted as confirmed absence", a)
	}
	partial := []data.PlayerGame{}
	for _, r := range rows {
		if r.PlayerID != 110 {
			partial = append(partial, r)
		}
	}
	if New(partial).Allocate(1, "2026-11-11", roster, nil).Valid {
		t.Fatal("partial team box accepted")
	}
}
func TestOvertimeIsNormalizedAndInsufficientRosterIsRejected(t *testing.T) {
	rows, roster := fixture(1)
	for i := range rows {
		rows[i].Minutes *= 265. / 240
	}
	a := New(rows).Allocate(1, "2026-11-11", roster, nil)
	sum := 0.
	for _, m := range a.Minutes {
		sum += m
	}
	if !a.Valid || math.Abs(sum-240) > 1e-7 {
		t.Fatal(a)
	}
	for pid := range roster {
		if pid >= 108 {
			delete(roster, pid)
		}
	}
	if New(rows).Allocate(1, "2026-11-11", roster, nil).Valid {
		t.Fatal("poor roster coverage accepted")
	}
}
func TestMultipleAbsencesAreJointAndUnseenCombinationsStayUnsupported(t *testing.T) {
	rows, roster := fixture(1)
	a := New(rows).Allocate(1, "2026-11-11", roster, map[int]bool{101: true, 104: true})
	if !a.Valid || a.Supported || a.MatchedGames != 0 || a.Minutes[101] != 0 || a.Minutes[104] != 0 {
		t.Fatal(a)
	}
	sum := 0.
	for _, m := range a.Minutes {
		sum += m
	}
	if math.Abs(sum-240) > 1e-7 {
		t.Fatal(sum)
	}
}
