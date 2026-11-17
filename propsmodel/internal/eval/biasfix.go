package eval

import (
	"encoding/json"
	"math"
	"os"

	"propsmodel/internal/data"
)

// BiasFix is a small per-stat correction to the model's projected means, fitted from the
// out-of-sample bias check (cmd/bias -write). It exists because the model can run high or low
// for a whole stretch of the season (early on, last year's averages are stale), and a pick board
// that leans one way for that reason is a model error, not a read on the games.
//
// Only a bias that is both large enough to matter and unlikely to be luck is applied, and it is
// shrunk toward 1 when the sample is small, so a few noisy games cannot move projections much.
type BiasFix struct {
	Season  int                `json:"season"`
	Samples int                `json:"samples"`
	Scale   map[string]float64 `json:"scale"`             // stat -> multiplier on the projected mean
	Z       map[string]float64 `json:"z,omitempty"`       // how many standard errors the raw bias was
	Raw     map[string]float64 `json:"raw,omitempty"`     // actual / projected before shrinking
	Updated string             `json:"updated,omitempty"` // date it was fitted
}

const (
	biasMinSamples = 300  // below this many player-games nothing is applied
	biasMinZ       = 2.0  // a bias this small relative to its noise could be luck
	biasShrinkN    = 200  // shrinkage: n/(n+400) of the raw gap is applied
	biasMin        = 0.85 // never move a projection more than this far
	biasMax        = 1.10
)

// FitBiasFix measures actual / projected for each stat and turns it into a conservative multiplier.
func FitBiasFix(samples []BiasSample) BiasFix {
	bf := BiasFix{Samples: len(samples), Scale: map[string]float64{}, Z: map[string]float64{}, Raw: map[string]float64{}}
	n := float64(len(samples))
	if n == 0 {
		return bf
	}
	for _, st := range data.Stats {
		k := data.StatIndex(st)
		var ps, as, sq float64
		for _, s := range samples {
			ps += s.Pred[k]
			as += s.Actual[k]
			d := s.Actual[k] - s.Pred[k]
			sq += d * d
		}
		if ps <= 0 {
			continue
		}
		mean := (as - ps) / n
		v := sq/n - mean*mean
		z := 0.0
		if v > 0 && n > 1 {
			z = mean / math.Sqrt(v/n)
		}
		raw := as / ps
		bf.Raw[string(st)], bf.Z[string(st)] = raw, z
		scale := 1.0
		if n >= biasMinSamples && math.Abs(z) >= biasMinZ {
			scale = 1 + (raw-1)*n/(n+biasShrinkN)
			scale = math.Min(math.Max(scale, biasMin), biasMax)
		}
		bf.Scale[string(st)] = scale
	}
	return bf
}

// For returns the multiplier for a stat (1 when there is none).
func (b BiasFix) For(s data.Stat) float64 {
	if v, ok := b.Scale[string(s)]; ok && v > 0 {
		return v
	}
	return 1
}

// Active reports whether any stat is actually adjusted.
func (b BiasFix) Active() bool {
	for _, v := range b.Scale {
		if math.Abs(v-1) > 1e-9 {
			return true
		}
	}
	return false
}

// Save writes the correction to path.
func (b BiasFix) Save(path string) error {
	buf, err := json.MarshalIndent(b, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// LoadBiasFix reads a saved correction; ok is false when there is none.
func LoadBiasFix(path string) (BiasFix, bool) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return BiasFix{}, false
	}
	var b BiasFix
	if json.Unmarshal(buf, &b) != nil {
		return BiasFix{}, false
	}
	return b, true
}
