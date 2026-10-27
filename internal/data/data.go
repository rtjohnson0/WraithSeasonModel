// Package data loads the raw BALLDONTLIE stat files into simple per-player-game records.
package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Stat is one of the stats the model predicts.
type Stat string

const (
	Points   Stat = "points"
	Rebounds Stat = "rebounds"
	Assists  Stat = "assists"
	Threes   Stat = "threes" // three-pointers made
)

// Combined stats: sums of the single stats above, which books list as their own markets.
// They are not modelled stats (they have no StatIndex); internal/combo prices them from the
// single-stat model.
const (
	PRA Stat = "pra" // points + rebounds + assists
	PR  Stat = "pr"  // points + rebounds
	PA  Stat = "pa"  // points + assists
	RA  Stat = "ra"  // rebounds + assists
)

// Combos lists every combined stat.
var Combos = []Stat{PRA, PR, PA, RA}

// Parts returns the single stats a combined stat adds up (nil for a single stat).
func (s Stat) Parts() []Stat {
	switch s {
	case PRA:
		return []Stat{Points, Rebounds, Assists}
	case PR:
		return []Stat{Points, Rebounds}
	case PA:
		return []Stat{Points, Assists}
	case RA:
		return []Stat{Rebounds, Assists}
	}
	return nil
}

// IsCombo reports whether s is a combined stat.
func (s Stat) IsCombo() bool { return s.Parts() != nil }

// NStats is how many stats the model handles (the length of Stats).
const NStats = 4

// Stats lists every stat the model handles.
var Stats = []Stat{Points, Rebounds, Assists, Threes}

// PlayerGame is one player's box score line for one game they played.
type PlayerGame struct {
	PlayerID int
	Name     string
	TeamID   int
	TeamAbbr string // BOS, if the API row carried it (display only)
	GameID   int
	Date     string // YYYY-MM-DD
	Season   int    // start year, 2025 = the 2025-26 season
	Minutes  float64
	Pts      float64
	Reb      float64
	Ast      float64
	Fga      float64 // field goal attempts (0 if the API row did not say)
	Fgm      float64
	Fg3a     float64 // three-point attempts (0 if the API row did not say)
	Fg3m     float64 // three-pointers made
	Blk      float64 // blocks (0 if the API row did not say)
	Pos      string  // G, F, C or a mix such as G-F (empty if the API row did not say)

	Day           int // days since 1970-01-01, for date arithmetic
	HomeTeamID    int // 0 if the API row did not say
	VisitorTeamID int
}

// StatIndex maps a stat to 0, 1, 2, 3 (points, rebounds, assists, threes).
func StatIndex(s Stat) int {
	switch s {
	case Points:
		return 0
	case Rebounds:
		return 1
	case Assists:
		return 2
	}
	return 3
}

// DayNumber returns days since 1970-01-01 for a YYYY-MM-DD date (0 if it cannot be parsed).
func DayNumber(date string) int {
	t, err := time.Parse("2006-01-02", NormalizeDate(date))
	if err != nil {
		return 0
	}
	return int(t.Unix() / 86400)
}

// Value returns the player's total for one stat.
func (g PlayerGame) Value(s Stat) float64 {
	switch s {
	case Points:
		return g.Pts
	case Rebounds:
		return g.Reb
	case Assists:
		return g.Ast
	case Threes:
		return g.Fg3m
	}
	if parts := s.Parts(); parts != nil {
		var sum float64
		for _, p := range parts {
			sum += g.Value(p)
		}
		return sum
	}
	return 0
}

// NormalizeDate trims a timestamp such as 2025-10-21T23:30:00.000Z to 2025-10-21.
func NormalizeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// SeasonOf returns the season start year for a date: August to December belong to that
// year's season, January to July belong to the season that started the year before.
func SeasonOf(date string) int {
	if len(date) < 7 {
		return 0
	}
	y, err1 := strconv.Atoi(date[:4])
	m, err2 := strconv.Atoi(date[5:7])
	if err1 != nil || err2 != nil {
		return 0
	}
	if m >= 8 {
		return y
	}
	return y - 1
}

