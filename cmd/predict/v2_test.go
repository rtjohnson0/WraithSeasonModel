package main

import (
	"math"
	"strings"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/projlog"
)

func TestFitV2UsesOnlyEarlierDaysAndShrinks(t *testing.T) {
	var entries []projlog.Entry
	act := map[[2]int]data.PlayerGame{}
	id := 0
	add := func(date string, projMin, actMin, projPts, actPts float64) {
		id++
		entries = append(entries, projlog.Entry{Date: date, GameID: id, PlayerID: id, ProjMin: projMin,
			Stats: map[string]projlog.StatProj{"points": {Proj: projPts, Line: projPts}}})
		act[[2]int{id, id}] = data.PlayerGame{Minutes: actMin, Pts: actPts}
	}
	for i := 0; i < 12; i++ {
		add("2026-10-27", 33, 35, 20, 23) // starters run 2 minutes over, 15% over on points
	}
	add("2026-10-28", 33, 50, 20, 60) // today: must be ignored (no look-ahead)
	f := fitV2(entries, "2026-10-28", func(g, p int) (data.PlayerGame, bool) { pg, ok := act[[2]int{g, p}]; return pg, ok })
	if f.Days != 1 || f.Players != 12 || f.MinN != 12 {
		t.Fatalf("only the earlier day counts: %+v", f)
	}
	// 12 player-games of +2: shrunk to 2*12/(12+40) = 0.46, far less than the raw miss.
	if f.MinShift < 0.3 || f.MinShift > 0.6 {
		t.Errorf("minutes shift should be shrunk hard from +2, got %v", f.MinShift)
	}
	if got := f.Ratio[data.Points]; got <= 1 || got > 1.03 {
		t.Errorf("points multiplier should move a little toward 1.15, got %v", got)
	}
	if orOne(f.Ratio[data.Rebounds]) != 1 {
		t.Errorf("a stat with no data stays at 1, got %v", f.Ratio[data.Rebounds])
	}
	empty := fitV2(nil, "2026-10-28", nil)
	if empty.MinShift != 0 || empty.statFactor(data.Points) != 1 {
		t.Errorf("no history must mean no correction: %+v", empty)
	}
}

func TestFitV2IsCapped(t *testing.T) {
	var entries []projlog.Entry
	act := map[[2]int]data.PlayerGame{}
	for i := 1; i <= 2000; i++ {
		entries = append(entries, projlog.Entry{Date: "2026-10-01", GameID: i, PlayerID: i, ProjMin: 34, Stats: map[string]projlog.StatProj{"points": {Proj: 10, Line: 10}}})
		act[[2]int{i, i}] = data.PlayerGame{Minutes: 44, Pts: 30}
	}
	f := fitV2(entries, "2026-10-28", func(g, p int) (data.PlayerGame, bool) { pg, ok := act[[2]int{g, p}]; return pg, ok })
	if f.MinShift > v2MinShiftCap+1e-9 || f.Ratio[data.Points] > 1+v2StatCap+1e-9 {
		t.Errorf("caps must hold: %v %v", f.MinShift, f.Ratio[data.Points])
	}
}

func TestShiftMinutesOnlyForStars(t *testing.T) {
	f := newV2Fix()
	f.MinShift = 1.5
	var star, bench predictor.Pred
	for k := range star.MinNodes {
		star.MinNodes[k], bench.MinNodes[k] = 34, 24
	}
	star.Minutes, bench.Minutes = 34, 24
	if got := f.shiftMinutes(star).Minutes; math.Abs(got-35.5) > 1e-6 {
		t.Errorf("star minutes should rise 1.5, got %v", got)
	}
	if got := f.shiftMinutes(bench).Minutes; got != 24 {
		t.Errorf("bench minutes must not change, got %v", got)
	}
}

func TestBlend(t *testing.T) {
	if got := blend(0.60, 0.50, 0.5); math.Abs(got-0.55) > 1e-9 {
		t.Errorf("50/50 blend of 60 and 50 should be 55, got %v", got)
	}
}

// play builds a V2-scored line that clears the bar with a deep, tight market and the real model on the
// same side.
func play(id int, stat data.Stat, side string, prob float64) scored {
	return scored{PlayerID: id, GameID: 1, Player: "P", Stat: stat, Side: side, Line: 20.5, Prob: prob, ProbRaw: prob + 0.02, Edge: prob - 0.52, Odds: -110,
		Books: 8, Spread: 1, V1Prob: 0.55, Mkt: prob - 0.04}
}

