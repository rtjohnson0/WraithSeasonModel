package main

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/minutesaudit"
	"propsmodel/internal/props"
	"propsmodel/internal/team"
	"strings"
)

func captureAuditContext(b *feat.Builder, row *feat.Row, tm, pid int, statuses map[int]string, out map[int]string, c team.Consensus) *minutesaudit.Context {
	a := &minutesaudit.Context{}
	if props.IsUncertain(statuses[pid]) {
		a.OwnStatus = statuses[pid]
	}
	for _, id := range b.RotationOf(tm, row.G.Day) {
		if id == pid {
			continue
		}
		s := strings.ToLower(strings.TrimSpace(statuses[id]))
		if s == "out" {
			a.ConfirmedOut++
		}
		if strings.Contains(s, "questionable") {
			a.Questionable++
		}
		if strings.Contains(s, "doubtful") {
			a.Doubtful++
		}
		if _, ok := out[id]; ok {
			a.AssumedOut++
		}
	}
	for _, n := range []string{"returning", "b2b"} {
		i, _ := feat.FeatureIndex(n, data.Points)
		if i >= 0 && i < len(row.X) && !math.IsNaN(row.X[i]) && row.X[i] > 0 {
			if n == "returning" {
				a.Returning = true
			} else {
				a.B2B = true
			}
		}
	}
	if c.HasSpread {
		sp := math.Abs(c.SpreadHome)
		a.Spread = &sp
	}
	return a
}
