package boardstate

import (
	"propsmodel/internal/props"
	"time"
)

// PreserveStarted keeps already-published pregame plays visible after tip. It
// never restores a removed upcoming pick or a game missing from today's slate.
func PreserveStarted(next Snapshot, previous *Snapshot, now time.Time) Snapshot {
	if previous == nil || previous.Date != next.Date {
		return next
	}
	started := map[int]bool{}
	for _, g := range next.Games {
		if t, e := time.Parse(time.RFC3339, g.Tip); e == nil && !t.After(now) {
			started[g.ID] = true
		}
	}
	seen := map[string]bool{}
	for _, p := range next.Plays {
		seen[p.ID()] = true
	}
	for _, p := range previous.Plays {
		if p.Date != next.Date || !started[p.GameID] || !p.Verified() || props.BeforeTip(p.Tip, now) || seen[p.ID()] {
			continue
		}
		next.Plays = append(next.Plays, p)
		seen[p.ID()] = true
	}
	return next
}