func provenGates(stat data.Stat, side string) map[string]V2Gate {
	return map[string]V2Gate{gateKey(string(stat), side): {Stat: string(stat), Side: side, N: 200, State: gateProven}}
}

func TestTierRules(t *testing.T) {
	proven := provenGates(data.Rebounds, "UNDER")
	cases := []struct {
		name  string
		s     scored
		gates map[string]V2Gate
		tier  string
		drop  string
	}{
		{"proven, real model agrees: strong", play(1, data.Rebounds, "UNDER", 0.58), proven, tierStrong, ""},
		{"no graded lines yet: lean", play(1, data.Rebounds, "UNDER", 0.58), nil, tierLean, ""},
		{"real model only 51%: lean even when proven", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.V1Prob = 0.51; return s }(), proven, tierLean, ""},
		{"real model 53% the other way: dropped", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.V1Prob = 0.46; return s }(), proven, "", "disagree"},
		{"real model 49% (a coin flip) is not a disagreement", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.V1Prob = 0.49; return s }(), nil, tierLean, ""},
		{"three books: dropped", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.Books = 3; return s }(), proven, "", "thin"},
		{"scattered books: dropped", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.Spread = 4; return s }(), proven, "", "thin"},
		{"model and book 28 points apart: check", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.ProbRaw, s.Mkt = 0.80, 0.52; return s }(), proven, tierCheck, ""},
		{"a 18 point gap is ordinary for the overconfident raw model: not a check", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.ProbRaw, s.Mkt = 0.70, 0.52; return s }(), proven, tierStrong, ""},
		{"juiced past the floor: dropped", func() scored { s := play(1, data.Rebounds, "UNDER", 0.67); s.Odds = -190; return s }(), proven, "", "juiced"},
		{"uncertain status: check", func() scored { s := play(1, data.Rebounds, "UNDER", 0.58); s.Maybe = "questionable"; return s }(), proven, tierCheck, ""},
		{"failed gate: dropped", play(1, data.Points, "OVER", 0.58), map[string]V2Gate{gateKey("points", "OVER"): {N: 300, State: gateFailed}}, "", "failed"},
	}
	for _, c := range cases {
		tier, drop := v2Tier(c.s, v2Rules{Gates: c.gates, MaxJuice: 160})
		if tier != c.tier || drop != c.drop {
			t.Errorf("%s: got tier %q drop %q, want %q %q", c.name, tier, drop, c.tier, c.drop)
		}
	}
}

func TestOversAndUndersFaceTheSameRules(t *testing.T) {
	gates := map[string]V2Gate{}
	for _, side := range []string{"OVER", "UNDER"} {
		gates[gateKey("assists", side)] = V2Gate{N: 200, State: gateProven}
	}
	over, under := play(1, data.Assists, "OVER", 0.58), play(2, data.Assists, "UNDER", 0.58)
	over.V1Prob, under.V1Prob = 0.56, 0.56
	to, do := v2Tier(over, v2Rules{Gates: gates})
	tu, du := v2Tier(under, v2Rules{Gates: gates})
	if to != tu || do != du || to != tierStrong {
		t.Errorf("a mirrored Over and Under must be treated identically: %q/%q vs %q/%q", to, do, tu, du)
	}
}

func TestSpreadOK(t *testing.T) {
	cases := []struct {
		books        int
		spread, line float64
		want         bool
	}{
		{8, 1, 20.5, true},
		{4, 1.5, 4.5, true},
		{3, 0.5, 20.5, false},
		{8, 2.5, 20.5, true}, // 12% of 20.5 is 2.46: a 2.5 spread is just over
		{8, 3, 30.5, true},
		{8, 2, 4.5, false},
	}
	// 2.5 on a 20.5 line is wider than 12% of the line, so it is scattered.
	cases[3].want = false
	for _, c := range cases {
		if got := spreadOK(c.books, 0, c.spread, c.line); got != c.want {
			t.Errorf("spreadOK(%d,%v,%v) = %v, want %v", c.books, c.spread, c.line, got, c.want)
		}
	}
}

