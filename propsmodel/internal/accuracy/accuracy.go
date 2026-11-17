// Package accuracy evaluates frozen pregame forecasts; it never tunes on outcomes
// that are subsequently reported as held-out evidence.
package accuracy

import (
	"fmt"
	"math"
	"propsmodel/internal/backtest"
	"propsmodel/internal/eval"
	"sort"
)

type Observation struct {
	Result                                        backtest.Result `json:"result"`
	Pred, Actual, Minutes, ActualMinutes, Typical float64
	Raw                                           float64
}
type Bin struct {
	N         int     `json:"n"`
	Predicted float64 `json:"predicted"`
	Actual    float64 `json:"actual"`
}
type Slice struct {
	Stat            string  `json:"stat"`
	Side            string  `json:"side"`
	N               int     `json:"n"`
	Days            int     `json:"days"`
	HitRate         float64 `json:"hitRate"`
	BreakEven       float64 `json:"breakEven"`
	Lower95         float64 `json:"lower95"`
	Brier           float64 `json:"brier"`
	MarketBrier     float64 `json:"marketBrier"`
	LogLoss         float64 `json:"logLoss"`
	MarketLogLoss   float64 `json:"marketLogLoss"`
	Gain            float64 `json:"gain"`
	Z               float64 `json:"dateClusteredZ"`
	Bias            float64 `json:"actualMinusProjected"`
	MAE             float64 `json:"mae"`
	MinutesBias     float64 `json:"minutesBias"`
	MinutesMAE      float64 `json:"minutesMAE"`
	MeanProbability float64 `json:"meanProbability"`
	OverShare       float64 `json:"overShare"`
	ROI             float64 `json:"roi"`
	Supported       bool    `json:"supported"`
	Reason          string  `json:"reason"`
	Bins            []Bin   `json:"reliability"`
}

func breakeven(o int) float64 {
	if o > 0 {
		return 100 / float64(100+o)
	}
	if o < 0 {
		return float64(-o) / float64(100-o)
	}
	return 1
}
func Summary(obs []Observation, stat, side string) Slice {
	s := Slice{Stat: stat, Side: side, Reason: "Needs more verified forward evidence"}
	dates := map[string]bool{}
	var model, market []backtest.Result
	bins := make([]Bin, 10)
	overs := 0
	for _, o := range obs {
		r := o.Result
		if stat != "all" && string(r.Stat) != stat {
			continue
		}
		chosen := "OVER"
		p := r.POver
		odds := r.OverOdds
		if p < .5 {
			chosen = "UNDER"
			p = 1 - p
			odds = r.UnderOdds
		}
		if side != "ALL" && side != chosen {
			continue
		}
		if r.Actual == r.Line || r.Market <= 0 || r.Market >= 1 || odds == 0 {
			continue
		}
		// The policy is fixed in advance, not chosen using the evaluated outcomes.
		if p < .55 || p-breakeven(odds) < .01 {
			continue
		}
		s.N++
		dates[r.Date] = true
		if chosen == "OVER" {
			overs++
		}
		win := (r.Actual > r.Line) == (chosen == "OVER")
		y := 0.
		if win {
			y = 1
			s.HitRate++
		}
		profit := -1.
		if win {
			if odds > 0 {
				profit = float64(odds) / 100
			} else {
				profit = 100 / float64(-odds)
			}
		}
		s.ROI += profit
		s.BreakEven += breakeven(odds)
		s.MeanProbability += p
		s.Bias += o.Actual - o.Pred
		s.MAE += math.Abs(o.Actual - o.Pred)
		s.MinutesBias += o.ActualMinutes - o.Minutes
		s.MinutesMAE += math.Abs(o.ActualMinutes - o.Minutes)
		k := int(p * 10)
		if k > 9 {
			k = 9
		}
		bins[k].N++
		bins[k].Predicted += p
		bins[k].Actual += y
		model = append(model, r)
		r.POver = r.Market
		market = append(market, r)
	}
	s.Days = len(dates)
	if s.N == 0 {
		return s
	}
	n := float64(s.N)
	s.HitRate /= n
	s.BreakEven /= n
	s.MeanProbability /= n
	s.Bias /= n
	s.MAE /= n
	s.MinutesBias /= n
	s.MinutesMAE /= n
	s.ROI /= n
	s.OverShare = float64(overs) / n
	a, b := eval.Score(model), eval.Score(market)
	s.Brier = a.Brier
	s.MarketBrier = b.Brier
	s.LogLoss = a.LogLoss
	s.MarketLogLoss = b.LogLoss
	s.Gain, s.Z = eval.PairedGain(model, market)
	// Wilson lower endpoint is descriptive; date-clustered gain is also required.
	z := 1.96
	s.Lower95 = (s.HitRate + z*z/(2*n) - z*math.Sqrt(s.HitRate*(1-s.HitRate)/n+z*z/(4*n*n))) / (1 + z*z/n)
	for _, b := range bins {
		if b.N > 0 {
			b.Predicted /= float64(b.N)
			b.Actual /= float64(b.N)
			s.Bins = append(s.Bins, b)
		}
	}
	if s.N >= 200 && s.Days >= 30 && s.Gain > 0 && s.Z >= 2 && s.Brier < s.MarketBrier && s.Lower95 > s.BreakEven && math.Abs(s.HitRate-s.MeanProbability) <= .05 {
		s.Supported = true
		s.Reason = "Passed fixed forward-evidence criteria; not a profitability guarantee"
	}
	return s
}

