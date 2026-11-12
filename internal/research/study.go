// Package research implements a preregistered, chronological comparison.
// It produces research artifacts, never production parameters.
package research

import (
	"fmt"
	"math"
	"propsmodel/internal/backtest"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"sort"
)

type Spec struct {
	Name            string  `json:"name"`
	MinutesBlend    float64 `json:"minutesBlend"`
	DispersionScale float64 `json:"dispersionScale"`
}

var Specs = []Spec{{"current_method", 0, 1}, {"minutes_challenger", .25, 1}, {"minutes_dispersion_challenger", .25, .75}}

type Question struct {
	Results       []backtest.Result `json:"results"`
	Means         []float64         `json:"means"`
	Minutes       []float64         `json:"minutes"`
	ActualMinutes float64           `json:"actualMinutes"`
}
type Score struct {
	Name       string  `json:"name"`
	N          int     `json:"n"`
	Days       int     `json:"days"`
	LogLoss    float64 `json:"logLoss"`
	Brier      float64 `json:"brier"`
	StatMAE    float64 `json:"statMAE"`
	MinutesMAE float64 `json:"minutesMAE"`
	Picks      int     `json:"picks"`
	Overs      int     `json:"overs"`
	Unders     int     `json:"unders"`
	HitRate    float64 `json:"hitRate"`
	ROI        float64 `json:"roi"`
}
type Study struct {
	Reasons           []string    `json:"reasons"`
	FitThrough        string      `json:"fitThrough"`
	ValidationThrough string      `json:"validationThrough"`
	TestFrom          string      `json:"testFrom"`
	TestThrough       string      `json:"testThrough"`
	FitN              int         `json:"fitN"`
	ValidationN       int         `json:"validationN"`
	TestN             int         `json:"testN"`
	Validation        []Score     `json:"validation"`
	Test              []Score     `json:"test"`
	Selected          string      `json:"selected"`
	GainVsReference   float64     `json:"gainVsReference"`
	ZVsReference      float64     `json:"dateClusteredZVsReference"`
	GainVsBook        float64     `json:"gainVsBook"`
	ZVsBook           float64     `json:"dateClusteredZVsBook"`
	Passed            bool        `json:"passedScreen"`
	ByStat            []StatScore `json:"byStat"`
}
type StatScore struct {
	Stat   string  `json:"stat"`
	Scores []Score `json:"scores"`
}

