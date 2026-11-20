// Package gamepace is a separate possessions/efficiency game challenger.
// Every historical feature uses earlier dates; present-day injury labels never
// reconstruct historical availability.
package gamepace

import (
	"fmt"
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/glm"
	"sort"
)

const Policy = "game-pace-v1"

type Game struct {
	ID, Home, Away                               int
	Date                                         string
	Pace, HomeEff, AwayEff, HomeScore, AwayScore float64
	Players                                      map[int]data.PlayerGame
}
type teamSum struct {
	min, pts, fga, fta, oreb, tov float64
	bad                           bool
	players                       map[int]data.PlayerGame
}

// Estimated full-game possessions average both teams' FGA+.44*FTA-OREB+TOV.
// Pace is normalized to 48 minutes; efficiency retains overtime possessions.
// Team minutes and summed points must agree with a complete official final.
func Build(rows []data.PlayerGame) ([]Game, map[string]int) {
	type acc struct {
		g     Game
		teams map[int]*teamSum
		bad   bool
	}
	all := map[int]*acc{}
	seen := map[[2]int]data.PlayerGame{}
	excluded := map[string]int{}
	for _, r := range rows {
		if !r.SourceFinal || !r.BoxScoreComplete || r.GameID <= 0 {
			continue
		}
		key := [2]int{r.GameID, r.PlayerID}
		if old, ok := seen[key]; ok {
			if old != r {
				if a := all[r.GameID]; a != nil {
					a.bad = true
				}
			}
			continue
		}
		seen[key] = r
		a := all[r.GameID]
		if a == nil {
			a = &acc{g: Game{ID: r.GameID, Home: r.HomeTeamID, Away: r.VisitorTeamID, Date: r.Date, HomeScore: r.FinalHomeScore, AwayScore: r.FinalAwayScore, Players: map[int]data.PlayerGame{}}, teams: map[int]*teamSum{}}
			all[r.GameID] = a
		}
		if a.g.Home != r.HomeTeamID || a.g.Away != r.VisitorTeamID || a.g.Date != r.Date || a.g.HomeScore != r.FinalHomeScore || a.g.AwayScore != r.FinalAwayScore {
			a.bad = true
		}
		t := a.teams[r.TeamID]
		if t == nil {
			t = &teamSum{players: map[int]data.PlayerGame{}}
			a.teams[r.TeamID] = t
		}
		if r.Minutes <= 0 {
			continue
		}
		if !r.PossessionComplete || !finite(r.Minutes) || r.Minutes > 70 || r.Fga < 0 || r.Fta < 0 || r.Oreb < 0 || r.Turnover < 0 {
			t.bad = true
		}
		t.min += r.Minutes
		t.pts += r.Pts
		t.fga += r.Fga
		t.fta += r.Fta
		t.oreb += r.Oreb
		t.tov += r.Turnover
		t.players[r.PlayerID] = r
		a.g.Players[r.PlayerID] = r
	}
	out := []Game{}
	for _, a := range all {
		h, v := a.teams[a.g.Home], a.teams[a.g.Away]
		if a.bad || h == nil || v == nil || h.bad || v.bad || len(h.players) < 5 || len(v.players) < 5 || math.Abs(h.pts-a.g.HomeScore) > .01 || math.Abs(v.pts-a.g.AwayScore) > .01 {
			excluded["incomplete or conflicting box score"]++
			continue
		}
		duration := (h.min + v.min) / 10
		ot := math.Round((duration - 48) / 5)
		if ot < 0 || ot > 6 || math.Abs(duration-(48+5*ot)) > .3 || math.Abs(h.min-v.min) > 1 {
			excluded["team workload incomplete"]++
			continue
		}
		poss := ((h.fga + .44*h.fta - h.oreb + h.tov) + (v.fga + .44*v.fta - v.oreb + v.tov)) / 2
		duration = 48 + 5*ot // clock length, not sub-second rounding in summed player minutes
		pace := poss * 48 / duration
		if pace < 70 || pace > 130 || !finite(pace) {
			excluded["invalid estimated possessions"]++
			continue
		}
		a.g.Pace = pace
		a.g.HomeEff = a.g.HomeScore / poss
		a.g.AwayEff = a.g.AwayScore / poss
		out = append(out, a.g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].ID < out[j].ID
	})
	return out, excluded
}

type appearance struct {
	Date    string
	Minutes float64
	Values  [6]float64
}
type profile struct {
	pace, off, def float64
	n              int
	last           string
	games          []Game
}
type State struct {
	teams                 map[int]*profile
	players               map[int][]appearance
	leaguePace, leagueEff float64
	n                     int
}

