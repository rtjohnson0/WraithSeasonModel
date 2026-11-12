package predictor

import (
	"encoding/json"
	"math"
	"path/filepath"
	"propsmodel/internal/data"
	"testing"
)

func TestWeightedMinutesDistributionAndPush(t *testing.T) {
	var p Pred
	p.MinNodes[0], p.MinNodes[1] = 2, 32
	p.MinWeights[0], p.MinWeights[1] = .1, .9
	p.Minutes = 29
	p.Rate[0], p.Alpha[0] = .5, .04
	d := p.Dist(data.Points, 120)
	sum, mean := 0., 0.
	for k, v := range d {
		sum += v
		mean += float64(k) * v
	}
	if math.Abs(sum-1) > .01 || math.Abs(mean-14.5) > .1 {
		t.Fatalf("weighted distribution mass %g mean %g", sum, mean)
	}
	pr, e := p.ProbOver(data.Points, 14)
	if e != nil {
		t.Fatal(e)
	}
	below := 0.
	for k := 0; k < 14; k++ {
		below += d[k]
	}
	if math.Abs(pr.Push-d[14]) > .001 || math.Abs(pr.Under-below/(1-d[14])) > .005 {
		t.Fatalf("weighted push mismatch %+v", pr)
	}
	shifted := p.WithMinutes(34)
	if math.Abs(shifted.Minutes-34) > .001 {
		t.Fatalf("shifted weighted mean %g", shifted.Minutes)
	}
}

func TestTailFitIsIsolatedAndRoundTrips(t *testing.T) {
	_, _, rows := world(t)
	rows = rows[:8000]
	base, e := Train(rows, nil)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := json.Marshal(base)
	candidate, e := FitTailUncertainty(base, rows)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(base)
	if string(before) != string(after) || base.Version != 4 {
		t.Fatal("candidate mutated reference")
	}
	if candidate.Version != 5 || candidate.TailFitAfter >= candidate.TrainedBefore {
		t.Fatal("invalid residual dates")
	}
	path := filepath.Join(t.TempDir(), "model.json")
	if e = candidate.Save(path); e != nil {
		t.Fatal(e)
	}
	loaded, e := Load(path)
	if e != nil {
		t.Fatal(e)
	}
	a, b := candidate.Predict(&rows[7999]), loaded.Predict(&rows[7999])
	if math.Abs(a.Mean(data.Points)-b.Mean(data.Points)) > 1e-10 {
		t.Fatal("weighted serialization changed forecast")
	}
	loaded.MinuteWeights[1] = make([]float64, Nodes)
	loaded.MinuteWeights[1][0] = -1
	if e = loaded.Save(path); e != nil {
		t.Fatal(e)
	}
	if _, e = Load(path); e == nil {
		t.Fatal("negative weight accepted")
	}
}
