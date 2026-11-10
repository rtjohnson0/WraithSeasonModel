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
	MinutesShift float64              `json:"minutes_shift"`
	BigCut       float64              `json:"big_cut"`
	Blend        float64              `json:"blend"`
	Typical      float64              `json:"typical"`
	Scale        [data.NStats]float64 `json:"scale"`
}

func (a Adjustments) Apply(p Pred) Pred {
	p = p.WithMinutes(p.Minutes + a.MinutesShift)
	if a.BigCut > 0 && a.Typical > 0 && !math.IsNaN(a.Typical) && p.Minutes-a.Typical <= -6 {
		p = p.WithMinutes(p.Minutes + a.BigCut*(a.Typical-p.Minutes))
	}
	p = p.BlendMinutes(a.Typical, a.Blend)
	for _, st := range data.Stats {
		if f := a.Scale[data.StatIndex(st)]; f > 0 {
			p = p.Scaled(st, f)
		}
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
