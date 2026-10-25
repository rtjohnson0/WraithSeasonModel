// Package backtest scores the projection model against past sportsbook lines.
//
// For every historical line it projects the player using only games before that game's date,
// turns the projection into P(over), and compares with what actually happened. The two things
// that matter are calibration (do stated percentages match reality?) and whether the picks
// that clear the posting threshold beat the break-even rate at the book's real odds.
package backtest

import (
	"math"
	"propsmodel/internal/bets"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/model"
	"propsmodel/internal/proj"
	"propsmodel/internal/props"
)

// Result is one scored line.
type Result struct {
	Date      string
	GameID    int
	PlayerID  int
	Stat      data.Stat
	Line      float64
	POver     float64 // model P(over), pushes removed
	Actual    float64
	OverOdds  int
	UnderOdds int
	Market    float64 // the book's vig-free P(over) from its own prices (0.5 if unknown)
}

// MedianLines keeps one line per (game, player, stat): the middle line across books, so the
// same outcome is not counted several times.
func MedianLines(lines []props.Line) []props.Line {
	type key struct {
		game, player int
		stat         data.Stat
	}
	groups := map[key][]props.Line{}
	var order []key
	for _, l := range lines {
		k := key{l.GameID, l.PlayerID, l.Stat}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], l)
	}
	out := make([]props.Line, 0, len(order))
	for _, k := range order {
		g := groups[k]
		sort.SliceStable(g, func(i, j int) bool { return g[i].Line < g[j].Line })
		out = append(out, g[len(g)/2])
	}
	return out
}

// Run scores each line. Lines are skipped when the game or player result is unknown (the
// player did not play), when there is too little history, or when the player is not eligible
// (20+ average minutes, or projected for 18+).
func Run(ix *proj.Index, lines []props.Line) []Result {
	var out []Result
	for _, l := range MedianLines(lines) {
		date, ok := ix.GameDate(l.GameID)
		if !ok {
			continue
		}
		actual, played := ix.Actual(l.GameID, l.PlayerID)
		if !played {
			continue
		}
		p, ok := ix.Project(l.PlayerID, date)
		if !ok || !model.Eligible(p.AvgMinutes, p.ProjMinutes) {
			continue
		}
		pr, err := model.ProbOver(p.Mean[l.Stat], p.Var[l.Stat], l.Line)
		if err != nil {
			continue
		}
		out = append(out, Result{
			Date:      date,
			GameID:    l.GameID,
			PlayerID:  l.PlayerID,
			Stat:      l.Stat,
			Line:      l.Line,
			POver:     pr.Over,
			Actual:    actual.Value(l.Stat),
			OverOdds:  l.OverOdds,
			UnderOdds: l.UnderOdds,
			Market:    bets.MarketOver(l.OverOdds, l.UnderOdds),
		})
	}
	return out
}

// SortChronologically orders results by game date (then game id).
func SortChronologically(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Date != rs[j].Date {
			return rs[i].Date < rs[j].Date
		}
		return rs[i].GameID < rs[j].GameID
	})
}

// Split cuts chronologically ordered results at the midpoint: fit on the first half, judge on
// the second, so the calibration is tested on games it never saw.
func Split(rs []Result) (first, second []Result) {
	mid := len(rs) / 2
	return rs[:mid], rs[mid:]
}

// Fit fits a calibrator to the results, ignoring pushes.
func Fit(rs []Result) model.Calibrator {
	var p []float64
	var went []bool
	for _, r := range rs {
		if r.Actual == r.Line {
			continue
		}
		p = append(p, r.POver)
		went = append(went, r.Actual > r.Line)
	}
	return model.FitCalibrator(p, went)
}

// Apply returns a copy of the results with POver replaced by the calibrated probability.
func Apply(rs []Result, c model.Calibrator) []Result {
	out := make([]Result, len(rs))
	for i, r := range rs {
		r.POver = c.Apply(r.POver)
		out[i] = r
	}
	return out
}

// PickStats summarises the picks a threshold would have posted.
type PickStats struct {
	Threshold    float64
	Picks        int
	Wins         int
	Losses       int
	Pushes       int
	Units        float64 // profit in units, flat 1-unit bets at the book's odds
	BreakEvenSum float64 // sum over picks of the win probability the odds require
}

// BreakEven is the average win rate the prices paid required (52.4% only at exactly -110).
func (p PickStats) BreakEven() float64 {
	if p.Picks == 0 {
		return 0
	}
	return p.BreakEvenSum / float64(p.Picks)
}

// HitRate is wins / (wins + losses).
func (p PickStats) HitRate() float64 {
	if p.Wins+p.Losses == 0 {
		return 0
	}
	return float64(p.Wins) / float64(p.Wins+p.Losses)
}

// ROI is profit per unit staked.
func (p PickStats) ROI() float64 {
	if p.Picks-p.Pushes == 0 {
		return 0
	}
	return p.Units / float64(p.Picks-p.Pushes)
}

// payout is the profit on a winning 1-unit bet at American odds (-110 if unknown).
func payout(odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	if odds > 0 {
		return float64(odds) / 100
	}
	return 100 / math.Abs(float64(odds))
}

// Evaluate posts OVER when P(over) >= hi and UNDER when P(over) <= 1-hi, then grades the picks.
func Evaluate(results []Result, hi float64) PickStats { return EvaluateEdge(results, hi, -1) }

// EvaluateEdge is Evaluate with one more rule: the pick's probability must also beat the
// break-even probability of the odds actually offered by at least minEdge (a negative minEdge
// turns the rule off). Posting at 55% into a -150 price is a losing bet, so this rule matters.
func EvaluateEdge(results []Result, hi, minEdge float64) PickStats {
	st := PickStats{Threshold: hi}
	for _, r := range results {
		var over bool
		var prob float64
		switch {
		case r.POver >= hi:
			over, prob = true, r.POver
		case r.POver <= 1-hi:
			over, prob = false, 1-r.POver
		default:
			continue
		}
		odds := r.UnderOdds
		if over {
			odds = r.OverOdds
		}
		if minEdge >= 0 && bets.Edge(prob, odds) < minEdge {
			continue
		}
		st.Picks++
		st.BreakEvenSum += bets.Implied(odds)
		switch {
		case r.Actual == r.Line:
			st.Pushes++
		case (r.Actual > r.Line) == over:
			st.Wins++
			st.Units += payout(odds)
		default:
			st.Losses++
			st.Units--
		}
	}
	return st
}

// EvaluateByStat grades picks separately for points, rebounds and assists.
func EvaluateByStat(results []Result, hi, minEdge float64) map[data.Stat]PickStats {
	out := map[data.Stat]PickStats{}
	for _, s := range data.Stats {
		var sub []Result
		for _, r := range results {
			if r.Stat == s {
				sub = append(sub, r)
			}
		}
		out[s] = EvaluateEdge(sub, hi, minEdge)
	}
	return out
}

// Calibration compares predicted P(over) with the real over-rate, ignoring pushes.
func Calibration(results []Result) []model.CalRow {
	var p []float64
	var went []bool
	for _, r := range results {
		if r.Actual == r.Line {
			continue
		}
		p = append(p, r.POver)
		went = append(went, r.Actual > r.Line)
	}
	return model.CalibrationTable(p, went, model.DefaultBins)
}
