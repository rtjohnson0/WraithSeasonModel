package model

import (
	"encoding/json"
	"math"
	"os"
)

// Calibrator corrects a model's probabilities using how it actually performed:
// P_final = sigmoid(Intercept + Slope * logit(P_model)).
//
// A slope below 1 pulls overconfident probabilities back toward 50%. That is the usual case
// here, because sportsbook lines already contain most of what a stats-only model knows, so
// the model's disagreements with the line are partly noise. A slope of 0 means the model
// showed no real signal, and every probability collapses to the base rate (so nothing posts).
type Calibrator struct {
	Intercept float64 `json:"intercept"`
	Slope     float64 `json:"slope"`
}

// Identity leaves probabilities unchanged.
var Identity = Calibrator{Intercept: 0, Slope: 1}

func logit(p float64) float64 {
	p = math.Min(math.Max(p, 1e-6), 1-1e-6)
	return math.Log(p / (1 - p))
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// Apply returns the calibrated probability.
func (c Calibrator) Apply(p float64) float64 {
	return sigmoid(c.Intercept + c.Slope*logit(p))
}

// FitCalibrator fits a logistic regression of outcomes (true = went over) on the model's
// logit probabilities. With fewer than 200 examples it returns Identity.
func FitCalibrator(p []float64, wentOver []bool) Calibrator {
	n := len(p)
	if n < 200 || len(wentOver) != n {
		return Identity
	}
	x := make([]float64, n)
	var base float64
	for i := range p {
		x[i] = logit(p[i])
		if wentOver[i] {
			base++
		}
	}
	base /= float64(n)

	a, b := 0.0, 1.0
	for iter := 0; iter < 50; iter++ {
		var g0, g1, h00, h01, h11 float64
		for i := range x {
			pi := sigmoid(a + b*x[i])
			y := 0.0
			if wentOver[i] {
				y = 1
			}
			r := y - pi
			w := pi * (1 - pi)
			g0 += r
			g1 += r * x[i]
			h00 += w
			h01 += w * x[i]
			h11 += w * x[i] * x[i]
		}
		h00 += 1e-6
		h11 += 1e-6
		det := h00*h11 - h01*h01
		if math.Abs(det) < 1e-12 {
			break
		}
		da := (h11*g0 - h01*g1) / det
		db := (-h01*g0 + h00*g1) / det
		a += da
		b += db
		if math.Abs(da)+math.Abs(db) < 1e-9 {
			break
		}
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return Identity
	}
	if b < 0 {
		// The model's confidence pointed the wrong way: treat it as no signal.
		return Calibrator{Intercept: logit(base), Slope: 0}
	}
	return Calibrator{Intercept: a, Slope: b}
}

// Save writes the calibrator as JSON.
func (c Calibrator) Save(path string) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadCalibrator reads a saved calibrator. ok is false if the file does not exist or is invalid.
func LoadCalibrator(path string) (c Calibrator, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Identity, false
	}
	if json.Unmarshal(b, &c) != nil {
		return Identity, false
	}
	return c, true
}
