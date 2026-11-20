// Package comboaudit diagnoses frozen pregame single-stat and combo forecasts; it never fits
// corrections or modifies picks, probabilities or records from their outcomes.
package comboaudit

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/picklog"
	"sort"
	"strings"
)

type Row struct {
	LoggedAt         string             `json:"loggedAt"`
	BaselineMinutes  *float64           `json:"baselineMinutes,omitempty"`
	PregameSpread    *float64           `json:"pregameSpread,omitempty"`
	FinalMargin      *float64           `json:"finalMargin,omitempty"`
	OwnAbsences      []string           `json:"ownAbsences,omitempty"`
	OpponentAbsences []string           `json:"opponentAbsences,omitempty"`
	PlayerID         int                `json:"playerId"`
	GameID           int                `json:"gameId"`
	Line             float64            `json:"line"`
	Opportunity      *OpportunityReview `json:"opportunity,omitempty"`
	WorkloadShift    *float64           `json:"workloadShift,omitempty"`
	Date             string             `json:"date"`
	Player           string             `json:"player"`
	Stat             string             `json:"stat"`
	Side             string             `json:"side"`
	Result           string             `json:"result"`
	Projection       float64            `json:"projection"`
	Actual           float64            `json:"actual"`
	ProjectedMinutes float64            `json:"projectedMinutes"`
	ActualMinutes    float64            `json:"actualMinutes"`
	MinutesError     float64            `json:"minutesError"`
	MinutesComponent float64            `json:"minutesComponent"`
	RateComponent    float64            `json:"rateComponent"`
	Estimate         float64            `json:"estimate"`
}
type Slice struct {
	OpportunityN                   int     `json:"opportunityN"`
	OpportunityVolumeComponent     float64 `json:"opportunityVolumeComponent"`
	OpportunityConversionComponent float64 `json:"opportunityConversionComponent"`
	Name                           string  `json:"name"`
	N                              int     `json:"n"`
	Dates                          int     `json:"dates"`
	PlayerGames                    int     `json:"playerGames"`
	Wins                           int     `json:"wins"`
	Losses                         int     `json:"losses"`
	Predicted                      float64 `json:"predicted"`
	HitRate                        float64 `json:"hitRate"`
	Bias                           float64 `json:"actualMinusProjection"`
	MinutesBias                    float64 `json:"actualMinusProjectedMinutes"`
	MinutesComponent               float64 `json:"minutesComponent"`
	RateComponent                  float64 `json:"rateComponent"`
}
type Report struct {
	Workloads  []WorkloadComparison `json:"workloadComparisons,omitempty"`
	Note       string               `json:"note"`
	Duplicates int                  `json:"duplicatesCollapsed"`
	Excluded   map[string]int       `json:"excluded"`
	Rows       []Row                `json:"rows"`
	Slices     []Slice              `json:"slices"`
}

func Build(settled []picklog.Settled, finals []data.PlayerGame, date string) Report {
	return build(settled, finals, date, true)
}
func BuildSingles(settled []picklog.Settled, finals []data.PlayerGame, date string) Report {
	return build(settled, finals, date, false)
}

