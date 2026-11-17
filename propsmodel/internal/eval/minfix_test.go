package eval

import (
	"math"
	"math/rand"
	"testing"
)

func TestFitMinFixFindsSpreadSlope(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	var ss []BiasSample
	for i := 0; i < 1500; i++ {
		spread := 1 + 15*r.Float64()
		pred := 22 + 12*r.Float64()
		// truth: 0.5 minutes lost per point of spread (relative to a 7-point game), plus noise
		actual := pred - 0.5*(spread-7)*pred/30 + 4*r.NormFloat64()
		ss = append(ss, BiasSample{PredMin: pred, ActualMin: actual, AbsSpread: spread})
	}
	mf := FitMinFix(ss)
	if !mf.Active() {
		t.Fatalf("fix should be active: %+v", mf)
	}
	if mf.Raw > -0.35 || mf.Raw < -0.65 {
		t.Fatalf("raw slope = %.3f, want about -0.5", mf.Raw)
	}
	// close game adds minutes, a blowout removes them, a 7-point game is near zero
	if mf.Shift(2, 32) <= 0 || mf.Shift(14, 32) >= 0 {
		t.Fatalf("shifts: close %.2f, blowout %.2f", mf.Shift(2, 32), mf.Shift(14, 32))
	}
	if math.Abs(mf.Shift(mf.Centre, 30)) > 1e-9 {
		t.Fatalf("no change expected at the centre spread")
	}
	if mf.Shift(math.NaN(), 30) != 0 {
		t.Fatalf("unknown spread should change nothing")
	}
}

func TestMinFixIgnoresNoise(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	var ss []BiasSample
	for i := 0; i < 600; i++ {
		ss = append(ss, BiasSample{PredMin: 28, ActualMin: 28 + 4*r.NormFloat64(), AbsSpread: 1 + 15*r.Float64()})
	}
	if mf := FitMinFix(ss); mf.Active() {
		t.Fatalf("pure noise should not activate the fix: %+v", mf)
	}
}
