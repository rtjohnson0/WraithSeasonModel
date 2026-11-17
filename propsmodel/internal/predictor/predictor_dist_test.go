package predictor

import (
	"math"
	"testing"

	"propsmodel/internal/data"
)

func TestDistMatchesMeanAndProbOver(t *testing.T) {
	var p Pred
	for i := range p.MinNodes {
		p.MinNodes[i] = 30
	}
	p.Minutes = 30
	p.Rate[data.StatIndex(data.Points)] = 0.5
	p.Alpha[data.StatIndex(data.Points)] = 0.03
	d := p.Dist(data.Points, 60)
	var sum, mean float64
	for k, v := range d {
		sum += v
		mean += float64(k) * v
	}
	if math.Abs(sum-1) > 0.02 || math.Abs(mean-15) > 0.3 {
		t.Fatalf("distribution should sum to 1 with mean 15, got sum %.3f mean %.2f", sum, mean)
	}
	pr, err := p.ProbOver(data.Points, 14.5)
	if err != nil {
		t.Fatal(err)
	}
	var under float64
	for k := 0; k <= 14 && k < len(d); k++ {
		under += d[k]
	}
	if math.Abs(under-pr.Under) > 0.02 {
		t.Errorf("P(<=14) from the curve %.3f should match ProbOver's under %.3f", under, pr.Under)
	}
}
