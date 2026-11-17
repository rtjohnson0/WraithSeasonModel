package eval

import (
	"encoding/json"
	"math"
	"os"
)

// MinFix moves projected minutes with the game's pre-game point spread. In a close game starters
// stay on the floor; in a rout they sit. The model has the spread as an input, but this season's
// opening weeks showed it under-reacting: players in close games ran several minutes over
// projection and players in blowouts ran under. The correction is fitted on every eligible
// player-game (nothing selected), is centred so it does not change the average level of minutes,
// and is only used when the slope is both clear and the right sign.
type MinFix struct {
	Season  int     `json:"season"`
	Samples int     `json:"samples"`
	Slope   float64 `json:"slope"`  // minutes per point of spread, at 30 projected minutes (negative: bigger spread, fewer minutes)
	Centre  float64 `json:"centre"` // the spread at which nothing changes
	Raw     float64 `json:"raw"`    // the slope before shrinking
	Z       float64 `json:"z"`      // raw slope / its standard error
	Updated string  `json:"updated,omitempty"`
}

const (
	minfixMinSamples = 300
	minfixMinZ       = 2.5
	minfixShrinkN    = 300.0 // applied share = n / (n + 300)
	minfixMaxShift   = 4.0   // minutes, either way
)

func minfixScale(projected float64) float64 {
	return math.Min(math.Max(projected/30, 0.3), 1.2)
}

// FitMinFix regresses the minutes miss (actual - projected) on the spread, weighting each game's
// effect by how big a part of the game the player plays.
func FitMinFix(samples []BiasSample) MinFix {
	var xs, ys, ss []float64
	for _, s := range samples {
		if math.IsNaN(s.AbsSpread) || s.AbsSpread <= 0 {
			continue
		}
		xs = append(xs, s.AbsSpread)
		ys = append(ys, s.ActualMin-s.PredMin)
		ss = append(ss, minfixScale(s.PredMin))
	}
	mf := MinFix{Samples: len(xs)}
	if len(xs) < 30 {
		return mf
	}
	// y_i = s_i * (a + b*x_i): ordinary least squares on the two columns (s_i, s_i*x_i).
	var saa, sab, sbb, say, sby float64
	var sx float64
	for i := range xs {
		c1, c2 := ss[i], ss[i]*xs[i]
		saa += c1 * c1
		sab += c1 * c2
		sbb += c2 * c2
		say += c1 * ys[i]
		sby += c2 * ys[i]
		sx += xs[i]
	}
	det := saa*sbb - sab*sab
	if det == 0 {
		return mf
	}
	a := (sbb*say - sab*sby) / det
	b := (saa*sby - sab*say) / det
	var rss float64
	for i := range xs {
		e := ys[i] - ss[i]*(a+b*xs[i])
		rss += e * e
	}
	sigma2 := rss / float64(len(xs)-2)
	se := math.Sqrt(sigma2 * saa / det)
	mf.Raw = b
	if se > 0 {
		mf.Z = b / se
	}
	mf.Centre = sx / float64(len(xs))
	n := float64(len(xs))
	mf.Slope = b * n / (n + minfixShrinkN)
	return mf
}

// Active reports whether the correction is clear enough to use: enough games, a slope that is
// statistically distinct from zero, and the sign that makes sense (bigger spread, fewer minutes).
func (m MinFix) Active() bool {
	return m.Samples >= minfixMinSamples && m.Raw < 0 && math.Abs(m.Z) >= minfixMinZ
}

// Shift is the change in projected minutes for a game with this spread. Unknown spread, or an
// inactive fix, changes nothing.
func (m MinFix) Shift(absSpread, projectedMinutes float64) float64 {
	if !m.Active() || math.IsNaN(absSpread) || absSpread <= 0 {
		return 0
	}
	d := minfixScale(projectedMinutes) * m.Slope * (absSpread - m.Centre)
	return math.Max(-minfixMaxShift, math.Min(minfixMaxShift, d))
}

func (m MinFix) Save(path string) error {
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func LoadMinFix(path string) (MinFix, bool) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return MinFix{}, false
	}
	var m MinFix
	if json.Unmarshal(buf, &m) != nil {
		return MinFix{}, false
	}
	return m, true
}
