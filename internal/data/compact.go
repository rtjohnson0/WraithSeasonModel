package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The daily refresh re-downloads the last few days and appends them, so the raw files collect
// duplicates (advanced stats re-fetch seven days every time and had passed GitHub's 100 MB limit
// after a few weeks). Compaction rewrites a file with one row per player and game. The loaders
// already keep only the latest row per player and game, so nothing they read changes.

type idPair struct{ player, game int }

// compactFile streams path, hands each line to pick (which returns the key, the line to keep and
// whether to keep it) and rewrites the file with the last row for each key, in first-seen order.
func compactFile(path string, pick func(line []byte) (idPair, []byte, bool)) (before, after int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	rows := map[idPair][]byte{}
	var order []idPair
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		before++
		k, line, ok := pick(sc.Bytes())
		if !ok {
			continue
		}
		if _, seen := rows[k]; !seen {
			order = append(order, k)
		}
		rows[k] = append([]byte(nil), line...)
	}
	err = sc.Err()
	f.Close()
	if err != nil {
		return before, 0, err
	}
	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return before, 0, err
	}
	w := bufio.NewWriter(out)
	for _, k := range order {
		w.Write(rows[k])
		w.WriteByte('\n')
	}
	if err = w.Flush(); err == nil {
		err = out.Close()
	} else {
		out.Close()
	}
	if err != nil {
		os.Remove(tmp)
		return before, 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return before, 0, err
	}
	return before, len(order), nil
}

// CompactAdvanced keeps one whole-game row per player and game with only the columns the model
// reads (the per-period rows are skipped by the loader anyway).
func CompactAdvanced(path string) (before, after int, err error) {
	type slim struct {
		Period     *int     `json:"period,omitempty"`
		Usage      *float64 `json:"usage_percentage,omitempty"`
		Pace       *float64 `json:"pace,omitempty"`
		AstPct     *float64 `json:"assist_percentage,omitempty"`
		RebPct     *float64 `json:"rebound_percentage,omitempty"`
		Touches    *float64 `json:"touches,omitempty"`
		Passes     *float64 `json:"passes,omitempty"`
		FTR        *float64 `json:"free_throw_attempt_rate,omitempty"`
		Pct3PA     *float64 `json:"pct_fga_3pt,omitempty"`
		RebChTotal *float64 `json:"rebound_chances_total,omitempty"`
		RebChOff   *float64 `json:"rebound_chances_off,omitempty"`
		SecAst     *float64 `json:"secondary_assists,omitempty"`
		FtAst      *float64 `json:"free_throw_assists,omitempty"`
		Player     struct {
			ID int `json:"id"`
		} `json:"player"`
		Game struct {
			ID int `json:"id"`
		} `json:"game"`
	}
	return compactFile(path, func(line []byte) (idPair, []byte, bool) {
		var s slim
		if json.Unmarshal(line, &s) != nil || s.Player.ID == 0 || s.Game.ID == 0 || (s.Period != nil && *s.Period != 0) {
			return idPair{}, nil, false
		}
		b, err := json.Marshal(s)
		if err != nil {
			return idPair{}, nil, false
		}
		return idPair{s.Player.ID, s.Game.ID}, b, true
	})
}

// CompactStats drops repeated rows (same player and game), keeping each line as it was.
func CompactStats(path string) (before, after int, err error) {
	return compactFile(path, func(line []byte) (idPair, []byte, bool) {
		var s struct {
			Player struct {
				ID int `json:"id"`
			} `json:"player"`
			Game struct {
				ID int `json:"id"`
			} `json:"game"`
		}
		if json.Unmarshal(line, &s) != nil || s.Player.ID == 0 || s.Game.ID == 0 {
			return idPair{}, nil, false
		}
		return idPair{s.Player.ID, s.Game.ID}, line, true
	})
}

// CompactDir compacts every stats_*.jsonl and advanced_*.jsonl file in dir and reports each.
func CompactDir(dir string) error {
	for _, kind := range []struct {
		glob string
		fn   func(string) (int, int, error)
	}{{"advanced_*.jsonl", CompactAdvanced}, {"stats_*.jsonl", CompactStats}} {
		files, _ := filepath.Glob(filepath.Join(dir, kind.glob))
		for _, p := range files {
			before, after, err := kind.fn(p)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			if before != after {
				fmt.Printf("compacted %s: %d rows to %d\n", filepath.Base(p), before, after)
			}
		}
	}
	return nil
}
