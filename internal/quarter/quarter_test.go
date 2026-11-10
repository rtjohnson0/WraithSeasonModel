package quarter

import (
	"propsmodel/internal/data"
	"testing"
)

func integer(n int) *int { return &n }
func TestExtractUsesOnlyQ1AndPreservesZero(t *testing.T) {
	g := Game{ID: 9, Date: "2026-11-01", Home: 1, Away: 2, HomeQ1: integer(2), AwayQ1: integer(1), Status: "Final"}
	roster := []data.PlayerGame{{GameID: 9, PlayerID: 7, Name: "A Player", TeamID: 1, Pts: 30}, {GameID: 9, PlayerID: 8, Name: "B Player", TeamID: 2, Pts: 25}, {GameID: 9, PlayerID: 10, Name: "Zero Player", TeamID: 1, Pts: 20}}
	plays := []Play{{GameID: 9, Order: 1, Period: 1, Scoring: true, Points: integer(2), Participants: []int{7}, Text: "A Player makes shot"}, {GameID: 9, Order: 2, Period: 1, Scoring: true, Points: integer(1), Participants: []int{8}, Text: "B Player makes free throw"}, {GameID: 9, Order: 3, Period: 2, Scoring: true, Points: integer(3), Participants: []int{7}, Text: "A Player makes three"}}
	plays[0].Team.ID = 1
	plays[1].Team.ID = 2
	result, e := Extract(g, plays, roster, map[int]bool{7: true, 8: true, 10: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range result.Rows {
		if r.PlayerID == 7 && r.Points != 2 {
			t.Fatal("full-game or second-quarter points leaked")
		}
		if r.PlayerID == 10 && (r.Points != 0 || !r.Participated) {
			t.Fatal("confirmed zero lost")
		}
	}
	g.HomeQ1 = integer(3)
	if _, e = Extract(g, plays, roster, nil); e == nil {
		t.Fatal("incomplete quarter accepted")
	}
	g.HomeQ1 = integer(2)
	plays[0].Participants = []int{10}
	if _, e = Extract(g, plays, roster, nil); e == nil {
		t.Fatal("wrong scorer accepted")
	}
}
func TestCompareAndSettleCannotUseFullGame(t *testing.T) {
	rows := []Observation{{GameID: 1, PlayerID: 7, Date: "2026-11-01", Period: "1q", Points: 0, Participated: true}, {GameID: 2, PlayerID: 7, Date: "2026-11-02", Period: "game", Points: 30, Participated: true}, {GameID: 3, PlayerID: 7, Date: "2026-11-04", Period: "1q", Points: 10, Participated: true}}
	c := Compare(rows, 7, "2026-11-04", 3.5, []int{1, 2, 3})
	if c.N != 1 || c.Missing != 2 || *c.Mean != 0 || c.Under != 1 {
		t.Fatalf("mixed periods or future leak: %+v", c)
	}
	if Result(rows[0], "UNDER", 3.5) != "WIN" || Result(rows[1], "UNDER", 3.5) != "REVIEW" {
		t.Fatal("wrong-period settlement")
	}
	if Result(Observation{Period: "1q", Points: 3, Participated: true}, "OVER", 3) != "PUSH" {
		t.Fatal("push graded wrong")
	}
	if Result(Observation{Period: "1q", Points: 0, Participated: false}, "UNDER", 3.5) != "REVIEW" {
		t.Fatal("missing participation counted win")
	}
}
