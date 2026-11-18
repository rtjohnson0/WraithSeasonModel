// Package lineup estimates joint-absence associations from earlier complete
// box scores. These are bounded research adjustments, not causal injury effects.
package lineup

import (
	"math"
	"sort"
	"strings"

	"propsmodel/internal/data"
)

type Profile struct {
	Status   string  `json:"status"`
	At       string  `json:"at"`
	PlayerID int     `json:"playerId"`
	Player   string  `json:"player"`
	Position string  `json:"position"`
	Games    int     `json:"games"`
	Minutes  float64 `json:"minutes"`
	Points   float64 `json:"points"`
	Rebounds float64 `json:"rebounds"`
	Assists  float64 `json:"assists"`
	Blocks   float64 `json:"blocks"`
}
type Effect struct {
	Stat        data.Stat     `json:"stat"`
	Scale       float64       `json:"scale"`
	Applied     bool          `json:"applied"`
	Without     int           `json:"without"`
	With        int           `json:"with"`
	Conditional float64       `json:"conditional"`
	Reference   float64       `json:"reference"`
	Reason      string        `json:"reason"`
	Method      string        `json:"method,omitempty"`
	Evidence    []Component   `json:"evidence,omitempty"`
	Role        *RoleEvidence `json:"role,omitempty"`
}
type Impact struct {
	Kind     string    `json:"kind"`
	Team     int       `json:"team"`
	Position string    `json:"position,omitempty"`
	Missing  []Profile `json:"missing,omitempty"`
	Effects  []Effect  `json:"effects"`
	Reason   string    `json:"reason"`
}
type game struct {
	id    int
	date  string
	teams map[int]map[int]data.PlayerGame
	valid bool
}
type Index struct {
	games   []*game
	history map[int][]data.PlayerGame
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func validStats(r data.PlayerGame) bool {
	if !r.AnalyticsReady || !finite(r.Blk) || r.Blk < 0 {
		return false
	}
	for _, s := range data.Stats {
		if v := r.Value(s); !finite(v) || v < 0 {
			return false
		}
	}
	return true
}
func New(rows []data.PlayerGame) *Index {
	ix := &Index{history: map[int][]data.PlayerGame{}}
	gs := map[int]*game{}
	for _, r := range rows {
		if !r.SourceFinal || r.GameID <= 0 || r.PlayerID <= 0 || r.TeamID <= 0 || data.DayNumber(r.Date) == 0 || !finite(r.Minutes) || r.Minutes < 0 || r.Minutes > 70 {
			continue
		}
		g := gs[r.GameID]
		if g == nil {
			g = &game{id: r.GameID, date: r.Date, teams: map[int]map[int]data.PlayerGame{}, valid: true}
			gs[r.GameID] = g
		}
		if g.date != r.Date {
			g.valid = false
		}
		if g.teams[r.TeamID] == nil {
			g.teams[r.TeamID] = map[int]data.PlayerGame{}
		}
		if old, ok := g.teams[r.TeamID][r.PlayerID]; ok && old != r {
			g.valid = false
		}
		g.teams[r.TeamID][r.PlayerID] = r
	}
	for _, g := range gs {
		if !g.valid || len(g.teams) != 2 {
			continue
		}
		good := true
		for _, rs := range g.teams {
			mins, played := 0., 0
			for _, r := range rs {
				mins += r.Minutes
				if r.Minutes > 0 {
					played++
					if !validStats(r) {
						good = false
					}
				}
			}
			expected := 240 + 25*math.Round((mins-240)/25)
			if played < 8 || expected < 240 || expected > 340 || math.Abs(mins-expected) > 1 {
				good = false
			}
		}
		if !good {
			continue
		}
		ix.games = append(ix.games, g)
	}
	sort.Slice(ix.games, func(i, j int) bool {
		a, b := ix.games[i], ix.games[j]
		if a.date != b.date {
			return a.date < b.date
		}
		return a.id < b.id
	})
	for _, g := range ix.games {
		for _, rs := range g.teams {
			for pid, r := range rs {
				if r.Minutes > 0 {
					ix.history[pid] = append(ix.history[pid], r)
				}
			}
		}
	}
	return ix
}

// Position intentionally does not guess a missing position.
func Position(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "G", "PG", "SG":
		return "G"
	case "G-F", "F-G":
		return "W"
	case "F", "PF", "SF":
		return "F"
	case "F-C", "C-F":
		return "B"
	case "C":
		return "C"
	}
	return ""
}
func (ix *Index) position(player int, date string) string {
	hs := ix.history[player]
	for i := len(hs) - 1; i >= 0; i-- {
		if hs[i].Date < date {
			return Position(hs[i].Pos)
		}
	}
	return ""
}

