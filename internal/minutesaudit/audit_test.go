package minutesaudit

import (
	"math"
	"testing"
)

func obs(st string) Observation {
	return Observation{Cohort: "verified replay", Date: "2026-11-01", At: "2026-11-01T18:00:00Z", GameID: 7, PlayerID: 1, Stat: st, Minutes: 20, ActualMinutes: 30, Typical: 28, Projection: 10, Actual: 18, Line: 12.5}
}
func TestDecompositionAndUniqueMinutes(t *testing.T) {
	a := obs("points")
	b := obs("rebounds")
	b.Projection = 4
	b.Actual = 7
	mins, stats := Summary([]Observation{a, b})
	if len(mins) == 0 || mins[0].N != 1 {
		t.Fatal("minutes counted once per prop", mins)
	}
	if len(stats) != 2 {
		t.Fatal(stats)
	}
	m, r := Decompose(a)
	if m != 5 || r != 3 || m+r != a.Actual-a.Projection {
		t.Fatal(m, r)
	}
	if mins[0].Bias != -10 || mins[0].MAE != 10 || mins[0].TypicalMAE != 2 || mins[0].TypicalModelMAE != 10 {
		t.Fatal(mins[0])
	}
}
func TestEarliestRecordKeepsMatchingLineAndProbabilities(t *testing.T) {
	a := obs("rebounds")
	a.At = "2026-11-01T18:00:00Z"
	b := a
	b.At = "2026-11-01T18:00:00.100Z"
	b.Line = 30.5
	rows := Dedupe([]Observation{b, a})
	if len(rows) != 1 || rows[0].Line != 12.5 {
		t.Fatal(rows)
	}
}
func TestLegacyAndUnknownContextDoNotBecomeVerifiedAvailabilityEvidence(t *testing.T) {
	a := obs("points")
	a.Cohort = "legacy projection diagnostic"
	b := a
	b.Cohort = "verified replay"
	b.Context = &Context{Questionable: 1}
	mins, _ := Summary([]Observation{a, b})
	for _, m := range mins {
		if m.Cohort == "legacy projection diagnostic" && m.Group == "confirmed teammate out" {
			t.Fatal("inferred an absence")
		}
		if m.Group == "all player-games" && m.N != 1 {
			t.Fatal("cohorts mixed", mins)
		}
	}
}
func TestCalibrationAndDistributionSkewAreSeparate(t *testing.T) {
	a := obs("rebounds")
	a.Projection = 13
	a.RawOver = fp(.48)
	a.FinalOver = fp(.42)
	a.ZeroInterceptOver = fp(.51)
	_, ss := Summary([]Observation{a})
	s := ss[0]
	if s.RawUnderAboveMean != 1 || s.RawUnder != 1 || s.FinalUnder != 1 || s.CalibrationToUnder != 0 || s.InterceptToUnder != 1 {
		t.Fatal(s)
	}
}
func TestInvalidForecastCannotContaminateMeansAndDNPDoesNotBecomeAStatMiss(t *testing.T) {
	a := obs("points")
	a.ActualMinutes = 0
	a.Actual = 0
	b := obs("rebounds")
	b.Minutes = math.NaN()
	mins, ss := Summary([]Observation{a, b})
	if len(ss) != 0 || mins[0].N != 1 {
		t.Fatal(mins, ss)
	}
}
func fp(v float64) *float64 { return &v }
