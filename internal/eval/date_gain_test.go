package eval

import (
	"math"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"testing"
)

func TestDateGainDoesNotCountSameSlateAsIndependentEvidence(t *testing.T) {
	var a, b []backtest.Result
	for day := 1; day <= 4; day++ {
		for i := 0; i < 100; i++ {
			r := backtest.Result{Date: []string{"2026-11-10", "2026-11-11", "2026-11-12", "2026-11-13"}[day-1], GameID: day, PlayerID: i, Stat: data.PRA, Line: 20.5, Actual: 25, POver: .6}
			a = append(a, r)
			r.POver = .5
			b = append(b, r)
		}
	}
	mean, _, days := PairedGainByDate(a, b)
	if days != 4 || math.Abs(mean-math.Log(.6/.5)) > 1e-9 {
		t.Fatal(mean, days)
	}
	_, _, days = PairedGainByDate(a[:100], b[:100])
	if days != 1 {
		t.Fatal("one slate inflated evidence", days)
	}
}
