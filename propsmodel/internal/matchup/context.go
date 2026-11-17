// Package matchup supplies descriptive pregame rankings, never probability multipliers.
package matchup

import (
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"sort"
	"strings"
)

type Rank struct {
	Stat     string  `json:"stat"`
	Rank     int     `json:"rank"`
	Of       int     `json:"of"`
	Games    int     `json:"games"`
	Average  float64 `json:"average"`
	Position string  `json:"position,omitempty"`
}
type Team struct {
	Name    string `json:"name"`
	Season  int    `json:"season"`
	Before  string `json:"before"`
	Scoring *Rank  `json:"scoring,omitempty"` // rank 1 scores most
	Allowed []Rank `json:"allowed,omitempty"` // rank 1 allows fewest
}
type Prop struct {
	Opponent Team   `json:"opponent"`
	Position []Rank `json:"position,omitempty"` // rank 1 allows fewest
}
type Index struct {
	Teams     map[int]Team
	Positions map[int]map[int][]Rank
	Players   map[int]string
	Games     map[int][2]int
	Abbr      map[string]int
}
type box struct {
	hp, ap           float64
	complete         bool
	team, home, away int
	vals             [4]float64
	pos              [5][4]float64
	mins, known      float64
}
type tally struct {
	n               int
	scored, allowed [4]float64
	pos             [5][4]float64
	posN            [5]int
}

// Build uses explicit final full-game box scores strictly before the prediction date.
// Position groups match the feed (guards, wings, forwards, big forwards, centers).
func Build(rows []data.PlayerGame, before string) *Index {
	ix := &Index{Teams: map[int]Team{}, Positions: map[int]map[int][]Rank{}, Players: map[int]string{}, Games: map[int][2]int{}, Abbr: map[string]int{}}
	games := map[int]map[int]*box{}
	seen := map[[2]int]bool{}
	latest := map[int]string{}
	season := data.SeasonOf(before)
	for _, r := range rows {
		if r.Date >= before || r.Season != season || !r.SourceFinal || r.TeamID <= 0 || (r.TeamID != r.HomeTeamID && r.TeamID != r.VisitorTeamID) {
			continue
		}
		key := [2]int{r.GameID, r.PlayerID}
		if seen[key] {
			continue
		}
		seen[key] = true
		if r.Date >= latest[r.PlayerID] {
			ix.Players[r.PlayerID] = strings.TrimSpace(r.Pos)
			latest[r.PlayerID] = r.Date
		}
		if r.TeamAbbr != "" {
			ix.Abbr[r.TeamAbbr] = r.TeamID
		}
		ix.Games[r.GameID] = [2]int{r.HomeTeamID, r.VisitorTeamID}
		if games[r.GameID] == nil {
			games[r.GameID] = map[int]*box{}
		}
		b := games[r.GameID][r.TeamID]
		if b == nil {
			b = &box{team: r.TeamID, home: r.HomeTeamID, away: r.VisitorTeamID, hp: r.FinalHomeScore, ap: r.FinalAwayScore, complete: true}
			games[r.GameID][r.TeamID] = b
		}
		if !r.AnalyticsReady || r.FinalHomeScore != b.hp || r.FinalAwayScore != b.ap {
			b.complete = false
		}
		b.mins += r.Minutes
		known := strings.TrimSpace(r.Pos) != ""
		if known {
			b.known += r.Minutes
		}
		for k, s := range data.Stats {
			v := r.Value(s)
			b.vals[k] += v
			if known {
				b.pos[feat.PosGroup(r.Pos)][k] += v
			}
		}
	}
	totals := map[int]*tally{}
	for _, g := range games {
		for id, b := range g {
			opp := b.home
			if id == b.home {
				opp = b.away
			}
			o := g[opp]
			// Avoid partial downloads. Regulation and overtime both have at least 230 team-minutes.
			if o == nil || !b.complete || !o.complete || b.hp != o.hp || b.ap != o.ap || b.hp <= 0 || b.ap <= 0 || b.mins < 230 || o.mins < 230 || b.vals[0] < 50 || o.vals[0] < 50 {
				continue
			}
			expected, opExpected := b.hp, b.ap
			if id != b.home {
				expected, opExpected = b.ap, b.hp
			}
			if b.vals[0] != expected || o.vals[0] != opExpected {
				continue
			}
			t := totals[id]
			if t == nil {
				t = &tally{}
				totals[id] = t
			}
			t.n++
			for k := range b.vals {
				t.scored[k] += b.vals[k]
				t.allowed[k] += o.vals[k]
			}
			if o.known >= .95*o.mins {
				for p := 0; p < 5; p++ {
					t.posN[p]++
					for k := 0; k < 4; k++ {
						t.pos[p][k] += o.pos[p][k]
					}
				}
			}
		}
	}
	ids := []int{}
	for id, t := range totals {
		if t.n >= 3 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	ranked := func(id, k, p int, scoring bool) *Rank {
		type item struct {
			id, n int
			v     float64
		}
		items := []item{}
		for _, tid := range ids {
			t := totals[tid]
			n := t.n
			v := t.allowed[k]
			if scoring {
				v = t.scored[k]
			}
			if p >= 0 {
				n = t.posN[p]
				v = t.pos[p][k]
			}
			if n >= 3 {
				items = append(items, item{tid, n, v / float64(n)})
			}
		}
		if len(items) < 2 {
			return nil
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].v == items[j].v {
				return items[i].id < items[j].id
			}
			if scoring {
				return items[i].v > items[j].v
			}
			return items[i].v < items[j].v
		})
		for i, v := range items {
			if v.id == id {
				r := &Rank{Stat: string(data.Stats[k]), Rank: i + 1, Of: len(items), Games: v.n, Average: v.v}
				if p >= 0 {
					r.Position = feat.PosName(p)
				}
				return r
			}
		}
		return nil
	}
	for _, id := range ids {
		t := Team{Season: season, Before: before, Scoring: ranked(id, 0, -1, true)}
		for name, tid := range ix.Abbr {
			if tid == id {
				t.Name = name
			}
		}
		for k := 0; k < 4; k++ {
			if r := ranked(id, k, -1, false); r != nil {
				t.Allowed = append(t.Allowed, *r)
			}
		}
		ix.Teams[id] = t
		ix.Positions[id] = map[int][]Rank{}
		for p := 0; p < 5; p++ {
			for k := 0; k < 4; k++ {
				if r := ranked(id, k, p, false); r != nil {
					ix.Positions[id][p] = append(ix.Positions[id][p], *r)
				}
			}
		}
	}
	return ix
}
func (ix *Index) ForPlayer(pid int, opponent string) *Prop {
	opp := ix.Abbr[opponent]
	t, ok := ix.Teams[opp]
	if !ok {
		return nil
	}
	p := &Prop{Opponent: t}
	if pos := ix.Players[pid]; pos != "" {
		p.Position = ix.Positions[opp][feat.PosGroup(pos)]
	}
	return p
}
