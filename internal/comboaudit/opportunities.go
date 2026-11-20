package comboaudit

import (
	"fmt"
	"propsmodel/internal/data"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/picklog"
	"time"
)

type OpportunityReview struct {
	Available           bool     `json:"available"`
	Reason              string   `json:"reason"`
	Source              string   `json:"source,omitempty"`
	CapturedAt          string   `json:"capturedAt,omitempty"`
	ExpectedVolume      float64  `json:"expectedVolume"`
	ActualVolume        *float64 `json:"actualVolume,omitempty"`
	ExpectedConversion  float64  `json:"expectedConversion"`
	ActualConversion    *float64 `json:"actualConversion,omitempty"`
	MinutesComponent    float64  `json:"minutesComponent"`
	VolumeComponent     float64  `json:"volumeComponent"`
	ConversionComponent float64  `json:"conversionComponent"`
	ContextComponent    float64  `json:"contextComponent"`
}

func reviewOpportunity(s picklog.Settled, g data.PlayerGame, ix *opportunity.Index) *OpportunityReview {
	r := &OpportunityReview{Reason: "No valid frozen pregame opportunity signal"}
	if s.Rotation == nil {
		return r
	}
	for _, signal := range s.Rotation.Opportunities {
		if string(signal.Stat) != s.Stat || !signal.Available {
			continue
		}
		if signal.Through == "" || signal.Through >= s.Date || signal.Games < 3 || !finite(signal.Chances) || signal.Chances < 0 || !finite(signal.Minutes) || signal.Minutes <= 0 || !finite(signal.Conversion) || signal.Conversion < 0 || (signal.Stat != data.Points && signal.Conversion > 1) {
			return r
		}
		if _, err := time.Parse("2006-01-02", signal.Through); err != nil {
			return r
		}
		if signal.LatestCapture != "" {
			capture, e1 := time.Parse(time.RFC3339Nano, signal.LatestCapture)
			logged, e2 := time.Parse(time.RFC3339Nano, s.LoggedAt)
			if e1 != nil || e2 != nil || capture.After(logged) {
				return r
			}
		} else if signal.Stat == data.Assists || signal.Stat == data.Rebounds {
			return r
		}
		rate := signal.Chances / signal.Minutes
		r.ExpectedVolume, r.ExpectedConversion = rate*s.ProjMin, signal.Conversion
		volume, source, captured, ok := ix.OutcomeVolume(g, signal.Stat)
		if captured != "" {
			stamp, e1 := time.Parse(time.RFC3339Nano, captured)
			tip, e2 := time.Parse(time.RFC3339Nano, s.Tip)
			ok = ok && e1 == nil && e2 == nil && stamp.After(tip)
		}
		if !ok {
			r.Reason = "Final opportunity tracking unavailable or incompatible"
			return r
		}
		r.Available, r.Source, r.CapturedAt = true, source, captured
		r.ActualVolume = &volume
		if volume > 0 {
			conversion := s.Actual / volume
			r.ActualConversion = &conversion
		}
		r.MinutesComponent = (g.Minutes - s.ProjMin) * rate * signal.Conversion
		r.VolumeComponent = (volume - g.Minutes*rate) * signal.Conversion
		r.ConversionComponent = s.Actual - volume*signal.Conversion
		r.ContextComponent = r.ExpectedVolume*signal.Conversion - s.ProjMean
		r.Reason = fmt.Sprintf("Frozen %d-game opportunity rate × frozen historical conversion; context residual includes fitted rate adjustments. Arithmetic attribution, not causation.", signal.Games)
		return r
	}
	return r
}
