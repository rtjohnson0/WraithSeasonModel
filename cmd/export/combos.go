package main

import (
	"math"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
)

// ComboRecord is the graded history of the combined-market plays (data/combosettled.jsonl). It is
// kept apart from the pick record: combos are tracked, never staked.
type ComboRecord struct {
	Since   string   `json:"since,omitempty"`
	Until   string   `json:"until,omitempty"`
	Days    int      `json:"days"`
	Rows    []Row    `json:"rows,omitempty"`
	Buckets []Bucket `json:"buckets,omitempty"`
	Recent  []Result `json:"recent,omitempty"`
}

// buildComboRecord summarises the settled combo plays. The same player, combo and side on one day
// counts once. Units are a flat 1 unit per play, for comparison only.
func buildComboRecord(settled []picklog.Settled, recentN int) ComboRecord {
	every, _ := picklog.DedupeSettled(settled)
	var rec ComboRecord
	if len(every) == 0 {
		return rec
	}
	days := map[string]bool{}
	for _, r := range every {
		days[r.Date] = true
		if rec.Since == "" || r.Date < rec.Since {
			rec.Since = r.Date
		}
		if r.Date > rec.Until {
			rec.Until = r.Date
		}
	}
	rec.Days = len(days)
	add := func(name string, keep func(picklog.Settled) bool) {
		var sub []picklog.Settled
		for _, r := range every {
			if keep(r) {
				sub = append(sub, r)
			}
		}
		if len(sub) == 0 {
			return
		}
		x := row(name, sub)
		x.Units = picklog.Summarize(sub).Flat
		rec.Rows = append(rec.Rows, x)
	}
	add("All combos", func(picklog.Settled) bool { return true })
	for _, st := range data.Combos {
		stat := string(st)
		add(stat, func(r picklog.Settled) bool { return r.Stat == stat })
	}
	for _, side := range []string{"OVER", "UNDER"} {
		s := side
		add(s, func(r picklog.Settled) bool { return r.Side == s })
	}
	for _, b := range []struct {
		label  string
		lo, hi float64
	}{{"53-55%", 0, 0.55}, {"55-58%", 0.55, 0.58}, {"58% and up", 0.58, 2}} {
		var w, l int
		var p float64
		for _, r := range every {
			if r.Prob < b.lo || r.Prob >= b.hi || (r.Result != "WIN" && r.Result != "LOSS") {
				continue
			}
			if r.Result == "WIN" {
				w++
			} else {
				l++
			}
			p += r.Prob
		}
		if n := w + l; n > 0 {
			rec.Buckets = append(rec.Buckets, Bucket{Label: b.label, Picks: n, Predicted: p / float64(n), Actual: float64(w) / float64(n)})
		}
	}
	sorted := append([]picklog.Settled(nil), every...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Date != sorted[j].Date {
			return sorted[i].Date > sorted[j].Date
		}
		return sorted[i].Prob > sorted[j].Prob
	})
	if len(sorted) > recentN {
		sorted = sorted[:recentN]
	}
	for _, r := range sorted {
		res := Result{Date: r.Date, Player: r.Player, Team: r.Team, Matchup: matchupOf(r.Entry), Stat: r.Stat, Side: r.Side,
			Line: r.Line, Result: r.Result, Tier: "COMBO", Prob: r.Prob, Label: "Tracking", Proj: r.ProjMean}
		if r.Result != "VOID" && !math.IsNaN(r.Actual) {
			res.Actual = r.Actual
		}
		rec.Recent = append(rec.Recent, res)
	}
	return rec
}
