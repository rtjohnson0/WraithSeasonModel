package glm

import (
	"math"
	"math/rand"
	"testing"
)

func poisson(r *rand.Rand, lambda float64) float64 {
	l := math.Exp(-lambda)
	k, p := 0, 1.0
	for {
		k++
		p *= r.Float64()
		if p <= l {
			return float64(k - 1)
		}
	}
}

func TestFitRecoversCoefficients(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	n := 40000
	X := make([][]float64, n)
	y := make([]float64, n)
	off := make([]float64, n)
	// True model: rate per minute = exp(-1.2 + 0.30*x1 - 0.20*x2), x2 sometimes missing (NaN).
	for i := range X {
		x1, x2 := r.NormFloat64(), r.NormFloat64()
		mins := 15 + 25*r.Float64()
		X[i] = []float64{x1, x2}
		y[i] = poisson(r, mins*math.Exp(-1.2+0.30*x1-0.20*x2))
		off[i] = math.Log(mins)
	}
	m, err := Fit(X, y, off, []string{"x1", "x2"}, 1e-3)
	if err != nil {
		t.Fatal(err)
	}
	// Features are N(0,1), so standardised coefficients are close to the raw ones.
	if math.Abs(m.Coef[0]-0.30) > 0.02 || math.Abs(m.Coef[1]+0.20) > 0.02 {
		t.Errorf("coefficients %v, want about [0.30 -0.20]", m.Coef)
	}
	if math.Abs(m.Intercept+1.2) > 0.02 {
		t.Errorf("intercept %v, want about -1.2", m.Intercept)
	}
	if m.Alpha > 0.02 {
		t.Errorf("dispersion %v for Poisson data, want about 0", m.Alpha)
	}
	if got := m.Rate([]float64{0, 0}); math.Abs(got-math.Exp(-1.2)) > 0.01 {
		t.Errorf("Rate(0,0) = %v, want about %v", got, math.Exp(-1.2))
	}
}

func TestFitHandlesMissingFeature(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	n := 5000
	X := make([][]float64, n)
	y := make([]float64, n)
	off := make([]float64, n)
	for i := range X {
		x := r.NormFloat64()
		if i%4 == 0 {
			x = math.NaN()
		}
		X[i] = []float64{x}
		y[i] = poisson(r, 30*0.3)
		off[i] = math.Log(30)
	}
	m, err := Fit(X, y, off, []string{"x"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if math.IsNaN(m.Rate([]float64{math.NaN()})) {
		t.Error("a missing feature should predict normally, got NaN")
	}
}

func TestFitDetectsOverdispersion(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	n := 30000
	X := make([][]float64, n)
	y := make([]float64, n)
	off := make([]float64, n)
	for i := range X {
		x := r.NormFloat64()
		mu := 20 * math.Exp(0.1*x)
		// Gamma-Poisson mixture with alpha = 0.2.
		g := mu * gamma(r, 5) / 5
		X[i] = []float64{x}
		y[i] = poisson(r, g)
		off[i] = 0
	}
	m, err := Fit(X, y, off, []string{"x"}, 1e-3)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(m.Alpha-0.2) > 0.04 {
		t.Errorf("dispersion %v, want about 0.2", m.Alpha)
	}
}

// gamma draws from Gamma(shape k integer, scale 1) as a sum of exponentials.
func gamma(r *rand.Rand, k int) float64 {
	s := 0.0
	for i := 0; i < k; i++ {
		s += r.ExpFloat64()
	}
	return s
}
