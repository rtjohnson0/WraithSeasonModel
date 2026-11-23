package main

import (
	"math"
	"propsmodel/internal/replay"
	"sort"
	"strings"
)

type SelectionSlice struct {
	Stat             string  `json:"stat"`
	Side             string  `json:"side"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason"`
	N                int     `json:"n"`
	Days             int     `json:"days"`
	Wins             int     `json:"wins"`
	Predicted        float64 `json:"predicted"`
	MinutesComponent float64 `json:"minutesComponent"`
	RateComponent    float64 `json:"rateComponent"`
}
type selectionAggregate struct {
	row   SelectionSlice
	dates map[string]bool
}
type selectionAudit map[string]*selectionAggregate

func (a selectionAudit) observe(r replay.Record, actual, minutes, projection, projectedMinutes float64) {
	side, prob := "OVER", r.P1
	if prob < .5 {
		side, prob = "UNDER", 1-prob
	}
	status, reason := "unknown", "historical selection not captured"
	if s := r.Selection; s != nil && s.Revision == "exact-quote-selection-v1" && s.Side == side {
		switch s.Status {
		case "posted", "excluded", "alternate-quote":
			status = s.Status
			reason = s.Reason
		}
	}
	key := strings.Join([]string{string(r.Stat), side, status, reason}, "/")
	g := a[key]
	if g == nil {
		g = &selectionAggregate{row: SelectionSlice{Stat: string(r.Stat), Side: side, Status: status, Reason: reason}, dates: map[string]bool{}}
		a[key] = g
	}
	g.row.N++
	g.dates[r.Date] = true
	g.row.Predicted += prob
	if (side == "OVER" && actual > r.Line) || (side == "UNDER" && actual < r.Line) {
		g.row.Wins++
	}
	if projectedMinutes > 0 {
		delta := (minutes - projectedMinutes) * projection / projectedMinutes
		g.row.MinutesComponent += delta
		g.row.RateComponent += actual - projection - delta
	}
}
func (a selectionAudit) snapshot() []SelectionSlice {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []SelectionSlice
	for _, k := range keys {
		g := a[k]
		s := g.row
		s.Days = len(g.dates)
		n := float64(s.N)
		s.Predicted /= n
		s.MinutesComponent /= n
		s.RateComponent /= n
		if !math.IsNaN(s.Predicted) {
			out = append(out, s)
		}
	}
	return out
}
