package main

import (
	"fmt"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/team"
	"testing"
)

func TestAuditContextKeepsQuestionableSeparateFromConfirmedOut(t *testing.T) {
	games := []data.PlayerGame{}
	for n := 1; n <= 10; n++ {
		for pid := 1; pid <= 3; pid++ {
			date := fmt.Sprintf("2026-11-%02d", n)
			games = append(games, data.PlayerGame{PlayerID: pid, TeamID: 1, GameID: n, Date: date, Day: data.DayNumber(date), Season: 2026, Minutes: 30, Pts: 10, Pos: "G"})
		}
	}
	b := feat.NewBuilder(games, nil)
	r := &feat.Row{G: data.PlayerGame{PlayerID: 1, TeamID: 1, Day: data.DayNumber("2026-11-11")}, X: make([]float64, len(feat.CommonSpecs))}
	c := captureAuditContext(b, r, 1, 1, map[int]string{2: "Questionable", 3: "Out"}, map[int]string{2: "assumed out", 3: "Out"}, team.Consensus{HasSpread: true, SpreadHome: -11.5})
	if c.Questionable != 1 || c.ConfirmedOut != 1 || c.AssumedOut != 2 || c.Spread == nil || *c.Spread != 11.5 {
		t.Fatal(c)
	}
}
