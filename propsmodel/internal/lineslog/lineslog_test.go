package lineslog

import (
	"math"
	"testing"

	"propsmodel/internal/data"
)

func act(m map[[2]int]data.PlayerGame) Actualer {
	return func(g, p int) (data.PlayerGame, bool) {
		pg, ok := m[[2]int{g, p}]
		return pg, ok
	}
}

func TestMarketRoundTripAndGrading(t *testing.T) {
	dir := t.TempDir()
	rows := []Market{
		{Date: "2026-10-27", GameID: 1, PlayerID: 10, Stat: "points", Line: 20.5, Books: 12, Lo: 19.5, Hi: 21.5, Open: 19.5, POver: 0.42, TeamOut: 1},
		{Date: "2026-10-27", GameID: 1, PlayerID: 11, Stat: "rebounds", Line: 8, Books: 6, POver: 0.58},
		{Date: "2026-10-27", GameID: 1, PlayerID: 12, Stat: "assists", Line: 5.5, Books: 6, POver: 0.5}, // did not play
	}
	if err := SaveMarket(dir, "2026-10-27", rows); err != nil {
		t.Fatal(err)
	}
	// A second save replaces the first (the latest run wins).
	if err := SaveMarket(dir, "2026-10-27", rows); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMarkets(dir)
	if err != nil || len(got) != 3 {
		t.Fatalf("round trip: %v %d", err, len(got))
	}
	if m := got[0].Move(); math.Abs(m-1) > 1e-9 {
		t.Errorf("median moved 19.5 to 20.5 = +1, got %v", m)
	}
	gm := GradeMarkets(got, act(map[[2]int]data.PlayerGame{
		{1, 10}: {Pts: 24}, // over 20.5
		{1, 11}: {Reb: 8},  // exactly the line: push
	}))
	if len(gm) != 2 || !gm[0].Over || !gm[1].Push {
		t.Fatalf("grading wrong: %+v", gm)
	}
	if bins := CalBins(gm); len(bins) != 1 || bins[0].N != 1 || bins[0].Actual != 1 {
		t.Errorf("a push must stay out of the calibration bins: %+v", bins)
	}
}

func TestAppendOffDedupes(t *testing.T) {
	dir := t.TempDir()
	o := Off{Date: "2026-10-27", PlayerID: 5, Stat: "rebounds", Book: "fanduel", Side: "UNDER", Line: 8.5, Median: 7.5, Books: 12, Odds: -110}
	n, err := AppendOff(dir, []Off{o, o})
	if err != nil || n != 1 {
		t.Fatalf("one new row expected, got %d %v", n, err)
	}
	if n, _ := AppendOff(dir, []Off{o}); n != 0 {
		t.Error("a repeat of the same line must not be logged again")
	}
	o2 := o
	o2.Line = 9.5
	if n, _ := AppendOff(dir, []Off{o2}); n != 1 {
		t.Error("a different line is a different row")
	}
	if all, _ := LoadOff(dir); len(all) != 2 {
		t.Errorf("want 2 rows, got %d", len(all))
	}
}

func TestGradeOffUsesBestPriceAndFlatUnits(t *testing.T) {
	base := Off{Date: "2026-10-27", GameID: 1, PlayerID: 5, Stat: "rebounds", Side: "UNDER", Line: 8.5, Median: 7.5, Books: 12, PSide: 0.6}
	a, b := base, base
	a.Book, a.Odds = "x", -150
	b.Book, b.Odds = "y", +100 // the better price: same outlier at two books counts once
	win := Off{Date: "2026-10-27", GameID: 1, PlayerID: 6, Stat: "points", Side: "OVER", Line: 10.5, Median: 11.5, Books: 5, Odds: -110, PSide: 0.4}
	gs := GradeOff([]Off{a, b, win}, act(map[[2]int]data.PlayerGame{
		{1, 5}: {Reb: 6}, // under 8.5 wins
		{1, 6}: {Pts: 9}, // over 10.5 loses
	}))
	if len(gs) != 2 {
		t.Fatalf("same player, stat and side collapses to one: %d", len(gs))
	}
	if gs[0].Odds != 100 || gs[0].Result != "WIN" || math.Abs(gs[0].Flat-1) > 1e-9 {
		t.Errorf("should keep the +100 price: %+v", gs[0])
	}
	if gs[1].Result != "LOSS" || gs[1].Flat != -1 {
		t.Errorf("loss wrong: %+v", gs[1])
	}
	rows := OffSummary(gs)
	if rows[0].Name != "All off-market lines" || rows[0].Wins != 1 || rows[0].Losses != 1 || math.Abs(rows[0].Units-0) > 1e-9 {
		t.Errorf("summary wrong: %+v", rows[0])
	}
	var agrees, not OffRow
	for _, r := range rows {
		switch r.Name {
		case "Model agrees":
			agrees = r
		case "Model does not agree":
			not = r
		}
	}
	if agrees.N != 1 || not.N != 1 {
		t.Errorf("agreement split wrong: %+v %+v", agrees, not)
	}
}

