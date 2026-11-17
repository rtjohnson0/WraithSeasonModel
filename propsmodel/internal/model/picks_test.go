package model

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Reference values come from scipy's negative binomial in the original Python prototype.
func TestProbOverMatchesReference(t *testing.T) {
	pr, err := ProbOver(27, 45, 24.5)
	if err != nil {
		t.Fatal(err)
	}
	if !near(pr.Over, 0.6264224994855988, 1e-6) {
		t.Errorf("P(over 24.5) = %v, want ~0.62642", pr.Over)
	}
	if !near(pr.Over+pr.Under, 1, 1e-9) || pr.Push != 0 {
		t.Errorf("half-point line should sum to 1 with no push: %+v", pr)
	}

	pr, err = ProbOver(27, 45, 27.5)
	if err != nil {
		t.Fatal(err)
	}
	if !near(pr.Under, 0.5526164202923408, 1e-6) {
		t.Errorf("P(under 27.5) = %v, want ~0.55262", pr.Under)
	}
}

func TestWholeNumberLineHasPush(t *testing.T) {
	pr, err := ProbOver(27, 45, 27)
	if err != nil {
		t.Fatal(err)
	}
	if pr.Push <= 0 {
		t.Errorf("whole-number line should have a push probability, got %v", pr.Push)
	}
	if !near(pr.Over+pr.Under, 1, 1e-9) {
		t.Errorf("over+under should be renormalised to 1, got %v", pr.Over+pr.Under)
	}
}

func TestMakePick(t *testing.T) {
	p, err := MakePick("A", "points", 24.5, 27, 45, 31, 33)
	if err != nil || p == nil || p.Side != "OVER" {
		t.Fatalf("expected OVER pick, got %+v err=%v", p, err)
	}
	p, err = MakePick("A", "points", 27.5, 27, 45, 31, 33)
	if err != nil || p == nil || p.Side != "UNDER" {
		t.Fatalf("expected UNDER pick, got %+v err=%v", p, err)
	}
	// Inside the 45-55 band: no pick.
	p, err = MakePick("A", "points", 26.5, 27, 45, 31, 33)
	if err != nil || p != nil {
		t.Fatalf("expected no pick inside the band, got %+v err=%v", p, err)
	}
	// Not enough minutes: no pick.
	p, err = MakePick("B", "points", 24.5, 27, 45, 15, 16)
	if err != nil || p != nil {
		t.Fatalf("expected no pick for low-minute player, got %+v err=%v", p, err)
	}
}

func TestEligible(t *testing.T) {
	cases := []struct {
		avg, proj float64
		want      bool
	}{
		{20, 15, true}, {15, 18, true}, {19.9, 17.9, false}, {25, 30, true},
	}
	for _, c := range cases {
		if got := Eligible(c.avg, c.proj); got != c.want {
			t.Errorf("Eligible(%v, %v) = %v, want %v", c.avg, c.proj, got, c.want)
		}
	}
}

func TestCalibrationTable(t *testing.T) {
	p := []float64{0.62, 0.64, 0.58, 0.41, 0.30}
	went := []bool{true, true, false, false, false}
	rows := CalibrationTable(p, went, DefaultBins)
	total := 0
	for _, r := range rows {
		total += r.N
	}
	if total != len(p) {
		t.Fatalf("every bet should land in exactly one bin, got %d of %d", total, len(p))
	}
}

func TestInvalidInputs(t *testing.T) {
	if _, err := ProbOver(0, 5, 10.5); err == nil {
		t.Error("expected error for non-positive mean")
	}
}
