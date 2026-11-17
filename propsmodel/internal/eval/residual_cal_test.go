package eval

import (
	"math"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"testing"
)

func TestResidualCalibrationCannotInventDirectionalIntercept(t *testing.T) {
	var rs []backtest.Result
	for i := 0; i < 800; i++ {
		y := 0.
		if i%10 < 6 {
			y = 1
		}
		rs = append(rs, backtest.Result{Date: "2026-02-03", Stat: data.Points, POver: .65, Market: .55, Line: .5, Actual: y})
	}
	c := FitResidualCalibration(rs)
	rule := c.ByStat[string(data.Points)]
	if rule.Intercept != 0 || rule.Slope < 0 || rule.Slope > 1 || math.Abs(rule.Slope+rule.Market-1) > 1e-10 {
		t.Fatalf("invalid symmetric calibration %+v", rule)
	}
	for _, pair := range [][2]float64{{.7, .55}, {.3, .6}, {.5, .5}} {
		a, b := rule.Apply(pair[0], pair[1]), rule.Apply(1-pair[0], 1-pair[1])
		if math.Abs(a+b-1) > 1e-10 {
			t.Fatal("opposite directions not complements")
		}
	}
	if c.TrainedBefore != "2026-02-03" {
		t.Fatal("calibration cutoff missing")
	}
}
