// Package comboaudit diagnoses frozen pregame combo forecasts; it never fits
// corrections or modifies picks, probabilities or records from their outcomes.
package comboaudit

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"sort"
	"strings"
)

type Row struct {
	WorkloadShift    *float64 `json:"workloadShift,omitempty"`
	Date             string   `json:"date"`
	Player           string   `json:"player"`
	Stat             string   `json:"stat"`
	Side             string   `json:"side"`
	Result           string   `json:"result"`
	Projection       float64  `json:"projection"`
	Actual           float64  `json:"actual"`
	ProjectedMinutes float64  `json:"projectedMinutes"`
	ActualMinutes    float64  `json:"actualMinutes"`
	MinutesError     float64  `json:"minutesError"`
	MinutesComponent float64  `json:"minutesComponent"`
	RateComponent    float64  `json:"rateComponent"`
	Estimate         float64  `json:"estimate"`
}
type Slice struct {
	Name             string  `json:"name"`
	N                int     `json:"n"`
	Dates            int     `json:"dates"`
	Wins             int     `json:"wins"`
	Losses           int     `json:"losses"`
	Predicted        float64 `json:"predicted"`
	HitRate          float64 `json:"hitRate"`
	Bias             float64 `json:"actualMinusProjection"`
	MinutesBias      float64 `json:"actualMinusProjectedMinutes"`
	MinutesComponent float64 `json:"minutesComponent"`
	RateComponent    float64 `json:"rateComponent"`
}
type Report struct {
	Note       string         `json:"note"`
	Duplicates int            `json:"duplicatesCollapsed"`
	Excluded   map[string]int `json:"excluded"`
	Rows       []Row          `json:"rows"`
	Slices     []Slice        `json:"slices"`
}

func Build(settled []picklog.Settled, finals []data.PlayerGame, date string) Report {
	return build(settled, finals, date, true)
}
func BuildSingles(settled []picklog.Settled, finals []data.PlayerGame, date string) Report {
	return build(settled, finals, date, false)
}
func build(settled []picklog.Settled, finals []data.PlayerGame, date string, combos bool) Report {
	out := Report{Note: "Frozen pregame estimates, verified settlements, matched final identities. Actual minus projection = minutes component + rate component. Arithmetic decomposition, not causal attribution. No corrections fitted.", Excluded: map[string]int{}}
	var eligible []picklog.Settled
	for _, s := range settled {
		if data.Stat(s.Stat).IsCombo() != combos || (date != "" && s.Date != date) {
			continue
		}
		if !s.Verified() || (s.Result != "WIN" && s.Result != "LOSS") || !strings.EqualFold(strings.TrimSpace(s.SourcePlayer), strings.TrimSpace(s.Player)) || picklog.Grade(s.Entry, s.Actual, 0, true).Result != s.Result {
			out.Excluded["unverified or not win/loss"]++
			continue
		}
		eligible = append(eligible, s)
	}
	eligible, out.Duplicates = picklog.DedupeSettled(eligible)
	byID := map[[2]int]data.PlayerGame{}
	for _, g := range finals {
		if g.SourceFinal && g.BoxScoreComplete {
			byID[[2]int{g.PlayerID, g.GameID}] = g
		}
	}
	for _, s := range eligible {
		g, ok := byID[[2]int{s.PlayerID, s.GameID}]
		if !ok || g.TeamID != s.SourceTeamID || g.Date != s.SourceDate || math.Abs(g.Value(data.Stat(s.Stat))-s.Actual) > 1e-6 {
			out.Excluded["missing or revised final identity/actual"]++
			continue
		}
		if !finite(s.ProjMin) || !finite(g.Minutes) || !finite(s.ProjMean) || !finite(s.Prob) || s.Prob < 0 || s.Prob > 1 || s.ProjMin <= 0 || s.ProjMin > 48 || g.Minutes <= 0 || g.Minutes > 70 || s.ProjMean <= 0 {
			out.Excluded["invalid workload or projection"]++
			continue
		}
		minutes := (g.Minutes - s.ProjMin) * s.ProjMean / s.ProjMin
		out.Rows = append(out.Rows, Row{Date: s.Date, Player: s.Player, Stat: s.Stat, Side: s.Side, Result: s.Result, Projection: s.ProjMean, Actual: s.Actual, ProjectedMinutes: s.ProjMin, ActualMinutes: g.Minutes, MinutesError: g.Minutes - s.ProjMin, MinutesComponent: minutes, RateComponent: s.Actual - s.ProjMean - minutes, Estimate: s.Prob})
		if s.Rotation != nil && finite(s.Rotation.BaselineMinutes) && s.Rotation.BaselineMinutes > 0 {
			shift := s.ProjMin - s.Rotation.BaselineMinutes
			out.Rows[len(out.Rows)-1].WorkloadShift = &shift
		}
	}
	groups := map[string][]Row{}
	for _, r := range out.Rows {
		for _, key := range []string{"all", r.Side, r.Stat, r.Stat + " " + r.Side} {
			groups[key] = append(groups[key], r)
		}
		band := "estimate below 60%"
		if r.Estimate >= .65 {
			band = "estimate 65%+"
		} else if r.Estimate >= .60 {
			band = "estimate 60-65%"
		}
		groups[band] = append(groups[band], r)
		if r.WorkloadShift != nil {
			key := "allocation within 6 min"
			if *r.WorkloadShift <= -6 {
				key = "allocation cut 6+ min"
			} else if *r.WorkloadShift >= 6 {
				key = "allocation rise 6+ min"
			}
			groups[key] = append(groups[key], r)
		}
	}
	keys := []string{}
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ss := Slice{Name: key, N: len(groups[key])}
		dates := map[string]bool{}
		for _, r := range groups[key] {
			dates[r.Date] = true
			if r.Result == "WIN" {
				ss.Wins++
			} else {
				ss.Losses++
			}
			ss.Predicted += r.Estimate
			ss.Bias += r.Actual - r.Projection
			ss.MinutesBias += r.MinutesError
			ss.MinutesComponent += r.MinutesComponent
			ss.RateComponent += r.RateComponent
		}
		n := float64(ss.N)
		ss.Dates = len(dates)
		ss.HitRate = float64(ss.Wins) / n
		ss.Predicted /= n
		ss.Bias /= n
		ss.MinutesBias /= n
		ss.MinutesComponent /= n
		ss.RateComponent /= n
		out.Slices = append(out.Slices, ss)
	}
	return out
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
