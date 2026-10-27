package eval

import (
	"encoding/json"
	"math"
	"os"

	"propsmodel/internal/data"
)

// WoFix says how much of the "with and without a teammate" history to believe. The history
// (feat.Row.WoMin and WoRate) is each player's own change in minutes and per-minute output in past
// games when today's absent teammates were out. It is fitted without selection on every eligible
// player-game: the minutes miss is regressed on WoMin, and each stat's miss (after the minutes
// shift) on WoRate. A slope near 1 means the history is right as it stands; near 0, it is noise.
// A slope is used only when it is clearly above zero.
type WoFix struct {
	Season  int                `json:"season"`
	Samples int                `json:"samples"`
	Minutes float64            `json:"minutes"` // applied multiplier on WoMin
	MinRaw  float64            `json:"min_raw"`
	MinZ    float64            `json:"min_z"`
	Rate    map[string]float64 `json:"rate"` // stat -> applied multiplier on WoRate
	RateRaw map[string]float64 `json:"rate_raw"`
	RateZ   map[string]float64 `json:"rate_z"`
	Updated string             `json:"updated,omitempty"`
}

const (
	woMinSamples = 300
	woMinZ       = 2.5
	woShrinkN    = 300.0
	woMaxSlope   = 1.5
	woMaxMinutes = 6.0 // largest minutes shift applied
)

// slope fits y = a + b*x and returns b and its z.
func slope(xs, ys []float64) (b, z float64) {
	n := float64(len(xs))
	if n < 30 {
		return 0, 0
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= n
	my /= n
	var sxx, sxy float64
	for i := range xs {
		sxx += (xs[i] - mx) * (xs[i] - mx)
		sxy += (xs[i] - mx) * (ys[i] - my)
	}
	if sxx == 0 {
		return 0, 0
	}
	b = sxy / sxx
	a := my - b*mx
	var rss float64
	for i := range xs {
		e := ys[i] - a - b*xs[i]
		rss += e * e
	}
	se := math.Sqrt(rss / (n - 2) / sxx)
	if se > 0 {
		z = b / se
	}
	return b, z
}

func applied(raw, z float64, n int) float64 {
	if n < woMinSamples || z < woMinZ || raw <= 0 {
		return 0
	}
	return math.Min(raw, woMaxSlope) * float64(n) / (float64(n) + woShrinkN)
}

// FitWoFix fits the minutes and per-stat multipliers.
func FitWoFix(samples []BiasSample) WoFix {
	f := WoFix{Samples: len(samples), Rate: map[string]float64{}, RateRaw: map[string]float64{}, RateZ: map[string]float64{}}
	var xs, ys []float64
	for _, s := range samples {
		xs = append(xs, s.WoMin)
		ys = append(ys, s.ActualMin-s.PredMin)
	}
	f.MinRaw, f.MinZ = slope(xs, ys)
	f.Minutes = applied(f.MinRaw, f.MinZ, len(xs))
	for _, st := range data.Stats {
		k := data.StatIndex(st)
		var rx, ry []float64
		for _, s := range samples {
			p := s.Pred0.WithMinutes(s.PredMin + f.MinutesShift(s.WoMin))
			mean := p.Mean(st)
			rx = append(rx, s.WoRate[k])
			ry = append(ry, (s.Actual[k]-mean)/math.Max(mean, 1))
		}
		raw, z := slope(rx, ry)
		f.RateRaw[string(st)], f.RateZ[string(st)] = raw, z
		f.Rate[string(st)] = applied(raw, z, len(rx))
	}
	return f
}

// MinutesShift is the change in projected minutes for a player whose absent teammates' history says woMin.
func (f WoFix) MinutesShift(woMin float64) float64 {
	if f.Minutes <= 0 || math.IsNaN(woMin) {
		return 0
	}
	return math.Max(-woMaxMinutes, math.Min(woMaxMinutes, f.Minutes*woMin))
}

// RateFactor multiplies the projected mean of a stat.
func (f WoFix) RateFactor(st data.Stat, woRate float64) float64 {
	m := f.Rate[string(st)]
	if m <= 0 || math.IsNaN(woRate) {
		return 1
	}
	return math.Max(0.7, math.Min(1.4, 1+m*woRate))
}

// Active reports whether anything will be applied.
func (f WoFix) Active() bool {
	if f.Minutes > 0 {
		return true
	}
	for _, v := range f.Rate {
		if v > 0 {
			return true
		}
	}
	return false
}

func (f WoFix) Save(path string) error {
	b, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func LoadWoFix(path string) (WoFix, bool) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return WoFix{}, false
	}
	var f WoFix
	if json.Unmarshal(buf, &f) != nil {
		return WoFix{}, false
	}
	return f, true
}
