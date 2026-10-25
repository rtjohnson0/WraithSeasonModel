// Package eval runs honest tests of the predictor: walk-forward backtests (the model is always
// retrained only on games before the month it predicts), scoring metrics, per-stat
// calibration, and feature-group ablation.
package eval

import (
	"math"
	"sort"

	"propsmodel/internal/backtest"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
)

type rowKey struct{ game, player int }

// WalkForward scores every eligible line in testSeason. For each calendar month the model is
// retrained on all rows dated before that month, so no prediction sees its own future.
// disabled lists feature groups to leave out (ablation).
func WalkForward(rows []feat.Row, lines []props.Line, testSeason int, disabled []string) ([]backtest.Result, error) {
	return WalkForwardSeasons(rows, lines, []int{testSeason}, disabled)
}

// WalkForwardSeasons is WalkForward over several test seasons at once, for example last season's
// opening lines plus this season's live snapshots.
func WalkForwardSeasons(rows []feat.Row, lines []props.Line, testSeasons []int, disabled []string) ([]backtest.Result, error) {
	out, _, err := WalkForwardMinutes(rows, lines, testSeasons, disabled)
	return out, err
}

// MinSample is one out-of-sample minutes prediction: the model's, the plain recent average
// (the number a person would eyeball), and what happened.
type MinSample struct {
	Date                string
	Pred, Naive, Actual float64
	Eligible            bool
	EarlySeason         bool // within the first three weeks of the season in the data
}

// WalkForwardMinutes is WalkForwardSeasons that also returns, for every player-game in the test
// months (not only those with a prop line), the minutes the model projected before tip-off.
func WalkForwardMinutes(rows []feat.Row, lines []props.Line, testSeasons []int, disabled []string) ([]backtest.Result, []MinSample, error) {
	inTest := map[int]bool{}
	for _, s := range testSeasons {
		inTest[s] = true
	}
	byKey := make(map[rowKey]*feat.Row, len(rows))
	for i := range rows {
		byKey[rowKey{rows[i].G.GameID, rows[i].G.PlayerID}] = &rows[i]
	}
	// rows are in date order, so "rows before month M" is a prefix.
	firstIdx := func(month string) int {
		return sort.Search(len(rows), func(i int) bool { return rows[i].G.Date >= month+"-01" })
	}

	byMonth := map[string][]props.Line{}
	for _, l := range backtest.MedianLines(lines) {
		r, ok := byKey[rowKey{l.GameID, l.PlayerID}]
		if !ok || !inTest[r.G.Season] {
			continue
		}
		month := r.G.Date[:7]
		byMonth[month] = append(byMonth[month], l)
	}
	months := make([]string, 0, len(byMonth))
	for m := range byMonth {
		months = append(months, m)
	}
	sort.Strings(months)

	seasonStart := map[int]int{}
	for i := range rows {
		if d := rows[i].G.Day; seasonStart[rows[i].G.Season] == 0 || d < seasonStart[rows[i].G.Season] {
			seasonStart[rows[i].G.Season] = d
		}
	}
	var out []backtest.Result
	var samples []MinSample
	for _, month := range months {
		n := firstIdx(month)
		if n < 500 {
			continue
		}
		m, err := predictor.Train(rows[:n], disabled)
		if err != nil {
			return nil, nil, err
		}
		for i := n; i < len(rows) && rows[i].G.Date[:7] == month; i++ {
			r := &rows[i]
			if !inTest[r.G.Season] || r.G.Minutes < 1 {
				continue
			}
			pred := m.Predict(r)
			naive := r.Vec(data.Points, []string{"m_ewma"})[0]
			if math.IsNaN(naive) || math.IsNaN(pred.Minutes) {
				continue
			}
			samples = append(samples, MinSample{
				Date: r.G.Date, Pred: pred.Minutes, Naive: naive, Actual: r.G.Minutes,
				Eligible:    model.Eligible(r.AvgMin15, pred.Minutes),
				EarlySeason: r.G.Day-seasonStart[r.G.Season] < 21,
			})
		}
		for _, l := range byMonth[month] {
			r := byKey[rowKey{l.GameID, l.PlayerID}]
			pred := m.Predict(r)
			if !model.Eligible(r.AvgMin15, pred.Minutes) {
				continue
			}
			pr, err := pred.ProbOver(l.Stat, l.Line)
			if err != nil {
				continue
			}
			out = append(out, backtest.Result{
				Date: r.G.Date, GameID: l.GameID, PlayerID: l.PlayerID, Stat: l.Stat, Line: l.Line,
				POver: pr.Over, Actual: r.G.Value(l.Stat), OverOdds: l.OverOdds, UnderOdds: l.UnderOdds,
				Market: bets.MarketOver(l.OverOdds, l.UnderOdds),
			})
		}
	}
	backtest.SortChronologically(out)
	return out, samples, nil
}

