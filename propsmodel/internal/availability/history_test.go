package availability

import (
	"fmt"
	"propsmodel/internal/data"
	"testing"
	"time"
)

func fixture() []data.PlayerGame {
	var rows []data.PlayerGame
	for i := 0; i < 40; i++ {
		date := time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		m, mate := 24., 30.
		if i%5 == 4 {
			m, mate = 34, 0
		}
		mins := []float64{m, mate, 30, 30, 30, 30, 30, 20, 16, 0}
		mins[9] = 240.
		for j := 0; j < 9; j++ {
			mins[9] -= mins[j]
		}
		for j, v := range mins {
			rows = append(rows, data.PlayerGame{SourceFinal: true, PlayerID: j + 1, TeamID: 1, GameID: i + 1, Date: date, Minutes: v})
		}
	}
	return rows
}
func TestHistoricalComparisonUsesObservedEarlierSameTeamMinutes(t *testing.T) {
	rows := fixture()
	ix := New(rows)
	e := ix.Compare(1, 1, 2, "2026-11-10")
	if !e.Supported || e.WithAvg != 24 || e.WithoutAvg != 34 || e.WithMinutes != 24 || e.WithoutMinutes != 34 {
		t.Fatalf("not observed averages: %+v", e)
	}
	if e.WithN < 5 || e.WithoutN < 3 {
		t.Fatal("unsupported counts")
	}
	// Alter all future outcomes and add another team's players; nothing changes.
	prior := ix.Compare(1, 1, 2, "2026-10-25")
	for i := range rows {
		if rows[i].Date >= "2026-10-25" {
			rows[i].Minutes = 99
		}
	}
	rows = append(rows, data.PlayerGame{TeamID: 2, PlayerID: 1, GameID: 999, Date: "2026-10-10", Minutes: 48})
	next := New(rows).Compare(1, 1, 2, "2026-10-25")
	if fmt.Sprint(prior) != fmt.Sprint(next) {
		t.Fatal("future or other-team outcomes changed history")
	}
}
func TestPartialBoxesAndThinEvidenceProduceNoNumericalScenarios(t *testing.T) {
	e := New(fixture()[:50]).Compare(1, 1, 2, "2026-11-10")
	if e.Supported || e.WithMinutes != 0 || e.WithProbability != nil {
		t.Fatal("thin history fabricated adjustment")
	}
	rows := fixture()
	var partial []data.PlayerGame
	for _, g := range rows {
		if g.PlayerID <= 7 {
			partial = append(partial, g)
		}
	}
	e = New(partial).Compare(1, 1, 2, "2026-11-10")
	if e.Supported || e.WithN != 0 || e.WithoutN != 0 {
		t.Fatal("partial download inferred absence")
	}
}

func TestNoFinalProvenanceAndIncompleteOvertimeAreRejected(t *testing.T) {
	rows := fixture()
	for i := range rows {
		rows[i].SourceFinal = false
	}
	e := New(rows).Compare(1, 1, 2, "2026-11-10")
	if e.WithN != 0 || e.WithoutN != 0 {
		t.Fatal("legacy unknown final status entered history")
	}
	rows = fixture()
	for i := range rows {
		if rows[i].PlayerID == 3 {
			rows[i].Minutes += 7
		}
	}
	e = New(rows).Compare(1, 1, 2, "2026-11-10")
	if e.WithN != 0 || e.WithoutN != 0 {
		t.Fatal("partial overtime box passed completeness guard")
	}
}
