package linalg

import (
	"math"
	"testing"
)

func TestSolveSPD(t *testing.T) {
	a := [][]float64{{4, 1, 0}, {1, 3, 1}, {0, 1, 2}}
	want := []float64{1, -2, 3}
	b := make([]float64, 3)
	for i := range a {
		for j := range a[i] {
			b[i] += a[i][j] * want[j]
		}
	}
	got, err := SolveSPD(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("x[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestSolveSPDRejectsSingular(t *testing.T) {
	if _, err := SolveSPD([][]float64{{1, 1}, {1, 1}}, []float64{1, 2}); err == nil {
		t.Error("expected an error for a singular matrix")
	}
}
