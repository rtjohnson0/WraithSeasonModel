package eval

import (
	"math"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/predictor"
)

// BiasSample is one out-of-sample projection of minutes and of every stat's mean, with what happened.
type BiasSample struct {
	Date        string
	EarlySeason bool
	PredMin     float64
	ActualMin   float64
	Pred        [data.NStats]float64
	Actual      [data.NStats]float64
}

// BiasCheck projects every eligible player-game of one season with a model trained only on games
// before that game's month, and returns the projections next to the results. It needs no prop lines,
// so it works on the current season while it is still being played. If the model is biased (it
// projects too low for everyone, say), this is where it shows, long before a record can.
func BiasCheck(rows []feat.Row, season int, disabled []string) ([]BiasSample, error) {
	firstIdx := func(month string) int {
		return sort.Search(len(rows), func(i int) bool { return rows[i].G.Date >= month+"-01" })
	}
	start := 0
	for i := range rows {
		if rows[i].G.Season == season && (start == 0 || rows[i].G.Day < start) {
			start = rows[i].G.Day
		}
	}
	var months []string
	seen := map[string]bool{}
	for i := range rows {
		if rows[i].G.Season == season {
			if m := rows[i].G.Date[:7]; !seen[m] {
				seen[m] = true
				months = append(months, m)
			}
		}
	}
	sort.Strings(months)
	var out []BiasSample
	for _, month := range months {
		n := firstIdx(month)
		if n < 500 {
			continue
		}
		m, err := predictor.Train(rows[:n], disabled)
		if err != nil {
			return nil, err
		}
		for i := n; i < len(rows) && rows[i].G.Date[:7] == month; i++ {
			r := &rows[i]
			if r.G.Season != season || r.G.Minutes < 1 {
				continue
			}
			pred := m.Predict(r)
			if !model.Eligible(r.AvgMin15, pred.Minutes) || math.IsNaN(pred.Minutes) {
				continue
			}
			s := BiasSample{Date: r.G.Date, EarlySeason: r.G.Day-start < 21, PredMin: pred.Minutes, ActualMin: r.G.Minutes}
			for _, st := range data.Stats {
				k := data.StatIndex(st)
				s.Pred[k], s.Actual[k] = pred.Mean(st), r.G.Value(st)
			}
			out = append(out, s)
		}
	}
	return out, nil
}
