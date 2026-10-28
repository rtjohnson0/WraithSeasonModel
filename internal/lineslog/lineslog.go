// Package lineslog keeps the market record that the pick log cannot: every line the model scored,
// not only the plays it posted, graded against what happened.
//
// The pick log grades about 50 plays a day, so a two or three point edge takes weeks to show. The
// model scores about 1,900 lines a day. Logging one compact row per player and stat gives about 400
// graded rows a day, enough to test whether the model is calibrated at all, whether a situation (a
// teammate out, a cut in minutes, a line that moved) changes the result, and whether lines priced off
// the market earn money. Nothing here changes a pick.
//
// Two kinds of row:
//
//	Market   one per player and stat per day: the median line across books, the model's P(over) at
//	         that line, and the context (who is out, how the line moved). Rewritten on every run, so
//	         the file holds the latest run's view.
//	Off      one per book whose line sits a point or more from the median of 4+ books: the side that
//	         line favours, at that book's price. Appended, never rewritten, so a line that was
//	         corrected later in the day is still graded at the price it was offered.
package lineslog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
)

// Market is one player's stat for one day at the market's median line.
type Market struct {
	Date     string  `json:"date"`
	At       string  `json:"at"` // when the run that wrote it happened (RFC 3339)
	GameID   int     `json:"game"`
	PlayerID int     `json:"pid"`
	Player   string  `json:"player"`
	Team     string  `json:"team"`
	Stat     string  `json:"stat"`
	Tip      string  `json:"tip,omitempty"`
	Line     float64 `json:"line"` // median across books
	Books    int     `json:"books"`
	Lo       float64 `json:"lo"` // lowest and highest line any book listed
	Hi       float64 `json:"hi"`
	Open     float64 `json:"open,omitempty"` // the median at the day's first capture (0 when unknown)
	POver    float64 `json:"p_over"`         // the model's calibrated P(over) at the median line
	P2       float64 `json:"p2,omitempty"`   // the V2 challenger's blended P(over) at the same line (0 when V2 did not score it)
	Proj     float64 `json:"proj"`
	ProjMin  float64 `json:"proj_min"`
	Shift    float64 `json:"shift"` // projected minutes minus his normal minutes
	TeamOut  int     `json:"team_out"`
	OppOut   int     `json:"opp_out"`
	B2B      bool    `json:"b2b,omitempty"`
}

// Move is how far the median line travelled since the day's first capture (0 when unknown).
func (m Market) Move() float64 {
	if m.Open <= 0 {
		return 0
	}
	return m.Line - m.Open
}

// Off is one book's line that sits off the market, and the side it favours.
type Off struct {
	Date     string  `json:"date"`
	At       string  `json:"at"`
	GameID   int     `json:"game"`
	PlayerID int     `json:"pid"`
	Player   string  `json:"player"`
	Team     string  `json:"team"`
	Stat     string  `json:"stat"`
	Tip      string  `json:"tip,omitempty"`
	Book     string  `json:"book"`
	Side     string  `json:"side"` // Under at a high line, Over at a low one
	Line     float64 `json:"line"`
	Median   float64 `json:"median"`
	Books    int     `json:"books"`
	Odds     int     `json:"odds"`
	PSide    float64 `json:"p_side"` // the model's probability for that side at that book's line and price
	Edge     float64 `json:"edge"`
	Proj     float64 `json:"proj"`
}

// Gap is how far the book's line is from the median.
func (o Off) Gap() float64 {
	if o.Line > o.Median {
		return o.Line - o.Median
	}
	return o.Median - o.Line
}

// Agrees reports whether the model also leans to that side at that line.
func (o Off) Agrees() bool { return o.PSide >= 0.5 }

func dir(base string) string { return filepath.Join(base, "lines") }

func writeJSONL[T any](path string, rows []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		w.Write(append(b, '\n'))
	}
	return w.Flush()
}

func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r T
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out, sc.Err()
}

// SaveMarket writes the day's market rows to data/lines/market_DATE.jsonl. A row replaces the earlier
// one for the same game, player and stat, and rows from earlier runs that this run did not score are
// kept: the 8 pm run no longer lists the early games, but their lines still need grading.
func SaveMarket(base, date string, rows []Market) error {
	path := filepath.Join(dir(base), fmt.Sprintf("market_%s.jsonl", date))
	old, err := readJSONL[Market](path)
	if err != nil {
		return err
	}
	old = Dedupe(old)
	type key struct {
		game, pid int
		stat      string
	}
	fresh := map[key]bool{}
	for _, r := range rows {
		fresh[key{r.GameID, r.PlayerID, r.Stat}] = true
	}
	merged := append([]Market(nil), rows...)
	for _, r := range old {
		if !fresh[key{r.GameID, r.PlayerID, r.Stat}] {
			merged = append(merged, r)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		a, b := merged[i], merged[j]
		if a.GameID != b.GameID {
			return a.GameID < b.GameID
		}
		if a.PlayerID != b.PlayerID {
			return a.PlayerID < b.PlayerID
		}
		return a.Stat < b.Stat
	})
	return writeJSONL(path, merged)
}

