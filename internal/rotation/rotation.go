// Package rotation allocates regulation team minutes using strictly earlier,
// complete box scores. It is an experimental forecast, not a causal injury model.
package rotation

import (
	"fmt"
	"math"
	"propsmodel/internal/data"
	"sort"
)

const Policy = "rotation-v1"
const LogFile = "v2_rotationlog.jsonl"
const SettledFile = "v2_rotationsettled.jsonl"

type Status struct {
	PlayerID int    `json:"playerId"`
	Player   string `json:"player"`
	Status   string `json:"status"`
	At       string `json:"at"`
}
type Scenario struct {
	PlayerID     int      `json:"playerId"`
	Player       string   `json:"player"`
	Status       string   `json:"status"`
	Minutes      float64  `json:"minutes"`
	MatchedGames int      `json:"matchedGames"`
	Supported    bool     `json:"supported"`
	Projection   *float64 `json:"projection,omitempty"`
	Probability  *float64 `json:"probability,omitempty"`
}
type Context struct {
	OwnStatus         string     `json:"ownStatus,omitempty"`
	Policy            string     `json:"policy"`
	BaselineMinutes   float64    `json:"baselineMinutes"`
	AllocationMinutes float64    `json:"allocationMinutes"`
	UsedMinutes       float64    `json:"usedMinutes"`
	TeamMinutes       float64    `json:"teamMinutes"`
	TeamGames         int        `json:"teamGames"`
	MatchedGames      int        `json:"matchedGames"`
	Supported         bool       `json:"supported"`
	Reason            string     `json:"reason"`
	ConfirmedOut      []Status   `json:"confirmedOut"`
	Scenarios         []Scenario `json:"scenarios,omitempty"`
}
type Allocation struct {
	Team         int             `json:"team"`
	Date         string          `json:"date"`
	Minutes      map[int]float64 `json:"minutes"`
	Baseline     map[int]float64 `json:"baseline"`
	TeamGames    int             `json:"teamGames"`
	MatchedGames int             `json:"matchedGames"`
	Supported    bool            `json:"supported"`
	Valid        bool            `json:"valid"`
	Reason       string          `json:"reason"`
	Out          []int           `json:"out"`
	RelevantOut  []int           `json:"relevantOut"`
}
type game struct {
	id      int
	date    string
	minutes map[int]float64
	valid   bool
}
type Index struct{ teams map[int][]game }

func New(rows []data.PlayerGame) *Index {
	byTeam := map[int]map[int]*game{}
	for _, r := range rows {
		if !r.SourceFinal || r.TeamID <= 0 || r.GameID <= 0 || r.PlayerID <= 0 || r.Date == "" || math.IsNaN(r.Minutes) || math.IsInf(r.Minutes, 0) || r.Minutes < 0 || r.Minutes > 70 {
			continue
		}
		if byTeam[r.TeamID] == nil {
			byTeam[r.TeamID] = map[int]*game{}
		}
		g := byTeam[r.TeamID][r.GameID]
		if g == nil {
			g = &game{id: r.GameID, date: r.Date, minutes: map[int]float64{}, valid: true}
			byTeam[r.TeamID][r.GameID] = g
		}
		if old, ok := g.minutes[r.PlayerID]; ok && math.Abs(old-r.Minutes) > .01 {
			g.valid = false
		}
		if g.date != r.Date {
			g.valid = false
		}
		g.minutes[r.PlayerID] = r.Minutes
	}
	ix := &Index{teams: map[int][]game{}}
	for team, gs := range byTeam {
		for _, g := range gs {
			sum, played := 0., 0
			for _, m := range g.minutes {
				sum += m
				if m > 0 {
					played++
				}
			}
			expected := 240 + 25*math.Round((sum-240)/25)
			if !g.valid || played < 8 || expected < 240 || expected > 340 || math.Abs(sum-expected) > 1 {
				continue
			}
			// Overtime exposure must not create extra regulation minutes.
			for pid, m := range g.minutes {
				g.minutes[pid] = m * 240 / sum
			}
			ix.teams[team] = append(ix.teams[team], *g)
		}
		sort.Slice(ix.teams[team], func(i, j int) bool {
			a, b := ix.teams[team][i], ix.teams[team][j]
			if a.date != b.date {
				return a.date < b.date
			}
			return a.id < b.id
		})
	}
	return ix
}

