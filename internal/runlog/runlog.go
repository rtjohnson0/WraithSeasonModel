// Package runlog keeps the board from every run of the day (9 am, 3 pm, 6 pm, 8 pm), so a later
// run can say what it dropped and why, and the site can highlight plays that keep showing up.
// picklog is the permanent record of picks; this file is the history of how the board changed.
package runlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Play is one play on a run's board.
type Play struct {
	PlayerID int     `json:"player_id"`
	Player   string  `json:"player"`
	Team     string  `json:"team,omitempty"`
	GameID   int     `json:"game_id"`
	Matchup  string  `json:"matchup,omitempty"`
	Stat     string  `json:"stat"`
	Side     string  `json:"side"`
	Line     float64 `json:"line"`
	Tier     string  `json:"tier"`
	Prob     float64 `json:"prob"`
	Edge     float64 `json:"edge"`
	Units    float64 `json:"units,omitempty"`
	ProjMean float64 `json:"proj_mean,omitempty"`
	ProjMin  float64 `json:"proj_min,omitempty"`
}

// Key identifies a play across runs: the line is left out, so a play whose line moved is still
// the same play.
func (p Play) Key() string { return fmt.Sprintf("%d|%s|%s", p.PlayerID, p.Stat, p.Side) }

// Dropped is a play that was on the previous run's board and is not on this one.
type Dropped struct {
	Play
	Reason string `json:"reason"`
}

// TierChange is a play that stayed on the board but moved between tiers.
type TierChange struct {
	Play
	From   string `json:"from"`
	Reason string `json:"reason,omitempty"`
}

// Run is one run's board and what changed since the run before it that day.
type Run struct {
	Date    string       `json:"date"`
	At      string       `json:"at"`    // RFC3339 UTC
	Label   string       `json:"label"` // "9 AM", "3 PM", "6 PM" or "8 PM": which scheduled run this is
	Clock   string       `json:"clock"` // Eastern clock time, for display
	Prev    string       `json:"prev,omitempty"`
	Plays   []Play       `json:"plays"`
	Dropped []Dropped    `json:"dropped,omitempty"`
	Changed []TierChange `json:"changed,omitempty"`
}

// LabelFor names the scheduled run a time belongs to, on the Eastern clock. The schedule moves an
// hour earlier after the clocks go back, so the buckets are wide.
func LabelFor(t time.Time) (label, clock string) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.Local
	}
	et := t.In(loc)
	h := float64(et.Hour()) + float64(et.Minute())/60
	switch {
	case h < 12:
		label = "9 AM"
	case h < 16.5:
		label = "3 PM"
	case h < 19:
		label = "6 PM"
	default:
		label = "8 PM"
	}
	return label, et.Format("3:04 PM")
}

// Load reads every run. A missing file is empty.
func Load(path string) ([]Run, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Run
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var r Run
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// Append adds a run to the log.
func Append(path string, r Run) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// ForDate returns that day's runs, oldest first.
func ForDate(runs []Run, date string) []Run {
	var out []Run
	for _, r := range runs {
		if r.Date == date {
			out = append(out, r)
		}
	}
	return out
}

// Diff compares a run's plays with the previous run's. kept holds the current version of plays on
// both boards; dropped and added are the plays on only one of them.
func Diff(prev, cur []Play) (kept, added, dropped []Play) {
	was := map[string]Play{}
	for _, p := range prev {
		was[p.Key()] = p
	}
	now := map[string]bool{}
	for _, p := range cur {
		now[p.Key()] = true
		if _, ok := was[p.Key()]; ok {
			kept = append(kept, p)
		} else {
			added = append(added, p)
		}
	}
	for _, p := range prev {
		if !now[p.Key()] {
			dropped = append(dropped, p)
		}
	}
	return kept, added, dropped
}

// Appearances counts, for every play key, how many of the day's runs had it on the board and which
// runs (by label) they were.
func Appearances(runs []Run) (count map[string]int, seenIn map[string][]string) {
	count, seenIn = map[string]int{}, map[string][]string{}
	for _, r := range runs {
		seen := map[string]bool{}
		for _, p := range r.Plays {
			if seen[p.Key()] {
				continue
			}
			seen[p.Key()] = true
			count[p.Key()]++
			seenIn[p.Key()] = append(seenIn[p.Key()], r.Label)
		}
	}
	return count, seenIn
}
