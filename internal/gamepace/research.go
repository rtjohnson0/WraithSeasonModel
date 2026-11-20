package gamepace

import (
	"math"
	"propsmodel/internal/eval"
	"sort"
)

type Context struct {
	Calibration   map[string]MarketFit `json:"calibration"`
	AsOf          string               `json:"asOf"`
	Date          string               `json:"date"`
	Model         *Model               `json:"model"`
	Baseline      Input                `json:"baseline"`
	Used          Input                `json:"used"`
	Forecast      Forecast             `json:"forecast"`
	LineupApplied bool                 `json:"lineupApplied"`
	LineupAt      string               `json:"lineupAt,omitempty"`
	Reason        string               `json:"reason"`
	Scenarios     []Scenario           `json:"scenarios,omitempty"`
}
type Scenario struct {
	Player   string  `json:"player"`
	PlayerID int     `json:"playerId"`
	Team     int     `json:"team"`
	Margin   float64 `json:"margin"`
	Total    float64 `json:"total"`
}

// LineupForecast holds learned player-composition effects to a bounded ±3 points
// per team relative to the earlier-workload baseline. These associations have
// not been validated as causal absence effects. No extra injury bonus is added.
func LineupForecast(m *Model, base, used Input) Forecast {
	a, b := m.Predict(base.X), m.Predict(used.X)
	b.HomeScore = clamp(b.HomeScore, a.HomeScore-3, a.HomeScore+3)
	b.AwayScore = clamp(b.AwayScore, a.AwayScore-3, a.AwayScore+3)
	b.Margin = b.HomeScore - b.AwayScore
	b.Total = b.HomeScore + b.AwayScore
	b.HomeEff = (b.HomeScore - m.OvertimePointsPerTeam) / b.Pace
	b.AwayEff = (b.AwayScore - m.OvertimePointsPerTeam) / b.Pace
	return b
}

type Replay struct {
	Date                                     string `json:"date"`
	GameID                                   int    `json:"gameId"`
	ActualMargin, ActualTotal, Margin, Total float64
}
type Study struct {
	Policy                                       string `json:"policy"`
	Before                                       string `json:"before"`
	Games, Dates                                 int
	MarginMAE, TotalMAE, MarginSigma, TotalSigma float64
	Replays                                      []Replay `json:"replays,omitempty"`
}

