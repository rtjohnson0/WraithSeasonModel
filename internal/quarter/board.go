package quarter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/props"
)

type Card struct {
	GameID          int        `json:"gameId"`
	PlayerID        int        `json:"playerId"`
	Player          string     `json:"player"`
	Matchup         string     `json:"matchup"`
	Period          string     `json:"period"`
	Stat            string     `json:"stat"`
	Book            string     `json:"book"`
	Line            float64    `json:"line"`
	OverOdds        int        `json:"overOdds"`
	UnderOdds       int        `json:"underOdds"`
	QuoteAt         string     `json:"quoteAt"`
	SourceUpdatedAt string     `json:"sourceUpdatedAt"`
	Tip             string     `json:"tip"`
	BookCount       int        `json:"bookCount"`
	Comparison      Comparison `json:"comparison"`
	Side            string     `json:"side,omitempty"`
	Estimate        *float64   `json:"estimate,omitempty"`
	BreakEven       float64    `json:"breakEven,omitempty"`
}
type Entry struct {
	Date     string  `json:"date"`
	GameID   int     `json:"game_id"`
	PlayerID int     `json:"player_id"`
	Player   string  `json:"player"`
	Period   string  `json:"period"`
	Stat     string  `json:"stat"`
	Side     string  `json:"side"`
	Book     string  `json:"book"`
	Line     float64 `json:"line"`
	Odds     int     `json:"odds"`
	LoggedAt string  `json:"logged_at"`
	Tip      string  `json:"tip"`
	QuoteAt  string  `json:"quote_at"`
	Result   string  `json:"result,omitempty"`
	Actual   *int    `json:"actual,omitempty"`
}

func (e Entry) ID() string {
	return fmt.Sprintf("%s|%d|%d|%s|%s|%s|%s|%g", e.Date, e.GameID, e.PlayerID, e.Period, e.Stat, e.Side, e.Book, e.Line)
}

type Record struct {
	Wins    int `json:"wins"`
	Losses  int `json:"losses"`
	Pushes  int `json:"pushes"`
	Review  int `json:"review"`
	Pending int `json:"pending"`
}
type Board struct {
	Date   string   `json:"date"`
	At     string   `json:"at"`
	Cards  []Card   `json:"cards"`
	Record Record   `json:"record"`
	Recent []Entry  `json:"recent"`
	Errors []string `json:"errors,omitempty"`
}

func LoadBoard(dir, date string) (*Board, error) {
	b, e := os.ReadFile(filepath.Join(dir, "quarterboard.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var v Board
	if e = json.Unmarshal(b, &v); e != nil {
		return nil, e
	}
	if v.Date != date {
		return nil, nil
	}
	return &v, nil
}
func LoadLog(path string) ([]Entry, error) {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var rows []Entry
	s := bufio.NewScanner(f)
	for s.Scan() {
		var r Entry
		if e = json.Unmarshal(s.Bytes(), &r); e != nil {
			return nil, e
		}
		if r.Period != "1q" || r.Stat != "points" {
			return nil, fmt.Errorf("invalid quarter log period/stat")
		}
		rows = append(rows, r)
	}
	return rows, s.Err()
}
func SaveLog(path string, rows []Entry) error {
	f, e := os.Create(path + ".tmp")
	if e != nil {
		return e
	}
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if e = enc.Encode(r); e != nil {
			f.Close()
			return e
		}
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
func BookCount(lines []props.Line) map[props.MarketKey]int {
	views := props.MarketViews(lines)
	out := map[props.MarketKey]int{}
	for k, v := range views {
		out[k] = v.Books
	}
	return out
}
