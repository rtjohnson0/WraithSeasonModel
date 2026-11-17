package eval

import (
	"math/rand"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/predictor"
)

func woSamples(r *rand.Rand, truth float64, n int) []BiasSample {
	var out []BiasSample
	for i := 0; i < n; i++ {
		wo := 6 * r.NormFloat64()
		var p predictor.Pred
		for k := range p.MinNodes {
			p.MinNodes[k] = 28
		}
		p.Minutes = 28
		for k := range p.Rate {
			p.Rate[k] = 0.4
		}
		s := BiasSample{PredMin: 28, WoMin: wo, Pred0: p, ActualMin: 28 + truth*wo + 5*r.NormFloat64()}
		for k := range s.Pred {
			s.Pred[k] = 28 * 0.4
			s.Actual[k] = s.Pred[k] + 3*r.NormFloat64()
		}
		out = append(out, s)
	}
	return out
}

func TestFitWoFixUsesRealSignal(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	f := FitWoFix(woSamples(r, 0.6, 1200))
	if f.Minutes <= 0.3 || f.Minutes > 0.7 {
		t.Fatalf("applied minutes multiplier = %.2f (raw %.2f z %.1f), want about 0.5", f.Minutes, f.MinRaw, f.MinZ)
	}
	if f.MinutesShift(5) <= 0 || f.MinutesShift(-5) >= 0 {
		t.Fatalf("shift should follow the sign of the history")
	}
}

func TestFitWoFixIgnoresNoise(t *testing.T) {
	r := rand.New(rand.NewSource(12))
	f := FitWoFix(woSamples(r, 0, 800))
	if f.Minutes != 0 {
		t.Fatalf("noise should not be applied: %+v", f)
	}
	if f.RateFactor(data.Points, 0.2) != 1 {
		t.Fatalf("inactive rate factor should be 1")
	}
}
