// Package linalg has the one piece of linear algebra the models need: solving a symmetric
// positive definite system with a Cholesky factorisation.
package linalg

import (
	"errors"
	"math"
)

// SolveSPD solves A x = b for a symmetric positive definite matrix A (n by n).
// A and b are not modified.
func SolveSPD(a [][]float64, b []float64) ([]float64, error) {
	n := len(b)
	if len(a) != n {
		return nil, errors.New("linalg: dimension mismatch")
	}
	l := make([][]float64, n)
	for i := range l {
		l[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			sum := a[i][j]
			for k := 0; k < j; k++ {
				sum -= l[i][k] * l[j][k]
			}
			if i == j {
				if sum <= 1e-12 {
					return nil, errors.New("linalg: matrix is not positive definite")
				}
				l[i][j] = math.Sqrt(sum)
			} else {
				l[i][j] = sum / l[j][j]
			}
		}
	}
	// Forward substitution: L y = b.
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := b[i]
		for k := 0; k < i; k++ {
			sum -= l[i][k] * y[k]
		}
		y[i] = sum / l[i][i]
	}
	// Back substitution: L^T x = y.
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := y[i]
		for k := i + 1; k < n; k++ {
			sum -= l[k][i] * x[k]
		}
		x[i] = sum / l[i][i]
	}
	return x, nil
}
