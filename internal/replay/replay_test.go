package replay

import (
	"encoding/json"
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/glm"
	"propsmodel/internal/predictor"
	"testing"
)

func TestExactReplayPreservesMissingFeaturesAndAdjustments(t *testing.T) {
	r := &feat.Row{G: data.PlayerGame{PlayerID: 7, GameID: 9}, X: make([]float64, len(feat.CommonSpecs)), AvgMin15: 30, MinTypical: 32}
	r.X[0] = math.NaN()
	for k := range r.S {
		r.S[k] = make([]float64, len(feat.StatSpecs[k]))
		r.S[k][0] = math.NaN()
	}
	names := feat.StatNames(data.Points, nil)
	m := &predictor.Model{Version: 4, Minutes: &glm.Linear{Intercept: 30}, Quant: make([][]float64, 5)}
	for i := range m.Quant {
		m.Quant[i] = make([]float64, predictor.Nodes)
	}
	for k := range m.Stat {
		m.Stat[k] = &glm.Model{Names: feat.StatNames(data.Stats[k], nil), Intercept: math.Log(0.5), Coef: make([]float64, len(feat.StatNames(data.Stats[k], nil))), Std: make([]float64, len(feat.StatNames(data.Stats[k], nil))), Mean: make([]float64, len(feat.StatNames(data.Stats[k], nil))), Alpha: 0.1}
		for j := range m.Stat[k].Std {
			m.Stat[k].Std[j] = 1
		}
	}
	_ = names
	policy := predictor.Adjustments{MinutesShift: 2, Typical: 32, Scale: [4]float64{1.1, 1, 1, 1}}
	rec := Record{Row: Capture(r), Policy: policy, Cal: eval.Cal{Slope: 0.3, Market: 0.7}, Stat: data.Points, Line: 15.5, Market: 0.5}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Record
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN(decoded.Row.Features().X[0]) {
		t.Fatal("missing feature turned into zero")
	}
	got, err := Recompute(m, decoded)
	if err != nil {
		t.Fatal(err)
	}
	_, want, err := predictor.Price(m.Forecast(r, policy), data.Points, 15.5, 0.5, func(_ data.Stat, p, b float64) float64 { return rec.Cal.Apply(p, b) })
	if err != nil || math.Abs(got-want) > 1e-12 {
		t.Fatalf("replay %v != live %v: %v", got, want, err)
	}
	// Rotation uses a different probability rule and an exact team-minute target.
	target := 27.35
	trial := rec
	trial.Policy = predictor.Adjustments{TargetMinutes: &target}
	trial.ProbabilityMethod = "linear"
	trial.BlendWeight = .5
	got, err = Recompute(m, trial)
	forecast := m.Forecast(r, trial.Policy)
	pr, pe := forecast.ProbOver(data.Points, trial.Line)
	want = .5*pr.Over + .5*trial.Market
	if err != nil || pe != nil || math.Abs(got-want) > 1e-12 || math.Abs(forecast.Minutes-target) > 1e-8 {
		t.Fatal("rotation replay differs", got, want, err)
	}
	if rec.Policy.TargetMinutes != nil {
		t.Fatal("rotation changed original policy")
	}
}
