package gamecontext

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"reflect"
	"testing"
	"time"
)

func TestPregameSpreadsLearnBoundedWorkloadEffectWithoutFutureFinals(t *testing.T) {
	var rows []data.PlayerGame
	lines := map[[2]int]feat.GameLine{}
	start, _ := time.Parse("2006-01-02", "2026-08-01")
	for day := 0; day < 60; day++ {
		date := start.AddDate(0, 0, day).Format("2006-01-02")
		spread := 2.
		if day%2 == 0 {
			spread = 14
		}
		lines[[2]int{data.DayNumber(date), 1}] = feat.GameLine{Spread: -spread}
		for player := 1; player <= 30; player++ {
			minutes := 34.
			if spread > 10 {
				minutes = 29
			}
			rows = append(rows, data.PlayerGame{GameID: day + 1, PlayerID: player, TeamID: 1, Date: date, Minutes: minutes, SourceFinal: true, BoxScoreComplete: true, FinalHomeScore: 100, FinalAwayScore: 85})
		}
	}
	m := Fit(rows, lines, "2026-10-01")
	low, high := m.Estimate("2026-10-01", -2, 34), m.Estimate("2026-10-01", -14, 34)
	if !low.Applied || !high.Applied || high.Shift >= low.Shift || math.Abs(high.Shift) > 3 {
		t.Fatal(m, low, high)
	}
	extra := append([]data.PlayerGame(nil), rows...)
	for _, r := range rows {
		r.Date = "2026-10-01"
		r.GameID += 100
		r.Minutes = 1
		extra = append(extra, r)
	}
	if !reflect.DeepEqual(m, Fit(extra, lines, "2026-10-01")) {
		t.Fatal("future finals leaked")
	}
	if Fit(rows, nil, "2026-10-01").Estimate("2026-10-01", 14, 34).Applied {
		t.Fatal("missing pregame lines became an effect")
	}
}