// MonthlyReplay refits the entire model only on earlier months. The historical
// lineup features are earlier-workload forecasts, never observed final rotations.
func MonthlyReplay(samples []Sample, before string) Study {
	out := Study{Policy: Policy, Before: before}
	months := map[string]bool{}
	for _, s := range samples {
		if s.Game.Date < before {
			months[s.Game.Date[:7]] = true
		}
	}
	keys := []string{}
	for m := range months {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	dates := map[string]bool{}
	for _, month := range keys {
		model, err := Fit(samples, month+"-01")
		if err != nil {
			continue
		}
		for _, s := range samples {
			if s.Game.Date[:7] != month || s.Game.Date >= before {
				continue
			}
			f := model.Predict(s.Input.X)
			actualM := s.Game.HomeScore - s.Game.AwayScore
			actualT := s.Game.HomeScore + s.Game.AwayScore
			em, et := actualM-f.Margin, actualT-f.Total
			out.MarginMAE += math.Abs(em)
			out.TotalMAE += math.Abs(et)
			out.MarginSigma += em * em
			out.TotalSigma += et * et
			dates[s.Game.Date] = true
			out.Replays = append(out.Replays, Replay{s.Game.Date, s.Game.ID, actualM, actualT, f.Margin, f.Total})
		}
	}
	out.Games = len(out.Replays)
	out.Dates = len(dates)
	if out.Games > 0 {
		n := float64(out.Games)
		out.MarginMAE /= n
		out.TotalMAE /= n
		out.MarginSigma = math.Sqrt(out.MarginSigma / n)
		out.TotalSigma = math.Sqrt(out.TotalSigma / n)
	}
	return out
}

type Outcome struct {
	Date, Market string
	Raw, Book, Y float64
}
type MarketFit struct {
	N, Dates int
	Through  string
	Ready    bool
	Cal      eval.Cal
}

// FitMarkets uses only this policy's frozen, graded earlier decisions, separately
// by market. Symmetric market-offset calibration has no global Over/home bonus.
// At least 300 games and 30 dates are required; missing evidence remains an estimate.
func FitMarkets(rows []Outcome, before string) map[string]MarketFit {
	out := map[string]MarketFit{}
	for _, mk := range []string{"moneyline", "spread", "total"} {
		f := MarketFit{}
		ds := map[string]bool{}
		rs := []Outcome{}
		for _, r := range rows {
			if r.Market != mk || r.Date >= before || !finite(r.Raw) || !finite(r.Book) || (r.Y != 0 && r.Y != 1) || r.Raw <= 0 || r.Raw >= 1 || r.Book <= 0 || r.Book >= 1 {
				continue
			}
			rs = append(rs, r)
			ds[r.Date] = true
			if r.Date > f.Through {
				f.Through = r.Date
			}
		}
		f.N = len(rs)
		f.Dates = len(ds)
		f.Ready = f.N >= 300 && f.Dates >= 30
		if f.Ready {
			b := .2
			for it := 0; it < 40; it++ {
				g, h := -25*b, 25.
				for _, r := range rs {
					x := logit(r.Raw) - logit(r.Book)
					p := sigmoid(logit(r.Book) + b*x)
					g += (r.Y - p) * x
					h += p * (1 - p) * x * x
				}
				next := clamp(b+g/h, 0, 1)
				if math.Abs(next-b) < 1e-8 {
					b = next
					break
				}
				b = next
			}
			f.Cal = eval.Cal{Slope: b, Market: 1 - b}
		}
		out[mk] = f
	}
	return out
}
func logit(p float64) float64   { p = clamp(p, 1e-6, 1-1e-6); return math.Log(p / (1 - p)) }
func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

type MarketScore struct {
	N, Dates, CalibratedN                         int
	RawLogLoss, BookLogLoss, CalibratedLogLoss    *float64
	CalibrationRawLogLoss, CalibrationBookLogLoss *float64
}

// EvaluateMarkets grades frozen directions separately. A fitted probability is
// evaluated only after fitting on strictly earlier dates; training loss is never
// reported as calibrated performance. Different markets never pool outcomes.
func EvaluateMarkets(rows []Outcome, before string) map[string]MarketScore {
	out := map[string]MarketScore{}
	for _, market := range []string{"moneyline", "spread", "total"} {
		s := MarketScore{}
		dates := map[string]bool{}
		byDate := map[string]MarketFit{}
		raw, book, cal, calRaw, calBook := 0., 0., 0., 0., 0.
		loss := func(p, y float64) float64 { p = clamp(p, 1e-6, 1-1e-6); return -y*math.Log(p) - (1-y)*math.Log(1-p) }
		for _, r := range rows {
			if r.Date >= before || r.Market != market || !finite(r.Raw) || !finite(r.Book) || r.Raw <= 0 || r.Raw >= 1 || r.Book <= 0 || r.Book >= 1 || (r.Y != 0 && r.Y != 1) {
				continue
			}
			s.N++
			dates[r.Date] = true
			raw += loss(r.Raw, r.Y)
			book += loss(r.Book, r.Y)
			fit, ok := byDate[r.Date]
			if !ok {
				fit = FitMarkets(rows, r.Date)[market]
				byDate[r.Date] = fit
			}
			if fit.Ready {
				s.CalibratedN++
				cal += loss(fit.Cal.Apply(r.Raw, r.Book), r.Y)
				calRaw += loss(r.Raw, r.Y)
				calBook += loss(r.Book, r.Y)
			}
		}
		s.Dates = len(dates)
		if s.N > 0 {
			raw /= float64(s.N)
			book /= float64(s.N)
			s.RawLogLoss = &raw
			s.BookLogLoss = &book
		}
		if s.CalibratedN > 0 {
			n := float64(s.CalibratedN)
			cal /= n
			calRaw /= n
			calBook /= n
			s.CalibratedLogLoss = &cal
			s.CalibrationRawLogLoss = &calRaw
			s.CalibrationBookLogLoss = &calBook
		}
		out[market] = s
	}
	return out
}