func TestGatesFromCells(t *testing.T) {
	cells := []lineslog.Cell{
		{Stat: "points", Side: "OVER", N: 40, Hit: 0.6, Brier: 0.24, Base: 0.25},     // too few lines
		{Stat: "points", Side: "UNDER", N: 200, Hit: 0.56, Brier: 0.24, Base: 0.25},  // beats the base rate
		{Stat: "assists", Side: "UNDER", N: 200, Hit: 0.55, Brier: 0.26, Base: 0.25}, // does not
		{Stat: "rebounds", Side: "OVER", N: 200, Hit: 0.45, Brier: 0.24, Base: 0.25}, // beats Brier but leans are wrong more than right
	}
	g := v2Gates(cells, 150)
	want := map[string]string{"points|OVER": gateUnproven, "points|UNDER": gateProven, "assists|UNDER": gateFailed, "rebounds|OVER": gateFailed}
	for k, state := range want {
		if g[k].State != state {
			t.Errorf("%s: got %s, want %s (%s)", k, g[k].State, state, g[k].Reason)
		}
	}
	if gateOf(g, "threes", "OVER").State != gateUnproven {
		t.Error("a cell with no lines is unproven")
	}
	if got := len(gateList(g)); got != 8 {
		t.Errorf("the gate list covers 4 stats and 2 sides, got %d", got)
	}
}

func TestSelectTiersCapsAndDrops(t *testing.T) {
	strong := play(1, data.Rebounds, "UNDER", 0.60)
	lean := play(2, data.Rebounds, "OVER", 0.57)
	lean.V1Prob = 0.50
	underBar := play(3, data.Points, "UNDER", 0.54)
	noEdge := play(4, data.Rebounds, "UNDER", 0.58)
	noEdge.Edge = 0
	thin := play(5, data.Rebounds, "UNDER", 0.58)
	thin.Books = 2
	disagree := play(6, data.Rebounds, "UNDER", 0.58)
	disagree.V1Prob = 0.45
	failed := play(7, data.Assists, "OVER", 0.58)
	other := play(8, data.Assists, "UNDER", 0.59)
	other.GameID = 2
	gates := provenGates(data.Rebounds, "UNDER")
	gates[gateKey("assists", "OVER")] = V2Gate{N: 300, State: gateFailed}
	got, drops := v2Select([]scored{strong, lean, underBar, noEdge, thin, disagree, failed, other}, v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 4, MaxPlayer: 2, Gates: gates})
	if drops != (V2Drops{Thin: 1, Disagree: 1, Failed: 1}) {
		t.Errorf("drops: %+v", drops)
	}
	tiers := map[int]string{}
	for _, s := range got {
		tiers[s.PlayerID] = s.Tier
		if s.Units != 0 {
			t.Error("V2 is never staked")
		}
	}
	if len(got) != 3 || tiers[1] != tierStrong || tiers[2] != tierLean || tiers[8] != tierLean {
		t.Errorf("want a strong, a lean Over and a lean from the other game, got %+v", tiers)
	}
	if got[0].PlayerID != 1 || got[1].PlayerID != 2 {
		t.Errorf("within a game the strong play comes first: %v %v", got[0].PlayerID, got[1].PlayerID)
	}

	// A Lean never takes a Strong play's place under the per-game cap.
	var crowd []scored
	for i := 0; i < 5; i++ {
		l := play(10+i, data.Rebounds, "OVER", 0.59)
		l.V1Prob = 0.50
		crowd = append(crowd, l)
	}
	crowd = append(crowd, play(20, data.Rebounds, "UNDER", 0.56))
	got, _ = v2Select(crowd, v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 2, MaxPlayer: 2, Gates: gates})
	hasStrong := false
	for _, s := range got {
		if s.PlayerID == 20 && s.Tier == tierStrong {
			hasStrong = true
		}
	}
	if len(got) > 3 || !hasStrong {
		t.Errorf("the strong play must survive the cap: %+v", got)
	}
}

func TestOversWatchExplainsWhyAnOverIsOut(t *testing.T) {
	good := play(1, data.Points, "OVER", 0.52) // just short of the bar
	good.Edge = 0
	disagrees := play(2, data.Points, "OVER", 0.58)
	disagrees.Player, disagrees.V1Prob = "Q", 0.40
	thin := play(3, data.Points, "OVER", 0.50) // thin and short of the bar: not worth listing
	thin.Player, thin.Books = "R", 2
	rows := v2OversWatch([]scored{good, disagrees, thin}, nil, v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 4, MaxPlayer: 2}, 12)
	var names []string
	for _, r := range rows {
		names = append(names, r.Player)
		if r.Player == "Q" && !strings.Contains(r.Status, "V2 left it out") {
			t.Errorf("a cleared Over that V2 dropped must say why: %q", r.Status)
		}
	}
	if len(rows) != 2 {
		t.Errorf("want the near miss and the dropped Over, not the thin market: %v", names)
	}
}

