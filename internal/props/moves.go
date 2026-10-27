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

// SeriesPoint is the market's median line for one player stat at one capture.
type SeriesPoint struct {
	At    string  `json:"at"` // capture time (RFC 3339)
	Line  float64 `json:"line"`
	Books int     `json:"books"`
}

// LineSeries reads a day's snapshot file and returns, for every player stat, the median line across
// books at each capture, oldest first. Each book's main line is used when it lists alternates.
func LineSeries(path string) (map[MoveKey][]SeriesPoint, error) {
	type capKey struct {
		at string
		MoveKey
	}
	byCapture := map[capKey]map[string][]Line{} // capture and market -> vendor -> that vendor's lines in the capture
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
			k := capKey{row.At, MoveKey{l.GameID, l.PlayerID, l.Stat}}
			if byCapture[k] == nil {
				byCapture[k] = map[string][]Line{}
			}
			byCapture[k][l.Vendor] = append(byCapture[k][l.Vendor], l)
		}
	})
	if err != nil {
		return nil, err
	}
	out := map[MoveKey][]SeriesPoint{}
	for k, vendors := range byCapture {
		var lines []float64
		for _, ls := range vendors {
			lines = append(lines, mainLine(ls).Line)
		}
		sort.Float64s(lines)
		n := len(lines)
		med := lines[n/2]
		if n%2 == 0 {
			med = (lines[n/2-1] + lines[n/2]) / 2
		}
		out[k.MoveKey] = append(out[k.MoveKey], SeriesPoint{At: k.at, Line: med, Books: n})
	}
	for k := range out {
		pts := out[k]
		sort.Slice(pts, func(i, j int) bool { return pts[i].At < pts[j].At })
		out[k] = pts
	}
	return out, nil
}
