// Package availability describes historical rotation associations. It does not
// infer injury diagnoses, guarantee replacement roles, or alter a posted pick.
package availability

import (
	"math"
	"propsmodel/internal/data"
	"sort"
)

type Sample struct {
	Date           string  `json:"date"`
	GameID         int     `json:"gameId"`
	Minutes        float64 `json:"minutes"`
	TeammatePlayed bool    `json:"teammatePlayed"`
}
type Evidence struct {
	Samples            []Sample `json:"samples,omitempty"`
	ReportedAt         string   `json:"reportedAt,omitempty"`
	PlayerID           int      `json:"playerId"`
	Player             string   `json:"player"`
	Status             string   `json:"status"`
	ConfirmedOut       bool     `json:"confirmedOut"`
	AssumedOut         bool     `json:"assumedOut,omitempty"`
	WithN              int      `json:"withN"`
	WithoutN           int      `json:"withoutN"`
	WithAvg            float64  `json:"withAvg"`
	WithoutAvg         float64  `json:"withoutAvg"`
	Supported          bool     `json:"supported"`
	WithMinutes        float64  `json:"withMinutes,omitempty"`
	WithoutMinutes     float64  `json:"withoutMinutes,omitempty"`
	WithProjection     float64  `json:"withProjection,omitempty"`
	WithoutProjection  float64  `json:"withoutProjection,omitempty"`
	WithProbability    *float64 `json:"withProbability,omitempty"`
	WithoutProbability *float64 `json:"withoutProbability,omitempty"`
	Reverses           bool     `json:"reverses,omitempty"`
	Through            string   `json:"through,omitempty"`
}
type Context struct {
	Version             int        `json:"version"`
	AsOf                string     `json:"asOf"`
	OwnStatus           string     `json:"ownStatus,omitempty"`
	BaselineMinutes     float64    `json:"baselineMinutes"`
	BaselineProbability float64    `json:"baselineProbability"`
	Teammates           []Evidence `json:"teammates"`
}
type teamGame struct {
	id       int
	date     string
	minutes  map[int]float64
	valid    bool
	conflict bool
}
type Index struct{ teams map[int][]teamGame }

func New(games []data.PlayerGame) *Index {
	byTeam := map[int]map[int]*teamGame{}
	for _, g := range games {
		if !g.SourceFinal || g.TeamID == 0 || g.GameID == 0 || g.Date == "" {
			continue
		}
		if byTeam[g.TeamID] == nil {
			byTeam[g.TeamID] = map[int]*teamGame{}
		}
		t := byTeam[g.TeamID][g.GameID]
		if t == nil {
			t = &teamGame{id: g.GameID, date: g.Date, minutes: map[int]float64{}}
			byTeam[g.TeamID][g.GameID] = t
		}
		if !math.IsNaN(g.Minutes) && g.Minutes >= 0 && g.Minutes <= 70 {
			if old, exists := t.minutes[g.PlayerID]; exists && math.Abs(old-g.Minutes) > .01 {
				t.conflict = true
			}
			if t.date != g.Date {
				t.conflict = true
			}
			t.minutes[g.PlayerID] = g.Minutes
		}
	}
	ix := &Index{teams: map[int][]teamGame{}}
	for team, gs := range byTeam {
		for _, g := range gs {
			sum, played := 0., 0
			for _, m := range g.minutes {
				sum += m
				if m > 0 {
					played++
				}
			}
			// Complete regulation or overtime boxes only. Reject partial-team downloads.
			expected := 240 + 25*math.Round((sum-240)/25)
			g.valid = !g.conflict && played >= 8 && expected >= 240 && expected <= 340 && math.Abs(sum-expected) <= 1

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
func avg(x []float64) float64 {
	s := 0.
	for _, v := range x {
		s += v
	}
	if len(x) == 0 {
		return 0
	}
	return s / float64(len(x))
}

// Compare uses strictly earlier same-team games within 180 days. A teammate is
// considered absent only after recent participation established their rotation
// membership. "Without" means no recorded playing time, not a known injury.
func (ix *Index) Compare(team, pid, teammate int, before string) Evidence {
	gs := ix.teams[team]
	cut := sort.Search(len(gs), func(i int) bool { return gs[i].date >= before })
	var with, without []float64
	var samples []Sample
	after := data.DayNumber(before) - 180
	for i := 0; i < cut; i++ {
		g := gs[i]
		if !g.valid || data.DayNumber(g.date) < after || g.minutes[pid] < 1 {
			continue
		}
		var prior []float64
		apps, last := 0, -1
		for j := i - 1; j >= 0 && j >= i-10; j-- {
			p := gs[j]
			if !p.valid {
				continue
			}
			if p.minutes[pid] > 0 {
				prior = append(prior, p.minutes[pid])
			}
			if p.minutes[teammate] > 0 {
				apps++
				if last < 0 {
					last = i - j
				}
			}
		}
		if len(prior) < 5 || apps < 3 || last > 6 {
			continue
		}
		m := g.minutes[pid]
		samples = append(samples, Sample{Date: g.date, GameID: g.id, Minutes: m, TeammatePlayed: g.minutes[teammate] > 0})
		if g.minutes[teammate] > 0 {
			with = append(with, m)
		} else {
			without = append(without, m)
		}
	}
	e := Evidence{Samples: samples, PlayerID: teammate, WithN: len(with), WithoutN: len(without), WithAvg: avg(with), WithoutAvg: avg(without)}
	if len(with) < 5 || len(without) < 3 {
		return e
	}
	e.Supported = true
	// These are observed historical averages, never invented rotation boosts.
	e.WithMinutes = e.WithAvg
	e.WithoutMinutes = e.WithoutAvg
	for i := cut - 1; i >= 0; i-- {
		if gs[i].valid {
			e.Through = gs[i].date
			break
		}
	}
	return e
}
