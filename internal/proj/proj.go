// Package proj is the baseline projection model. For a player and a date it uses only games
// played BEFORE that date to estimate minutes, per-minute production and spread for points,
// rebounds and assists.
//
// This is a deliberately simple first version: recency-weighted minutes, recency-weighted
// per-minute rates blended with the player's longer history, and the player's own recent
// variance. Context features (opponent, pace, usage, teammates out) are the next layer.
package proj

import (
	"math"
	"sort"

	"propsmodel/internal/data"
)

const (
	minGames         = 8    // need at least this many prior games to project a player
	minutesWindow    = 10   // games used for the minutes projection
	minutesHalfLife  = 4.0  // games; recent games count more
	avgMinutesWindow = 15   // games used for the "averages 20+ minutes" check
	rateWindow       = 30   // games in the recent per-minute rate
	rateHalfLife     = 12.0 // games
	longWindow       = 120  // games in the long-run per-minute rate
	recentWeight     = 0.65 // blend: recent rate vs long-run rate
	shrinkMinutes    = 40.0 // pseudo-minutes of league-average production added to every rate
	varWindow        = 30   // games used for the spread
	varInflate       = 1.10 // widen the spread slightly for estimation error
)

// Projection is the model's view of one player for one date.
type Projection struct {
	PlayerID    int
	Games       int
	AvgMinutes  float64
	ProjMinutes float64
	Mean        map[data.Stat]float64
	Var         map[data.Stat]float64
}

type gameKey struct{ player, game int }

// Index holds all history for fast lookups.
type Index struct {
	byPlayer map[int][]data.PlayerGame // each slice sorted oldest to newest
	names    map[int]string
	actual   map[gameKey]data.PlayerGame
	gameDate map[int]string
	league   map[int]map[data.Stat]float64
	all      []data.PlayerGame
}

// NewIndex builds an Index from games sorted by date (data.LoadStatsDir does this).
func NewIndex(games []data.PlayerGame) *Index {
	ix := &Index{
		byPlayer: map[int][]data.PlayerGame{},
		names:    map[int]string{},
		actual:   map[gameKey]data.PlayerGame{},
		gameDate: map[int]string{},
		league:   map[int]map[data.Stat]float64{},
		all:      games,
	}
	for _, g := range games {
		ix.byPlayer[g.PlayerID] = append(ix.byPlayer[g.PlayerID], g)
		ix.names[g.PlayerID] = g.Name
		ix.actual[gameKey{g.PlayerID, g.GameID}] = g
		ix.gameDate[g.GameID] = g.Date
	}
	return ix
}

// Name returns a player's name, or "" if unknown.
func (ix *Index) Name(playerID int) string { return ix.names[playerID] }

// History returns a player's games, oldest to newest. Do not modify the slice.
func (ix *Index) History(playerID int) []data.PlayerGame { return ix.byPlayer[playerID] }

// Actual returns what a player really did in a game, if they played.
func (ix *Index) Actual(gameID, playerID int) (data.PlayerGame, bool) {
	g, ok := ix.actual[gameKey{playerID, gameID}]
	return g, ok
}

// GameDate returns the date of a game id, if known.
func (ix *Index) GameDate(gameID int) (string, bool) {
	d, ok := ix.gameDate[gameID]
	return d, ok
}

// typicalLeagueRate is the NBA-wide production per player-minute (about 114 points, 43 rebounds
// 26 assists and 10 threes per 240 player-minutes). It is used only for the earliest season in the data,
// where no earlier season exists to measure the league average from. A fixed constant keeps
// that season free of look-ahead.
var typicalLeagueRate = map[data.Stat]float64{data.Points: 0.475, data.Rebounds: 0.18, data.Assists: 0.108, data.Threes: 0.043}

// leagueRate returns league-wide per-minute rates using only seasons before asOfSeason, so a
// backtest never peeks at the future. With no earlier season it returns typicalLeagueRate.
func (ix *Index) leagueRate(asOfSeason int) map[data.Stat]float64 {
	if r, ok := ix.league[asOfSeason]; ok {
		return r
	}
	sums := map[data.Stat]float64{}
	var mins float64
	for _, g := range ix.all {
		if g.Season >= asOfSeason {
			continue
		}
		mins += g.Minutes
		for _, s := range data.Stats {
			sums[s] += g.Value(s)
		}
	}
	r := map[data.Stat]float64{}
	for _, s := range data.Stats {
		if mins > 0 {
			r[s] = sums[s] / mins
		} else {
			r[s] = typicalLeagueRate[s]
		}
	}
	ix.league[asOfSeason] = r
	return r
}