// Allocate uses a recent same-team workload baseline. Concurrent absences are
// handled jointly, never as additive player-by-player bonuses. Matching games
// require each requested absent player to have an explicit zero-minute row.
func (ix *Index) Allocate(team int, date string, roster map[int]int, absent map[int]bool) Allocation {
	a := Allocation{Team: team, Date: date, Minutes: map[int]float64{}, Baseline: map[int]float64{}}
	if _, e := dataDate(date); e != nil {
		a.Reason = "invalid forecast date"
		return a
	}
	for p, out := range absent {
		if out && roster[p] == team {
			a.Out = append(a.Out, p)
		}
	}
	sort.Ints(a.Out)
	cutoff := data.DayNumber(date) - 180
	var history []game
	for _, g := range ix.teams[team] {
		if g.date < date && data.DayNumber(g.date) >= cutoff {
			history = append(history, g)
		}
	}
	if len(history) < 5 {
		a.Reason = "fewer than five complete earlier team games"
		return a
	}
	recent := history
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	a.TeamGames = len(recent)
	weightSum := 0.
	for i, g := range recent {
		w := math.Pow(.85, float64(len(recent)-1-i))
		weightSum += w
		for pid, m := range g.minutes {
			if roster[pid] == team {
				a.Baseline[pid] += w * m
			}
		}
	}
	for pid, m := range a.Baseline {
		a.Baseline[pid] = m / weightSum
	}
	covered := 0.
	for _, m := range a.Baseline {
		covered += m
	}
	if covered < 216 {
		a.Reason = "current roster covers less than 90% of recent team minutes"
		return a
	}
	for pid, m := range a.Baseline {
		a.Baseline[pid] = m * 240 / covered
	}
	for _, pid := range a.Out {
		if a.Baseline[pid] > 0 {
			a.RelevantOut = append(a.RelevantOut, pid)
		}
	}
	available := map[int]float64{}
	for pid, m := range a.Baseline {
		if !absent[pid] && m > 0 {
			available[pid] = m
		}
	}
	if len(available) < 8 {
		a.Reason = "fewer than eight available players with observed team minutes"
		return a
	}
	conditional := map[int]float64{}
	if len(a.RelevantOut) > 0 {
		for _, g := range history {
			match := true
			for _, pid := range a.RelevantOut {
				m, ok := g.minutes[pid]
				if !ok || m > 0 {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			coverage := 0.
			for pid, m := range g.minutes {
				if roster[pid] == team {
					coverage += m
				}
			}
			if coverage < 216 {
				continue
			}
			a.MatchedGames++
			for pid := range available {
				conditional[pid] += g.minutes[pid]
			}
		}
	}
	baselineShares, ok := normalize(available)
	if !ok {
		a.Reason = "available workload cannot fill 240 minutes"
		return a
	}
	target := baselineShares
	if a.MatchedGames > 0 {
		if empirical, ok := normalize(conditional); ok {
			w := float64(a.MatchedGames) / (float64(a.MatchedGames) + 5)
			target = map[int]float64{}
			for pid, m := range baselineShares {
				target[pid] = (1-w)*m + w*empirical[pid]
			}
		}
	}
	a.Minutes, ok = normalize(target)
	if !ok {
		a.Reason = "allocation is infeasible"
		return a
	}
	a.Valid = true
	a.Supported = len(a.RelevantOut) == 0 || a.MatchedGames >= 3
	if len(a.RelevantOut) == 0 {
		a.Reason = "recent complete-team workload; no fresh confirmed absences"
		if len(a.Out) > 0 {
			a.Reason = "recent complete-team workload; confirmed outs have no recent rotation minutes to redistribute"
		}
	} else if !a.Supported {
		a.Reason = "limited matching rotations; recent-workload fallback, review only"
	} else {
		a.Reason = "shrunk joint-absence team rotations; experimental, not validated"
	}
	return a
}

// normalize is a bounded water-fill. It conserves the 240 regulation minutes
// and never allocates more than 48 to any one player. No artificial reserve is added.
func normalize(weights map[int]float64) (map[int]float64, bool) {
	out := map[int]float64{}
	active := map[int]float64{}
	for pid, w := range weights {
		if w > 0 && !math.IsNaN(w) && !math.IsInf(w, 0) {
			active[pid] = w
		}
	}
	remaining := 240.
	for len(active) > 0 {
		sum := 0.
		for _, w := range active {
			sum += w
		}
		capped := []int{}
		for pid, w := range active {
			if remaining*w/sum > 48 {
				capped = append(capped, pid)
			}
		}
		if len(capped) == 0 {
			for pid, w := range active {
				out[pid] = remaining * w / sum
			}
			remaining = 0
			break
		}
		for _, pid := range capped {
			out[pid] = 48
			remaining -= 48
			delete(active, pid)
		}
	}
	return out, math.Abs(remaining) < 1e-7
}
func dataDate(s string) (int, error) {
	d := data.DayNumber(s)
	if d == 0 {
		return 0, fmt.Errorf("invalid date")
	}
	return d, nil
}
