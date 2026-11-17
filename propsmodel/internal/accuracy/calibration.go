package accuracy

import (
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
)

type CalibrationTrial struct {
	TrainN   int                `json:"trainN"`
	TestN    int                `json:"testN"`
	TestDays int                `json:"testDays"`
	ByStat   []CalibrationCheck `json:"byStat"`
}
type CalibrationCheck struct {
	Stat             string  `json:"stat"`
	N                int     `json:"n"`
	Days             int     `json:"days"`
	CandidateLogLoss float64 `json:"candidateLogLoss"`
	ExistingLogLoss  float64 `json:"existingLogLoss"`
	MarketLogLoss    float64 `json:"marketLogLoss"`
	Gain             float64 `json:"gainVsExisting"`
	Z                float64 `json:"dateClusteredZ"`
	Supported        bool    `json:"supported"`
}

// CalibrationExperiment independently calibrates the raw stat distributions on
// earlier dates and compares against frozen existing predictions on later dates.
func CalibrationExperiment(obs []Observation) CalibrationTrial {
	var rs []backtest.Result
	existing := map[string]float64{}
	for _, o := range obs {
		r := o.Result
		existing[calKey(r)] = r.POver
		r.POver = o.Raw
		rs = append(rs, r)
	}
	backtest.SortChronologically(rs)
	train, test := backtest.Split(rs)
	t := CalibrationTrial{TrainN: len(train), TestN: len(test)}
	if len(train) < 300 || len(test) < 100 {
		return t
	}
	dates := map[string]bool{}
	for _, r := range test {
		dates[r.Date] = true
	}
	t.TestDays = len(dates)
	fitted := eval.FitCalibrationStats(train, data.Stats, 300)
	trial := fitted.Results(test)
	for _, st := range data.Stats {
		var candidate, base, book []backtest.Result
		days := map[string]bool{}
		for _, r := range trial {
			if r.Stat != st {
				continue
			}
			candidate = append(candidate, r)
			days[r.Date] = true
			v := r
			r.POver = existing[calKey(r)]
			base = append(base, r)
			v.POver = v.Market
			book = append(book, v)
		}
		gain, z := eval.PairedGain(candidate, base)
		bg, bz := eval.PairedGain(candidate, book)
		c, b, m := eval.Score(candidate), eval.Score(base), eval.Score(book)
		_, direct := fitted.ByStat[string(st)]
		t.ByStat = append(t.ByStat, CalibrationCheck{Stat: string(st), N: c.N, Days: len(days), CandidateLogLoss: c.LogLoss, ExistingLogLoss: b.LogLoss, MarketLogLoss: m.LogLoss, Gain: gain, Z: z, Supported: direct && c.N >= 200 && len(days) >= 30 && gain > 0 && z >= 2 && bg > 0 && bz >= 2})
	}
	return t
}
func calKey(r backtest.Result) string {
	return r.Date + "/" + fmtKey(r.GameID, r.PlayerID) + "/" + string(r.Stat)
}