func TestSkillAndSlices(t *testing.T) {
	var gm []GradedMarket
	// A model that knows: P 0.8 where it goes over, 0.2 where it does not.
	for i := 0; i < 100; i++ {
		over := i%2 == 0
		p := 0.2
		if over {
			p = 0.8
		}
		gm = append(gm, GradedMarket{Market: Market{POver: p, Stat: "points", Line: 10, Books: 12}, Over: over})
	}
	m, b, n := Skill(gm)
	if n != 100 || m >= b {
		t.Errorf("an informed model should beat the base rate: model %.3f base %.3f", m, b)
	}
	sl := Slices(gm)
	if sl[0].Name != "every line" || sl[0].N != 100 || math.Abs(sl[0].OverRate-0.5) > 1e-9 {
		t.Errorf("slice wrong: %+v", sl[0])
	}
}

func TestSaveMarketKeepsEarlierRowsTheLaterRunDropped(t *testing.T) {
	dir := t.TempDir()
	early := Market{Date: "d", GameID: 1, PlayerID: 1, Stat: "points", Line: 20.5, POver: 0.4}
	late := Market{Date: "d", GameID: 2, PlayerID: 2, Stat: "points", Line: 10.5, POver: 0.6}
	if err := SaveMarket(dir, "d", []Market{early}); err != nil {
		t.Fatal(err)
	}
	// The later run lists only the late game, and re-scores nothing for game 1.
	if err := SaveMarket(dir, "d", []Market{late}); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadMarkets(dir)
	if len(got) != 2 {
		t.Fatalf("want both games kept, got %d", len(got))
	}
	// A re-score of game 2 replaces it.
	late.POver = 0.7
	SaveMarket(dir, "d", []Market{late})
	got, _ = LoadMarkets(dir)
	if len(got) != 2 || got[1].POver != 0.7 {
		t.Errorf("re-scored row should replace the old one: %+v", got)
	}
}

func TestDedupeKeepsLatestRun(t *testing.T) {
	a := Market{Date: "d", GameID: 1, PlayerID: 1, Stat: "points", At: "2026-10-27T13:00:00Z", POver: 0.4}
	b := a
	b.At, b.POver = "2026-10-27T19:00:00Z", 0.5
	c := Market{Date: "d", GameID: 1, PlayerID: 2, Stat: "points", At: "x"}
	got := Dedupe([]Market{b, a, c}) // the later run listed first, as after a merge
	if len(got) != 2 || got[0].POver != 0.5 {
		t.Errorf("want the 19:00 run kept: %+v", got)
	}
}

func TestLeanCellsSplitsBySideAndBeatsTheBaseRate(t *testing.T) {
	var gm []GradedMarket
	add := func(stat string, p float64, over bool, n int) {
		for i := 0; i < n; i++ {
			gm = append(gm, GradedMarket{Market: Market{Stat: stat, P2: p}, Over: over})
		}
	}
	// Points: the probability separates the lines. Under leans mostly go under, Over leans mostly over.
	add("points", 0.40, false, 8)
	add("points", 0.40, true, 2)
	add("points", 0.60, true, 8)
	add("points", 0.60, false, 2)
	// Rows V2 did not score are ignored, and pushes are left out.
	add("points", 0, true, 5)
	gm = append(gm, GradedMarket{Market: Market{Stat: "points", P2: 0.6}, Push: true})
	cells := LeanCells(gm, func(g GradedMarket) float64 { return g.P2 })
	if len(cells) != 2 {
		t.Fatalf("want an Over and an Under cell for points, got %+v", cells)
	}
	var over, under Cell
	for _, c := range cells {
		if c.Side == "OVER" {
			over = c
		} else {
			under = c
		}
	}
	if over.N != 10 || under.N != 10 {
		t.Errorf("counts: over %d under %d", over.N, under.N)
	}
	if math.Abs(over.Hit-0.8) > 1e-9 || math.Abs(under.Hit-0.8) > 1e-9 {
		t.Errorf("hit rates: over %v under %v", over.Hit, under.Hit)
	}
	if !(over.Brier < over.Base) || !(under.Brier < under.Base) {
		t.Errorf("an informative probability should beat the base rate: over %v/%v under %v/%v", over.Brier, over.Base, under.Brier, under.Base)
	}

	// A probability that leans Over on everything while half the lines go over does not beat it.
	gm = nil
	add("rebounds", 0.60, true, 5)
	add("rebounds", 0.60, false, 5)
	cells = LeanCells(gm, func(g GradedMarket) float64 { return g.P2 })
	if len(cells) != 1 || cells[0].Side != "OVER" || cells[0].Brier < cells[0].Base {
		t.Errorf("an uninformative Over lean must not beat the base rate: %+v", cells)
	}
}
