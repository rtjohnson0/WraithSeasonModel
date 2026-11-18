package opportunity

import (
	"bufio"
	"encoding/json"
	"os"
	"propsmodel/internal/props"
	"time"
)

type Starter struct {
	Position string `json:"position"`
	Player   struct {
		ID    int    `json:"id"`
		First string `json:"first_name"`
		Last  string `json:"last_name"`
	} `json:"player"`
}
type Projected struct {
	TeamAbbr string    `json:"teamAbbr"`
	ID       int       `json:"id"`
	Game     int       `json:"game_id"`
	Team     int       `json:"team_id"`
	Status   string    `json:"status"`
	Lineup   []Starter `json:"lineup"`
	Updated  string    `json:"updated_at"`
	Captured string    `json:"captured_at"`
}

// Only the pregame projected_lineups feed is eligible here. Actual starters from
// /lineups are post-start information and never substituted into this loader.
func LoadPregame(path string, games map[int]props.GameInfo, roster map[int]int, asof time.Time) (map[int]Projected, error) {
	out := map[int]Projected{}
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 65536), 1<<22)
	for sc.Scan() {
		var w struct {
			At  string    `json:"captured_at"`
			Row Projected `json:"row"`
		}
		if json.Unmarshal(sc.Bytes(), &w) != nil {
			continue
		}
		p := w.Row
		p.Captured = w.At
		g, ok := games[p.Game]
		if !ok || p.Team != g.Home && p.Team != g.Visitor || len(p.Lineup) != 5 || p.Status != "confirmed" && p.Status != "expected" {
			continue
		}
		tip, et := time.Parse(time.RFC3339Nano, g.Start)
		at, ea := time.Parse(time.RFC3339Nano, w.At)
		updated, eu := time.Parse(time.RFC3339Nano, p.Updated)
		if et != nil || ea != nil || eu != nil || !at.Before(tip) || !asof.Before(tip) || at.After(asof) || asof.Sub(at) > 30*time.Minute || updated.After(at) {
			continue
		}
		valid := true
		players := map[int]bool{}
		slots := map[string]bool{}
		for _, s := range p.Lineup {
			if s.Player.ID <= 0 || roster[s.Player.ID] != p.Team || players[s.Player.ID] || slots[s.Position] || s.Position != "PG" && s.Position != "SG" && s.Position != "SF" && s.Position != "PF" && s.Position != "C" {
				valid = false
			}
			players[s.Player.ID] = true
			slots[s.Position] = true
		}
		if p.Team == g.Home {
			p.TeamAbbr = g.HomeAbbr
		} else {
			p.TeamAbbr = g.VisitorAbbr
		}
		if !valid {
			continue
		}
		if old, ok := out[p.Team]; ok {
			oldAt, _ := time.Parse(time.RFC3339Nano, old.Captured)
			oldUpdated, _ := time.Parse(time.RFC3339Nano, old.Updated)
			if at.Before(oldAt) || updated.Before(oldUpdated) {
				continue
			}
		}
		out[p.Team] = p
	}
	return out, sc.Err()
}
