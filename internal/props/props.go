// Package props parses sportsbook player-prop lines from BALLDONTLIE responses.
package props

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"

	"propsmodel/internal/data"
)

// Line is one sportsbook over/under line for one player and stat.
type Line struct {
	GameID    int
	PlayerID  int
	Vendor    string
	Stat      data.Stat
	Line      float64
	OverOdds  int // American odds, 0 if unknown
	UnderOdds int
	UpdatedAt string
	Row       int // which response row of the snapshot file the line came from (LoadAllLines only)
}

type apiProp struct {
	GameID    int             `json:"game_id"`
	PlayerID  int             `json:"player_id"`
	Vendor    string          `json:"vendor"`
	PropType  string          `json:"prop_type"`
	LineValue json.RawMessage `json:"line_value"`
	Market    struct {
		Type      string `json:"type"`
		OverOdds  int    `json:"over_odds"`
		UnderOdds int    `json:"under_odds"`
	} `json:"market"`
	UpdatedAt string `json:"updated_at"`
}

func parseFloatRaw(raw json.RawMessage) (float64, bool) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func statFor(propType string) (data.Stat, bool) {
	switch strings.ToLower(propType) {
	case "points":
		return data.Points, true
	case "rebounds":
		return data.Rebounds, true
	case "assists":
		return data.Assists, true
	case "threes":
		return data.Threes, true
	case "points_rebounds_assists":
		return data.PRA, true
	case "points_rebounds":
		return data.PR, true
	case "points_assists":
		return data.PA, true
	case "rebounds_assists":
		return data.RA, true
	}
	return "", false
}

// ParseResponse extracts the single-stat over/under lines (points, rebounds, assists, threes) from
// one API response body ({"data": [...]}). fallbackGame is used if a row has no game_id. Combined
// stats (points + rebounds + assists and so on) are left out here so nothing that indexes by single
// stat ever sees one; ParseCombos returns those.
func ParseResponse(body json.RawMessage, fallbackGame int) []Line {
	return parseResponse(body, fallbackGame, false)
}

// ParseCombos is ParseResponse for the combined stats only (PRA, PR, PA, RA).
func ParseCombos(body json.RawMessage, fallbackGame int) []Line {
	return parseResponse(body, fallbackGame, true)
}

func parseResponse(body json.RawMessage, fallbackGame int, combos bool) []Line {
	var resp struct {
		Data []apiProp `json:"data"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return nil
	}
	var out []Line
	for _, p := range resp.Data {
		stat, ok := statFor(p.PropType)
		if !ok || stat.IsCombo() != combos {
			continue
		}
		isOU := p.Market.Type == "over_under" || (p.Market.Type == "" && p.Market.OverOdds != 0)
		if !isOU {
			continue
		}
		line, ok := parseFloatRaw(p.LineValue)
		if !ok || p.PlayerID == 0 {
			continue
		}
		game := p.GameID
		if game == 0 {
			game = fallbackGame
		}
		out = append(out, Line{
			GameID:    game,
			PlayerID:  p.PlayerID,
			Vendor:    p.Vendor,
			Stat:      stat,
			Line:      line,
			OverOdds:  p.Market.OverOdds,
			UnderOdds: p.Market.UnderOdds,
			UpdatedAt: p.UpdatedAt,
		})
	}
	return out
}

func readLines(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	return sc.Err()
}

type lineKey struct {
	game, player int
	vendor       string
	stat         data.Stat
}

func impliedProb(odds int) float64 {
	if odds > 0 {
		return 100 / float64(odds+100)
	}
	if odds < 0 {
		return float64(-odds) / float64(-odds+100)
	}
	return 0
}

// mainLine picks the real posted line out of several lines one book listed for the same player and
// stat. Books list alternate lines too (a high or low number with a lopsided price); the main line is
// the one whose over and under prices are closest to even. Lines with a missing price lose to
// lines with both, and ties keep the first. A single line is returned as is.
func mainLine(c []Line) Line {
	best, bestGap := c[0], 2.0
	for _, l := range c {
		gap := 1.5
		if l.OverOdds != 0 && l.UnderOdds != 0 {
			gap = math.Abs(impliedProb(l.OverOdds) - impliedProb(l.UnderOdds))
		}
		if gap < bestGap {
			best, bestGap = l, gap
		}
	}
	return best
}

// LoadSnapshot reads a data/snapshots/props_DATE.jsonl file. Where a snapshot ran several
// times, the latest capture of each (game, player, stat, vendor) line wins. If a book listed several
// lines for the same market in one capture (alternate lines), the main line wins, not whichever came last.
func LoadSnapshot(path string) ([]Line, error) { return loadSnapshot(path, false) }

// LoadSnapshotCombos is LoadSnapshot for the combined stats only (PRA, PR, PA, RA).
func LoadSnapshotCombos(path string) ([]Line, error) { return loadSnapshot(path, true) }

func loadSnapshot(path string, combos bool) ([]Line, error) {
	latest := map[lineKey]Line{}
	var order []lineKey
	err := readLines(path, func(b []byte) {
		var row struct {
			GameID   int             `json:"game_id"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(b, &row) != nil {
			return
		}
		inRow := map[lineKey][]Line{}
		var rowOrder []lineKey
		for _, l := range parseResponse(row.Response, row.GameID, combos) {
			k := lineKey{l.GameID, l.PlayerID, l.Vendor, l.Stat}
			if _, seen := inRow[k]; !seen {
				rowOrder = append(rowOrder, k)
			}
			inRow[k] = append(inRow[k], l)
		}
		for _, k := range rowOrder {
			if _, seen := latest[k]; !seen {
				order = append(order, k)
			}
			latest[k] = mainLine(inRow[k])
		}
	})
	if err != nil {
		return nil, err
	}
	out := make([]Line, 0, len(order))
	for _, k := range order {
		out = append(out, latest[k])
	}
	return out, nil
}

// LoadAllLines returns every over/under line in a snapshot file exactly as the feed gave it, with no
// choosing between captures or between alternate lines. It exists for audits (cmd/audit).
func LoadAllLines(path string) ([]Line, error) {
	var out []Line
	row := 0
	err := readLines(path, func(b []byte) {
		var r struct {
			GameID   int             `json:"game_id"`
			Response json.RawMessage `json:"response"`
		}
		row++
		if json.Unmarshal(b, &r) != nil {
			return
		}
		for _, l := range ParseResponse(r.Response, r.GameID) {
			l.Row = row
			out = append(out, l)
		}
	})
	return out, err
}

// LoadOpening reads a data/opening_props_SEASON.jsonl file.
func LoadOpening(path string) ([]Line, error) {
	var out []Line
	err := readLines(path, func(b []byte) {
		var row struct {
			GameID   int             `json:"_game_id"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(b, &row) != nil {
			return
		}
		out = append(out, ParseResponse(row.Response, row.GameID)...)
	})
	return out, err
}
