package gamepace

import (
	"fmt"
	"propsmodel/internal/data"
	"testing"
)

func TestResidualCorrectionUsesOnlyEarlierDatesAndShrinks(t *testing.T) {
	rows := []Replay{{Date: "2026-11-20", GameID: 1, Margin: 3, Total: 220, ActualMargin: 5, ActualTotal: 230}}
	m, total, n := ResidualCorrection(rows, "2026-11-21")
	if n != 1 || m <= 0 || m >= 2 || total <= 0 || total >= 10 {
		t.Fatal(m, total, n)
	}
	rows = append(rows, Replay{Date: "2026-11-21", GameID: 2, ActualTotal: 999}, Replay{Date: "2026-11-22", GameID: 3, ActualTotal: 999}, rows[0])
	a, b, c := ResidualCorrection(rows, "2026-11-21")
	if a != m || b != total || c != n {
		t.Fatal("future, same-day or duplicate outcomes leaked")
	}
}
func TestResidualStudyHasMatchedCohortsAndWarmup(t *testing.T) {
	var rows []Replay
	for d := 1; d <= 28; d++ {
		for g := 0; g < 10; g++ {
			rows = append(rows, Replay{Date: fmt.Sprintf("2026-10-%02d", d), GameID: d*10 + g, Margin: 0, ActualMargin: 2, Total: 220, ActualTotal: 230})
		}
	}
	s := StudyResiduals(rows)
	if s.Games == 0 || s.CorrectedTotalMAE >= s.RawTotalMAE || s.CorrectedMarginMAE >= s.RawMarginMAE {
		t.Fatal(s)
	}
	for _, r := range rows {
		if data.DayNumber(r.Date) == 0 {
			t.Fatal("bad fixture")
		}
	}
}
