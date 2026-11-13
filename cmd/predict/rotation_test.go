package main

import (
	"fmt"
	"math"
	"propsmodel/internal/availability"
	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/glm"
	"propsmodel/internal/model"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"testing"
	"time"
)

func rotationFixture() ([]data.PlayerGame, map[int]int) {
	var rows []data.PlayerGame
	roster := map[int]int{}
	for d := 1; d <= 8; d++ {
		for p := 1; p <= 10; p++ {
			roster[100+p] = 1
			rows = append(rows, data.PlayerGame{SourceFinal: true, Date: fmt.Sprintf("2026-11-%02d", d), TeamID: 1, GameID: d, PlayerID: 100 + p, Name: fmt.Sprintf("Player %d", 100+p), Minutes: 24})
		}
	}
	return rows, roster
}
func TestQuestionableTeammateIsScenarioNotAssumedOut(t *testing.T) {
	rows, roster := rotationFixture()
	fresh := map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out"}, 102: {PlayerID: 102, Name: "Player 102", Status: "Questionable"}}
	f := newRotationForecast(rows, roster, fresh, map[int]string{101: "Out", 102: "Questionable"}, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09")
	if !f.teams[1].Valid || f.teams[1].Minutes[101] != 0 || f.teams[1].Minutes[102] <= 0 {
		t.Fatal(f.teams[1])
	}
	alt := f.scenarios[1][102]
	if !alt.Valid || alt.Minutes[102] != 0 {
		t.Fatal(alt)
	}
	p := predictor.Pred{Minutes: 24}
	for n := range p.MinNodes {
		p.MinNodes[n] = 24
	}
	p.Rate[data.StatIndex(data.Points)] = .5
	row := &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}
	adjusted := f.adjust(p, row)
	if math.Abs(adjusted.Minutes-f.teams[1].Minutes[103]) > 1e-8 || adjusted.Rate != p.Rate {
		t.Fatal("allocation or rate changed incorrectly")
	}
	s := scored{PlayerID: 103, GameID: 9, Stat: data.Points, Side: "OVER", Line: 12.5, ProjMin: adjusted.Minutes}
	c := f.context(s, p, row, .5, .5, combo.Corr{})
	if len(c.Scenarios) != 1 || c.Scenarios[0].Probability == nil || c.Scenarios[0].Minutes <= c.UsedMinutes {
		t.Fatal(c)
	}
	if c.Supported {
		t.Fatal("unobserved absence combination should stay review-only")
	}
}
func TestStaleOutReportDoesNotNumericallyAlterRotation(t *testing.T) {
	rows, roster := rotationFixture()
	f := newRotationForecast(rows, roster, nil, map[int]string{101: "Out"}, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09")
	if f.teams[1].Valid {
		t.Fatal("stale known absence accepted")
	}
	p := predictor.Pred{Minutes: 24}
	if f.adjust(p, &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}).Minutes != 24 {
		t.Fatal("baseline changed")
	}
}

func TestRotationCanScoreNewBenchOpportunityAndReplayExactly(t *testing.T) {
	rows, roster := rotationFixture()
	b := feat.NewBuilder(rows, nil)
	b.SetRoster(roster)
	games := map[int]props.GameInfo{9: {ID: 9, Home: 1, Visitor: 2, HomeAbbr: "ATL", VisitorAbbr: "BOS", Start: "2026-11-09T23:00:00Z"}}
	fresh := map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out"}}
	f := newRotationForecast(rows, roster, fresh, map[int]string{101: "Out"}, games, "2026-11-09")
	m := &predictor.Model{Version: 4, Minutes: &glm.Linear{Intercept: 12}, Quant: make([][]float64, 5)}
	for k := range m.Quant {
		m.Quant[k] = make([]float64, predictor.Nodes)
	}
	for k := range m.Stat {
		m.Stat[k] = &glm.Model{Intercept: math.Log(.5), Alpha: .05}
	}
	r := &feat.Row{G: data.PlayerGame{PlayerID: 103, TeamID: 1, Date: "2026-11-09"}, AvgMin15: 8, Games: 8, X: make([]float64, len(feat.CommonSpecs))}
	for k := range r.S {
		r.S[k] = make([]float64, len(feat.StatSpecs[k]))
	}
	if model.Eligible(r.AvgMin15, m.Predict(r).Minutes) {
		t.Fatal("fixture must be ineligible for baseline")
	}
	baseline := m.Predict(r)
	asOf, _ := time.Parse(time.RFC3339, "2026-11-09T18:00:00Z")
	line := props.Line{GameID: 9, PlayerID: 103, Stat: data.Points, Line: 10.5, OverOdds: -110, UnderOdds: -110, Vendor: "book", CapturedAt: asOf.Format(time.RFC3339), UpdatedAt: asOf.Format(time.RFC3339)}
	all, records := scoreRotationSingles(f, m, b, []props.Line{line}, games, func(pid int) (int, bool) { tm, ok := roster[pid]; return tm, ok }, func(int, props.GameInfo, int) *feat.Row { return r }, nil, "2026-11-09", .5, 1, asOf)
	if len(all) != 1 || len(records) != 1 || all[0].ProjMin <= 18 {
		t.Fatal("new bench opportunity was filtered by baseline eligibility")
	}
	p, err := replay.Recompute(m, records[0])
	if err != nil || math.Abs(p-records[0].P1) > 1e-10 {
		t.Fatal("rotation forecast not reproducible", p, err)
	}
	if records[0].Row.G.Name == "" || records[0].Rotation == nil || records[0].Policy.TargetMinutes == nil {
		t.Fatal("incomplete capture")
	}
	if m.Predict(r) != baseline {
		t.Fatal("V1 baseline changed")
	}
}
