// Package opportunity stores supplemental full-game tracking and pregame lineup
// observations. It never fills an unavailable field with zero or uses same-day
// tracking to explain a pregame forecast.
package opportunity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"strings"
	"time"
)

type Observation struct {
	Player, Game, Team          int
	Date, Type, Range, Captured string
	Stats                       map[string]*float64
}
type Key struct {
	Player, Game int
	Type, Range  string
}
type signalKey struct {
	Player   int
	Date     string
	Baseline [data.NStats]float64
	AsOf     int64
}
type Index struct {
	cache    map[signalKey][]Signal
	AsOf     time.Time
	Rows     map[Key]Observation
	History  map[int][]data.PlayerGame
	Coverage map[string]int
	Rejected map[string]int
}

func Load(dir string, games []data.PlayerGame) (*Index, error) {
	ix := &Index{AsOf: time.Now(), cache: map[signalKey][]Signal{}, Rows: map[Key]Observation{}, History: map[int][]data.PlayerGame{}, Coverage: map[string]int{}, Rejected: map[string]int{}}
	identities := map[[2]int]data.PlayerGame{}
	for _, g := range games {
		if g.SourceFinal && g.BoxScoreComplete && g.Minutes > 0 && g.Minutes <= 70 {
			identities[[2]int{g.GameID, g.PlayerID}] = g

		}
	}
	for _, g := range identities {
		ix.History[g.PlayerID] = append(ix.History[g.PlayerID], g)
	}
	files, e := filepath.Glob(filepath.Join(dir, "context", "*.jsonl"))
	if e != nil {
		return nil, e
	}
	for _, p := range files {
		feed := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if n := strings.LastIndex(feed, "_"); n >= 0 {
			feed = feed[:n]
		}
		if feed == "lineup_stats" || feed == "matchups" {
			continue
		}
		f, e := os.Open(p)
		if e != nil {
			return nil, e
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 65536), 1<<24)
		for sc.Scan() {
			var w struct {
				At  string `json:"captured_at"`
				Row struct {
					Player int                 `json:"player_id"`
					Game   int                 `json:"game_id"`
					Team   int                 `json:"team_id"`
					Type   string              `json:"type"`
					Range  string              `json:"range"`
					Period *int                `json:"period"`
					Stats  map[string]*float64 `json:"stats"`
				} `json:"row"`
			}
			if json.Unmarshal(sc.Bytes(), &w) != nil {
				ix.Rejected["invalid JSON"]++
				continue
			}
			r := w.Row
			g, ok := identities[[2]int{r.Game, r.Player}]
			if !ok || r.Team != g.TeamID || (r.Period != nil && *r.Period != 0) {
				ix.Rejected["identity, final or period"]++
				continue
			}
			stamp, e := time.Parse(time.RFC3339Nano, w.At)
			if e != nil {
				ix.Rejected["capture timestamp"]++
				continue
			}
			typ := feed // file namespace separates tracking and shot-defense types
			k := Key{r.Player, r.Game, typ, r.Range}
			old, exists := ix.Rows[k]
			if exists {
				oldAt, _ := time.Parse(time.RFC3339Nano, old.Captured)
				if stamp.Before(oldAt) {
					continue
				}
			}
			if feed == "rebounding" {
				v := Observation{Stats: r.Stats}
				total, off, def := v.Value("reb_chances"), v.Value("oreb_chances"), v.Value("dreb_chances")
				if !math.IsNaN(total) && !math.IsNaN(off) && !math.IsNaN(def) && math.Abs(total-off-def) > 1e-6 {
					ix.Rejected["rebound chance total mismatch"]++
					continue
				}
			}
			ix.Rows[k] = Observation{r.Player, r.Game, r.Team, g.Date, typ, r.Range, w.At, r.Stats}
		}
		e = sc.Err()
		f.Close()
		if e != nil {
			return nil, fmt.Errorf("context %s: %w", p, e)
		}
	}
	for k := range ix.Rows {
		ix.Coverage[k.Type]++
	}
	return ix, nil
}
func (o Observation) Value(keys ...string) float64 {
	for _, k := range keys {
		if p, ok := o.Stats[k]; ok && p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) && *p >= 0 {
			return *p
		}
	}
	return math.NaN()
}
func (ix *Index) row(pid, gid int, typ, date string) (Observation, bool) {
	r, ok := ix.Rows[Key{Player: pid, Game: gid, Type: typ}]
	if !ok || r.Date >= date {
		return r, false
	}
	stamp, e := time.Parse(time.RFC3339Nano, r.Captured)
	return r, e == nil && (ix.AsOf.IsZero() || !stamp.After(ix.AsOf))
}
