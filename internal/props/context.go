package props

import (
	"encoding/json"
	"strings"
	"time"
)

// GameInfo says which teams play in a game and when.
type GameInfo struct {
	ID      int
	Home    int
	Visitor int
	Date    string
	// Team abbreviations such as BOS, for display (empty if the snapshot lacks them).
	HomeAbbr, VisitorAbbr string
	// Start is the scheduled tip-off as an RFC3339 UTC time, or empty when the feed has no time.
	Start string
}

// startTime picks the tip-off from the game's datetime or, for games not yet played, its status
// (which holds the start time). A bare date with no clock time is not a tip-off.
func startTime(datetime, status string) string {
	for _, v := range []string{datetime, status} {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			continue
		}
		if t.UTC().Hour() == 0 && t.UTC().Minute() == 0 && t.UTC().Second() == 0 {
			continue // midnight UTC is a date placeholder, not a start time
		}
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}

// LoadGames reads the games embedded in a data/snapshots/props_DATE.jsonl file (each row
// carries the game object that `pull snapshot` fetched alongside the props).
func LoadGames(path string) (map[int]GameInfo, error) {
	out := map[int]GameInfo{}
	err := readLines(path, func(b []byte) {
		var row struct {
			GameID int `json:"game_id"`
			Game   struct {
				ID       int    `json:"id"`
				Date     string `json:"date"`
				Datetime string `json:"datetime"`
				Status   string `json:"status"`
				Home     struct {
					ID   int    `json:"id"`
					Abbr string `json:"abbreviation"`
				} `json:"home_team"`
				Visitor struct {
					ID   int    `json:"id"`
					Abbr string `json:"abbreviation"`
				} `json:"visitor_team"`
				HomeID    int `json:"home_team_id"`
				VisitorID int `json:"visitor_team_id"`
			} `json:"game"`
		}
		if json.Unmarshal(b, &row) != nil {
			return
		}
		g := GameInfo{ID: row.GameID, Home: row.Game.Home.ID, Visitor: row.Game.Visitor.ID, Date: row.Game.Date,
			HomeAbbr: row.Game.Home.Abbr, VisitorAbbr: row.Game.Visitor.Abbr,
			Start: startTime(row.Game.Datetime, row.Game.Status)}
		if g.Home == 0 {
			g.Home = row.Game.HomeID
		}
		if g.Visitor == 0 {
			g.Visitor = row.Game.VisitorID
		}
		if g.ID == 0 {
			g.ID = row.Game.ID
		}
		if g.ID != 0 && g.Home != 0 && g.Visitor != 0 {
			out[g.ID] = g
		}
	})
	return out, err
}

// LoadOut reads a data/snapshots/injuries_DATE.jsonl file and returns the players whose latest
// status says they are out (any status containing "out", such as "Out" or "Out For Season").
// With includeDoubtful, "doubtful" counts too.
func LoadOut(path string, includeDoubtful bool) (map[int]string, error) {
	latest := map[int]string{}
	err := readLines(path, func(b []byte) {
		var row struct {
			Row struct {
				Status string `json:"status"`
				Player struct {
					ID int `json:"id"`
				} `json:"player"`
			} `json:"row"`
		}
		if json.Unmarshal(b, &row) != nil || row.Row.Player.ID == 0 {
			return
		}
		latest[row.Row.Player.ID] = row.Row.Status
	})
	out := map[int]string{}
	for id, st := range latest {
		s := strings.ToLower(st)
		if strings.Contains(s, "out") || (includeDoubtful && strings.Contains(s, "doubtful")) {
			out[id] = st
		}
	}
	return out, err
}

// LoadStatuses reads a data/snapshots/injuries_DATE.jsonl file and returns every listed player's
// latest status text ("Out", "Questionable", "Day-To-Day", ...), keyed by player id.
func LoadStatuses(path string) (map[int]string, error) {
	latest := map[int]string{}
	err := readLines(path, func(b []byte) {
		var row struct {
			Row struct {
				Status string `json:"status"`
				Player struct {
					ID int `json:"id"`
				} `json:"player"`
			} `json:"row"`
		}
		if json.Unmarshal(b, &row) != nil || row.Row.Player.ID == 0 {
			return
		}
		latest[row.Row.Player.ID] = row.Row.Status
	})
	return latest, err
}

// IsUncertain reports whether an injury status means the player may or may not play.
func IsUncertain(status string) bool {
	s := strings.ToLower(status)
	if strings.Contains(s, "out") {
		return false
	}
	for _, k := range []string{"question", "doubt", "day-to-day", "day to day", "game time", "gtd"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// LoadActiveTeams reads data/players_active.jsonl (from `pull players`) and returns each active
// player's current team id.
func LoadActiveTeams(path string) (map[int]int, error) {
	out := map[int]int{}
	err := readLines(path, func(b []byte) {
		var row struct {
			ID     int `json:"id"`
			TeamID int `json:"team_id"`
			Team   struct {
				ID int `json:"id"`
			} `json:"team"`
		}
		if json.Unmarshal(b, &row) != nil || row.ID == 0 {
			return
		}
		t := row.Team.ID
		if t == 0 {
			t = row.TeamID
		}
		if t != 0 {
			out[row.ID] = t
		}
	})
	return out, err
}
