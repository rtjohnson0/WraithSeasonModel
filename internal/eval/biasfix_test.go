package eval

import (
	"math"
	"testing"

	"propsmodel/internal/data"
)

func biasSamples(n int, pred, actualHigh, actualLow float64) []BiasSample {
	out := make([]BiasSample, n)
	for i := range out {
		a := actualHigh
		if i%2 == 1 {
			a = actualLow
		}
		for k := range out[i].Pred {
			out[i].Pred[k], out[i].Actual[k] = pred, a
		}
	}
	return out
}

func TestBiasFixShrinksAndClamps(t *testing.T) {
	// Players beat the projection by ~10% on average with modest noise: a real bias.
	bf := FitBiasFix(biasSamples(800, 10, 12, 10))
	s := bf.For(data.Points)
	if s <= 1 || s > 1.10 {
		t.Fatalf("scale = %v, want a modest upward correction", s)
	}
	raw := bf.Raw["points"]
	if math.Abs(raw-1.1) > 1e-9 || s >= raw {
		t.Errorf("raw %v scale %v: the applied scale should be shrunk toward 1", raw, s)
	}
	if !bf.Active() {
		t.Error("expected an active correction")
	}
}

func TestBiasFixIgnoresSmallOrNoisyBias(t *testing.T) {
	if FitBiasFix(biasSamples(100, 10, 12, 10)).Active() {
		t.Error("too few samples should not adjust anything")
	}
	// No bias at all.
	if FitBiasFix(biasSamples(800, 10, 11, 9)).Active() {
		t.Error("an unbiased model should not be adjusted")
	}
	// A big bias but huge noise: z is small.
	noisy := biasSamples(400, 10, 40, 0)
	if bf := FitBiasFix(noisy); bf.Active() && bf.Z["points"] < biasMinZ {
		t.Error("a bias within the noise should not be applied")
	}
}