// BuildSinglesWithOpportunities compares frozen pregame opportunity measurements
// with final same-game opportunities. Missing tracking remains explicitly absent.
func BuildSinglesWithOpportunities(settled []picklog.Settled, finals []data.PlayerGame, date string, ix *opportunity.Index) Report {
	return build(settled, finals, date, false, ix)
}
func build(settled []picklog.Settled, finals []data.PlayerGame, date string, combos bool, indexes ...*opportunity.Index) Report {
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
		out.Rows = append(out.Rows, Row{LoggedAt: s.LoggedAt, PlayerID: s.PlayerID, GameID: s.GameID, Line: s.Line, Date: s.Date, Player: s.Player, Stat: s.Stat, Side: s.Side, Result: s.Result, Projection: s.ProjMean, Actual: s.Actual, ProjectedMinutes: s.ProjMin, ActualMinutes: g.Minutes, MinutesError: g.Minutes - s.ProjMin, MinutesComponent: minutes, RateComponent: s.Actual - s.ProjMean - minutes, Estimate: s.Prob})
		if !combos && len(indexes) > 0 {
			out.Rows[len(out.Rows)-1].Opportunity = reviewOpportunity(s, g, indexes[0])
		}
		if s.Rotation != nil && finite(s.Rotation.BaselineMinutes) && s.Rotation.BaselineMinutes > 0 {
			shift := s.ProjMin - s.Rotation.BaselineMinutes
			out.Rows[len(out.Rows)-1].WorkloadShift = &shift
			baseline := s.Rotation.BaselineMinutes
			out.Rows[len(out.Rows)-1].BaselineMinutes = &baseline
		}
		r := &out.Rows[len(out.Rows)-1]
		if g.FinalHomeScore+g.FinalAwayScore > 0 {
			margin := math.Abs(g.FinalHomeScore - g.FinalAwayScore)
			r.FinalMargin = &margin
		}
		if s.Rotation != nil {
			for _, absence := range s.Rotation.ConfirmedOut {
				r.OwnAbsences = append(r.OwnAbsences, absence.Player)
			}
			for _, impact := range s.Rotation.Production {
				if impact.Kind == "opponent-allowance" {
					for _, p := range impact.Missing {
						r.OpponentAbsences = append(r.OpponentAbsences, p.Player)
					}
				}
			}
			if c := s.Rotation.GameContext; c != nil && finite(c.Spread) && c.Before != "" && c.Before <= s.Date && c.N > 0 {
				spread := math.Abs(c.Spread)
				r.PregameSpread = &spread
			}
		}
	}
	groups := map[string][]Row{}
	for _, r := range out.Rows {
		for _, key := range []string{"all", r.Side, r.Stat, r.Stat + " " + r.Side, r.Result, r.Side + " " + r.Result, r.Stat + " " + r.Result} {
			groups[key] = append(groups[key], r)
		}
		band := "estimate below 60%"
		if r.Estimate >= .65 {
			band = "estimate 65%+"
		} else if r.Estimate >= .60 {
			band = "estimate 60-65%"
		}
		groups[band] = append(groups[band], r)
		if len(r.OwnAbsences) > 0 {
			groups["confirmed teammate outs"] = append(groups["confirmed teammate outs"], r)
		}
		if len(r.OpponentAbsences) > 0 {
			groups["confirmed opponent outs"] = append(groups["confirmed opponent outs"], r)
		}
		if r.PregameSpread != nil {
			key := "pregame spread 3.5-10"
			if *r.PregameSpread <= 3.5 {
				key = "pregame spread <=3.5"
			} else if *r.PregameSpread >= 10 {
				key = "pregame spread >=10"
			}
			groups[key] = append(groups[key], r)
		}
		if r.FinalMargin != nil && *r.FinalMargin >= 15 {
			groups["final margin >=15 (postgame)"] = append(groups["final margin >=15 (postgame)"], r)
		}
		if r.Opportunity != nil {
			coverage := "opportunities unavailable"
			if r.Opportunity.Available {
				coverage = "opportunities covered"
			}
			groups[coverage] = append(groups[coverage], r)
		}
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
		playerGames := map[[2]int]bool{}
		for _, r := range groups[key] {
			dates[r.Date] = true
			playerGames[[2]int{r.PlayerID, r.GameID}] = true
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
			if r.Opportunity != nil && r.Opportunity.Available {
				ss.OpportunityN++
				ss.OpportunityVolumeComponent += r.Opportunity.VolumeComponent
				ss.OpportunityConversionComponent += r.Opportunity.ConversionComponent
			}
		}
		n := float64(ss.N)
		ss.Dates = len(dates)
		ss.PlayerGames = len(playerGames)
		ss.HitRate = float64(ss.Wins) / n
		ss.Predicted /= n
		ss.Bias /= n
		ss.MinutesBias /= n
		ss.MinutesComponent /= n
		ss.RateComponent /= n
		if ss.OpportunityN > 0 {
			ss.OpportunityVolumeComponent /= float64(ss.OpportunityN)
			ss.OpportunityConversionComponent /= float64(ss.OpportunityN)
		}
		out.Slices = append(out.Slices, ss)
	}
	out.Workloads = compareWorkloads(out.Rows)
	return out
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