func TestAnchorAndFactor(t *testing.T) {
	if got := v2Anchor(23, 22); math.Abs(got-23.0/22) > 1e-9 {
		t.Errorf("anchor should be line/projection, got %v", got)
	}
	if got := v2Anchor(100, 10); got != v2AnchorHi {
		t.Errorf("anchor must be capped, got %v", got)
	}
	if got := v2Anchor(10, 100); got != v2AnchorLo {
		t.Errorf("anchor must be floored, got %v", got)
	}
	if v2Anchor(0, 5) != 1 || v2Anchor(5, 0) != 1 {
		t.Error("nothing to anchor to means no change")
	}
	f := newV2Fix()
	f.Anchor[data.Points], f.Ratio[data.Points] = 1.05, 1.02
	if got := f.statFactor(data.Points); math.Abs(got-1.071) > 1e-9 {
		t.Errorf("factor is anchor times live miss, got %v", got)
	}
	if f.statFactor(data.Threes) != 1 {
		t.Error("unset stats stay at 1")
	}
}

func TestKeepDisagreeTurnsTheDropIntoALean(t *testing.T) {
	s := play(1, data.Rebounds, "UNDER", 0.58)
	s.V1Prob = 0.40
	if tier, drop := v2Tier(s, v2Rules{}); tier != "" || drop != "disagree" {
		t.Errorf("default drops it: %q %q", tier, drop)
	}
	if tier, drop := v2Tier(s, v2Rules{KeepDisagree: true}); tier != tierLean || drop != "" {
		t.Errorf("with -v2keep it is a Lean, never Strong: %q %q", tier, drop)
	}
	got, drops := v2Select([]scored{s}, v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 4, MaxPlayer: 2, KeepDisagree: true})
	if len(got) != 1 || drops.Disagree != 0 {
		t.Errorf("kept play should post: %+v %+v", got, drops)
	}
}

func TestEdgeRouteLetsInAPlusMoneyOverButNotAThinFavourite(t *testing.T) {
	r := v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 4, MaxPlayer: 2, EdgeBar: 0.04, MaxJuice: 160}
	plus := play(1, data.Rebounds, "OVER", 0.54) // 54% at +120: break-even 45.5%, an 8 point edge
	plus.Odds, plus.Edge = 120, 0.082
	plus.V1Prob = 0.5
	fav := play(2, data.Assists, "OVER", 0.67) // 67% at -190: break-even 65.5%, a 1.5 point edge
	fav.Odds, fav.Edge = -190, 0.015
	coin := play(3, data.Assists, "UNDER", 0.47) // an edge on a side V2 gives under 50%: never posted
	coin.Odds, coin.Edge = 250, 0.07
	if !r.qualifies(plus) || r.qualifies(coin) {
		t.Errorf("the edge route needs 4%% edge and V2 at 50%%+: plus %v coin %v", r.qualifies(plus), r.qualifies(coin))
	}
	if (v2Rules{Hi: 0.55, MinEdge: 0.01}).qualifies(plus) {
		t.Error("without the edge route a 54% Over cannot pass the 55% bar")
	}
	got, drops := v2Select([]scored{plus, fav, coin}, r)
	if len(got) != 1 || got[0].PlayerID != 1 || drops.Juiced != 1 {
		t.Errorf("want only the plus-money Over, one juiced drop: %+v %+v", got, drops)
	}
	// Within a game the biggest edge is first.
	a, b := play(4, data.Rebounds, "UNDER", 0.70), play(5, data.Rebounds, "UNDER", 0.58)
	a.Edge, b.Edge = 0.03, 0.06
	got, _ = v2Select([]scored{a, b}, v2Rules{Hi: 0.55, MinEdge: 0.01, MaxGame: 4, MaxPlayer: 2})
	if len(got) != 2 || got[0].PlayerID != 5 {
		t.Errorf("rank by edge, not probability: %+v", got)
	}
}
