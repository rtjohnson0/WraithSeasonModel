// Package synthworld simulates a small NBA-like league so the whole pipeline can be tested
// without any real data.
//
// The world has real, learnable structure: opponent defense, game pace, home court,
// back-to-backs, blowouts that cut starters' minutes, hot and cold streaks, and injured
// teammates whose minutes and shots go to the players still on the floor. The simulated
// sportsbook knows about most of this, but only partly sees injuries and streaks, so a model
// that uses context can beat it a little. That is the point of the test: it checks that the
// code finds structure that is really there. It says NOTHING about real NBA edge. Real
// sportsbooks are smarter than this one, and only a backtest on real data answers that.
package synthworld

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/props"
)

// Config sets the size of the world.
type Config struct {
	Seed    int64
	Seasons []int // season start years, e.g. 2023, 2024, 2025
	Teams   int   // default 30
	Days    int   // days per season, default 170
}

// UpGame is a game on the day after the last simulated one.
type UpGame struct{ ID, Home, Visitor int }

// Upcoming is the day after the simulation ends, as a live run would see it.
type Upcoming struct {
	Date  string
	Games []UpGame
	Out   map[int]bool // players ruled out
	Lines []props.Line
}

// World is the simulated league.
type World struct {
	Games    []data.PlayerGame
	Adv      map[data.AdvKey]data.AdvRow
	Lines    []props.Line // opening lines, every season
	Names    map[int]string
	Upcoming Upcoming
}

var baseMinutes = [10]float64{35, 33, 31, 29, 26, 23, 19, 15, 11, 8}

type player struct {
	id, team, slot int
	bm             float64
	r0             [data.NStats]float64 // per-minute rates for points, rebounds, assists, threes
	form           [data.NStats]float64 // log multiplier, mean-reverting
	outUntil       int
	usage          float64
}

type team struct {
	id       int
	strength float64
	def      [data.NStats]float64 // log multiplier on what opponents score against this team
	pace     float64
	players  []*player
	lastDay  int
}

// Effect sizes in the simulated world (per point of an absent teammate's per-game average).
var missBeta = [data.NStats]float64{0.010, 0.005, 0.008, 0.006}

const (
	alpha      = 0.07 // negative binomial dispersion of the simulated counts
	pOut       = 0.045
	marketSee  = 0.35 // share of injury effects the simulated sportsbook has priced in
	marketMin  = 0.35 // share of the minutes redistribution the book has priced in
	marketForm = 0.3  // share of each player's streak the simulated sportsbook sees
)

