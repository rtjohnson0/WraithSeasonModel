package gamepace

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/glm"
	"reflect"
	"testing"
	"time"
)

func boxFixture() []data.PlayerGame {
	rows := []data.PlayerGame{}
	for team := 1; team <= 2; team++ {
		for i := 1; i <= 5; i++ {
			pts := 20.
			if team == 2 {
				pts = 18
			}
			rows = append(rows, data.PlayerGame{PlayerID: team*100 + i, TeamID: team, GameID: 1, HomeTeamID: 1, VisitorTeamID: 2, Date: "2025-01-01", Minutes: 48, Pts: pts, Fga: 18, Fta: 4, Oreb: 2, Turnover: 1, SourceFinal: true, BoxScoreComplete: true, PossessionComplete: true, FinalHomeScore: 100, FinalAwayScore: 90})
		}
	}
	return rows
}
func TestPossessionsRequireCompleteFinalAndNormalizeOvertime(t *testing.T) {
	rows := boxFixture()
	games, ex := Build(rows)
	if len(games) != 1 || len(ex) != 0 || math.Abs(games[0].Pace-93.8) > 1e-8 {
		t.Fatal(games, ex)
	}
	rows[0].PossessionComplete = false
	if g, _ := Build(rows); len(g) != 0 {
		t.Fatal("missing turnovers or rebounds became zero")
	}
	rows = boxFixture()
	rows[0].Minutes = 20
	if g, _ := Build(rows); len(g) != 0 {
		t.Fatal("incomplete team minutes accepted")
	}
	rows = boxFixture()
	rows[0].Pts = 1
	if g, _ := Build(rows); len(g) != 0 {
		t.Fatal("summed score mismatch accepted")
	}
	rows = boxFixture()
	for i := range rows {
		rows[i].Minutes = 53
	}
	games, _ = Build(rows)
	if len(games) != 1 || math.Abs(games[0].Pace-93.8*48/53) > 1e-8 {
		t.Fatal("overtime counted as faster regulation pace", games)
	}
}
func sampleGames(n int) []Game {
	rows := boxFixture()
	out := []Game{}
	start, _ := time.Parse("2006-01-02", "2023-01-01")
	for d := 0; d < n; d++ {
		g := Game{ID: d + 1, Home: 1, Away: 2, Date: start.AddDate(0, 0, d).Format("2006-01-02"), Pace: 94 + float64(d%9), HomeEff: 1.12 + .01*float64(d%4), AwayEff: 1.08 + .01*float64(d%3), Players: map[int]data.PlayerGame{}}
		g.HomeScore = g.Pace * g.HomeEff
		g.AwayScore = g.Pace * g.AwayEff
		for _, r := range rows {
			r.Date = g.Date
			r.GameID = g.ID
			r.Reb = 4
			r.Ast = 3
			r.Fg3m = 2
			g.Players[r.PlayerID] = r
		}
		out = append(out, g)
	}
	return out
}
func TestEveryHistoricalInputUsesEarlierDatesOnly(t *testing.T) {
	games := sampleGames(400)
	date := games[350].Date
	a, sa := Samples(games, date)
	b, sb := Samples(games[:350], date)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(sa.Features(1, 2, date, nil, nil), sb.Features(1, 2, date, nil, nil)) {
		t.Fatal("future history changed past input")
	}
	m, err := Fit(a, date)
	if err != nil {
		t.Fatal(err)
	}
	future := Sample{Game: Game{Date: date, Pace: 999, HomeEff: 5, AwayEff: 5}, Input: Input{X: make([]float64, len(Names))}}
	m2, err := Fit(append(a, future), date)
	if err != nil || !reflect.DeepEqual(m, m2) {
		t.Fatal("same-day final changed fitted model", err)
	}
	study := MonthlyReplay(a, date)
	for _, r := range study.Replays {
		if r.Date >= date {
			t.Fatal("future game entered study")
		}
	}
}
func TestLineupAssociationIsBoundedAndNotStacked(t *testing.T) {
	model := &Model{Pace: &glm.Linear{Intercept: 100}, HomeEff: &glm.Linear{Intercept: 1.1, Names: Names, Mean: make([]float64, len(Names)), Std: make([]float64, len(Names)), Coef: make([]float64, len(Names))}, AwayEff: &glm.Linear{Intercept: 1.1}}
	for i := range model.HomeEff.Std {
		model.HomeEff.Std[i] = 1
	}
	model.HomeEff.Coef[10] = .1
	base := Input{X: make([]float64, len(Names))}
	used := Input{X: make([]float64, len(Names))}
	used.X[10] = 100
	a, b := model.Predict(base.X), LineupForecast(model, base, used)
	if math.Abs(b.HomeScore-a.HomeScore-3) > 1e-8 || b.AwayScore != a.AwayScore || b.Margin != b.HomeScore-b.AwayScore {
		t.Fatal(a, b)
	}
}
func TestIndependentMarketCalibrationRequiresEarlierDatesAndSymmetry(t *testing.T) {
	rows := []Outcome{}
	start, _ := time.Parse("2006-01-02", "2025-01-01")
	for i := 0; i < 360; i++ {
		y := 0.
		if i%3 != 0 {
			y = 1
		}
		rows = append(rows, Outcome{Date: start.AddDate(0, 0, i/10).Format("2006-01-02"), Market: "moneyline", Raw: .7, Book: .55, Y: y})
	}
	a := FitMarkets(rows, "2025-03-01")
	if !a["moneyline"].Ready || a["spread"].Ready || a["total"].Ready || a["moneyline"].Cal.Intercept != 0 {
		t.Fatal(a)
	}
	thin := append([]Outcome(nil), rows...)
	for i := range thin {
		thin[i].Date = "2025-01-01"
	}
	if FitMarkets(thin, "2025-03-01")["moneyline"].Ready {
		t.Fatal("one slate manufactured independent dates")
	}
	after := append(rows, Outcome{Date: "2025-03-01", Market: "moneyline", Raw: .1, Book: .9, Y: 1})
	if !reflect.DeepEqual(a, FitMarkets(after, "2025-03-01")) {
		t.Fatal("future outcomes calibrated past")
	}
	c := a["moneyline"].Cal
	if math.Abs(c.Apply(.6, .55)+c.Apply(.4, .45)-1) > 1e-8 {
		t.Fatal("directional intercept bias")
	}
}

func TestForwardMarketEvaluationNeverUsesSameDateTraining(t *testing.T) {
	rows := []Outcome{}
	start, _ := time.Parse("2006-01-02", "2025-01-01")
	for i := 0; i < 310; i++ {
		y := 0.
		if i%3 != 0 {
			y = 1
		}
		rows = append(rows, Outcome{Date: start.AddDate(0, 0, i/10).Format("2006-01-02"), Market: "total", Raw: .7, Book: .55, Y: y})
	}
	scores := EvaluateMarkets(rows, "2025-02-01")
	if scores["total"].N != 310 || scores["total"].CalibratedN != 10 || scores["moneyline"].N != 0 || scores["moneyline"].RawLogLoss != nil {
		t.Fatal(scores)
	}
	if scores["total"].CalibrationBookLogLoss == nil || scores["total"].CalibrationRawLogLoss == nil {
		t.Fatal("missing same-subset baselines")
	}
	earlier := EvaluateMarkets(rows, "2025-01-31")
	if earlier["total"].CalibratedN != 0 || earlier["total"].CalibratedLogLoss != nil {
		t.Fatal("fitted calibration tested on its own training outcomes")
	}
}
