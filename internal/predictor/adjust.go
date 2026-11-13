package predictor

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
)

// Adjustments is the complete numerical post-fit policy shared by live scoring and replay.
// Learned shifts must be fitted using observations strictly before the prediction date.
type Adjustments struct {
	TargetMinutes *float64             `json:"target_minutes,omitempty"`
	MinutesShift  float64              `json:"minutes_shift"`
	BigCut        float64              `json:"big_cut"`
	Blend         float64              `json:"blend"`
	Typical       float64              `json:"typical"`
	Scale         [data.NStats]float64 `json:"scale"`
}

func (a Adjustments) Apply(p Pred) Pred {
	p = p.WithMinutes(p.Minutes + a.MinutesShift)
	if a.BigCut > 0 && a.Typical > 0 && !math.IsNaN(a.Typical) && p.Minutes-a.Typical <= -6 {
		p = p.WithMinutes(p.Minutes + a.BigCut*(a.Typical-p.Minutes))
	}
	p = p.BlendMinutes(a.Typical, a.Blend)
	if a.TargetMinutes != nil {
		p = p.WithTargetMinutes(*a.TargetMinutes)
	}
	for _, st := range data.Stats {
		if f := a.Scale[data.StatIndex(st)]; f > 0 {
			p = p.Scaled(st, f)
		}
	}
	return p
}

// WithTargetMinutes translates the minutes distribution to an exact bounded
// mean. Rotation forecasts use this instead of a one-pass clipped shift.
// Existing policies continue to use WithMinutes unchanged.
func (p Pred) WithTargetMinutes(target float64) Pred {
	if math.IsNaN(target) || math.IsInf(target, 0) {
		return p
	}
	target = math.Max(0, math.Min(48, target))
	lo, hi := -48., 48.
	for i := 0; i < 70; i++ {
		shift := (lo + hi) / 2
		mean := 0.
		for n, m := range p.MinNodes {
			mean += math.Max(0, math.Min(48, m+shift)) * p.NodeWeight(n)
		}
		if mean < target {
			lo = shift
		} else {
			hi = shift
		}
	}
	shift := (lo + hi) / 2
	p.Minutes = 0
	for n, m := range p.MinNodes {
		p.MinNodes[n] = math.Max(0, math.Min(48, m+shift))
		p.Minutes += p.MinNodes[n] * p.NodeWeight(n)
	}
	return p
}

// Forecast is used for both a live feature row and a stored decision replay.
func (m *Model) Forecast(r *feat.Row, a Adjustments) Pred { return a.Apply(m.Predict(r)) }

// Price is the common raw-distribution and market-calibration path.
func Price(p Pred, stat data.Stat, line, market float64, calibrate func(data.Stat, float64, float64) float64) (model.Probs, float64, error) {
	raw, err := p.ProbOver(stat, line)
	if err != nil {
		return raw, 0, err
	}
	final := raw.Over
	if calibrate != nil {
		final = calibrate(stat, raw.Over, market)
	}
	return raw, final, nil
}
