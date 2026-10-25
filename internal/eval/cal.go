package eval

import (
	"encoding/json"
	"math"
	"os"

	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/linalg"
)

// Cal turns the model's probability and the sportsbook's own probability into a final one:
//
//	P = sigmoid(Intercept + Slope*logit(P_model) + Market*logit(P_market))
//
// P_market is the vig-free probability implied by the book's two prices. This is the step that
// blends the model with the market. Slope near 0 means the model adds nothing beyond the
// market; Market near 1 means the book's prices already carry the information.
type Cal struct {
	Intercept float64 `json:"intercept"`
	Slope     float64 `json:"slope"`
	Market    float64 `json:"market"`
}

// IdentityCal passes the model probability through unchanged.
var IdentityCal = Cal{Slope: 1}

func logit(p float64) float64 {
	p = math.Min(math.Max(p, 1e-6), 1-1e-6)
	return math.Log(p / (1 - p))
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// Apply returns the final P(over).
func (c Cal) Apply(pModel, pMarket float64) float64 {
	return sigmoid(c.Intercept + c.Slope*logit(pModel) + c.Market*logit(pMarket))
}

// fitLogistic fits y on the columns of x (plus an intercept) by Newton's method with a small
// ridge penalty on the slopes. It returns [intercept, coef...].
func fitLogistic(x [][]float64, y []float64) []float64 {
	k := len(x[0]) + 1
	beta := make([]float64, k)
	var ybar float64
	for _, v := range y {
		ybar += v
	}
	ybar = math.Min(math.Max(ybar/float64(len(y)), 0.01), 0.99)
	beta[0] = math.Log(ybar / (1 - ybar))
	const ridge = 1e-3
	for iter := 0; iter < 30; iter++ {
		h := make([][]float64, k)
		for i := range h {
			h[i] = make([]float64, k)
		}
		g := make([]float64, k)
		row := make([]float64, k)
		for i := range x {
			row[0] = 1
			copy(row[1:], x[i])
			eta := 0.0
			for j := 0; j < k; j++ {
				eta += beta[j] * row[j]
			}
			p := sigmoid(eta)
			w := p * (1 - p)
			for a := 0; a < k; a++ {
				g[a] += (y[i] - p) * row[a]
				for b := 0; b <= a; b++ {
					h[a][b] += w * row[a] * row[b]
				}
			}
		}
		for a := 0; a < k; a++ {
			for b := a + 1; b < k; b++ {
				h[a][b] = h[b][a]
			}
			if a > 0 {
				h[a][a] += ridge
				g[a] -= ridge * beta[a]
			}
		}
		step, err := linalg.SolveSPD(h, g)
		if err != nil {
			break
		}
		var d float64
		for j := range beta {
			beta[j] += step[j]
			d = math.Max(d, math.Abs(step[j]))
		}
		if d < 1e-8 {
			break
		}
	}
	return beta
}

// fitCal fits a Cal. With fewer than 200 examples it returns IdentityCal. If the model's slope
// comes out negative (pure noise), the model is dropped and only the market and an intercept are kept.
func fitCal(rs []backtest.Result) Cal {
	var x [][]float64
	var y []float64
	for _, r := range rs {
		if r.Actual == r.Line {
			continue
		}
		x = append(x, []float64{logit(r.POver), logit(marketOf(r))})
		if r.Actual > r.Line {
			y = append(y, 1)
		} else {
			y = append(y, 0)
		}
	}
	if len(y) < 200 {
		return IdentityCal
	}
	b := fitLogistic(x, y)
	if b[1] < 0 {
		m := make([][]float64, len(x))
		for i := range x {
			m[i] = []float64{x[i][1]}
		}
		bm := fitLogistic(m, y)
		return Cal{Intercept: bm[0], Slope: 0, Market: bm[1]}
	}
	return Cal{Intercept: b[0], Slope: b[1], Market: b[2]}
}

func marketOf(r backtest.Result) float64 {
	if r.Market <= 0 || r.Market >= 1 {
		return 0.5
	}
	return r.Market
}

// Calibration holds the final-probability rule: one global fit, plus one per stat where there
// is enough data.
type Calibration struct {
	Config string         `json:"config"` // "pregame" or "lineup": which feature set it was fitted for
	Global Cal            `json:"global"`
	ByStat map[string]Cal `json:"by_stat"`
}

const minPerStat = 1500

// FitCalibration fits the rule on rs.
func FitCalibration(rs []backtest.Result) Calibration {
	c := Calibration{Global: fitCal(rs), ByStat: map[string]Cal{}}
	for _, s := range data.Stats {
		var sub []backtest.Result
		for _, r := range rs {
			if r.Stat == s {
				sub = append(sub, r)
			}
		}
		if len(sub) >= minPerStat {
			c.ByStat[string(s)] = fitCal(sub)
		}
	}
	return c
}

// Apply returns the final P(over) for a stat, given the model's P(over) and the book's.
func (c Calibration) Apply(s data.Stat, pModel, pMarket float64) float64 {
	if cal, ok := c.ByStat[string(s)]; ok {
		return cal.Apply(pModel, pMarket)
	}
	return c.Global.Apply(pModel, pMarket)
}

// Results returns a copy of rs with the final probabilities.
func (c Calibration) Results(rs []backtest.Result) []backtest.Result {
	out := make([]backtest.Result, len(rs))
	for i, r := range rs {
		r.POver = c.Apply(r.Stat, r.POver, marketOf(r))
		out[i] = r
	}
	return out
}

// Save writes the calibration as JSON.
func (c Calibration) Save(path string) error {
	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// LoadCalibration reads a saved calibration. Files from the earlier format (no market term)
// still load, with the market weight at zero.
func LoadCalibration(path string) (Calibration, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Calibration{}, false
	}
	var c Calibration
	if json.Unmarshal(b, &c) != nil {
		return Calibration{}, false
	}
	if c.ByStat == nil {
		c.ByStat = map[string]Cal{}
	}
	return c, true
}
