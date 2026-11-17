// Package team models whole games (point spread, moneyline, game total) from team scoring
// history. Game markets are among the sharpest in sports betting, so this model is judged
// against the sportsbook's own line before anything is posted.
package team

import (
	"math"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/glm"
)

// Game is one finished game with final scores (summed from the player box scores).
type Game struct {
	ID         int
	Day        int
	Date       string
	Season     int
	Home, Away int
	HPts, APts float64
}

func (g Game) Margin() float64 { return g.HPts - g.APts }
func (g Game) Total() float64  { return g.HPts + g.APts }

// Build turns player box scores into team games, in date order.
func Build(rows []data.PlayerGame) []Game {
	type acc struct {
		g    Game
		pts  map[int]float64
		seen bool
	}
	by := map[int]*acc{}
	for _, r := range rows {
		if r.HomeTeamID == 0 || r.VisitorTeamID == 0 {
			continue
		}
		a := by[r.GameID]
		if a == nil {
			a = &acc{pts: map[int]float64{}, g: Game{ID: r.GameID, Day: r.Day, Date: r.Date, Season: r.Season, Home: r.HomeTeamID, Away: r.VisitorTeamID}}
			by[r.GameID] = a
		}
		a.pts[r.TeamID] += r.Pts
	}
	out := make([]Game, 0, len(by))
	for _, a := range by {
		a.g.HPts, a.g.APts = a.pts[a.g.Home], a.pts[a.g.Away]
		if a.g.HPts < 50 || a.g.APts < 50 { // incomplete box score
			continue
		}
		out = append(out, a.g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// FeatureNames are the pre-game inputs, all relative to the league average at the time.
var FeatureNames = []string{
	"h_off10", "h_def10", "a_off10", "a_def10",
	"h_off30", "h_def30", "a_off30", "a_def30",
	"h_b2b", "a_b2b", "h_rest", "a_rest",
}

type tstate struct {
	off10, def10, off30, def30 float64 // points scored / allowed, EWMA
	last                       int
	n                          int
	init                       bool
}

// State tracks every team's running ratings. Feed games in date order: read Vec for a game
// BEFORE calling Update with it, so nothing about the game leaks into its own features.
type State struct {
	t      map[int]*tstate
	league float64 // running mean points per team per game
	nLg    int
	// MinGames is how many games (after an offseason reset, last year counts as 4) both teams
	// need before a game is projected. Training always uses DefaultMinGames; prediction may lower it.
	MinGames int
}

// DefaultMinGames is the readiness bar used when fitting the model.
const DefaultMinGames = 8

func NewState() *State {
	return &State{t: map[int]*tstate{}, league: 113, MinGames: DefaultMinGames}
}

const (
	a10 = 2.0 / 11
	a30 = 2.0 / 31
)

func (s *State) get(id int) *tstate {
	ts := s.t[id]
	if ts == nil {
		ts = &tstate{}
		s.t[id] = ts
	}
	return ts
}

// Ready reports whether both teams have enough games for a trustworthy projection.
func (s *State) Ready(home, away int) bool {
	return s.get(home).n >= s.MinGames && s.get(away).n >= s.MinGames
}

// Games returns how many games (counting carried-over history) a team has behind its ratings.
func (s *State) Games(id int) int { return s.get(id).n }

// Vec returns the feature vector for a game between home and away on day.
func (s *State) Vec(home, away, day int) []float64 {
	h, a := s.get(home), s.get(away)
	L := s.league
	rest := func(t *tstate) (b2b, r float64) {
		if t.last == 0 {
			return 0, 3
		}
		d := float64(day - t.last)
		if d > 7 {
			d = 7
		}
		if d <= 1 {
			b2b = 1
		}
		return b2b, d
	}
	hb, hr := rest(h)
	ab, ar := rest(a)
	return []float64{
		h.off10 - L, h.def10 - L, a.off10 - L, a.def10 - L,
		h.off30 - L, h.def30 - L, a.off30 - L, a.def30 - L,
		hb, ab, hr, ar,
	}
}

func (s *State) bump(t *tstate, scored, allowed float64, day int) {
	// A long gap means an offseason: pull ratings most of the way back toward average,
	// since rosters change.
	if t.init && day-t.last > 60 {
		L := s.league
		for _, p := range []*float64{&t.off10, &t.def10, &t.off30, &t.def30} {
			*p = L + 0.6*(*p-L)
		}
		t.n = 4
	}
	if !t.init {
		t.off10, t.off30, t.def10, t.def30 = scored, scored, allowed, allowed
		t.init = true
	} else {
		t.off10 += a10 * (scored - t.off10)
		t.def10 += a10 * (allowed - t.def10)
		t.off30 += a30 * (scored - t.off30)
		t.def30 += a30 * (allowed - t.def30)
	}
	t.last = day
	t.n++
}

// Update folds a finished game into the ratings.
func (s *State) Update(g Game) {
	s.bump(s.get(g.Home), g.HPts, g.APts, g.Day)
	s.bump(s.get(g.Away), g.APts, g.HPts, g.Day)
	s.nLg += 2
	w := 1.0 / math.Min(float64(s.nLg), 1500)
	s.league += w * ((g.HPts+g.APts)/2 - s.league)
}

// Sample is one historical game with its pre-game features.
type Sample struct {
	G     Game
	X     []float64
	Ready bool
}

// Samples replays history and returns each game's features computed from earlier games only,
// plus the final state (for projecting upcoming games).
func Samples(games []Game) ([]Sample, *State) {
	st := NewState()
	out := make([]Sample, 0, len(games))
	i := 0
	for i < len(games) {
		j := i
		for j < len(games) && games[j].Day == games[i].Day {
			j++
		}
		// Every game of a day is featured before any of them is absorbed (teams play once a day).
		for k := i; k < j; k++ {
			g := games[k]
			out = append(out, Sample{G: g, X: st.Vec(g.Home, g.Away, g.Day), Ready: st.Ready(g.Home, g.Away)})
		}
		for k := i; k < j; k++ {
			st.Update(games[k])
		}
		i = j
	}
	return out, st
}

// Model projects margin (home minus away) and total points from the ratings.
type Model struct {
	Margin, Total  *glm.Linear
	SigmaM, SigmaT float64
}

// Fit trains on every ready sample.
func Fit(samples []Sample) (*Model, error) {
	var X [][]float64
	var ym, yt []float64
	for _, s := range samples {
		if !s.Ready {
			continue
		}
		X = append(X, s.X)
		ym = append(ym, s.G.Margin())
		yt = append(yt, s.G.Total())
	}
	mm, err := glm.FitLinear(X, ym, FeatureNames, 20)
	if err != nil {
		return nil, err
	}
	mt, err := glm.FitLinear(X, yt, FeatureNames, 20)
	if err != nil {
		return nil, err
	}
	m := &Model{Margin: mm, Total: mt}
	var sm, st float64
	for i := range X {
		d := mm.Predict(X[i]) - ym[i]
		sm += d * d
		e := mt.Predict(X[i]) - yt[i]
		st += e * e
	}
	m.SigmaM = math.Sqrt(sm / float64(len(X)))
	m.SigmaT = math.Sqrt(st / float64(len(X)))
	return m, nil
}

// Project returns the model's margin and total for a feature vector.
func (m *Model) Project(x []float64) (margin, total float64) {
	return m.Margin.Predict(x), m.Total.Predict(x)
}

// NormCDF is the standard normal cumulative distribution.
func NormCDF(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }
