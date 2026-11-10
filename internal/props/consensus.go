package props

import (
	"math"
	"sort"
	"strings"
)

type MarketKey struct {
	Game, Player int
	Stat         string
}

func marketKeyFor(l Line) MarketKey { return MarketKey{l.GameID, l.PlayerID, string(l.Stat)} }

type BookQuote struct {
	Book       string  `json:"book"`
	Source     string  `json:"source"`
	Line       float64 `json:"line"`
	OverOdds   int     `json:"overOdds"`
	UnderOdds  int     `json:"underOdds"`
	CapturedAt string  `json:"capturedAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type MarketView struct {
	Median float64     `json:"median"`
	Books  int         `json:"books"`
	Quotes []BookQuote `json:"quotes"`
}

// ReconcileBooks quarantines a physical book when the two fresh feeds disagree
// on its threshold. A different line is not silently assigned to the other book.
func ReconcileBooks(lines []Line) ([]Line, int) {
	type sourceValues struct{ direct, sgo map[float64]bool }
	groups := map[lineKey]*sourceValues{}
	for _, l := range lines {
		k := lineKey{l.GameID, l.PlayerID, BookID(l.Vendor), l.Stat}
		g := groups[k]
		if g == nil {
			g = &sourceValues{map[float64]bool{}, map[float64]bool{}}
			groups[k] = g
		}
		if strings.HasPrefix(l.Vendor, "sgo/") {
			g.sgo[l.Line] = true
		} else {
			g.direct[l.Line] = true
		}
	}
	blocked := map[lineKey]bool{}
	for k, g := range groups {
		if len(g.direct) == 0 || len(g.sgo) == 0 {
			continue
		}
		same := len(g.direct) == len(g.sgo)
		for v := range g.direct {
			same = same && g.sgo[v]
		}
		if !same {
			blocked[k] = true
		}
	}
	var keep []Line
	for _, l := range lines {
		if !blocked[lineKey{l.GameID, l.PlayerID, BookID(l.Vendor), l.Stat}] {
			keep = append(keep, l)
		}
	}
	return UniqueBooks(keep), len(blocked)
}

func MarketViews(lines []Line) map[MarketKey]MarketView {
	groups := map[MarketKey][]Line{}
	for _, l := range UniqueBooks(lines) {
		k := marketKeyFor(l)
		groups[k] = append(groups[k], l)
	}
	out := map[MarketKey]MarketView{}
	for k, group := range groups {
		values := make([]float64, len(group))
		view := MarketView{Books: len(group)}
		for i, l := range group {
			values[i] = l.Line
			source := "BALLDONTLIE"
			if strings.HasPrefix(l.Vendor, "sgo/") {
				source = "SportsGameOdds"
			}
			view.Quotes = append(view.Quotes, BookQuote{Book: BookID(l.Vendor), Source: source, Line: l.Line, OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, CapturedAt: l.CapturedAt, UpdatedAt: l.UpdatedAt})
		}
		sort.Float64s(values)
		view.Median = values[len(values)/2]
		if len(values)%2 == 0 {
			view.Median = (values[len(values)/2-1] + values[len(values)/2]) / 2
		}
		sort.Slice(view.Quotes, func(i, j int) bool { return view.Quotes[i].Book < view.Quotes[j].Book })
		out[k] = view
	}
	return out
}

// FilterConsensus is a conservative posting rule, not proof that an outlier is
// erroneous. It refuses thin markets and large deviations instead of advertising
// them as unusually large edges. The original quotes remain in raw evidence.
func FilterConsensus(lines []Line, minBooks int, maxGap float64) ([]Line, int) {
	views := MarketViews(lines)
	var out []Line
	rejected := 0
	for _, l := range lines {
		view := views[marketKeyFor(l)]
		if view.Books < minBooks || (maxGap >= 0 && math.Abs(l.Line-view.Median) > maxGap+1e-9) {
			rejected++
			continue
		}
		out = append(out, l)
	}
	counts := map[MarketKey]int{}
	for _, l := range out {
		counts[marketKeyFor(l)]++
	}
	var confirmed []Line
	for _, l := range out {
		if counts[marketKeyFor(l)] < minBooks {
			rejected++
			continue
		}
		confirmed = append(confirmed, l)
	}
	return confirmed, rejected
}
