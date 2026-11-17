package predictor

import (
	"math"
	"propsmodel/internal/data"
	"testing"
)

func TestBlendMinutes(t *testing.T) {
	var p Pred
	for k := range p.MinNodes {
		p.MinNodes[k] = 28 + float64(k%3) // spread around 29
	}
	for k := range p.MinNodes {
		p.Minutes += p.MinNodes[k] / Nodes
	}
	p.Rate[0] = 0.5
	before := p.Minutes
	if got := p.BlendMinutes(36, 0); got.Minutes != before {
		t.Fatalf("w=0 changed minutes: %v", got.Minutes)
	}
	if got := p.BlendMinutes(0, 0.5); got.Minutes != before {
		t.Fatalf("unknown typical changed minutes")
	}
	if got := p.BlendMinutes(math.NaN(), 0.5); got.Minutes != before {
		t.Fatalf("NaN typical changed minutes")
	}
	got := p.BlendMinutes(36, 0.5)
	want := before + 0.5*(36-before)
	if math.Abs(got.Minutes-want) > 1e-9 {
		t.Fatalf("blended minutes = %v, want %v", got.Minutes, want)
	}
	if math.Abs(got.Mean(data.Points)-want*0.5) > 1e-9 {
		t.Fatalf("mean should follow minutes at a fixed rate")
	}
}
