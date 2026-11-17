package eval

import (
	"math"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
)

// FitResidualCalibration fits a symmetric market-offset logistic correction.
// P=logistic(logit(book)+b*(logit(model)-logit(book))), 0<=b<=1.
// No direction intercept is permitted. This is a challenger, not a replacement
// for an empirically supported intercept. Fewer than 300 outcomes use the book.
func FitResidualCalibration(rs []backtest.Result) Calibration {
	c := Calibration{ByStat: map[string]Cal{}, Global: Cal{Market: 1}}
	fit := func(rows []backtest.Result) Cal {
		n := 0
		for _, r := range rows {
			if r.Actual != r.Line {
				n++
			}
		}
		if n < 300 {
			return Cal{Market: 1}
		}
		b := .2
		for it := 0; it < 40; it++ {
			g, h := -25*b, 25.
			for _, r := range rows {
				if r.Actual == r.Line {
					continue
				}
				x := logit(r.POver) - logit(marketOf(r))
				p := sigmoid(logit(marketOf(r)) + b*x)
				y := 0.
				if r.Actual > r.Line {
					y = 1
				}
				g += (y - p) * x
				h += p * (1 - p) * x * x
			}
			next := math.Min(1, math.Max(0, b+g/h))
			if math.Abs(next-b) < 1e-8 {
				b = next
				break
			}
			b = next
		}
		return Cal{Slope: b, Market: 1 - b}
	}
	c.Global = fit(rs)
	for _, r := range rs {
		if r.Date > c.TrainedBefore {
			c.TrainedBefore = r.Date
		}
	}
	for _, st := range data.Stats {
		var sub []backtest.Result
		for _, r := range rs {
			if r.Stat == st {
				sub = append(sub, r)
			}
		}
		c.ByStat[string(st)] = fit(sub)
	}
	return c
}
