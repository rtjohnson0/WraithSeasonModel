package glm

import (
	"errors"
	"math"

	"propsmodel/internal/linalg"
)

// Linear is a ridge regression on standardised features. It is used for the minutes model.
type Linear struct {
	Names     []string  `json:"names"`
	Mean      []float64 `json:"mean"`
	Std       []float64 `json:"std"`
	Intercept float64   `json:"intercept"`
	Coef      []float64 `json:"coef"`
}

// Predict returns the fitted value. Missing (NaN) features count as the training average.
func (m *Linear) Predict(x []float64) float64 {
	y := m.Intercept
	for j, c := range m.Coef {
		v := x[j]
		if math.IsNaN(v) {
			continue
		}
		y += c * (v - m.Mean[j]) / m.Std[j]
	}
	return y
}

// FitLinear fits y on X with a ridge penalty on the standardised coefficients.
func FitLinear(X [][]float64, y []float64, names []string, ridge float64) (*Linear, error) {
	n, p := len(X), len(names)
	if n == 0 || len(y) != n {
		return nil, errors.New("glm: need matching, non-empty X and y")
	}
	m := &Linear{Names: names, Mean: make([]float64, p), Std: make([]float64, p), Coef: make([]float64, p)}
	for j := 0; j < p; j++ {
		var sum, cnt float64
		for i := 0; i < n; i++ {
			if v := X[i][j]; !math.IsNaN(v) {
				sum += v
				cnt++
			}
		}
		if cnt > 0 {
			m.Mean[j] = sum / cnt
		}
		var ss float64
		for i := 0; i < n; i++ {
			if v := X[i][j]; !math.IsNaN(v) {
				ss += (v - m.Mean[j]) * (v - m.Mean[j])
			}
		}
		m.Std[j] = 1
		if cnt > 1 {
			if s := math.Sqrt(ss / cnt); s > 1e-9 {
				m.Std[j] = s
			}
		}
	}
	var ybar float64
	for _, v := range y {
		ybar += v
	}
	ybar /= float64(n)
	m.Intercept = ybar

	a := make([][]float64, p)
	for i := range a {
		a[i] = make([]float64, p)
	}
	b := make([]float64, p)
	z := make([]float64, p)
	for i := 0; i < n; i++ {
		for j := 0; j < p; j++ {
			v := X[i][j]
			if math.IsNaN(v) {
				z[j] = 0
			} else {
				z[j] = (v - m.Mean[j]) / m.Std[j]
			}
		}
		d := y[i] - ybar
		for r := 0; r < p; r++ {
			b[r] += z[r] * d
			for c := 0; c <= r; c++ {
				a[r][c] += z[r] * z[c]
			}
		}
	}
	for r := 0; r < p; r++ {
		for c := r + 1; c < p; c++ {
			a[r][c] = a[c][r]
		}
		a[r][r] += ridge
	}
	coef, err := linalg.SolveSPD(a, b)
	if err != nil {
		return nil, err
	}
	copy(m.Coef, coef)
	return m, nil
}
