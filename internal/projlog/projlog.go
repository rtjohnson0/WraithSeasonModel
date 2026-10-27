// Package projlog keeps what the model projected for EVERY player it scored on a run, not only the
// plays it picked. Picks are chosen where the model disagrees with the book, so grading only picks
// flatters or punishes the model in ways that hide its real bias. This file lets the next day's
// check compare projections with what happened for everyone who had a line.
package projlog

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// StatProj is one stat's numbers for a player.
type StatProj struct {
	Raw  float64 `json:"raw"`            // the model's mean before any scaling
	Proj float64 `json:"proj"`           // the mean used for the picks (after -anchor or -bias)
	Line float64 `json:"line,omitempty"` // median line across books
}

// Entry is one player on one day, from the latest run.
type Entry struct {
	Date     string              `json:"date"`
	At       string              `json:"at"`
	GameID   int                 `json:"game_id"`
	PlayerID int                 `json:"player_id"`
	Player   string              `json:"player"`
	Team     string              `json:"team,omitempty"`
	ProjMin  float64             `json:"proj_min"`
	AvgMin15 float64             `json:"avg_min15"`
	TypMin   float64             `json:"typ_min,omitempty"`
	B2B      bool                `json:"b2b,omitempty"`
	TeamOut  int                 `json:"team_out,omitempty"` // rotation teammates ruled out
	OppOut   int                 `json:"opp_out,omitempty"`  // rotation players ruled out on the other side
	Maybe    string              `json:"maybe,omitempty"`    // his own uncertain status
	Spread   float64             `json:"spread,omitempty"`   // size of the game's consensus spread (0 if unknown)
	Total    float64             `json:"total,omitempty"`    // the game's consensus total (0 if unknown)
	Stats    map[string]StatProj `json:"stats"`
}

// Key identifies an entry: one per player per day.
func (e Entry) Key() string { return e.Date + "|" + itoa(e.PlayerID) }

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Load reads the file (missing is empty). Later lines replace earlier ones for the same player and day.
func Load(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	byKey := map[string]Entry{}
	var order []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		k := e.Key()
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = e
	}
	out := make([]Entry, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out, sc.Err()
}

// Save merges new entries into the file: a player already logged for the day is replaced by the
// newer run's numbers, everything else is kept.
func Save(path string, add []Entry) error {
	cur, err := Load(path)
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, e := range cur {
		idx[e.Key()] = i
	}
	for _, e := range add {
		if i, ok := idx[e.Key()]; ok {
			cur[i] = e
		} else {
			idx[e.Key()] = len(cur)
			cur = append(cur, e)
		}
	}
	sort.SliceStable(cur, func(i, j int) bool { return cur[i].Date < cur[j].Date })
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range cur {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		w.Write(append(b, '\n'))
	}
	return w.Flush()
}