func splits(qs []Question) (int, int, error) {
	if len(qs) < 500 {
		return 0, 0, fmt.Errorf("need at least 500 matched historical questions")
	}
	a, b := len(qs)*6/10, len(qs)*8/10
	for a > 0 && qs[a-1].Results[0].Date == qs[a].Results[0].Date {
		a--
	}
	for b > a && qs[b-1].Results[0].Date == qs[b].Results[0].Date {
		b--
	}
	if a < 200 || b-a < 100 || len(qs)-b < 100 {
		return 0, 0, fmt.Errorf("need 200 fit, 100 validation and 100 final-test questions on separate dates")
	}
	return a, b, nil
}
func results(qs []Question, variant int, cal *eval.Calibration, book bool) []backtest.Result {
	var out []backtest.Result
	for _, q := range qs {
		r := q.Results[variant]
		if book {
			r.POver = r.Market
		} else if cal != nil {
			r.POver = cal.Apply(r.Stat, r.POver, r.Market)
		}
		out = append(out, r)
	}
	return out
}
func score(name string, qs []Question, rs []backtest.Result, v int) Score {
	m := eval.Score(rs)
	s := Score{Name: name, N: m.N, LogLoss: m.LogLoss, Brier: m.Brier}
	days := map[string]bool{}
	n := 0
	for i, r := range rs {
		days[r.Date] = true
		if v >= 0 {
			s.StatMAE += math.Abs(qs[i].Means[v] - r.Actual)
			s.MinutesMAE += math.Abs(qs[i].Minutes[v] - qs[i].ActualMinutes)
			n++
		}
		p, odds, over := r.POver, r.OverOdds, true
		if p < .5 {
			p = 1 - p
			odds = r.UnderOdds
			over = false
		}
		be := 1.
		if odds > 0 {
			be = 100 / float64(100+odds)
		} else if odds < 0 {
			be = float64(-odds) / float64(100-odds)
		}
		if r.Actual == r.Line || p < .55 || p-be < .01 {
			continue
		}
		s.Picks++
		if over {
			s.Overs++
		} else {
			s.Unders++
		}
		win := (r.Actual > r.Line) == over
		if win {
			s.HitRate++
			if odds > 0 {
				s.ROI += float64(odds) / 100
			} else {
				s.ROI += 100 / float64(-odds)
			}
		} else {
			s.ROI--
		}
	}
	s.Days = len(days)
	if n > 0 {
		s.StatMAE /= float64(n)
		s.MinutesMAE /= float64(n)
	}
	if s.Picks > 0 {
		s.HitRate /= float64(s.Picks)
		s.ROI /= float64(s.Picks)
	}
	return s
}
func Run(qs []Question) (Study, error) {
	s := Study{}
	sort.Slice(qs, func(i, j int) bool {
		a, b := qs[i].Results[0], qs[j].Results[0]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.GameID != b.GameID {
			return a.GameID < b.GameID
		}
		if a.PlayerID != b.PlayerID {
			return a.PlayerID < b.PlayerID
		}
		return a.Stat < b.Stat
	})
	for _, q := range qs {
		if len(q.Results) != len(Specs) || len(q.Means) != len(Specs) || len(q.Minutes) != len(Specs) {
			return s, fmt.Errorf("unmatched variants")
		}
		for _, r := range q.Results {
			a := q.Results[0]
			if r.Date != a.Date || r.GameID != a.GameID || r.PlayerID != a.PlayerID || r.Stat != a.Stat || r.Line != a.Line || r.Actual != a.Actual || r.OverOdds != a.OverOdds || r.UnderOdds != a.UnderOdds || r.Market != a.Market {
				return s, fmt.Errorf("question identity differs between variants")
			}
		}
		for _, r := range q.Results {
			if r.Market <= 0 || r.Market >= 1 || r.POver < 0 || r.POver > 1 || math.IsNaN(r.POver) {
				return s, fmt.Errorf("invalid probability")
			}
		}
	}
	a, b, e := splits(qs)
	if e != nil {
		return s, e
	}
	s.FitN = a
	s.ValidationN = b - a
	s.TestN = len(qs) - b
	s.FitThrough = qs[a-1].Results[0].Date
	s.ValidationThrough = qs[b-1].Results[0].Date
	s.TestFrom = qs[b].Results[0].Date
	s.TestThrough = qs[len(qs)-1].Results[0].Date
	cals := make([]eval.Calibration, len(Specs))
	best := -1
	bestLoss := math.Inf(1)
	for i, spec := range Specs {
		cals[i] = eval.FitCalibrationStats(results(qs[:a], i, nil, false), data.Stats, 300)
		rs := results(qs[a:b], i, &cals[i], false)
		sc := score(spec.Name, qs[a:b], rs, i)
		s.Validation = append(s.Validation, sc)
		if i > 0 && sc.LogLoss < bestLoss {
			best = i
			bestLoss = sc.LogLoss
		}
	}
	// The challenger is chosen once from validation; final test never selects it.
	s.Selected = Specs[best].Name
	base := results(qs[b:], 0, &cals[0], false)
	candidate := results(qs[b:], best, &cals[best], false)
	book := results(qs[b:], 0, nil, true)
	s.Test = []Score{score(Specs[0].Name, qs[b:], base, 0), score(Specs[best].Name, qs[b:], candidate, best), score("sportsbook", qs[b:], book, -1)}
	s.GainVsReference, s.ZVsReference = eval.PairedGain(candidate, base)
	s.GainVsBook, s.ZVsBook = eval.PairedGain(candidate, book)
	s.Passed = s.Test[1].N >= 200 && s.Test[1].Days >= 30 && bestLoss < s.Validation[0].LogLoss && s.GainVsReference > 0 && s.ZVsReference >= 2 && s.GainVsBook > 0 && s.ZVsBook >= 2 && s.Test[1].Brier < s.Test[0].Brier
	if bestLoss >= s.Validation[0].LogLoss {
		s.Reasons = append(s.Reasons, "Challenger did not beat the reference on validation dates")
	}
	if s.Test[1].N < 200 || s.Test[1].Days < 30 {
		s.Reasons = append(s.Reasons, "Insufficient final-test outcomes or date coverage")
	}
	if s.GainVsReference <= 0 || s.ZVsReference < 2 {
		s.Reasons = append(s.Reasons, "No clear date-clustered final-test gain over reference")
	}
	if s.GainVsBook <= 0 || s.ZVsBook < 2 {
		s.Reasons = append(s.Reasons, "No clear final-test gain over sportsbook baseline")
	}
	if s.Test[1].Brier >= s.Test[0].Brier {
		s.Reasons = append(s.Reasons, "Final Brier score did not improve")
	}
	for _, st := range data.Stats {
		var subset []Question
		for _, q := range qs[b:] {
			if q.Results[0].Stat == st {
				subset = append(subset, q)
			}
		}
		s.ByStat = append(s.ByStat, StatScore{Stat: string(st), Scores: []Score{score(Specs[0].Name, subset, results(subset, 0, &cals[0], false), 0), score(Specs[best].Name, subset, results(subset, best, &cals[best], false), best), score("sportsbook", subset, results(subset, 0, nil, true), -1)}})
	}
	return s, nil
}