func NewState() *State {
	return &State{teams: map[int]*profile{}, players: map[int][]appearance{}, leaguePace: 100, leagueEff: 1.12}
}
func (s *State) team(id int, date string) *profile {
	p := s.teams[id]
	if p == nil || data.DayNumber(date)-data.DayNumber(p.last) > 90 {
		return &profile{pace: s.leaguePace, off: s.leagueEff, def: s.leagueEff}
	}
	return p
}
func values(r data.PlayerGame) [6]float64 {
	return [6]float64{r.Pts, r.Reb, r.Ast, r.Fg3m, r.Turnover, r.Blk}
}

// Baseline workload averages the earlier team's last ten games, counting an
// absent player as zero rather than treating his last appearance as every game.
func (s *State) baseline(team int, date string) map[int]float64 {
	p := s.team(team, date)
	m := map[int]float64{}
	n := 0
	for i := len(p.games) - 1; i >= 0 && n < 10; i-- {
		g := p.games[i]
		if g.Date >= date {
			continue
		}
		n++
		for pid, r := range g.Players {
			if r.TeamID == team {
				m[pid] += r.Minutes
			}
		}
	}
	if n > 0 {
		for pid, v := range m {
			m[pid] = v / float64(n)
		}
	}
	return m
}
func (s *State) composition(minutes map[int]float64, date string) ([6]float64, float64) {
	var result [6]float64
	total, known := 0., 0.
	for _, m := range minutes {
		total += m
	}
	if total <= 0 {
		return result, 0
	}
	ids := []int{}
	for pid := range minutes {
		ids = append(ids, pid)
	}
	sort.Ints(ids)
	for _, pid := range ids {
		m := minutes[pid] * 240 / total
		hs := s.players[pid]
		var sums [6]float64
		mins := 0.
		n := 0
		for i := len(hs) - 1; i >= 0 && n < 20; i-- {
			r := hs[i]
			if r.Date >= date || data.DayNumber(date)-data.DayNumber(r.Date) > 180 {
				continue
			}
			n++
			mins += r.Minutes
			for k, v := range r.Values {
				sums[k] += v
			}
		}
		if n < 3 || mins <= 0 {
			continue
		}
		known += m
		for k := range result {
			result[k] += m * sums[k] / mins
		}
	}
	return result, known / 240
}

type Input struct {
	X                          []float64
	HomeCoverage, AwayCoverage float64
	HomeGames, AwayGames       int
}

var Names = []string{"home_pace", "away_pace", "home_off", "home_def", "away_off", "away_def", "home_rest", "away_rest", "home_b2b", "away_b2b", "home_scoring", "home_rebounding", "home_creation", "home_threes", "home_turnovers", "home_blocks", "away_scoring", "away_rebounding", "away_creation", "away_threes", "away_turnovers", "away_blocks"}

func (s *State) Features(home, away int, date string, homeMinutes, awayMinutes map[int]float64) Input {
	h, a := s.team(home, date), s.team(away, date)
	if homeMinutes == nil {
		homeMinutes = s.baseline(home, date)
	}
	if awayMinutes == nil {
		awayMinutes = s.baseline(away, date)
	}
	hc, hcov := s.composition(homeMinutes, date)
	ac, acov := s.composition(awayMinutes, date)
	rest := func(p *profile) (float64, float64) {
		r := float64(data.DayNumber(date) - data.DayNumber(p.last))
		if p.last == "" || r > 7 {
			r = 7
		}
		b := 0.
		if r == 1 {
			b = 1
		}
		return r, b
	}
	hr, hb := rest(h)
	ar, ab := rest(a)
	x := []float64{h.pace, a.pace, h.off, h.def, a.off, a.def, hr, ar, hb, ab}
	x = append(x, hc[:]...)
	x = append(x, ac[:]...)
	return Input{X: x, HomeCoverage: hcov, AwayCoverage: acov, HomeGames: h.n, AwayGames: a.n}
}
func (s *State) update(g Game) {
	for _, id := range []int{g.Home, g.Away} {
		p := s.team(id, g.Date)
		own, opp := g.HomeEff, g.AwayEff
		if id == g.Away {
			own, opp = opp, own
		}
		// Priors shrink thin team histories toward the earlier league baseline.
		w := .12
		if p.n < 8 {
			w = 1 / float64(8+p.n)
		}
		p.pace += (g.Pace - p.pace) * w
		p.off += (own - p.off) * w
		p.def += (opp - p.def) * w
		p.n++
		p.last = g.Date
		p.games = append(p.games, g)
		if len(p.games) > 10 {
			p.games = p.games[len(p.games)-10:]
		}
		s.teams[id] = p
	}
	ids := []int{}
	for pid := range g.Players {
		ids = append(ids, pid)
	}
	sort.Ints(ids)
	for _, pid := range ids {
		r := g.Players[pid]
		hs := append(s.players[pid], appearance{g.Date, r.Minutes, values(r)})
		if len(hs) > 20 {
			hs = hs[len(hs)-20:]
		}
		s.players[pid] = hs
	}
	s.n++
	s.leaguePace += (g.Pace - s.leaguePace) * .002
	s.leagueEff += ((g.HomeEff+g.AwayEff)/2 - s.leagueEff) * .002
}

