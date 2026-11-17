package accuracy

import (
	"fmt"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"testing"
)

func TestSummaryPricesSidesPushesAndProjectionErrors(t *testing.T) {
	rows := []Observation{
		{Result: backtest.Result{Date: "2026-01-01", Stat: data.Rebounds, POver: .7, Market: .5, Line: 5.5, Actual: 6, OverOdds: -110, UnderOdds: -110}, Pred: 7, Actual: 6, Minutes: 30, ActualMinutes: 32},
		{Result: backtest.Result{Date: "2026-01-02", Stat: data.Rebounds, POver: .3, Market: .5, Line: 5.5, Actual: 7, OverOdds: -110, UnderOdds: -110}, Pred: 4, Actual: 7, Minutes: 28, ActualMinutes: 33},
		{Result: backtest.Result{Date: "2026-01-03", Stat: data.Rebounds, POver: .7, Market: .5, Line: 6, Actual: 6, OverOdds: -110, UnderOdds: -110}},
	}
	s := Summary(rows, "rebounds", "ALL")
	if s.N != 2 || s.Days != 2 || s.HitRate != .5 || s.Bias != 1 || s.MinutesBias != 3.5 || s.Supported {
		t.Fatalf("bad summary %+v", s)
	}
	under := Summary(rows, "rebounds", "UNDER")
	if under.N != 1 || under.HitRate != 0 {
		t.Fatalf("under matched wrong outcome %+v", under)
	}
	if len(s.Bins) != 1 || s.Bins[0].N != 2 {
		t.Fatal(s.Bins)
	}
}
func TestOneDateCannotEstablishEvidence(t *testing.T) {
	var rows []Observation
	for i := 0; i < 500; i++ {
		actual := 10.
		if i%10 >= 6 {
			actual = 0
		}
		rows = append(rows, Observation{Result: backtest.Result{Date: "2026-01-01", Stat: data.Points, POver: .6, Market: .5, Line: 5.5, Actual: actual, OverOdds: -110, UnderOdds: -110}})
	}
	s := Summary(rows, "points", "OVER")
	if s.Supported || s.Days != 1 {
		t.Fatal(s)
	}
}
func TestMinutesExperimentUsesEarlierDatesAndUniquePlayerGames(t *testing.T) {
	var rows []Observation
	for i := 0; i < 90; i++ {
		day := fmt.Sprintf("2026-%02d-%02d", 1+i/28, 1+i%28)
		actual := 30.
		if i >= 60 {
			actual = 20
		}
		o := Observation{Result: backtest.Result{Date: day, GameID: i + 1, PlayerID: 1}, Minutes: 20, Typical: 30, ActualMinutes: actual}
		rows = append(rows, o, o)
	}
	trial := MinutesExperiment(rows)
	if trial.TrainN != 60 || trial.TestN != 30 || trial.Weight != 1 || trial.BaseMAE != 0 || trial.TrialMAE != 10 || trial.Supported {
		t.Fatalf("leakage/dedup failure %+v", trial)
	}
}
func TestGateDoesNotUseFutureOrDifferentConfig(t *testing.T) {
	r := Report{Through: "2026-01-01", Config: "pregame", ModelVersion: 4, Slices: []Slice{{Stat: "points", Side: "OVER", Supported: true}}}
	if ok, _ := r.Allows("points", "OVER", "2026-01-02", "pregame", 4); !ok {
		t.Fatal("expected matching proof")
	}
	for _, date := range []string{"2026-01-01", "2025-12-31"} {
		if ok, _ := r.Allows("points", "OVER", date, "pregame", 4); ok {
			t.Fatal("future evidence")
		}
	}
	if ok, _ := r.Allows("points", "UNDER", "2026-01-02", "pregame", 4); ok {
		t.Fatal("opposite side proof")
	}
	if ok, _ := r.Allows("points", "OVER", "2026-01-02", "lineup", 4); ok {
		t.Fatal("wrong config")
	}
}
