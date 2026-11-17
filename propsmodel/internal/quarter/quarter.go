// Package quarter isolates 1Q observations and paper tracking from full-game models.
package quarter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"sort"
	"strings"
)

type Play struct {
	GameID       int    `json:"game_id"`
	Order        int    `json:"order"`
	Period       int    `json:"period"`
	Type         string `json:"type"`
	Text         string `json:"text"`
	Scoring      bool   `json:"scoring_play"`
	Points       *int   `json:"score_value"`
	Participants []int  `json:"participants"`
	Team         struct {
		ID int `json:"id"`
	} `json:"team"`
}
type Game struct {
	ID     int    `json:"id"`
	Date   string `json:"date"`
	Home   int    `json:"home_team_id"`
	Away   int    `json:"visitor_team_id"`
	HomeQ1 *int   `json:"home_q1"`
	AwayQ1 *int   `json:"visitor_q1"`
	Status string `json:"status"`
}
type Observation struct {
	GameID       int    `json:"gameId"`
	PlayerID     int    `json:"playerId"`
	Date         string `json:"date"`
	Period       string `json:"period"`
	Points       int    `json:"points"`
	Participated bool   `json:"participated"`
}
type GameData struct {
	GameID    int           `json:"gameId"`
	Date      string        `json:"date"`
	FetchedAt string        `json:"at"`
	Version   int           `json:"version"`
	Rows      []Observation `json:"rows"`
}

// Extract rejects incomplete, unassigned and conflicting scoring. All Q1 scoring
// must reconcile to the official quarter totals; later-quarter points are ignored.
func Extract(g Game, plays []Play, roster []data.PlayerGame, starters map[int]bool) (GameData, error) {
	out := GameData{GameID: g.ID, Date: data.NormalizeDate(g.Date), Version: 1}
	if g.HomeQ1 == nil || g.AwayQ1 == nil || !strings.EqualFold(g.Status, "Final") {
		return out, fmt.Errorf("missing final quarter totals")
	}
	names := map[int]string{}
	teams := map[int]int{}
	points := map[int]int{}
	seen := map[int]bool{}
	for _, r := range roster {
		if r.GameID == g.ID {
			names[r.PlayerID] = r.Name
			teams[r.PlayerID] = r.TeamID
			seen[r.PlayerID] = starters[r.PlayerID]
		}
	}
	if len(names) == 0 {
		return out, fmt.Errorf("no player roster")
	}
	sort.SliceStable(plays, func(i, j int) bool { return plays[i].Order < plays[j].Order })
	sums := map[int]int{}
	orders := map[int]string{}
	completed := false
	for _, p := range plays {
		if p.GameID != g.ID {
			return out, fmt.Errorf("wrong play game")
		}
		if p.Period > 1 {
			completed = true
		}
		if p.Period != 1 {
			continue
		}
		raw, _ := json.Marshal(p)
		if prev, ok := orders[p.Order]; ok {
			if prev != string(raw) {
				return out, fmt.Errorf("conflicting play order")
			}
			continue
		}
		orders[p.Order] = string(raw)
		if strings.Contains(strings.ToLower(p.Type), "end") && strings.Contains(strings.ToLower(p.Type), "period") {
			completed = true
		}
		for _, id := range p.Participants {
			if _, ok := names[id]; ok {
				seen[id] = true
			}
		}
		if !p.Scoring {
			continue
		}
		if p.Points == nil || *p.Points < 1 || *p.Points > 3 || len(p.Participants) == 0 {
			return out, fmt.Errorf("unassigned scoring")
		}
		id := p.Participants[0]
		name := names[id]
		if name == "" || teams[id] != p.Team.ID || !strings.HasPrefix(strings.ToLower(p.Text), strings.ToLower(name)+" ") {
			return out, fmt.Errorf("ambiguous scorer")
		}
		points[id] += *p.Points
		sums[p.Team.ID] += *p.Points
	}
	if !completed || sums[g.Home] != *g.HomeQ1 || sums[g.Away] != *g.AwayQ1 {
		return out, fmt.Errorf("quarter scoring did not reconcile")
	}
	for id := range names {
		out.Rows = append(out.Rows, Observation{g.ID, id, out.Date, "1q", points[id], seen[id]})
	}
	sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].PlayerID < out.Rows[j].PlayerID })
	return out, nil
}

type Comparison struct {
	N       int           `json:"n"`
	Mean    *float64      `json:"mean"`
	Over    int           `json:"over"`
	Under   int           `json:"under"`
	Push    int           `json:"push"`
	Missing int           `json:"missing"`
	History []Observation `json:"history"`
}

// Compare accepts Q1 rows only, strictly before the prediction date. Zeroes are
// retained for confirmed quarter participants; missing data never becomes zero.
func Compare(rows []Observation, player int, date string, line float64, expected []int) Comparison {
	c := Comparison{History: []Observation{}}
	byID := map[int]Observation{}
	for _, r := range rows {
		if r.Period == "1q" && r.PlayerID == player && r.Date < date {
			byID[r.GameID] = r
		}
	}
	sum := 0.0
	for _, id := range expected {
		r, ok := byID[id]
		if !ok {
			c.Missing++
			continue
		}
		if !r.Participated {
			continue
		}
		c.History = append(c.History, r)
		sum += float64(r.Points)
		if float64(r.Points) > line {
			c.Over++
		} else if float64(r.Points) < line {
			c.Under++
		} else {
			c.Push++
		}
	}
	c.N = len(c.History)
	if c.N > 0 {
		m := sum / float64(c.N)
		c.Mean = &m
	}
	return c
}
func Result(row Observation, side string, line float64) string {
	if row.Period != "1q" || !row.Participated || math.IsNaN(line) || math.IsInf(line, 0) || (side != "OVER" && side != "UNDER") {
		return "REVIEW"
	}
	if float64(row.Points) == line {
		return "PUSH"
	}
	if (side == "OVER" && float64(row.Points) > line) || (side == "UNDER" && float64(row.Points) < line) {
		return "WIN"
	}
	return "LOSS"
}
func Save(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	if e = os.WriteFile(path+".tmp", b, 0644); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
