package model

import (
	"math"
	"math/rand"
	"path/filepath"
	"testing"
)

func TestFitCalibratorRecoversOverconfidence(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	// True world: outcome probability is sigmoid(0.4 * logit(model probability)),
	// i.e. the model is overconfident by a factor of 2.5.
	var p []float64
	var y []bool
	for i := 0; i < 20000; i++ {
		mp := 0.25 + 0.5*r.Float64()
		truth := sigmoid(0.4 * logit(mp))
		p = append(p, mp)
		y = append(y, r.Float64() < truth)
	}
	c := FitCalibrator(p, y)
	if math.Abs(c.Slope-0.4) > 0.08 {
		t.Errorf("fitted slope %.3f, want about 0.4", c.Slope)
	}
	if math.Abs(c.Intercept) > 0.1 {
		t.Errorf("fitted intercept %.3f, want about 0", c.Intercept)
	}
	if got := c.Apply(0.70); got >= 0.70 || got <= 0.5 {
		t.Errorf("Apply(0.70) = %.3f, want it pulled toward 0.5", got)
	}
}

func TestFitCalibratorTooLittleData(t *testing.T) {
	if c := FitCalibrator([]float64{0.6, 0.4}, []bool{true, false}); c != Identity {
		t.Errorf("want Identity for tiny samples, got %+v", c)
	}
}

func TestFitCalibratorNoSignal(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	var p []float64
	var y []bool
	for i := 0; i < 5000; i++ {
		p = append(p, 0.3+0.4*r.Float64())
		y = append(y, r.Float64() < 0.5) // outcomes unrelated to the predictions
	}
	c := FitCalibrator(p, y)
	if c.Slope > 0.15 {
		t.Errorf("slope %.3f for pure noise, want about 0", c.Slope)
	}
}

func TestCalibratorSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calibration.json")
	want := Calibrator{Intercept: 0.1, Slope: 0.5}
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadCalibrator(path)
	if !ok || got != want {
		t.Errorf("round trip failed: ok=%v got=%+v", ok, got)
	}
	if _, ok := LoadCalibrator(filepath.Join(t.TempDir(), "missing.json")); ok {
		t.Error("missing file should report ok=false")
	}
}