func tail(h []data.PlayerGame, n int) []data.PlayerGame {
	if len(h) <= n {
		return h
	}
	return h[len(h)-n:]
}

// ewmaMinutes is a recency-weighted average of minutes (newest game has weight 1). It also
// returns the variance of that estimate: how far the projected minutes could be off just
// from having a limited number of games to learn from.
func ewmaMinutes(h []data.PlayerGame, halfLife float64) (mean, varOfMean float64) {
	var num, sumW, sumW2 float64
	for i, g := range h {
		age := float64(len(h) - 1 - i)
		w := math.Pow(0.5, age/halfLife)
		num += w * g.Minutes
		sumW += w
		sumW2 += w * w
	}
	if sumW == 0 {
		return 0, 0
	}
	mean = num / sumW
	var vm float64
	for _, g := range h {
		d := g.Minutes - mean
		vm += d * d
	}
	vm /= float64(len(h))
	return mean, vm * sumW2 / (sumW * sumW)
}

// rate returns the blended per-minute rate for a stat and the variance of that estimate
// (how uncertain the rate is, given the minutes of evidence behind it).
func (ix *Index) rate(hist []data.PlayerGame, s data.Stat, league float64) (rate, rateVar float64) {
	rw := tail(hist, rateWindow)
	var num, den, minSum, w2min float64
	for i, g := range rw {
		age := float64(len(rw) - 1 - i)
		w := math.Pow(0.5, age/rateHalfLife)
		num += w * g.Value(s)
		den += w * g.Minutes
		minSum += g.Minutes
		w2min += w * w * g.Minutes
	}
	recent, recentVar := league, 0.0
	if den > 0 {
		recent = num / den
		recentVar = recent * w2min / (den * den) // Poisson-style: var of a weighted rate
	}

	var ls, lm float64
	for _, g := range tail(hist, longWindow) {
		ls += g.Value(s)
		lm += g.Minutes
	}
	long, longVar := recent, recentVar
	if lm > 0 {
		long = ls / lm
		longVar = long / lm
	}

	r := recentWeight*recent + (1-recentWeight)*long
	// The recent and long windows overlap, so treat their errors as fully correlated.
	sd := recentWeight*math.Sqrt(recentVar) + (1-recentWeight)*math.Sqrt(longVar)

	keep := minSum / (minSum + shrinkMinutes)
	return r*keep + league*(1-keep), sd * sd * keep * keep
}

func spread(hist []data.PlayerGame, s data.Stat) (mean, variance float64) {
	vw := tail(hist, varWindow)
	if len(vw) < 2 {
		return 0, 0
	}
	for _, g := range vw {
		mean += g.Value(s)
	}
	mean /= float64(len(vw))
	for _, g := range vw {
		d := g.Value(s) - mean
		variance += d * d
	}
	variance /= float64(len(vw) - 1)
	return mean, variance
}

// Project estimates a player as of a date using only games strictly before that date.
func (ix *Index) Project(playerID int, asOf string) (*Projection, bool) {
	games := ix.byPlayer[playerID]
	end := sort.Search(len(games), func(i int) bool { return games[i].Date >= asOf })
	hist := games[:end]
	if len(hist) < minGames {
		return nil, false
	}

	projMin, projMinVar := ewmaMinutes(tail(hist, minutesWindow), minutesHalfLife)
	var avgMin float64
	aw := tail(hist, avgMinutesWindow)
	for _, g := range aw {
		avgMin += g.Minutes
	}
	avgMin /= float64(len(aw))

	league := ix.leagueRate(data.SeasonOf(asOf))
	p := &Projection{
		PlayerID:    playerID,
		Games:       len(hist),
		AvgMinutes:  avgMin,
		ProjMinutes: projMin,
		Mean:        map[data.Stat]float64{},
		Var:         map[data.Stat]float64{},
	}
	for _, s := range data.Stats {
		rate, rateVar := ix.rate(hist, s, league[s])
		mean := projMin * rate
		sampleMean, sampleVar := spread(hist, s)

		// Night-to-night spread of the stat itself.
		v := mean * 1.5 // fallback when there is no usable history
		if sampleMean > 0 {
			scale := math.Min(math.Max(mean/sampleMean, 0.7), 1.4)
			v = sampleVar * scale * scale
		}
		v *= varInflate
		if floor := mean * 1.05; v < floor {
			v = floor
		}
		// Plus our own uncertainty about the mean: the model only has a limited number of games
		// behind its minutes and rate estimates. Leaving this out makes the percentages overconfident.
		v += projMin*projMin*rateVar + rate*rate*projMinVar

		p.Mean[s] = mean
		p.Var[s] = v
	}
	return p, true
}
