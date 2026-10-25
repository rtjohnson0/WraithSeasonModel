package team

import (
	"math"
	"testing"

	"propsmodel/internal/data"
)

func league(nGames int) []data.PlayerGame {
	var rows []data.PlayerGame
	teams := []int{1, 2, 3, 4}
	day := 19000
	id := 1
	for g := 0; g < nGames; g++ {
		h, a := teams[g%4], teams[(g+1+g/4)%4]
		if h == a {
			a = teams[(g+2)%4]
		}
		day += 1 + g%2
		hp, ap := 100+float64((g*7)%20), 98+float64((g*5)%17)
		for _, t := range []struct {
			team int
			pts  float64
		}{{h, hp}, {a, ap}} {
			rows = append(rows, data.PlayerGame{PlayerID: t.team*10 + 1, TeamID: t.team, GameID: id, Day: day,
				Date: "2025-11-01", Season: 2025, Minutes: 30, Pts: t.pts, HomeTeamID: h, VisitorTeamID: a})
		}
		id++
	}
	return rows
}

func TestNoLookAhead(t *testing.T) {
	rows := league(120)
	full := Build(rows)
	a, _ := Samples(full)
	b, _ := Samples(full[:60])
	for i := range b {
		for j := range b[i].X {
			if math.Abs(a[i].X[j]-b[i].X[j]) > 1e-12 {
				t.Fatalf("game %d feature %s changed when later games were removed", i, FeatureNames[j])
			}
		}
	}
}

func TestBuildScores(t *testing.T) {
	gs := Build(league(8))
	if len(gs) != 8 {
		t.Fatalf("got %d games", len(gs))
	}
	for _, g := range gs {
		if g.HPts < 50 || g.APts < 50 {
			t.Fatalf("bad scores %+v", g)
		}
	}
}

func TestFitAndMix(t *testing.T) {
	gs := Build(league(200))
	s, _ := Samples(gs)
	m, err := Fit(s)
	if err != nil {
		t.Fatal(err)
	}
	if m.SigmaM <= 0 || m.SigmaT <= 0 {
		t.Fatal("sigma not set")
	}
	var recs []Rec
	for i := 0; i < 100; i++ {
		x := float64(i%9) - 4
		recs = append(recs, Rec{Market: x, Proj: x * 0.5, Actual: x + float64(i%3) - 1})
	}
	mx, err := FitMix(recs)
	if err != nil {
		t.Fatal(err)
	}
	if mu := mx.Mean(3, 1.5); math.Abs(mu-3) > 1 {
		t.Fatalf("mix mean %v", mu)
	}
}

func TestNormCDF(t *testing.T) {
	if math.Abs(NormCDF(0)-0.5) > 1e-12 || math.Abs(NormCDF(1.96)-0.975) > 1e-3 {
		t.Fatal("NormCDF wrong")
	}
}
