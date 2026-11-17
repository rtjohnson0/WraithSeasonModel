// Package glm fits a negative binomial regression with a log link and an exposure offset:
//
//	mean = exposure * exp(intercept + sum(coef_j * z_j))
//
// where z are standardised features. For player stats the exposure is minutes played, so the
// model learns a per-minute rate and how context (opponent, pace, teammates out...) scales it.
// Fitting is iteratively reweighted least squares with a ridge penalty, which is stable and
// fast enough for a hundred thousand rows.
package glm

import (
	"errors"
	"math"

	"propsmodel/internal/linalg"
)

// Model is a fitted regression. Everything needed to predict is stored, so it can be saved as JSON.
type Model struct {
	Names     []string  `json:"names"`
	Mean      []float64 `json:"mean"` // feature means (NaN values are replaced by the mean)
	Std       []float64 `json:"std"`
	Intercept float64   `json:"intercept"`
	Coef      []float64 `json:"coef"`  // on standardised features
	Alpha     float64   `json:"alpha"` // negative binomial dispersion: var = mean + alpha*mean^2
}

func (m *Model) z(x []float64, j int) float64 {
	v := x[j]
	if math.IsNaN(v) {
		return 0
	}
	return (v - m.Mean[j]) / m.Std[j]
}

// LogRate is the log of the per-unit-exposure mean for a feature vector.
func (m *Model) LogRate(x []float64) float64 {
	eta := m.Intercept
	for j := range m.Coef {
		eta += m.Coef[j] * m.z(x, j)
	}
	return eta
}

// Rate is the per-unit-exposure mean (for example, points per minute).
func (m *Model) Rate(x []float64) float64 { return math.Exp(m.LogRate(x)) }

// Fit estimates the model. X is rows by features, y the counts, logExposure the log of the
// exposure (log minutes). ridge penalises the coefficients on standardised features.
func Fit(X [][]float64, y, logExposure []float64, names []string, ridge float64) (*Model, error) {
	n, p := len(X), len(names)
	if n == 0 || len(y) != n || len(logExposure) != n {
		return nil, errors.New("glm: need matching, non-empty X, y and exposure")
	}

	m := &Model{Names: names, Mean: make([]float64, p), Std: make([]float64, p), Coef: make([]float64, p)}
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
				d := v - m.Mean[j]
				ss += d * d
			}
		}
		m.Std[j] = 1
		if cnt > 1 {
			if s := math.Sqrt(ss / cnt); s > 1e-9 {
				m.Std[j] = s
			}
		}
	}

	// Standardised design, row-major.
	z := make([]float64, n*p)
	for i := 0; i < n; i++ {
		for j := 0; j < p; j++ {
			z[i*p+j] = m.z(X[i], j)
		}
	}

	var sumY, sumExp float64
	for i := 0; i < n; i++ {
		sumY += y[i]
		sumExp += math.Exp(logExposure[i])
	}
	if sumY <= 0 || sumExp <= 0 {
		return nil, errors.New("glm: no positive counts to fit")
	}
	beta := make([]float64, p+1) // beta[0] is the intercept
	beta[0] = math.Log(sumY / sumExp)

	dim := p + 1
	a := make([][]float64, dim)
	for i := range a {
		a[i] = make([]float64, dim)
	}
	b := make([]float64, dim)
	row := make([]float64, dim)

	for iter := 0; iter < 40; iter++ {
		for i := range a {
			for j := range a[i] {
				a[i][j] = 0
			}
			b[i] = 0
		}
		for i := 0; i < n; i++ {
			row[0] = 1
			copy(row[1:], z[i*p:(i+1)*p])
			eta := beta[0]
			for j := 0; j < p; j++ {
				eta += beta[j+1] * row[j+1]
			}
			eta = math.Min(math.Max(eta, -9), 4) // keep the per-minute rate sane
			mu := math.Exp(eta + logExposure[i])
			if mu < 1e-9 {
				mu = 1e-9
			}
			w := mu
			work := eta + (y[i]-mu)/mu // working response with the offset removed
			for r := 0; r < dim; r++ {
				wr := w * row[r]
				b[r] += wr * work
				for c := 0; c <= r; c++ {
					a[r][c] += wr * row[c]
				}
			}
		}
		for r := 0; r < dim; r++ {
			for c := r + 1; c < dim; c++ {
				a[r][c] = a[c][r]
			}
			if r > 0 {
				a[r][r] += ridge
			}
		}
		next, err := linalg.SolveSPD(a, b)
		if err != nil {
			return nil, err
		}
		var delta float64
		for j := range next {
			delta = math.Max(delta, math.Abs(next[j]-beta[j]))
		}
		beta = next
		if delta < 1e-6 {
			break
		}
	}

	m.Intercept = beta[0]
	copy(m.Coef, beta[1:])

	// Dispersion by the method of moments: E[(y-mu)^2 - mu] = alpha * mu^2.
	var num, den float64
	for i := 0; i < n; i++ {
		eta := m.Intercept
		for j := 0; j < p; j++ {
			eta += m.Coef[j] * z[i*p+j]
		}
		mu := math.Exp(math.Min(math.Max(eta, -9), 4) + logExposure[i])
		d := y[i] - mu
		num += d*d - mu
		den += mu * mu
	}
	m.Alpha = 0.02
	if den > 0 {
		m.Alpha = math.Min(math.Max(num/den, 1e-4), 3)
	}
	return m, nil
}
