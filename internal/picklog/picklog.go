// Package picklog keeps an append-only record of every pick the model posts, so results and
// closing line value can be measured honestly later. Entries are never rewritten.
package picklog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Entry is one posted pick.
type Entry struct {
	LoggedAt string  `json:"logged_at"`
	Date     string  `json:"date"`
	GameID   int     `json:"game_id"`
	PlayerID int     `json:"player_id"`
	Player   string  `json:"player"`
	Team     string  `json:"team,omitempty"`
	Stat     string  `json:"stat"`
	Side     string  `json:"side"` // OVER or UNDER
	Line     float64 `json:"line"`
	Odds     int     `json:"odds"`
	Book     string  `json:"book"`
	Prob     float64 `json:"prob"`     // calibrated probability of the chosen side
	ProbRaw  float64 `json:"prob_raw"` // before calibration
	ProjMean float64 `json:"proj_mean"`
	ProjMin  float64 `json:"proj_min"`
	Units    float64 `json:"units"`
	// Tier is PICK (posted), PAPER (a stat the backtest did not support staking), CHECK (cleared
	// the bar but the minutes projection needs a human look) or LEAN (fill-in play). Empty means PICK, for entries logged before tiers existed.
	Tier string `json:"tier,omitempty"`
	// Why is the model's own reasoning at pick time (defense, minutes change, who is out), kept so the
	// next-day review can show what the model thought next to what happened.
	Why string `json:"why,omitempty"`
	// Matchup (for example "CHI @ MIL") and Edge are kept for the website and the review.
	Matchup string `json:"matchup,omitempty"`
	// Tip is the game's scheduled tip-off (RFC3339 UTC), for showing the start time on the site.
	Tip  string  `json:"tip,omitempty"`
	Edge float64 `json:"edge,omitempty"`
}

func (e Entry) key() string {
	return fmt.Sprintf("%s|%d|%s|%s|%s|%.1f", e.Date, e.PlayerID, e.Stat, e.Side, e.Book, e.Line)
}

// Load reads every entry in the log. A missing file is an empty log.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// Append adds entries that are not already logged (same date, player, stat, side, book and
// line) and returns how many were new.
func Append(path string, entries []Entry) (int, error) {
	existing, err := Load(path)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, e := range existing {
		seen[e.key()] = true
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	n := 0
	for _, e := range entries {
		if seen[e.key()] {
			continue
		}
		seen[e.key()] = true
		e.LoggedAt = now
		b, err := json.Marshal(e)
		if err != nil {
			return n, err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