// ParseMinutes accepts the minute formats the API may use: 34, "34", "34:12", null, "".
func ParseMinutes(raw json.RawMessage) float64 {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		return 0
	}
	if mm, ss, ok := strings.Cut(s, ":"); ok {
		m, err1 := strconv.ParseFloat(mm, 64)
		sec, err2 := strconv.ParseFloat(ss, 64)
		if err1 != nil || err2 != nil {
			return 0
		}
		return m + sec/60
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

type rawStat struct {
	Min    json.RawMessage `json:"min"`
	Pts    *float64        `json:"pts"`
	Reb    *float64        `json:"reb"`
	Ast    *float64        `json:"ast"`
	Fga    *float64        `json:"fga"`
	Fgm    *float64        `json:"fgm"`
	Fg3a   *float64        `json:"fg3a"`
	Fg3m   *float64        `json:"fg3m"`
	Blk    *float64        `json:"blk"`
	Player struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		TeamID    int    `json:"team_id"`
		Position  string `json:"position"`
	} `json:"player"`
	Team struct {
		ID   int    `json:"id"`
		Abbr string `json:"abbreviation"`
	} `json:"team"`
	Game struct {
		ID            int    `json:"id"`
		Date          string `json:"date"`
		Season        int    `json:"season"`
		HomeTeamID    int    `json:"home_team_id"`
		VisitorTeamID int    `json:"visitor_team_id"`
	} `json:"game"`
}

func val(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// LoadStatsFile reads one JSONL stats file. Rows where the player did not play are dropped.
func LoadStatsFile(path string) ([]PlayerGame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []PlayerGame
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r rawStat
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		mins := ParseMinutes(r.Min)
		if mins <= 0 || r.Player.ID == 0 || r.Game.ID == 0 {
			continue
		}
		team := r.Team.ID
		if team == 0 {
			team = r.Player.TeamID
		}
		date := NormalizeDate(r.Game.Date)
		season := r.Game.Season
		if season == 0 {
			season = SeasonOf(date)
		}
		out = append(out, PlayerGame{
			PlayerID: r.Player.ID,
			Name:     strings.TrimSpace(r.Player.FirstName + " " + r.Player.LastName),
			TeamID:   team,
			TeamAbbr: r.Team.Abbr,
			GameID:   r.Game.ID,
			Date:     date,
			Season:   season,
			Minutes:  mins,
			Pts:      val(r.Pts),
			Reb:      val(r.Reb),
			Ast:      val(r.Ast),
			Fga:      val(r.Fga),
			Fgm:      val(r.Fgm),
			Fg3a:     val(r.Fg3a),
			Fg3m:     val(r.Fg3m),
			Blk:      val(r.Blk),
			Pos:      strings.TrimSpace(r.Player.Position),

			Day:           DayNumber(date),
			HomeTeamID:    r.Game.HomeTeamID,
			VisitorTeamID: r.Game.VisitorTeamID,
		})
	}
	return out, sc.Err()
}

// LoadStatsDir reads every stats_*.jsonl file in dir, removes duplicate (player, game)
// rows, and returns them sorted by date.
func LoadStatsDir(dir string) ([]PlayerGame, error) {
	files, err := filepath.Glob(filepath.Join(dir, "stats_*.jsonl"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no stats_*.jsonl files in %s; run: go run ./cmd/pull stats -seasons 2022,2023,2024,2025", dir)
	}
	type key struct{ p, g int }
	seen := map[key]PlayerGame{}
	for _, f := range files {
		rows, err := LoadStatsFile(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		for _, r := range rows {
			seen[key{r.PlayerID, r.GameID}] = r
		}
	}
	out := make([]PlayerGame, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out, nil
}
