package main

import (
	"propsmodel/internal/availability"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/predictor"
	"testing"
	"time"
)

func TestAvailabilityComparisonDoesNotChangePickAndUsesSelectedSide(t *testing.T) {
	var rows []data.PlayerGame
	for i := 0; i < 40; i++ {
		date := time.Date(2026, 10, i+1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		m, mate := 24., 30.
		if i%5 == 4 {
			m, mate = 34, 0
		}
		mins := []float64{m, mate, 30, 30, 30, 30, 30, 20, 16, 0}
		mins[9] = 240.
		for j := 0; j < 9; j++ {
			mins[9] -= mins[j]
		}
		for j, v := range mins {
			rows = append(rows, data.PlayerGame{SourceFinal: true, PlayerID: j + 1, TeamID: 1, GameID: i + 1, Date: date, Minutes: v})
		}
	}
	view := availabilityView(availability.New(rows), map[int]availability.Status{2: {PlayerID: 2, Name: "Teammate", Status: "Doubtful", At: "2026-11-10T18:55:00Z"}}, "2026-11-10", "2026-11-10T19:00:00Z", func(int) (int, bool) { return 1, true }, nil)
	s := scored{PlayerID: 1, Side: "UNDER", Prob: .6, ProjMin: 28, Stat: data.Points, Line: 12.5}
	var p predictor.Pred
	p.Minutes = 28
	for i := range p.MinNodes {
		p.MinNodes[i] = 28
	}
	p.Rate[0] = .5
	original := p
	c := view(s, p, &feat.Row{G: data.PlayerGame{TeamID: 1}}, func(p predictor.Pred) (float64, error) { return p.Minutes / 48, nil })
	if c == nil || len(c.Teammates) != 1 || !c.Teammates[0].Supported {
		t.Fatal("missing supported history")
	}
	e := c.Teammates[0]
	if e.ConfirmedOut || e.AssumedOut {
		t.Fatal("doubtful became confirmed out")
	}
	if e.WithProbability == nil || e.WithoutProbability == nil || *e.WithoutProbability >= *e.WithProbability {
		t.Fatal("increased minutes did not work against under")
	}
	if s.Prob != .6 || s.ProjMin != 28 || p != original {
		t.Fatal("comparison mutated headline or predictor")
	}
	entry := boardEntry("2026-11-10", scored{Availability: c}, time.Now())
	if entry.Availability != c {
		t.Fatal("comparison dropped from publication")
	}
}
