package props

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"time"
)

// Schedule is saved independently of sportsbook coverage, after a complete
// successful schedule response. A zero-game day is also an explicit snapshot.
type Schedule struct {
	Date       string     `json:"date"`
	CapturedAt string     `json:"captured_at"`
	Games      []GameInfo `json:"games"`
}

func SaveSchedule(dir, date, captured string, raw []json.RawMessage) error {
	s := Schedule{Date: date, CapturedAt: captured, Games: []GameInfo{}}
	seen := map[int]bool{}
	for _, body := range raw {
		var g struct {
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
		}
		if e := json.Unmarshal(body, &g); e != nil {
			return e
		}
		home, away := g.Home.ID, g.Visitor.ID
		if home == 0 {
			home = g.HomeID
		}
		if away == 0 {
			away = g.VisitorID
		}
		if g.ID <= 0 || home <= 0 || away <= 0 || home == away || data.NormalizeDate(g.Date) != date {
			return fmt.Errorf("schedule row does not match date or game/team identity")
		}
		if seen[g.ID] {
			return fmt.Errorf("duplicate game in schedule")
		}
		seen[g.ID] = true
		s.Games = append(s.Games, GameInfo{ID: g.ID, Date: date, Home: home, Visitor: away, HomeAbbr: g.Home.Abbr, VisitorAbbr: g.Visitor.Abbr, Start: startTime(g.Datetime, g.Status)})
	}
	if _, e := time.Parse(time.RFC3339, captured); e != nil {
		return e
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", " ")
	if e != nil {
		return e
	}
	path := filepath.Join(dir, "schedule_"+date+".json")
	f, e := os.CreateTemp(dir, "schedule-*.tmp")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	// os.Rename replaces the destination on supported local platforms, including
	// Go's Windows implementation. Never publish a partially written schedule.
	return os.Rename(temp, path)
}

// LoadSlateGames prefers the complete schedule to quote-derived game coverage.
// Missing schedules retain a labeled partial-data fallback for older installs.
func LoadSlateGames(propsPath, date string, asOf time.Time) (map[int]GameInfo, bool, string, error) {
	quoted, e := LoadGames(propsPath)
	if e != nil && !os.IsNotExist(e) {
		return nil, false, "", e
	}
	if quoted == nil {
		quoted = map[int]GameInfo{}
	}
	path := filepath.Join(filepath.Dir(propsPath), "schedule_"+date+".json")
	body, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return quoted, false, "", nil
	}
	if e != nil {
		return nil, false, "", e
	}
	var s Schedule
	if e = json.Unmarshal(body, &s); e != nil {
		return nil, false, "", e
	}
	at, e := time.Parse(time.RFC3339, s.CapturedAt)
	if e != nil || at.After(asOf) || s.Date != date || s.Games == nil {
		return nil, false, "", fmt.Errorf("schedule capture/date is invalid")
	}
	result := map[int]GameInfo{}
	for _, g := range s.Games {
		if g.ID <= 0 || g.Home <= 0 || g.Visitor <= 0 || g.Home == g.Visitor || data.NormalizeDate(g.Date) != date {
			return nil, false, "", fmt.Errorf("schedule identity is invalid")
		}
		if _, exists := result[g.ID]; exists {
			return nil, false, "", fmt.Errorf("duplicate schedule identity")
		}
		if q, ok := quoted[g.ID]; ok && q.Home == g.Home && q.Visitor == g.Visitor {
			if g.Start == "" {
				g.Start = q.Start
			}
			if g.HomeAbbr == "" {
				g.HomeAbbr = q.HomeAbbr
			}
			if g.VisitorAbbr == "" {
				g.VisitorAbbr = q.VisitorAbbr
			}
		}
		result[g.ID] = g
	}
	return result, asOf.Sub(at) <= 24*time.Hour, s.CapturedAt, nil
}
