package backtest

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/proj"
	"propsmodel/internal/props"
)

func poisson(r *rand.Rand, lambda float64) float64 {
	l := math.Exp(-lambda)
	k, p := 0, 1.0
	for {
		k++
		p *= r.Float64()
		if p <= l {
			return float64(k - 1)
		}
	}
}

type fakePlayer struct {
	id            int
	ppm, rpm, apm float64
}

func (p fakePlayer) trueMean(s data.Stat) float64 {
	switch s {
	case data.Points:
		return 30 * p.ppm
	case data.Rebounds:
		return 30 * p.rpm
	}
	return 30 * p.apm
}

// buildLeague makes two seasons of synthetic games with known per-minute rates, so the
// pipeline can be checked end to end without real data.
func buildLeague(seed int64) ([]data.PlayerGame, []fakePlayer, []int) {
	r := rand.New(rand.NewSource(seed))
	var players []fakePlayer
	for i := 1; i <= 40; i++ {
		players = append(players, fakePlayer{
			id:  i,
			ppm: 0.35 + 0.5*r.Float64(),
			rpm: 0.08 + 0.22*r.Float64(),
			apm: 0.05 + 0.23*r.Float64(),
		})
	}
	var games []data.PlayerGame
	var testGames []int // game ids in the 2025 season
	gameID := 0
	for _, season := range []int{2024, 2025} {
		start := time.Date(season, 10, 22, 0, 0, 0, 0, time.UTC)
		for d := 0; d < 100; d++ {
			gameID++
			date := start.AddDate(0, 0, d).Format("2006-01-02")
			if season == 2025 {
				testGames = append(testGames, gameID)
			}
			for _, p := range players {
				mins := math.Min(38, math.Max(22, 30+3*r.NormFloat64()))
				games = append(games, data.PlayerGame{
					PlayerID: p.id, Name: "P", GameID: gameID, Date: date, Season: season,
					Minutes: mins,
					Pts:     poisson(r, mins*p.ppm),
					Reb:     poisson(r, mins*p.rpm),
					Ast:     poisson(r, mins*p.apm),
				})
			}
		}
	}
	sort.SliceStable(games, func(i, j int) bool {
		if games[i].Date != games[j].Date {
			return games[i].Date < games[j].Date
		}
		return games[i].PlayerID < games[j].PlayerID
	})
	return games, players, testGames
}

func makeLines(players []fakePlayer, testGames []int, lineFor func(mean float64) float64) []props.Line {
	var lines []props.Line
	for _, g := range testGames {
		for _, p := range players {
			for _, s := range data.Stats {
				lines = append(lines, props.Line{
					GameID: g, PlayerID: p.id, Vendor: "x", Stat: s,
					Line: lineFor(p.trueMean(s)), OverOdds: -110, UnderOdds: -110,
				})
			}
		}
	}
	return lines
}

func TestFairLinesAreCalibrated(t *testing.T) {
	games, players, testGames := buildLeague(1)
	ix := proj.NewIndex(games)
	lines := makeLines(players, testGames, func(m float64) float64 { return math.Floor(m) + 0.5 })

	results := Run(ix, lines)
	if len(results) < 10000 {
		t.Fatalf("expected most lines to be scored, got %d", len(results))
	}
	SortChronologically(results)
	first, second := Split(results)

	// Against efficient lines the raw model is overconfident: its disagreements with the line
	// are mostly noise. The fitted calibrator must detect that (slope well below 1) and, applied
	// to games it was not fitted on, produce probabilities that match reality.
	cal := Fit(first)
	if cal.Slope >= 0.8 || cal.Slope <= 0 {
		t.Errorf("fitted slope %.3f, want a clear shrink (between 0 and 0.8)", cal.Slope)
	}
	for _, row := range Calibration(Apply(second, cal)) {
		if row.N < 400 {
			continue
		}
		if math.Abs(row.Predicted-row.Actual) > 0.05 {
			t.Errorf("calibrated bin %s: predicted %.3f vs actual %.3f over %d lines", row.Bin, row.Predicted, row.Actual, row.N)
		}
	}
}

func TestStaleLinesAreFoundAndWin(t *testing.T) {
	games, players, testGames := buildLeague(2)
	ix := proj.NewIndex(games)
	// Lines set 15% below what each player really produces: the model should post overs and win.
	lines := makeLines(players, testGames, func(m float64) float64 { return math.Max(0.5, math.Floor(m*0.85)+0.5) })

	st := Evaluate(Run(ix, lines), 0.55)
	if st.Picks < 1000 {
		t.Fatalf("expected many posted picks against stale lines, got %d", st.Picks)
	}
	if st.HitRate() < 0.60 {
		t.Errorf("hit rate %.3f against stale lines, want at least 0.60", st.HitRate())
	}
	if st.Units <= 0 {
		t.Errorf("expected profit against stale lines, got %.1f units", st.Units)
	}
}

func TestMedianLinesDeduplicates(t *testing.T) {
	in := []props.Line{
		{GameID: 1, PlayerID: 1, Stat: data.Points, Line: 20.5, Vendor: "a"},
		{GameID: 1, PlayerID: 1, Stat: data.Points, Line: 22.5, Vendor: "b"},
		{GameID: 1, PlayerID: 1, Stat: data.Points, Line: 21.5, Vendor: "c"},
		{GameID: 1, PlayerID: 1, Stat: data.Rebounds, Line: 6.5, Vendor: "a"},
	}
	out := MedianLines(in)
	if len(out) != 2 {
		t.Fatalf("want 2 groups, got %d", len(out))
	}
	for _, l := range out {
		if l.Stat == data.Points && l.Line != 21.5 {
			t.Errorf("median points line = %v, want 21.5", l.Line)
		}
	}
}

func TestEvaluateGradesPushesAndOdds(t *testing.T) {
	rs := []Result{
		{POver: 0.70, Actual: 25, Line: 24.5, OverOdds: 100, UnderOdds: -120},  // OVER wins at +100 -> +1.0
		{POver: 0.30, Actual: 25, Line: 24.5, OverOdds: 100, UnderOdds: -120},  // UNDER loses -> -1
		{POver: 0.30, Actual: 20, Line: 24.5, OverOdds: 100, UnderOdds: -120},  // UNDER wins at -120 -> +0.8333
		{POver: 0.70, Actual: 24, Line: 24, OverOdds: -110, UnderOdds: -110},   // push
		{POver: 0.50, Actual: 30, Line: 24.5, OverOdds: -110, UnderOdds: -110}, // not posted
	}
	st := Evaluate(rs, 0.55)
	if st.Picks != 4 || st.Wins != 2 || st.Losses != 1 || st.Pushes != 1 {
		t.Fatalf("unexpected grading: %+v", st)
	}
	if math.Abs(st.Units-(1.0-1.0+100.0/120.0)) > 1e-9 {
		t.Errorf("units = %v", st.Units)
	}
}
