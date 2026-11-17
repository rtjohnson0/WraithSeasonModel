package main

import (
	"fmt"
	"math"
	"sort"

	"propsmodel/internal/picklog"
)

// outcome is one logged play, graded against what actually happened, with the likely reason.
type outcome struct {
	E        picklog.Entry
	Played   bool
	Actual   float64
	Minutes  float64
	Margin   float64 // actual game margin for the player's team
	HasMar   bool
	Result   string // WIN, LOSS, PUSH, VOID
	Category string
	Reason   string
}

// pastLine is how far the stat landed beyond the line in the direction we bet: positive is good.
func (o outcome) pastLine() float64 {
	if o.E.Side == "OVER" {
		return o.Actual - o.E.Line
	}
	return o.E.Line - o.Actual
}

// Units is the profit of the play at its logged stake, or at a flat 1u when it was never staked.
func (o outcome) Units() float64 {
	g := picklog.Grade(o.E, o.Actual, 0, o.Played)
	if o.E.Units > 0 {
		return g.Profit
	}
	return g.Flat
}

const (
	minutesFar  = 6.0  // minutes off projection that count as a minutes miss on their own
	blowoutGap  = 18.0 // final margin at which starters usually sit the fourth quarter
	farFromProj = 0.4  // share of the projection the stat can miss by before it is "the model was wrong"
)

// diagnose grades the play and explains the result in one line. The explanation is a diagnosis
// from the box score, not proof: it says which of the simple causes fits, in this order: the
// player did not play, minutes were far from the projection in a direction that hurt the bet,
// the miss was within a point or so of the line, the stat landed far from the projection, or
// plain variance.
func diagnose(o *outcome) {
	e := o.E
	if !o.Played {
		o.Result, o.Category = "VOID", "did_not_play"
		o.Reason = "Did not play (not in the box score), so the bet has no action. Check whether he was ruled out after the picks were made."
		return
	}
	o.Result = picklog.Grade(e, o.Actual, 0, true).Result

	d := o.Minutes - e.ProjMin
	minutesOff := e.ProjMin > 0 && (math.Abs(d) >= minutesFar || o.Minutes < 0.6*e.ProjMin)
	// Fewer minutes hurt an over; more minutes hurt an under.
	minutesHurt := minutesOff && ((e.Side == "OVER" && d < 0) || (e.Side == "UNDER" && d > 0))
	minNote := fmt.Sprintf("%.0f min vs %.0f projected", o.Minutes, e.ProjMin)
	past := o.pastLine()
	line := fmt.Sprintf("actual %.0f vs line %.1f (projected %.1f)", o.Actual, e.Line, e.ProjMean)

	if o.Result == "PUSH" {
		o.Category, o.Reason = "push", "Landed exactly on the line: "+line+"."
		return
	}

	if o.Result == "WIN" {
		switch {
		case minutesOff && !minutesHurt:
			o.Category = "hit_minutes_helped"
			o.Reason = fmt.Sprintf("Hit, helped by minutes (%s). %s.", minNote, line)
		case minutesHurt:
			o.Category = "hit_despite_minutes"
			o.Reason = fmt.Sprintf("Hit even though minutes went against it (%s). %s.", minNote, line)
		case past <= 1:
			o.Category = "hit_narrow"
			o.Reason = fmt.Sprintf("Hit narrowly, by %.1f; minutes on track (%s). %s.", past, minNote, line)
		default:
			o.Category = "hit_as_modeled"
			o.Reason = fmt.Sprintf("Hit by %.1f with minutes on track (%s). %s.", past, minNote, line)
		}
		return
	}

	// LOSS
	switch {
	case minutesHurt:
		o.Category = "minutes"
		why := "rotation or role change"
		if o.HasMar && math.Abs(o.Margin) >= blowoutGap {
			why = fmt.Sprintf("blowout (final margin %+.0f), so minutes were cut", o.Margin)
		} else if o.Minutes < 0.5*e.ProjMin {
			why = "left early or was limited (injury or foul trouble?)"
		}
		if d > 0 {
			why = "more minutes than expected (extra run from injuries, a close game or a role change)"
		}
		o.Reason = fmt.Sprintf("Minutes miss: %s: %s. %s.", minNote, why, line)
	case past > -1.0001 && past < 0:
		o.Category = "close_call"
		o.Reason = fmt.Sprintf("Missed by %.1f, within a point of the line; the read was fine and it went the other way. %s.", -past, line)
	case e.ProjMean > 0 && math.Abs(o.Actual-e.ProjMean) > farFromProj*math.Max(e.ProjMean, 3):
		o.Category = "projection_miss"
		perMin, projPerMin := o.Actual/math.Max(o.Minutes, 1), e.ProjMean/math.Max(e.ProjMin, 1)
		o.Reason = fmt.Sprintf("Projection miss with minutes on track (%s): produced %.2f per minute vs %.2f projected. %s. Role, matchup or usage was misread.",
			minNote, perMin, projPerMin, line)
	default:
		o.Category = "variance"
		o.Reason = fmt.Sprintf("Ordinary variance: the projection was close (%s) and the line sat inside normal game-to-game spread. %s.", line, minNote)
	}
}

var tierOrder = map[string]int{"PICK": 0, "": 0, "CHECK": 1, "PAPER": 2, "LEAN": 3}
var resultOrder = map[string]int{"WIN": 0, "LOSS": 1, "PUSH": 2, "VOID": 3}

// dedupe collapses the same player, stat and side logged more than once (a re-run, or another
// book's line) into one play: the highest tier, then the earliest logged. It returns how many
// duplicates it dropped, so one repeated pick does not count as several results.
func dedupe(in []picklog.Entry) ([]picklog.Entry, int) {
	best := map[string]picklog.Entry{}
	var order []string
	for _, e := range in {
		k := fmt.Sprintf("%s|%d|%s|%s", e.Date, e.PlayerID, e.Stat, e.Side)
		cur, ok := best[k]
		if !ok {
			best[k] = e
			order = append(order, k)
			continue
		}
		if tierOrder[e.Tier] < tierOrder[cur.Tier] || (tierOrder[e.Tier] == tierOrder[cur.Tier] && e.LoggedAt < cur.LoggedAt) {
			best[k] = e
		}
	}
	out := make([]picklog.Entry, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out, len(in) - len(out)
}

func sortOutcomes(os []outcome) {
	sort.SliceStable(os, func(i, j int) bool {
		a, b := os[i], os[j]
		if a.E.Date != b.E.Date {
			return a.E.Date > b.E.Date
		}
		if tierOrder[a.E.Tier] != tierOrder[b.E.Tier] {
			return tierOrder[a.E.Tier] < tierOrder[b.E.Tier]
		}
		if resultOrder[a.Result] != resultOrder[b.Result] {
			return resultOrder[a.Result] < resultOrder[b.Result]
		}
		return a.E.Prob > b.E.Prob
	})
}
