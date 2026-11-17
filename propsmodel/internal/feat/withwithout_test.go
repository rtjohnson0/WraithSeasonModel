package feat

import (
	"fmt"
	"testing"

	"propsmodel/internal/data"
)

// A teammate's absence should show up as a minutes gain for the player who covers for him, and
// only from games the teammate could have played in.
func TestWithWithout(t *testing.T) {
	var games []data.PlayerGame
	for i := 0; i < 20; i++ {
		date := fmt.Sprintf("2025-11-%02d", i+1)
		day := data.DayNumber(date)
		aPlays := i < 10 || i >= 15
		pMin, pPts := 30.0, 15.0
		if !aPlays {
			pMin, pPts = 38, 21
		}
		if aPlays {
			games = append(games, data.PlayerGame{PlayerID: 1, TeamID: 7, GameID: i + 1, Date: date, Day: day, Season: 2025, Minutes: 34, Pts: 20, Pos: "G"})
		}
		games = append(games, data.PlayerGame{PlayerID: 2, TeamID: 7, GameID: i + 1, Date: date, Day: day, Season: 2025, Minutes: pMin, Pts: pPts, Pos: "G"})
	}
	b := NewBuilder(games, nil)
	hist := b.history(2, data.DayNumber("2025-12-01"))
	dMin, dRate := b.withWithout(hist, 7, []rotMember{{pid: 1, min: 34}})
	// 5 games out against 15 in: raw gain 8 minutes, shrunk by 5/(5+5).
	if dMin < 3.9 || dMin > 4.1 {
		t.Fatalf("minutes change = %.2f, want about 4 (8 shrunk by half)", dMin)
	}
	if dRate[0] <= 0 {
		t.Fatalf("points rate should rise without him, got %.3f", dRate[0])
	}
	// An unknown teammate, or none, changes nothing.
	if m, _ := b.withWithout(hist, 7, []rotMember{{pid: 99}}); m != 0 {
		t.Fatalf("unknown teammate gave %.2f", m)
	}
	if m, _ := b.withWithout(hist, 7, nil); m != 0 {
		t.Fatalf("no absentees gave %.2f", m)
	}
}