// Dedupe keeps one row per day, game, player and stat: the latest run's. Two copies exist when a
// merge kept both sides of a conflicted file.
func Dedupe(ms []Market) []Market {
	type key struct {
		date, stat string
		game, pid  int
	}
	idx := map[key]int{}
	var out []Market
	for _, m := range ms {
		k := key{m.Date, m.Stat, m.GameID, m.PlayerID}
		if i, ok := idx[k]; ok {
			if m.At >= out[i].At {
				out[i] = m
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, m)
	}
	return out
}

// LoadMarkets reads every day's market file, oldest first.
func LoadMarkets(base string) ([]Market, error) {
	files, _ := filepath.Glob(filepath.Join(dir(base), "market_*.jsonl"))
	sort.Strings(files)
	var out []Market
	for _, f := range files {
		rs, err := readJSONL[Market](f)
		if err != nil {
			return nil, err
		}
		out = append(out, rs...)
	}
	return Dedupe(out), nil
}

func (o Off) key() string {
	return fmt.Sprintf("%s|%d|%s|%s|%.1f|%s", o.Date, o.PlayerID, o.Stat, o.Book, o.Line, o.Side)
}

// AppendOff adds off-market rows not already logged (same day, player, stat, book, line and side)
// and returns how many were new.
func AppendOff(base string, rows []Off) (int, error) {
	path := filepath.Join(dir(base), "offmarket.jsonl")
	existing, err := readJSONL[Off](path)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, e := range existing {
		seen[e.key()] = true
	}
	if err := os.MkdirAll(dir(base), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n := 0
	for _, r := range rows {
		if seen[r.key()] {
			continue
		}
		seen[r.key()] = true
		b, err := json.Marshal(r)
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

// LoadOff reads the off-market log.
func LoadOff(base string) ([]Off, error) {
	return readJSONL[Off](filepath.Join(dir(base), "offmarket.jsonl"))
}

// Actualer looks up what a player did in a game (false when he did not play or the game is not in the stats).
type Actualer func(gameID, playerID int) (data.PlayerGame, bool)

// GradedMarket is a market row with its result.
type GradedMarket struct {
	Market
	Actual float64
	Push   bool
	Over   bool
}

// GradeMarkets grades every market row whose game is final and where the player played.
func GradeMarkets(ms []Market, act Actualer) []GradedMarket {
	var out []GradedMarket
	for _, m := range ms {
		pg, ok := act(m.GameID, m.PlayerID)
		if !ok {
			continue
		}
		v := pg.Value(data.Stat(m.Stat))
		out = append(out, GradedMarket{Market: m, Actual: v, Push: v == m.Line, Over: v > m.Line})
	}
	return out
}

// CalBin is one band of the model's probability and what happened.
type CalBin struct {
	Label     string
	N         int
	Predicted float64 // mean model P(over)
	Actual    float64 // share that went over (pushes left out)
}

// CalBins groups rows by the model's P(over) at the median line. A calibrated model's Predicted
// matches Actual in every band.
func CalBins(gm []GradedMarket) []CalBin {
	edges := []float64{0, 0.40, 0.45, 0.50, 0.55, 0.60, 2}
	labels := []string{"under 40%", "40-45%", "45-50%", "50-55%", "55-60%", "60% and up"}
	var out []CalBin
	for i := range labels {
		var n, over int
		var p float64
		for _, g := range gm {
			if g.Push || g.POver < edges[i] || g.POver >= edges[i+1] {
				continue
			}
			n++
			p += g.POver
			if g.Over {
				over++
			}
		}
		if n > 0 {
			out = append(out, CalBin{labels[i], n, p / float64(n), float64(over) / float64(n)})
		}
	}
	return out
}

// Skill compares the model's Brier score with the Brier score of always saying the day's base rate
// (the share of all lines that went over). Positive means the model's probabilities carry
// information beyond "the median line is a coin flip with this lean". n is the rows used.
func Skill(gm []GradedMarket) (model, base float64, n int) {
	var overs int
	for _, g := range gm {
		if g.Push {
			continue
		}
		n++
		if g.Over {
			overs++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	rate := float64(overs) / float64(n)
	for _, g := range gm {
		if g.Push {
			continue
		}
		y := 0.0
		if g.Over {
			y = 1
		}
		model += (g.POver - y) * (g.POver - y)
		base += (rate - y) * (rate - y)
	}
	return model / float64(n), base / float64(n), n
}

// Cell is how one probability did on one stat when it leaned one way: every graded line where the
// probability was above 50% (an Over lean) or below it (an Under lean).
type Cell struct {
	Stat  string
	Side  string  // OVER or UNDER: the side the probability leaned to
	N     int     // graded lines (pushes left out)
	Hit   float64 // share of those leans that were right
	Brier float64 // mean squared error of the probability on those lines
	Base  float64 // the same for always saying the stat's overall Over rate
}

// LeanCells cuts the graded rows into stat and lean-side cells for the probability p picks out of a
// row (p returns 0 for a row that probability did not score; those rows are skipped). The base rate
// is each stat's Over rate over the same scored rows, so a cell beats it only when the probability
// carries information about which lines go over, not merely because the stat leans one way.
func LeanCells(gm []GradedMarket, p func(GradedMarket) float64) []Cell {
	type acc struct {
		n, hit      int
		brier, base float64
	}
	overRate := map[string]float64{}
	{
		n, over := map[string]int{}, map[string]int{}
		for _, g := range gm {
			if g.Push || p(g) <= 0 {
				continue
			}
			n[g.Stat]++
			if g.Over {
				over[g.Stat]++
			}
		}
		for st, c := range n {
			overRate[st] = float64(over[st]) / float64(c)
		}
	}
	cells := map[[2]string]*acc{}
	for _, g := range gm {
		pv := p(g)
		if g.Push || pv <= 0 || pv == 0.5 {
			continue
		}
		side := "OVER"
		if pv < 0.5 {
			side = "UNDER"
		}
		a := cells[[2]string{g.Stat, side}]
		if a == nil {
			a = &acc{}
			cells[[2]string{g.Stat, side}] = a
		}
		y := 0.0
		if g.Over {
			y = 1
		}
		a.n++
		if (side == "OVER") == g.Over {
			a.hit++
		}
		a.brier += (pv - y) * (pv - y)
		a.base += (overRate[g.Stat] - y) * (overRate[g.Stat] - y)
	}
	var out []Cell
	for _, st := range data.Stats {
		for _, side := range []string{"OVER", "UNDER"} {
			if a := cells[[2]string{string(st), side}]; a != nil && a.n > 0 {
				n := float64(a.n)
				out = append(out, Cell{Stat: string(st), Side: side, N: a.n, Hit: float64(a.hit) / n, Brier: a.brier / n, Base: a.base / n})
			}
		}
	}
	return out
}

// Slice is a situation and how the Over rate compared with what the model said.
type Slice struct {
	Name      string
	N         int
	OverRate  float64
	ModelOver float64
}

// Slices cuts the graded rows by situation. The Over rate against ModelOver shows where the model is
// off (too low everywhere a teammate is out, say); a slice only matters with a few hundred rows.
func Slices(gm []GradedMarket) []Slice {
	defs := []struct {
		name string
		in   func(GradedMarket) bool
	}{
		{"every line", func(GradedMarket) bool { return true }},
		{"a teammate is out", func(g GradedMarket) bool { return g.TeamOut > 0 }},
		{"no teammate out", func(g GradedMarket) bool { return g.TeamOut == 0 }},
		{"an opponent is out", func(g GradedMarket) bool { return g.OppOut > 0 }},
		{"back-to-back", func(g GradedMarket) bool { return g.B2B }},
		{"minutes cut 4+ below normal", func(g GradedMarket) bool { return g.Shift <= -4 }},
		{"minutes 4+ above normal", func(g GradedMarket) bool { return g.Shift >= 4 }},
		{"median line moved up 0.5+", func(g GradedMarket) bool { return g.Open > 0 && g.Move() >= 0.5 }},
		{"median line moved down 0.5+", func(g GradedMarket) bool { return g.Open > 0 && g.Move() <= -0.5 }},
		{"books disagree (high-low 2+)", func(g GradedMarket) bool { return g.Hi-g.Lo >= 2 }},
		{"12+ books", func(g GradedMarket) bool { return g.Books >= 12 }},
		{"under 8 books", func(g GradedMarket) bool { return g.Books < 8 }},
	}
	for _, st := range data.Stats {
		stat := string(st)
		defs = append(defs, struct {
			name string
			in   func(GradedMarket) bool
		}{stat, func(g GradedMarket) bool { return g.Stat == stat }})
	}
	var out []Slice
	for _, d := range defs {
		var n, over int
		var p float64
		for _, g := range gm {
			if g.Push || !d.in(g) {
				continue
			}
			n++
			p += g.POver
			if g.Over {
				over++
			}
		}
		if n > 0 {
			out = append(out, Slice{d.name, n, float64(over) / float64(n), p / float64(n)})
		}
	}
	return out
}

// GradedOff is an off-market row with its result at that book's line and price.
type GradedOff struct {
	Off
	Actual float64
	Result string  // WIN, LOSS or PUSH
	Flat   float64 // profit of one unit at the odds
}

// GradeOff grades each off-market line. The same player, stat and side on one day counts once, at the
// best price offered (several books often list the same outlier).
func GradeOff(os []Off, act Actualer) []GradedOff {
	best := map[string]Off{}
	var order []string
	for _, o := range os {
		k := fmt.Sprintf("%s|%d|%s|%s", o.Date, o.PlayerID, o.Stat, o.Side)
		cur, ok := best[k]
		if !ok {
			order = append(order, k)
		}
		if !ok || picklog.Payout(o.Odds) > picklog.Payout(cur.Odds) {
			best[k] = o
		}
	}
	var out []GradedOff
	for _, k := range order {
		o := best[k]
		pg, ok := act(o.GameID, o.PlayerID)
		if !ok {
			continue
		}
		v := pg.Value(data.Stat(o.Stat))
		g := GradedOff{Off: o, Actual: v}
		switch {
		case v == o.Line:
			g.Result = "PUSH"
		case (v > o.Line) == (o.Side == "OVER"):
			g.Result, g.Flat = "WIN", picklog.Payout(o.Odds)
		default:
			g.Result, g.Flat = "LOSS", -1
		}
		out = append(out, g)
	}
	return out
}

// OffRow is the record of one group of off-market lines.
type OffRow struct {
	Name      string
	N         int // graded, pushes included
	Wins      int
	Losses    int
	Pushes    int
	HitRate   float64
	Predicted float64 // mean model probability for the side
	Units     float64 // flat 1 unit per line at the odds
}

// ROI is units over the lines that were decided or pushed.
func (r OffRow) ROI() float64 {
	if r.N == 0 {
		return 0
	}
	return r.Units / float64(r.N)
}

// OffSummary groups the graded off-market lines: all of them, by side, by how far off the line was,
// by how many books agreed on the median, by whether the model agreed, and by stat.
func OffSummary(gs []GradedOff) []OffRow {
	defs := []struct {
		name string
		in   func(GradedOff) bool
	}{
		{"All off-market lines", func(GradedOff) bool { return true }},
		{"Unders at a high line", func(g GradedOff) bool { return g.Side == "UNDER" }},
		{"Overs at a low line", func(g GradedOff) bool { return g.Side == "OVER" }},
		{"A point off", func(g GradedOff) bool { return g.Gap() < 1.5 }},
		{"A point and a half or more off", func(g GradedOff) bool { return g.Gap() >= 1.5 }},
		{"4-7 books set the median", func(g GradedOff) bool { return g.Books < 8 }},
		{"8+ books set the median", func(g GradedOff) bool { return g.Books >= 8 }},
		{"Model agrees", func(g GradedOff) bool { return g.Agrees() }},
		{"Model does not agree", func(g GradedOff) bool { return !g.Agrees() }},
	}
	for _, st := range data.Stats {
		stat := string(st)
		defs = append(defs, struct {
			name string
			in   func(GradedOff) bool
		}{strings.Title(stat), func(g GradedOff) bool { return g.Stat == stat }})
	}
	var out []OffRow
	for _, d := range defs {
		r := OffRow{Name: d.name}
		var p float64
		for _, g := range gs {
			if !d.in(g) {
				continue
			}
			r.N++
			p += g.PSide
			r.Units += g.Flat
			switch g.Result {
			case "WIN":
				r.Wins++
			case "LOSS":
				r.Losses++
			default:
				r.Pushes++
			}
		}
		if r.N == 0 {
			continue
		}
		if r.Wins+r.Losses > 0 {
			r.HitRate = float64(r.Wins) / float64(r.Wins+r.Losses)
		}
		r.Predicted = p / float64(r.N)
		out = append(out, r)
	}
	return out
}