type Sample struct {
	Game  Game
	Input Input
}

func Samples(games []Game, before string) ([]Sample, *State) {
	s := NewState()
	out := []Sample{}
	for i := 0; i < len(games); {
		j := i + 1
		for j < len(games) && games[j].Date == games[i].Date {
			j++
		}
		if games[i].Date >= before {
			break
		}
		for _, g := range games[i:j] {
			in := s.Features(g.Home, g.Away, g.Date, nil, nil)
			if in.HomeGames >= 6 && in.AwayGames >= 6 && in.HomeCoverage >= .8 && in.AwayCoverage >= .8 {
				out = append(out, Sample{g, in})
			}
		}
		for _, g := range games[i:j] {
			s.update(g)
		}
		i = j
	}
	return out, s
}

type Model struct {
	OvertimePointsPerTeam   float64
	Before                  string
	N                       int
	Pace, HomeEff, AwayEff  *glm.Linear
	MarginSigma, TotalSigma float64
}
type Forecast struct{ Pace, HomeEff, AwayEff, HomeScore, AwayScore, Margin, Total float64 }

func Fit(samples []Sample, before string) (*Model, error) {
	x := [][]float64{}
	yp, yh, ya := []float64{}, []float64{}, []float64{}
	actualM, actualT := []float64{}, []float64{}
	extra := 0.
	for _, s := range samples {
		if s.Game.Date >= before {
			continue
		}
		x = append(x, s.Input.X)
		yp = append(yp, s.Game.Pace)
		yh = append(yh, s.Game.HomeEff)
		ya = append(ya, s.Game.AwayEff)
		actualM = append(actualM, s.Game.HomeScore-s.Game.AwayScore)
		actualT = append(actualT, s.Game.HomeScore+s.Game.AwayScore)
		extra += math.Max(0, s.Game.HomeScore+s.Game.AwayScore-s.Game.Pace*(s.Game.HomeEff+s.Game.AwayEff)) / 2
	}
	if len(x) < 300 {
		return nil, fmt.Errorf("need 300 earlier possession-complete games, have %d", len(x))
	}
	m := &Model{Before: before, N: len(x)}
	m.OvertimePointsPerTeam = extra / float64(len(x))
	var err error
	if m.Pace, err = glm.FitLinear(x, yp, Names, 50); err != nil {
		return nil, err
	}
	if m.HomeEff, err = glm.FitLinear(x, yh, Names, 50); err != nil {
		return nil, err
	}
	if m.AwayEff, err = glm.FitLinear(x, ya, Names, 50); err != nil {
		return nil, err
	}
	// Residual scales below are only initial estimates. Live uses monthly OOF
	// residual scales when available, never claims training residuals validate skill.
	for i, v := range x {
		f := m.Predict(v)
		margin, total := actualM[i], actualT[i]
		m.MarginSigma += math.Pow(margin-f.Margin, 2)
		m.TotalSigma += math.Pow(total-f.Total, 2)
	}
	m.MarginSigma = math.Max(8, math.Sqrt(m.MarginSigma/float64(len(x))))
	m.TotalSigma = math.Max(12, math.Sqrt(m.TotalSigma/float64(len(x))))
	return m, nil
}
func (m *Model) Predict(x []float64) Forecast {
	f := Forecast{Pace: clamp(m.Pace.Predict(x), 80, 120), HomeEff: clamp(m.HomeEff.Predict(x), .8, 1.5), AwayEff: clamp(m.AwayEff.Predict(x), .8, 1.5)}
	f.HomeScore = f.Pace*f.HomeEff + m.OvertimePointsPerTeam
	f.AwayScore = f.Pace*f.AwayEff + m.OvertimePointsPerTeam
	f.Margin = f.HomeScore - f.AwayScore
	f.Total = f.HomeScore + f.AwayScore
	return f
}
func finite(v float64) bool         { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
