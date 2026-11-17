package boardstate

import (
	"propsmodel/internal/picklog"
	"testing"
	"time"
)

func TestKeepPublishedStartedPlaysButDoNotRestoreDroppedUpcomingPicks(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-12T20:00:00Z")
	base := picklog.Entry{Date: "2026-11-12", GameID: 1, PlayerID: 7, Period: "game", Stat: "points", Side: "OVER", Line: 20.5, Odds: -110, Book: "dk", AuditVersion: 2, LoggedAt: "2026-11-12T18:00:00Z", QuoteAt: "2026-11-12T18:00:00Z", SourceUpdatedAt: "2026-11-12T17:59:00Z", Tip: "2026-11-12T19:00:00Z"}
	upcoming := base
	upcoming.GameID = 2
	upcoming.PlayerID = 8
	upcoming.Tip = "2026-11-13T00:00:00Z"
	otherDate := base
	otherDate.Date = "2026-11-11"
	otherDate.PlayerID = 9
	removed := base
	removed.GameID = 99
	removed.PlayerID = 10
	old := &Snapshot{Date: base.Date, Plays: []picklog.Entry{base, upcoming, otherDate, removed}}
	next := Snapshot{Date: base.Date, Games: []Game{{ID: 1, Tip: base.Tip}, {ID: 2, Tip: upcoming.Tip}}}
	got := PreserveStarted(next, old, now)
	if len(got.Plays) != 1 || got.Plays[0].PlayerID != 7 {
		t.Fatalf("invalid retained plays %+v", got.Plays)
	}
	if len(PreserveStarted(got, old, now).Plays) != 1 {
		t.Fatal("duplicate historical play")
	}
	next.Date = "2026-11-13"
	if len(PreserveStarted(next, old, now).Plays) != 0 {
		t.Fatal("previous slate restored")
	}
}