// Generate builds the world.
func Generate(cfg Config) *World {
	if cfg.Teams == 0 {
		cfg.Teams = 30
	}
	if cfg.Days == 0 {
		cfg.Days = 170
	}
	r := rand.New(rand.NewSource(cfg.Seed))
	w := &World{Adv: map[data.AdvKey]data.AdvRow{}, Names: map[int]string{}}

	teams := make([]*team, cfg.Teams)
	for t := range teams {
		tm := &team{id: t + 1, strength: 3.5 * r.NormFloat64(), pace: 0.03 * r.NormFloat64()}
		for k := range tm.def {
			tm.def[k] = 0.04 * r.NormFloat64()
		}
		for s := 0; s < 10; s++ {
			skill := 1.0 + 0.35*(1-float64(s)/9)
			p := &player{
				id: tm.id*100 + s, team: tm.id, slot: s, bm: baseMinutes[s],
				r0: [data.NStats]float64{
					(0.36 + 0.40*r.Float64()) * skill,
					0.08 + 0.20*r.Float64(),
					0.05 + 0.20*r.Float64(),
					0.01 + 0.07*r.Float64(),
				},
				usage: 0.16 + 0.10*r.Float64()*skill,
			}
			tm.players = append(tm.players, p)
			w.Names[p.id] = fmt.Sprintf("Player%03d Synthetic", p.id)
		}
		teams[t] = tm
	}

	gameID := 10000
	var lastDay int
	for si, season := range cfg.Seasons {
		start := time.Date(season, 10, 22, 0, 0, 0, 0, time.UTC)
		for _, tm := range teams {
			tm.lastDay = -10
			for _, p := range tm.players {
				p.outUntil = -1
			}
		}
		for d := 0; d < cfg.Days; d++ {
			date := start.AddDate(0, 0, d)
			day := data.DayNumber(date.Format("2006-01-02"))
			lastDay = day
			pairs := pairUp(r, teams, 0.48)
			advance(r, teams, d)
			for _, pr := range pairs {
				gameID++
				w.playGame(r, season, date, day, d, gameID, pr[0], pr[1])
			}
			for _, tm := range teams {
				for _, p := range tm.players {
					for k := range p.form {
						p.form[k] = 0.93*p.form[k] + 0.035*r.NormFloat64()
					}
				}
			}
			for _, pr := range pairs {
				pr[0].lastDay, pr[1].lastDay = d, d
			}
		}
		_ = si
	}

	// The day after the simulation: a live slate.
	last := time.Unix(int64(lastDay)*86400, 0).UTC()
	up := last.AddDate(0, 0, 1)
	d := cfg.Days
	advance(r, teams, d)
	w.Upcoming = Upcoming{Date: up.Format("2006-01-02"), Out: map[int]bool{}}
	for _, pr := range pairUp(r, teams, 0.48) {
		gameID++
		w.Upcoming.Games = append(w.Upcoming.Games, UpGame{ID: gameID, Home: pr[0].id, Visitor: pr[1].id})
		for i, tm := range pr {
			opp := pr[1-i]
			abs := absent(tm, d)
			for _, p := range tm.players {
				if p.outUntil >= d {
					w.Upcoming.Out[p.id] = true
				}
			}
			for _, p := range tm.players {
				if p.outUntil >= d || p.slot > 5 {
					continue
				}
				mins, rates := marketView(p, tm, opp, i == 0, d, abs)
				for k, s := range data.Stats {
					mean := mins * rates[k]
					for v := 0; v < 3; v++ {
						line := nbMedianLine(mean*(1+0.01*r.NormFloat64()), alpha)
						w.Upcoming.Lines = append(w.Upcoming.Lines, props.Line{
							GameID: gameID, PlayerID: p.id, Vendor: vendors[v], Stat: s,
							Line: line, OverOdds: -110, UnderOdds: -110, UpdatedAt: w.Upcoming.Date,
						})
					}
				}
			}
		}
	}
	sort.SliceStable(w.Games, func(i, j int) bool {
		a, b := w.Games[i], w.Games[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.GameID != b.GameID {
			return a.GameID < b.GameID
		}
		return a.PlayerID < b.PlayerID
	})
	return w
}

var vendors = []string{"draftkings", "fanduel", "betmgm"}

// pairUp picks the teams that play today (each plays with probability frac) and pairs them.
func pairUp(r *rand.Rand, teams []*team, frac float64) [][2]*team {
	idx := r.Perm(len(teams))
	var playing []*team
	for _, i := range idx {
		if r.Float64() < frac {
			playing = append(playing, teams[i])
		}
	}
	var out [][2]*team
	for i := 0; i+1 < len(playing); i += 2 {
		out = append(out, [2]*team{playing[i], playing[i+1]})
	}
	return out
}

// advance updates injuries for day d.
func advance(r *rand.Rand, teams []*team, d int) {
	for _, tm := range teams {
		for _, p := range tm.players {
			if p.slot > 7 || p.outUntil >= d {
				continue
			}
			prob := pOut
			if p.slot < 2 {
				prob *= 1.4
			}
			if r.Float64() < prob {
				p.outUntil = d + int(r.ExpFloat64()*5)
			}
		}
	}
}

type absentInfo struct {
	mins   float64
	prod   [data.NStats]float64
	slotOK map[int]bool
}

func absent(tm *team, d int) absentInfo {
	a := absentInfo{slotOK: map[int]bool{}}
	for _, p := range tm.players {
		if p.outUntil >= d && p.slot <= 7 {
			a.mins += p.bm
			for k := range a.prod {
				a.prod[k] += p.r0[k] * p.bm
			}
		} else if p.outUntil < d {
			a.slotOK[p.slot] = true
		}
	}
	return a
}

// share of absent minutes a player picks up.
func pickup(p *player, tm *team, d int) float64 {
	var weight, mine float64
	for _, q := range tm.players {
		if q.outUntil >= d {
			continue
		}
		wq := q.bm
		if q.slot > 6 {
			wq *= 0.3
		}
		weight += wq
		if q == p {
			mine = wq
		}
	}
	if weight == 0 {
		return 0
	}
	return mine / weight
}

func homeEff(home bool) float64 {
	if home {
		return 0.015
	}
	return -0.015
}

// trueRates is the simulated world's actual per-minute rates for a player.
func trueRates(p *player, tm, opp *team, home bool, b2b bool, abs absentInfo) [data.NStats]float64 {
	var out [data.NStats]float64
	for k := 0; k < data.NStats; k++ {
		eta := p.form[k] + opp.def[k] + 0.6*(tm.pace+opp.pace) + homeEff(home) + missBeta[k]*abs.prod[k]/10*1.0
		if b2b {
			eta -= 0.02
		}
		out[k] = p.r0[k] * math.Exp(eta)
	}
	return out
}

// marketView is what the simulated sportsbook believes: it sees schedule, opponent, pace and
// home court, half of each player's streak, and part of the injury news.
func marketView(p *player, tm, opp *team, home bool, d int, abs absentInfo) (mins float64, rates [data.NStats]float64) {
	b2b := tm.lastDay == d-1
	var seen [data.NStats]float64
	for k := 0; k < data.NStats; k++ {
		seen[k] = marketSee * missBeta[k] * abs.prod[k] / 10
	}
	for k := 0; k < data.NStats; k++ {
		eta := marketForm*p.form[k] + opp.def[k] + 0.6*(tm.pace+opp.pace) + homeEff(home) + seen[k]
		if b2b {
			eta -= 0.02
		}
		rates[k] = p.r0[k] * math.Exp(eta)
	}
	mins = p.bm + marketMin*abs.mins*pickup(p, tm, d)
	if b2b {
		mins -= 1.2
	}
	return math.Min(mins, 40), rates
}

func (w *World) playGame(r *rand.Rand, season int, date time.Time, day, d, gameID int, home, away *team) {
	dateStr := date.Format("2006-01-02")
	margin := (home.strength-away.strength)*0.9 + 2.5 + 11*r.NormFloat64()
	for i, tm := range []*team{home, away} {
		opp := []*team{away, home}[i]
		isHome := i == 0
		b2b := tm.lastDay == d-1
		abs := absent(tm, d)
		myMargin := margin
		if !isHome {
			myMargin = -margin
		}
		for _, p := range tm.players {
			if p.outUntil >= d {
				continue
			}
			mins := p.bm + abs.mins*pickup(p, tm, d) + 3.2*r.NormFloat64()
			if b2b {
				mins -= 1.2
			}
			if m := math.Abs(myMargin); m > 14 && p.slot < 5 {
				mins -= (m - 14) * 0.55
			}
			if r.Float64() < 0.03 {
				mins *= 0.15 + 0.45*r.Float64()
			}
			mins = math.Min(math.Max(mins, 0), 44)
			if mins < 3 {
				continue
			}
			mins = math.Round(mins)
			if mins < 1 {
				continue
			}
			rates := trueRates(p, tm, opp, isHome, b2b, abs)
			var y [data.NStats]float64
			for k := 0; k < data.NStats; k++ {
				y[k] = nbSample(r, mins*rates[k], alpha)
			}
			fga := math.Round(y[0] / (2 * (0.50 + 0.03*r.NormFloat64())))
			fgm := math.Round(fga * math.Min(math.Max(0.46+0.07*r.NormFloat64(), 0.2), 0.8))
			fg3a := math.Round(y[3] / math.Min(math.Max(0.36+0.04*r.NormFloat64(), 0.2), 0.55))
			pg := data.PlayerGame{
				PlayerID: p.id, Name: w.Names[p.id], TeamID: tm.id, GameID: gameID,
				Date: dateStr, Season: season, Minutes: mins,
				Pts: y[0], Reb: y[1], Ast: y[2], Fga: fga, Fgm: fgm, Fg3a: fg3a, Fg3m: y[3],
				Day: day, HomeTeamID: home.id, VisitorTeamID: away.id,
			}
			w.Games = append(w.Games, pg)

			// Advanced stats: usage and shares move with streaks and with absent teammates.
			teamBoost := missBeta[0] * abs.prod[0] / 10
			w.Adv[data.AdvKey{Player: p.id, Game: gameID}] = data.AdvRow{
				Usage:   p.usage * math.Exp(0.8*p.form[0]+3*teamBoost) * (1 + 0.04*r.NormFloat64()),
				Pace:    100 * math.Exp(tm.pace+opp.pace),
				AstPct:  100 * p.r0[2] / 3 * math.Exp(0.8*p.form[2]) * (1 + 0.05*r.NormFloat64()),
				RebPct:  100 * p.r0[1] / 3 * math.Exp(0.8*p.form[1]) * (1 + 0.05*r.NormFloat64()),
				Touches: mins * (1.6 + 2*p.usage),
				Passes:  mins * (1.0 + 4*p.r0[2]) * math.Exp(0.5*p.form[2]),
				FTR:     math.NaN(), Pct3PA: math.NaN(),
				// Rebound chances follow the player's role, not his luck: about 2.3 chances
				// per rebound on average, with the actual rebounds scattered around that.
				RebCh:    mins * p.r0[1] * math.Exp(0.8*p.form[1]) / 0.43 * (1 + 0.06*r.NormFloat64()),
				RebChOff: mins * p.r0[1] * math.Exp(0.8*p.form[1]) / 0.43 * 0.25,
				SecAst:   math.NaN(), FtAst: math.NaN(),
			}

			// Opening lines from three simulated books, only for rotation players.
			if p.slot <= 5 {
				bm, br := marketView(p, tm, opp, isHome, d, abs)
				for k, s := range data.Stats {
					mean := bm * br[k]
					for v := 0; v < 3; v++ {
						w.Lines = append(w.Lines, props.Line{
							GameID: gameID, PlayerID: p.id, Vendor: vendors[v], Stat: s,
							Line:     nbMedianLine(mean*(1+0.012*r.NormFloat64()), alpha),
							OverOdds: -110, UnderOdds: -110, UpdatedAt: dateStr,
						})
					}
				}
			}
		}
	}
}

// ---- count distribution helpers ----

func poisson(r *rand.Rand, lambda float64) float64 {
	if lambda <= 0 {
		return 0
	}
	if lambda > 60 { // normal approximation keeps this fast and safe for huge means
		return math.Max(0, math.Round(lambda+math.Sqrt(lambda)*r.NormFloat64()))
	}
	l := math.Exp(-lambda)
	k, p := 0, 1.0
	for {
		k++
		p *= r.Float64()
		if p <= l {
			return float64(k - 1)
		}
	}
}

// gammaShape draws Gamma(shape, 1) with the Marsaglia-Tsang method.
func gammaShape(r *rand.Rand, shape float64) float64 {
	if shape < 1 {
		return gammaShape(r, shape+1) * math.Pow(r.Float64(), 1/shape)
	}
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := r.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if math.Log(u) < 0.5*x*x+d-d*v+d*math.Log(v) {
			return d * v
		}
	}
}

// nbSample draws from a negative binomial with the given mean and dispersion alpha
// (variance = mean + alpha*mean^2) as a gamma-mixed Poisson.
func nbSample(r *rand.Rand, mean, a float64) float64 {
	if mean <= 0 {
		return 0
	}
	k := 1 / a
	return poisson(r, mean*gammaShape(r, k)/k)
}

// nbMedianLine returns the half-point line a book would post: the one where over and under
// are closest to even.
func nbMedianLine(mean, a float64) float64 {
	if mean <= 0.05 {
		return 0.5
	}
	k := 1 / a
	p := k / (k + mean)
	best, bestDiff, cdf := 0, 2.0, 0.0
	for n := 0; n < 400; n++ {
		lg1, _ := math.Lgamma(float64(n) + k)
		lg2, _ := math.Lgamma(k)
		lg3, _ := math.Lgamma(float64(n) + 1)
		cdf += math.Exp(lg1 - lg2 - lg3 + k*math.Log(p) + float64(n)*math.Log(1-p))
		if diff := math.Abs(cdf - 0.5); diff < bestDiff {
			best, bestDiff = n, diff
		}
		if cdf > 0.9999 {
			break
		}
	}
	return float64(best) + 0.5
}
