package predictor

import (
	"math"
	"testing"
)

func TestRotationTargetConservesMeanAtMinuteBounds(t *testing.T) {
	p := Pred{}
	for i := range p.MinNodes {
		p.MinNodes[i] = float64(i) * 48 / float64(Nodes-1)
		p.Minutes += p.MinNodes[i] * p.NodeWeight(i)
	}
	original := p
	for _, target := range []float64{0, .2, 6, 27, 43, 48} {
		q := p.WithTargetMinutes(target)
		if math.Abs(q.Minutes-target) > 1e-8 {
			t.Fatalf("target %.2f got %.8f", target, q.Minutes)
		}
		for _, m := range q.MinNodes {
			if m < 0 || m > 48 {
				t.Fatal(m)
			}
		}
	}
	if p != original {
		t.Fatal("mutated baseline")
	}
}