// Metrics scores probabilistic predictions, ignoring pushes.
type Metrics struct {
	N       int
	LogLoss float64 // lower is better; a coin flip scores 0.6931
	Brier   float64 // lower is better; a coin flip scores 0.25
	OverPct float64 // share of results that went over
}

// Score computes Metrics for results.
func Score(rs []backtest.Result) Metrics {
	var m Metrics
	var over float64
	for _, r := range rs {
		if r.Actual == r.Line {
			continue
		}
		p := math.Min(math.Max(r.POver, 1e-6), 1-1e-6)
		y := 0.0
		if r.Actual > r.Line {
			y = 1
			over++
		}
		m.LogLoss += -(y*math.Log(p) + (1-y)*math.Log(1-p))
		m.Brier += (p - y) * (p - y)
		m.N++
	}
	if m.N > 0 {
		m.LogLoss /= float64(m.N)
		m.Brier /= float64(m.N)
		m.OverPct = over / float64(m.N)
	}
	return m
}

// Intersect keeps only the results present in both lists (same game, player, stat), so two
// models are compared on identical questions.
func Intersect(a, b []backtest.Result) (ar, br []backtest.Result) {
	type k struct {
		g, p int
		s    data.Stat
	}
	idx := map[k]backtest.Result{}
	for _, r := range b {
		idx[k{r.GameID, r.PlayerID, r.Stat}] = r
	}
	for _, r := range a {
		if o, ok := idx[k{r.GameID, r.PlayerID, r.Stat}]; ok {
			ar = append(ar, r)
			br = append(br, o)
		}
	}
	return ar, br
}

// Holdout fits the calibration on the first half of rs (by date) and returns the second half
// both raw and calibrated: the fair way to judge the calibrated model.
func Holdout(rs []backtest.Result) (raw, cal []backtest.Result, c Calibration) {
	first, second := backtest.Split(rs)
	c = FitCalibration(first)
	return second, c.Results(second), c
}

// AblationRow is one feature group's contribution.
type AblationRow struct {
	Group   string
	LogLoss float64 // calibrated, second-half log loss with the group removed
	Delta   float64 // LogLoss minus the full model's; positive means the group helped
}

// Ablate retrains the walk-forward backtest once per feature group with that group removed
// and reports how much worse (or better) the calibrated hold-out log loss gets.
//
// base lists the groups already switched off in the full model being compared (for example
// teammates in the "pregame" set). Each run keeps them off, so the only difference between a
// run and the full model is the one group under test.
func Ablate(rows []feat.Row, lines []props.Line, testSeason int, groups []string, full Metrics, base []string) ([]AblationRow, error) {
	var out []AblationRow
	for _, g := range groups {
		rs, err := WalkForward(rows, lines, testSeason, append(append([]string(nil), base...), g))
		if err != nil {
			return nil, err
		}
		_, cal, _ := Holdout(rs)
		m := Score(cal)
		out = append(out, AblationRow{Group: g, LogLoss: m.LogLoss, Delta: m.LogLoss - full.LogLoss})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Delta > out[j].Delta })
	return out, nil
}

func lossOf(r backtest.Result) (float64, bool) {
	if r.Actual == r.Line {
		return 0, false
	}
	p := math.Min(math.Max(r.POver, 1e-6), 1-1e-6)
	if r.Actual > r.Line {
		return -math.Log(p), true
	}
	return -math.Log(1 - p), true
}

func zScore(diffs []float64) (mean, z float64) {
	n := float64(len(diffs))
	if n < 2 {
		return 0, 0
	}
	for _, d := range diffs {
		mean += d
	}
	mean /= n
	var ss float64
	for _, d := range diffs {
		ss += (d - mean) * (d - mean)
	}
	se := math.Sqrt(ss/(n-1)) / math.Sqrt(n)
	if se == 0 {
		return mean, 0
	}
	return mean, mean / se
}

// SkillVsCoinFlip is how much better (positive) the predictions score than always saying 50%,
// in log loss per line, with a z-score. Lines are set near 50/50, so this is the bar a model
// has to clear to know anything the sportsbook does not. Treat z below about 2 as noise.
func SkillVsCoinFlip(rs []backtest.Result) (gain, z float64) {
	var d []float64
	for _, r := range rs {
		if l, ok := lossOf(r); ok {
			d = append(d, math.Ln2-l)
		}
	}
	return zScore(d)
}

// PairedGain is how much better model a scores than model b on the same lines (a and b must
// be aligned, as Intersect returns them), with a z-score.
func PairedGain(a, b []backtest.Result) (gain, z float64) {
	var d []float64
	for i := range a {
		la, ok1 := lossOf(a[i])
		lb, ok2 := lossOf(b[i])
		if ok1 && ok2 {
			d = append(d, lb-la)
		}
	}
	return zScore(d)
}
