package matchup

import (
	"fmt"
	"propsmodel/internal/data"
	"testing"
)

func TestRanksUsePastFinalGamesAndNeverGuessPosition(t *testing.T) {
	rows := []data.PlayerGame{}
	for n := 1; n <= 4; n++ {
		date := fmt.Sprintf("2026-11-%02d", n)
		for team := 1; team <= 2; team++ {
			rows = append(rows, data.PlayerGame{SourceFinal: true, AnalyticsReady: true, FinalHomeScore: 100, FinalAwayScore: 110, PlayerID: team, TeamID: team, TeamAbbr: map[int]string{1: "BOS", 2: "NYK"}[team], Pos: "C", HomeTeamID: 1, VisitorTeamID: 2, GameID: n, Date: date, Season: 2026, Minutes: 240, Pts: float64(90 + team*10), Reb: float64(30 + team*10), Ast: 20, Fg3m: 10})
		}
	}
	// This huge future game must not reverse the defense ranking or count in the sample.
	rows[6].Pts = 900
	ix := Build(rows, "2026-11-04")
	p := ix.ForPlayer(1, "NYK")
	if p == nil || p.Opponent.Scoring.Games != 3 || p.Opponent.Scoring.Average != 110 || len(p.Position) != 4 {
		t.Fatalf("%+v", p)
	}
	if p.Position[0].Rank != 1 || p.Position[0].Average != 100 {
		t.Fatal(p.Position)
	}
	for i := range rows {
		rows[i].Pos = ""
	}
	ix = Build(rows, "2026-11-04")
	p = ix.ForPlayer(1, "NYK")
	if p == nil || len(p.Position) != 0 {
		t.Fatal("unknown position guessed", p)
	}
	rows[0].SourceFinal = false
	ix = Build(rows, "2026-11-04")
	if len(ix.Teams) != 0 {
		t.Fatal("partial or unfinished box was used")
	}
}
