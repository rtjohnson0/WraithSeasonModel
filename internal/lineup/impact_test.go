package lineup

import (
	"fmt"
	"math"
	"propsmodel/internal/data"
	"reflect"
	"testing"
)

func fixture(absentTeam int) ([]data.PlayerGame, map[int]int) {
	var rows []data.PlayerGame
	roster := map[int]int{}
	for day := 1; day <= 30; day++ {
		for tm := 1; tm <= 2; tm++ {
			for p := 1; p <= 10; p++ {
				id := tm*100 + p
				roster[id] = tm
				r := data.PlayerGame{AnalyticsReady: true, SourceFinal: true, PlayerID: id, TeamID: tm, GameID: day, Date: fmt.Sprintf("2026-09-%02d", day), Name: fmt.Sprintf("Player %d", id), Pos: "C", Minutes: 24, Pts: 12, Reb: 4, Ast: 3, Fg3m: 1, Blk: 1}
				if day >= 26 {
					if tm == absentTeam {
						if p == 1 {
							r.Minutes = 0
						}
						if p == 2 || p == 3 {
							r.Minutes = 36
						}
					}
					// Actual teammate role changes and opponent scoring allowances are distinct.
					if tm != absentTeam || p == 2 {
						r.Pts = 18
						r.Reb = 6
						r.Ast = 4
						r.Fg3m = .5
					}
				}
				if r.Minutes == 36 {
					r.Pts *= 1.5
					r.Reb *= 1.5
					r.Ast *= 1.5
					r.Fg3m *= 1.5
				}
				if r.Minutes == 0 {
					r.Pts = 0
					r.Reb = 0
					r.Ast = 0
					r.Fg3m = 0
					r.Blk = 0
				}
				rows = append(rows, r)
			}
		}
	}
	return rows, roster
}
func TestOwnAndOpponentAbsencesChangeAllStatRatesWithoutForcedDirection(t *testing.T) {
	rows, roster := fixture(2)
	ix := New(rows)
	for _, tc := range []struct {
		player   int
		opponent bool
	}{{202, false}, {101, true}} {
		got := ix.Estimate(2, tc.player, "2026-10-01", roster, []int{201}, tc.opponent)
		if len(got.Missing) != 1 || len(got.Effects) != 4 {
			t.Fatal(got)
		}
		for k, e := range got.Effects {
			if !e.Applied || e.Without != 5 || e.With < 5 {
				t.Fatal(e)
			}
			if k == 3 {
				if e.Scale >= 1 {
					t.Fatal("missing defender forced all Overs", e)
				}
			} else if e.Scale <= 1 {
				t.Fatal("observed production improvement lost", e)
			}
			if e.Scale < .95 || e.Scale > 1.05 {
				t.Fatal("unbounded effect", e)
			}
		}
	}
}
func TestEvidenceMissingRowsAndFutureGamesNeverBecomeAbsences(t *testing.T) {
	rows, roster := fixture(2)
	base := New(rows).Estimate(2, 101, "2026-10-01", roster, []int{201}, true)
	future := append([]data.PlayerGame{}, rows...)
	for _, r := range rows {
		r.GameID += 100
		r.Date = "2026-10-02"
		r.Pts *= 50
		future = append(future, r)
	}
	if got := New(future).Estimate(2, 101, "2026-10-01", roster, []int{201}, true); !reflect.DeepEqual(got, base) {
		t.Fatal("future leaked", got, base)
	}
	var missing []data.PlayerGame
	for _, r := range rows {
		if r.PlayerID == 201 && r.Minutes == 0 {
			continue
		}
		missing = append(missing, r)
	}
	got := New(missing).Estimate(2, 101, "2026-10-01", roster, []int{201}, true)
	for _, e := range got.Effects {
		if e.Applied || e.Without != 0 || e.Scale != 1 {
			t.Fatal("missing row inferred as absence", e)
		}
	}
}
func TestUnknownPositionThinSamplesAndNoAbsenceKeepRatesUnchanged(t *testing.T) {
	rows, roster := fixture(2)
	for i := range rows {
		if rows[i].PlayerID == 101 {
			rows[i].Pos = ""
		}
	}
	got := New(rows).Estimate(2, 101, "2026-10-01", roster, []int{201}, true)
	if len(got.Effects) != 0 {
		t.Fatal("guessed a position", got)
	}
	got = New(rows).Estimate(2, 202, "2026-09-29", roster, []int{201}, false)
	for _, e := range got.Effects {
		if e.Applied || e.Scale != 1 {
			t.Fatal("thin evidence changed forecast", e)
		}
	}
	if got := New(rows).Estimate(2, 202, "2026-10-01", roster, nil, false); len(got.Effects) != 0 {
		t.Fatal(got)
	}
}
func TestIncompleteGamesAndJointAbsenceSetsAreNotCombinedAsIndependentBonuses(t *testing.T) {
	rows, roster := fixture(2)
	ix := New(rows)
	// Player 202 played in every completed game; no joint missing examples exist.
	got := ix.Estimate(2, 101, "2026-10-01", roster, []int{201, 202}, true)
	for _, e := range got.Effects {
		if e.Applied || e.Without != 0 {
			t.Fatal("independent absences stacked", e)
		}
	}
	for i := range rows {
		if rows[i].Date >= "2026-09-26" && rows[i].PlayerID == 101 {
			rows[i].AnalyticsReady = false
		}
	}
	got = New(rows).Estimate(2, 102, "2026-10-01", roster, []int{201}, true)
	for _, e := range got.Effects {
		if e.Applied || e.Without != 0 {
			t.Fatal("incomplete stats accepted", e)
		}
	}
}
func TestCombinedScaleIsBounded(t *testing.T) {
	x := Impact{Effects: []Effect{{Stat: data.Points, Applied: true, Scale: 1.05}}}
	if got := Scale([]Impact{x, x})[0]; math.Abs(got-1.08) > 1e-9 {
		t.Fatal(got)
	}
}
