package team

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/glm"
	"sort"
)

// BuildVerified uses explicit provider final scores instead of summing a
// potentially incomplete player download. Legacy production history is retained.
func BuildVerified(rows []data.PlayerGame) []Game {
	by := map[int]Game{}
	bad := map[int]bool{}
	for _, r := range rows {
		if !r.SourceFinal || r.GameID <= 0 || r.HomeTeamID <= 0 || r.VisitorTeamID <= 0 || r.HomeTeamID == r.VisitorTeamID || data.DayNumber(r.Date) == 0 {
			continue
		}
		if r.TeamID != r.HomeTeamID && r.TeamID != r.VisitorTeamID {
			continue
		}
		if math.IsNaN(r.FinalHomeScore) || math.IsNaN(r.FinalAwayScore) || math.IsInf(r.FinalHomeScore, 0) || math.IsInf(r.FinalAwayScore, 0) || r.FinalHomeScore < 50 || r.FinalAwayScore < 50 || r.FinalHomeScore == r.FinalAwayScore {
			continue
		}
		g := Game{ID: r.GameID, Day: data.DayNumber(r.Date), Date: r.Date, Season: r.Season, Home: r.HomeTeamID, Away: r.VisitorTeamID, HPts: r.FinalHomeScore, APts: r.FinalAwayScore}
		if old, ok := by[g.ID]; ok && old != g {
			bad[g.ID] = true
		}
		by[g.ID] = g
	}
	var out []Game
	for id, g := range by {
		if !bad[id] {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// FitAnchoredMix fits only a correction to the market, not a free market slope.
// The nonnegative [0,1] ratings weight cannot reverse the model's direction or
// amplify a disagreement. This is an experimental candidate, not an edge claim.
func FitAnchoredMix(rs []Rec) (*Mix, error) {
	X := make([][]float64, len(rs))
	y := make([]float64, len(rs))
	for i, r := range rs {
		X[i] = []float64{r.Proj - r.Market}
		y[i] = r.Actual - r.Market
	}
	fit, err := glm.FitLinear(X, y, []string{"disagreement"}, 20)
	if err != nil {
		return nil, err
	}
	weight := math.Max(0, math.Min(1, fit.Coef[0]/fit.Std[0]))
	bias := 0.
	for _, r := range rs {
		bias += r.Actual - r.Market - weight*(r.Proj-r.Market)
	}
	bias /= float64(len(rs))
	// Express the constrained fit in the existing two-input prediction contract.
	l := &glm.Linear{Names: []string{"market", "projection"}, Mean: []float64{0, 0}, Std: []float64{1, 1}, Intercept: bias, Coef: []float64{1 - weight, weight}}
	ss := 0.
	for _, r := range rs {
		d := l.Predict([]float64{r.Market, r.Proj}) - r.Actual
		ss += d * d
	}
	return &Mix{Lin: l, Sigma: math.Sqrt(ss / float64(len(rs))), N: len(rs)}, nil
}