type MinutesTrial struct {
	Weight      float64 `json:"weight"`
	TrainN      int     `json:"trainN"`
	TestN       int     `json:"testN"`
	TestDays    int     `json:"testDays"`
	TrainBefore string  `json:"trainBefore"`
	BaseMAE     float64 `json:"baseMAE"`
	TrialMAE    float64 `json:"trialMAE"`
	Gain        float64 `json:"gain"`
	Z           float64 `json:"dateClusteredZ"`
	Supported   bool    `json:"supported"`
}

// MinutesExperiment chooses a recent-role blend on earlier dates, then evaluates
// it once on later dates. Nothing is automatically applied to production.
func MinutesExperiment(obs []Observation) MinutesTrial {
	seen := map[string]bool{}
	var rows []Observation
	for _, o := range obs {
		k := o.Result.Date + "/" + fmtKey(o.Result.GameID, o.Result.PlayerID)
		if !seen[k] && o.Typical > 0 {
			seen[k] = true
			rows = append(rows, o)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Result.Date < rows[j].Result.Date })
	t := MinutesTrial{}
	if len(rows) < 2 {
		return t
	}
	cut := len(rows) * 2 / 3
	for cut > 0 && rows[cut-1].Result.Date == rows[cut].Result.Date {
		cut--
	}
	if cut == 0 {
		return t
	}
	t.TrainN = cut
	t.TestN = len(rows) - cut
	t.TrainBefore = rows[cut].Result.Date
	best := math.Inf(1)
	for _, w := range []float64{0, .25, .5, .75, 1} {
		loss := 0.
		for _, o := range rows[:cut] {
			loss += math.Abs(o.ActualMinutes - (o.Minutes*(1-w) + o.Typical*w))
		}
		if loss < best {
			best = loss
			t.Weight = w
		}
	}
	day := map[string][]float64{}
	for _, o := range rows[cut:] {
		base := math.Abs(o.ActualMinutes - o.Minutes)
		trial := math.Abs(o.ActualMinutes - (o.Minutes*(1-t.Weight) + o.Typical*t.Weight))
		t.BaseMAE += base
		t.TrialMAE += trial
		day[o.Result.Date] = append(day[o.Result.Date], base-trial)
	}
	t.BaseMAE /= float64(t.TestN)
	t.TrialMAE /= float64(t.TestN)
	t.Gain = t.BaseMAE - t.TrialMAE
	t.TestDays = len(day)
	// Date-clustered standard error for the observation-weighted mean gain.
	ss := 0.
	for _, v := range day {
		total := 0.
		for _, g := range v {
			total += g - t.Gain
		}
		ss += total * total
	}
	if t.TestDays > 1 && ss > 0 {
		se := math.Sqrt(float64(t.TestDays)/float64(t.TestDays-1)*ss) / float64(t.TestN)
		t.Z = t.Gain / se
	}
	t.Supported = t.TrainN >= 200 && t.TestN >= 100 && t.TestDays >= 30 && t.Gain > 0 && t.Z >= 2
	return t
}

func fmtKey(g, p int) string { return fmt.Sprintf("%d/%d", g, p) }
