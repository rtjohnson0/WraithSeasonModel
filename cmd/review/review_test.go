package main

import (
	"strings"
	"testing"

	"propsmodel/internal/picklog"
)

func entry(side string, line, proj, projMin float64) picklog.Entry {
	return picklog.Entry{Date: "2026-10-22", PlayerID: 1, Player: "A", Stat: "rebounds", Side: side, Line: line,
		ProjMean: proj, ProjMin: projMin, Prob: 0.6, Odds: -110, Units: 0.5, Tier: "PICK"}
}

func run(e picklog.Entry, actual, mins, margin float64) outcome {
	o := outcome{E: e, Played: true, Actual: actual, Minutes: mins, Margin: margin, HasMar: true}
	diagnose(&o)
	return o
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name        string
		o           outcome
		result, cat string
		reasonHas   string
	}{
		{"hit as modeled", run(entry("OVER", 7.5, 9, 30), 11, 31, 3), "WIN", "hit_as_modeled", "on track"},
		{"hit narrow", run(entry("OVER", 7.5, 9, 30), 8, 30, 3), "WIN", "hit_narrow", "narrowly"},
		{"blowout minutes", run(entry("OVER", 7.5, 9, 30), 4, 17, 28), "LOSS", "minutes", "blowout"},
		{"left early", run(entry("OVER", 7.5, 9, 30), 2, 9, 4), "LOSS", "minutes", "left early"},
		{"more minutes hurt an under", run(entry("UNDER", 7.5, 6, 24), 10, 35, 2), "LOSS", "minutes", "more minutes"},
		{"close call", run(entry("OVER", 7.5, 9, 30), 7, 30, 2), "LOSS", "close_call", "within a point"},
		{"projection miss", run(entry("OVER", 7.5, 9, 30), 2, 29, 2), "LOSS", "projection_miss", "misread"},
		{"variance", run(entry("OVER", 7.5, 9, 30), 6, 30, 2), "LOSS", "variance", "variance"},
		{"push", run(entry("OVER", 8, 9, 30), 8, 30, 2), "PUSH", "push", "exactly"},
		{"more minutes helped an over still hits", run(entry("OVER", 7.5, 9, 24), 11, 34, 2), "WIN", "hit_minutes_helped", "helped"},
		{"short minutes but under hits", run(entry("UNDER", 7.5, 8, 30), 4, 18, 25), "WIN", "hit_minutes_helped", "helped"},
	}
	for _, c := range cases {
		if c.o.Result != c.result || c.o.Category != c.cat {
			t.Errorf("%s: got %s/%s, want %s/%s (%s)", c.name, c.o.Result, c.o.Category, c.result, c.cat, c.o.Reason)
		}
		if !strings.Contains(c.o.Reason, c.reasonHas) {
			t.Errorf("%s: reason %q should mention %q", c.name, c.o.Reason, c.reasonHas)
		}
	}
}

func TestDidNotPlayIsVoid(t *testing.T) {
	o := outcome{E: entry("OVER", 7.5, 9, 30)}
	diagnose(&o)
	if o.Result != "VOID" || o.Category != "did_not_play" {
		t.Fatalf("got %s/%s", o.Result, o.Category)
	}
	if o.Units() != 0 {
		t.Fatalf("a void play should have no profit, got %v", o.Units())
	}
}

func TestUnitsStakedAndFlat(t *testing.T) {
	win := run(entry("OVER", 7.5, 9, 30), 11, 31, 3)
	if u := win.Units(); u < 0.45 || u > 0.46 { // 0.5u at -110
		t.Fatalf("staked win units = %v", u)
	}
	e := entry("OVER", 7.5, 9, 30)
	e.Units, e.Tier = 0, "PAPER"
	loss := run(e, 3, 30, 3)
	if loss.Units() != -1 {
		t.Fatalf("unstaked loss should be a flat -1u, got %v", loss.Units())
	}
}

func TestDedupeKeepsHighestTierThenEarliest(t *testing.T) {
	a := entry("OVER", 7.5, 9, 30)
	a.Tier, a.LoggedAt, a.Book = "LEAN", "2026-10-22T10:00:00Z", "x"
	b := a
	b.Tier, b.LoggedAt, b.Book = "PICK", "2026-10-22T15:00:00Z", "y"
	c := a
	c.Tier, c.LoggedAt, c.Book = "PICK", "2026-10-22T12:00:00Z", "z"
	other := entry("UNDER", 7.5, 9, 30)
	out, dropped := dedupe([]picklog.Entry{a, b, c, other})
	if dropped != 2 || len(out) != 2 {
		t.Fatalf("got %d kept, %d dropped", len(out), dropped)
	}
	if out[0].Book != "z" {
		t.Fatalf("expected the earliest PICK (book z), got %s", out[0].Book)
	}
}