// priorRate normalizes the historical opposing player's output by his own
// strictly earlier production, reducing (not eliminating) schedule confounding.
func (ix *Index) priorRate(player int, date string, k int) (float64, bool) {
	n, mins, total := 0, 0., 0.
	hs := ix.history[player]
	for i := len(hs) - 1; i >= 0 && n < 10; i-- {
		r := hs[i]
		if r.Date >= date || data.DayNumber(r.Date) < data.DayNumber(date)-180 {
			continue
		}
		n++
		mins += r.Minutes
		total += r.Value(data.Stats[k])
	}
	if n < 3 || mins <= 0 || total <= 0 {
		return 0, false
	}
	return total / mins, true
}
func state(rs map[int]data.PlayerGame, outs []int) int {
	missing, present := 0, 0
	for _, pid := range outs {
		r, ok := rs[pid]
		if !ok {
			return 0
		}
		if r.Minutes == 0 {
			missing++
		} else {
			present++
		}
	}
	if missing == len(outs) {
		return -1
	}
	if present == len(outs) {
		return 1
	}
	return 0
}
func covered(rs map[int]data.PlayerGame, roster map[int]int, team int) bool {
	all, current := 0., 0.
	for pid, r := range rs {
		all += r.Minutes
		if roster[pid] == team {
			current += r.Minutes
		}
	}
	return all > 0 && current/all >= .9
}
func (ix *Index) profiles(team int, date string, roster map[int]int, outs []int) []Profile {
	var ps []Profile
	seen := map[int]bool{}
	for _, pid := range outs {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		if roster[pid] != team {
			continue
		}
		p := Profile{PlayerID: pid}
		hs := ix.history[pid]
		for i := len(hs) - 1; i >= 0 && p.Games < 10; i-- {
			r := hs[i]
			if r.TeamID != team || r.Date >= date || data.DayNumber(r.Date) < data.DayNumber(date)-180 {
				continue
			}
			p.Games++
			if p.Player == "" {
				p.Player = r.Name
				p.Position = r.Pos
			}
			p.Minutes += r.Minutes
			p.Points += r.Pts
			p.Rebounds += r.Reb
			p.Assists += r.Ast
			p.Blocks += r.Blk
		}
		if p.Games > 0 {
			n := float64(p.Games)
			p.Minutes /= n
			p.Points /= n
			p.Rebounds /= n
			p.Assists /= n
			p.Blocks /= n
			if p.Minutes >= 8 {
				ps = append(ps, p)
			}
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].PlayerID < ps[j].PlayerID })
	return ps
}

// Estimate compares the joint set of confirmed missing rotation players. Own
// production uses player-specific rates; opponent impact uses position-specific
// allowed output relative to each historical scorer's prior per-minute rate.
func (ix *Index) Estimate(team, player int, date string, roster map[int]int, outs []int, opponent bool) Impact {
	return ix.estimate(team, player, date, roster, outs, opponent)
}

func (ix *Index) estimate(team, player int, date string, roster map[int]int, outs []int, opponent bool) Impact {
	kind := "teammate-production"
	if opponent {
		kind = "opponent-allowance"
	}
	out := Impact{Kind: kind, Team: team, Reason: "no relevant confirmed absences"}
	out.Missing = ix.profiles(team, date, roster, outs)
	ids := []int{}
	for _, p := range out.Missing {
		ids = append(ids, p.PlayerID)
	}
	if len(ids) == 0 {
		return out
	}
	pos := ix.position(player, date)
	if opponent {
		out.Position = pos
		if pos == "" {
			out.Reason = "position unavailable; opponent rates unchanged"
			return out
		}
	}
	var totals, exposure [2][data.NStats]float64
	var counts [2][data.NStats]int
	for _, g := range ix.games {
		if g.date >= date || data.DayNumber(g.date) < data.DayNumber(date)-180 {
			continue
		}
		rs, ok := g.teams[team]
		if !ok || !covered(rs, roster, team) {
			continue
		}
		s := state(rs, ids)
		if s == 0 {
			continue
		}
		bucket := 0
		if s == 1 {
			bucket = 1
		}
		if !opponent {
			r, ok := rs[player]
			if !ok || r.Minutes < 8 {
				continue
			}
			for k, st := range data.Stats {
				counts[bucket][k]++
				totals[bucket][k] += r.Value(st)
				exposure[bucket][k] += r.Minutes
			}
		} else {
			var actual, expected [data.NStats]float64
			for tm, players := range g.teams {
				if tm == team {
					continue
				}
				for pid, r := range players {
					if r.Minutes <= 0 || Position(r.Pos) != pos {
						continue
					}
					for k, st := range data.Stats {
						if rate, ok := ix.priorRate(pid, g.date, k); ok {
							actual[k] += r.Value(st)
							expected[k] += rate * r.Minutes
						}
					}
				}
			}
			for k := range data.Stats {
				if expected[k] > 0 {
					counts[bucket][k]++
					totals[bucket][k] += actual[k]
					exposure[bucket][k] += expected[k]
				}
			}
		}
	}
	out.Reason = "joint-absence observational rates; research only, not causal or validated"
	for k, st := range data.Stats {
		e := Effect{Method: "joint", Stat: st, Scale: 1, Without: counts[0][k], With: counts[1][k], Reason: "need five earlier without and five with the joint set present"}
		if exposure[0][k] > 0 {
			e.Conditional = totals[0][k] / exposure[0][k]
		}
		if exposure[0][k]+exposure[1][k] > 0 {
			e.Reference = (totals[0][k] + totals[1][k]) / (exposure[0][k] + exposure[1][k])
		}
		if e.Without >= 5 && e.With >= 5 && e.Reference > 0 {
			// Shrink toward the mixed-history baseline already represented by the
			// production model, not an unadjusted full-strength lineup. Never stack
			// independent player bonuses or force a favorable direction.
			weight := float64(e.Without) / (float64(e.Without) + 20)
			e.Scale = math.Max(.95, math.Min(1.05, 1+weight*(e.Conditional/e.Reference-1)))
			e.Applied = true
			e.Reason = "shrunk joint-absence association, capped at ±5%"
		}
		out.Effects = append(out.Effects, e)
	}
	return out
}
func Scale(impacts []Impact) [data.NStats]float64 {
	out := [data.NStats]float64{1, 1, 1, 1}
	for _, impact := range impacts {
		for _, e := range impact.Effects {
			if e.Applied {
				k := data.StatIndex(e.Stat)
				out[k] *= e.Scale
			}
		}
	}
	for k, x := range out {
		out[k] = math.Max(.92, math.Min(1.08, x))
	}
	return out
}

// Profiles exposes strictly earlier observed role contributions for explanations.
func (ix *Index) Profiles(team int, date string, roster map[int]int, players []int) []Profile {
	return ix.profiles(team, date, roster, players)
}
