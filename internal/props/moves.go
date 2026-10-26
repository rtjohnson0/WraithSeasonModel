package props

import (
	"encoding/json"
	"sort"

	"propsmodel/internal/data"
)

// MoveKey identifies one player's stat in one game.
type MoveKey struct {
	Game, Player int
	Stat         data.Stat
}

// Move is how the market's line for one player stat changed between the first and the latest
// capture in a day's snapshot file.
type Move struct {
	From, To        float64 // median line across books at the first and the latest capture
	Books           int     // books with a line at both captures
	Up, Down        int     // books whose own line went up or down
	FirstAt, LastAt string  // capture times (RFC 3339)
}

// Moved reports whether the market line itself changed or most books changed their own line.
func (m Move) Moved() bool {
	return m.Books >= 3 && (m.To != m.From || m.Up+m.Down >= (m.Books+1)/2)
}

// LineMoves reads a data/snapshots/props_DATE.jsonl file and reports, for every player stat that was
// captured more than once, how the lines moved from the first capture to the latest. A snapshot
// that ran only once has no movement. It says nothing about why a line moved: the feed has no
// bet counts, so a move caused by news, by one-sided betting and by a sharp bettor look the same.
func LineMoves(path string) (map[MoveKey]Move, error) {
	type bookKey struct {
		MoveKey
		vendor string
	}
	type point struct {
		at   string
		line float64
	}
	first := map[bookKey]point{}
	last := map[bookKey]point{}
	err := readLines(path, func(b []byte) {
		var row struct {
			At       string          `json:"captured_at"`
			GameID   int             `json:"game_id"`
			Response json.RawMessage `json:"response"`
		}
		if json.Unmarshal(b, &row) != nil || row.At == "" {
			return
		}
		for _, l := range ParseResponse(row.Response, row.GameID) {
			k := bookKey{MoveKey{l.GameID, l.PlayerID, l.Stat}, l.Vendor}
			p := point{row.At, l.Line}
			if f, ok := first[k]; !ok || p.at < f.at {
				first[k] = p
			}
			if x, ok := last[k]; !ok || p.at >= x.at {
				last[k] = p
			}
		}
	})
	if err != nil {
		return nil, err
	}
	type acc struct {
		from, to        []float64
		up, down        int
		firstAt, lastAt string
	}
	byStat := map[MoveKey]*acc{}
	for k, f := range first {
		l := last[k]
		if l.at == f.at {
			continue // captured once: no movement to measure
		}
		a := byStat[k.MoveKey]
		if a == nil {
			a = &acc{firstAt: f.at, lastAt: l.at}
			byStat[k.MoveKey] = a
		}
		a.from = append(a.from, f.line)
		a.to = append(a.to, l.line)
		switch {
		case l.line > f.line:
			a.up++
		case l.line < f.line:
			a.down++
		}
		if f.at < a.firstAt {
			a.firstAt = f.at
		}
		if l.at > a.lastAt {
			a.lastAt = l.at
		}
	}
	median := func(v []float64) float64 {
		s := append([]float64(nil), v...)
		sort.Float64s(s)
		return s[len(s)/2]
	}
	out := make(map[MoveKey]Move, len(byStat))
	for k, a := range byStat {
		out[k] = Move{From: median(a.from), To: median(a.to), Books: len(a.from), Up: a.up, Down: a.down, FirstAt: a.firstAt, LastAt: a.lastAt}
	}
	return out, nil
}
