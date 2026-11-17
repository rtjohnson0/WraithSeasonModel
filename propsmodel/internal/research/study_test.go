package research

import (
	"fmt"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"testing"
)

func fixture() []Question {
	var qs []Question
	for i := 0; i < 1000; i++ {
		day := fmt.Sprintf("2025-%02d-%02d", 1+(i/10)/28, 1+(i/10)%28)
		a := 0.
		if i%10 < 5 {
			a = 10
		}
		q := Question{Means: []float64{5, 5, 5}, Minutes: []float64{30, 30, 30}, ActualMinutes: 31}
		for v := 0; v < 3; v++ {
			q.Results = append(q.Results, backtest.Result{Date: day, GameID: i + 1, PlayerID: 1, Stat: data.Points, Line: 5.5, Actual: a, POver: .5 + float64(i%3-1)*.02, Market: .5, OverOdds: -110, UnderOdds: -110})
		}
		qs = append(qs, q)
	}
	return qs
}
func TestChronologicalPhasesAndFinalTestCannotSelectChallenger(t *testing.T) {
	q := fixture()
	a, e := Run(q)
	if e != nil {
		t.Fatal(e)
	}
	if !(a.FitThrough < a.ValidationThrough && a.ValidationThrough < a.TestFrom) {
		t.Fatal("date leakage", a)
	}
	for i := 800; i < len(q); i++ {
		q[i].Results[1].POver = .99
	}
	b, e := Run(q)
	if e != nil {
		t.Fatal(e)
	}
	if a.Selected != b.Selected {
		t.Fatal("final test selected model")
	}
	if a.Passed {
		t.Fatal("identical challengers have no demonstrated improvement")
	}
}
func TestRejectsUnmatchedQuestions(t *testing.T) {
	q := fixture()
	q[0].Results[1].PlayerID = 42
	if _, e := Run(q); e == nil {
		t.Fatal("different outcomes must not be paired")
	}
}
func TestRejectsSingleDate(t *testing.T) {
	q := fixture()
	for i := range q {
		for j := range q[i].Results {
			q[i].Results[j].Date = "2025-01-01"
		}
	}
	if _, e := Run(q); e == nil {
		t.Fatal("cannot split one date")
	}
}
