package main

import (
	"math"
	"propsmodel/internal/accuracy"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/predictor"
	"time"
)

// probabilityFloor tests a modest minutes scenario on both sides of the central
// estimate. It is a sensitivity check, not a confidence interval.
func probabilityFloor(p predictor.Pred, st data.Stat, line, market float64, c eval.Cal, side string) float64 {
	floor := 1.
	for _, shift := range []float64{-4, 0, 4} {
		pr, e := p.WithMinutes(math.Max(1, p.Minutes+shift)).ProbOver(st, line)
		if e != nil {
			return 0
		}
		v := c.Apply(pr.Over, market)
		if side == "UNDER" {
			v = 1 - v
		}
		floor = math.Min(floor, v)
	}
	return floor
}
func evidenceGate(picks []scored, report accuracy.Report, date, config string, version int) []scored {
	through, e := time.Parse("2006-01-02", report.Through)
	today, et := time.Parse("2006-01-02", date)
	fresh := e == nil && et == nil && today.Sub(through) <= 14*24*time.Hour
	for i := range picks {
		s := &picks[i]
		if s.Tier != "PICK" {
			continue
		}
		ok, reason := report.Allows(string(s.Stat), s.Side, date, config, version)
		if !fresh {
			ok = false
			reason = "Recent verified forward evaluation is not available"
		}
		if !ok {
			s.Tier = "PAPER"
			s.Units = 0
			s.Why = addWhy(s.Why, reason+"; research pick, not validated for staking")
		}
	}
	return picks
}
