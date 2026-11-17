package team

import (
	"encoding/json"
	"math"
	"os"

	"propsmodel/internal/glm"
)

// Rec is one game for the second stage: the market's number, the ratings model's number and
// what happened.
type Rec struct {
	Market, Proj, Actual float64
}

// Mix combines the market line with the ratings projection by regression on past games:
// actual = a + b*market + c*projection. If c comes out near zero the ratings add nothing the
// line did not already know, and the output is just the market.
type Mix struct {
	Lin   *glm.Linear `json:"lin"`
	Sigma float64     `json:"sigma"`
	N     int         `json:"n"`
}

// FitMix fits the second stage.
func FitMix(rs []Rec) (*Mix, error) {
	X := make([][]float64, len(rs))
	y := make([]float64, len(rs))
	for i, r := range rs {
		X[i] = []float64{r.Market, r.Proj}
		y[i] = r.Actual
	}
	l, err := glm.FitLinear(X, y, []string{"market", "projection"}, 5)
	if err != nil {
		return nil, err
	}
	var ss float64
	for i := range X {
		d := l.Predict(X[i]) - y[i]
		ss += d * d
	}
	return &Mix{Lin: l, Sigma: math.Sqrt(ss / float64(len(rs))), N: len(rs)}, nil
}

// Mean is the blended expectation for a game.
func (m *Mix) Mean(market, proj float64) float64 { return m.Lin.Predict([]float64{market, proj}) }

// Weights returns the raw weight on the market and on the projection.
func (m *Mix) Weights() (market, proj float64) {
	return m.Lin.Coef[0] / m.Lin.Std[0], m.Lin.Coef[1] / m.Lin.Std[1]
}

// Calibration is the saved second stage for margins and totals.
type Calibration struct {
	Margin *Mix `json:"margin"`
	Total  *Mix `json:"total"`
	// Z is, per market (spread, total, moneyline), how many standard errors the model beat the
	// book's own price by in the backtest. A market with Z below 2 has shown no edge, and the
	// game picks command will not stake it.
	Z map[string]float64 `json:"z,omitempty"`
}

func (c Calibration) Save(path string) error {
	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func LoadCalibration(path string) (Calibration, bool) {
	var c Calibration
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &c) != nil || c.Margin == nil || c.Total == nil {
		return c, false
	}
	return c, true
}
