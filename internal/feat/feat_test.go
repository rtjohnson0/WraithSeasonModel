package feat

import (
	"math"
	"testing"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/synthworld"
)

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return math.Abs(a-b) < 1e-9
}

// The most important property: a row's features must not depend on anything after its date.
func TestNoLookAhead(t *testing.T) {
	w := synthworld.Generate(synthworld.Config{Seed: 5, Seasons: []int{2023, 2024}, Teams: 12, Days: 120})
	full := NewBuilder(w.Games, w.Adv).Rows(0)

	cutDate := "2024-12-01"
	var kept []data.PlayerGame
	for _, g := range w.Games {
		if g.Date <= cutDate {
			kept = append(kept, g)
		}
	}
	trunc := NewBuilder(kept, w.Adv).Rows(0)

	idx := map[[2]int]*Row{}
	for i := range trunc {
		idx[[2]int{trunc[i].G.GameID, trunc[i].G.PlayerID}] = &trunc[i]
	}
	checked := 0
	for i := range full {
		r := &full[i]
		if r.G.Date > cutDate {
			continue
		}
		o, ok := idx[[2]int{r.G.GameID, r.G.PlayerID}]
		if !ok {
			t.Fatalf("row for game %d player %d missing after truncation", r.G.GameID, r.G.PlayerID)
		}
		for j := range r.X {
			if !sameFloat(r.X[j], o.X[j]) {
				t.Fatalf("common feature %s changed when future games were removed: %v vs %v", CommonSpecs[j].Name, r.X[j], o.X[j])
			}
		}
		for k := range r.S {
			for j := range r.S[k] {
				if !sameFloat(r.S[k][j], o.S[k][j]) {
					t.Fatalf("stat feature %s (stat %d) changed when future games were removed: %v vs %v", StatSpecs[k][j].Name, k, r.S[k][j], o.S[k][j])
				}
			}
		}
		checked++
	}
	if checked < 2000 {
		t.Fatalf("only %d rows compared", checked)
	}
}

// A hand-built league: team 1 plays team 2 every other day. Player 11 (the star) misses the last game.
func miniLeague() []data.PlayerGame { return miniLeagueN(12) }

func miniLeagueN(n int) []data.PlayerGame {
	var out []data.PlayerGame
	add := func(day, game, player, team int, mins, pts, reb, ast float64) {
		date := time.Date(2025, 11, day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		out = append(out, data.PlayerGame{
			PlayerID: player, Name: "p", TeamID: team, GameID: game, Date: date, Season: 2025,
			Minutes: mins, Pts: pts, Reb: reb, Ast: ast, Day: data.DayNumber(date),
			HomeTeamID: 1, VisitorTeamID: 2,
		})
	}
	for g := 0; g < n; g++ {
		day := 1 + 2*g
		game := 100 + g
		last := g == n-1 && n == 12
		if !last {
			add(day, game, 11, 1, 36, 28, 6, 6) // star
		}
		add(day, game, 12, 1, 30, 14, 5, 3)
		add(day, game, 13, 1, 28, 12, 4, 4)
		add(day, game, 14, 1, 24, 9, 3, 2)
		add(day, game, 15, 1, 20, 8, 6, 1)
		for p := 21; p <= 25; p++ {
			add(day, game, p, 2, 30-float64(p-21)*2, 12, 4, 3)
		}
	}
	return out
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestTeammatesOutFeatures(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	var starRow, otherRow *Row
	rows := b.Rows(0)
	for i := range rows {
		r := &rows[i]
		if r.G.GameID == 111 && r.G.PlayerID == 12 {
			otherRow = r
		}
		if r.G.GameID == 110 && r.G.PlayerID == 12 {
			starRow = r
		}
	}
	if otherRow == nil || starRow == nil {
		t.Fatal("expected rows for player 12 in games 110 and 111")
	}
	mm := commonIdx["miss_min"]
	if starRow.X[mm] != 0 {
		t.Errorf("star played in game 110, so nobody is missing, got miss_min %v", starRow.X[mm])
	}
	if got := otherRow.X[mm]; got != 36 {
		t.Errorf("star is out in game 111: miss_min should be his 36 minutes, got %v", got)
	}
	pts := data.StatIndex(data.Points)
	if got := otherRow.S[pts][statIdx[pts]["miss_s"]]; got != 28 {
		t.Errorf("miss_s for points should be the star's 28 ppg, got %v", got)
	}
	if got := otherRow.X[commonIdx["miss_starters"]]; got != 1 {
		t.Errorf("one starter out, got %v", got)
	}

	// The live path must agree: ruling the star out gives the same features.
	live := b.Live(12, 1, 2, true, "2025-11-23", map[int]bool{11: true})
	if live == nil {
		t.Fatal("live row is nil")
	}
	if live.X[mm] != 36 {
		t.Errorf("live miss_min %v, want 36", live.X[mm])
	}
	none := b.Live(12, 1, 2, true, "2025-11-23", nil)
	if none.X[mm] != 0 {
		t.Errorf("with nobody out, miss_min should be 0, got %v", none.X[mm])
	}
}

func TestRestFeatures(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	// Games are every other day: rest = 2, not a back-to-back.
	r := b.Live(12, 1, 2, true, "2025-11-23", nil)
	if r.X[commonIdx["rest"]] != 2 || r.X[commonIdx["b2b"]] != 0 {
		t.Errorf("rest %v b2b %v", r.X[commonIdx["rest"]], r.X[commonIdx["b2b"]])
	}
	r = b.Live(12, 1, 2, true, "2025-11-22", nil) // day after the last game (day 21)
	if r.X[commonIdx["b2b"]] != 1 {
		t.Errorf("a game the day after the last should be a back-to-back, got b2b %v", r.X[commonIdx["b2b"]])
	}
}

func TestVecAndSelection(t *testing.T) {
	disabled := map[string]bool{GroupTeammates: true, GroupUsage: true}
	names := StatNames(data.Points, disabled)
	for _, n := range names {
		if n == "miss_s" || n == "usage_tr" || n == "miss_min" {
			t.Errorf("%s should be disabled", n)
		}
	}
	mn := MinutesNames(disabled)
	has := map[string]bool{}
	for _, n := range mn {
		has[n] = true
	}
	if !has["m_ewma"] || has["miss_min"] || has["pace"] {
		t.Errorf("minutes names wrong: %v", mn)
	}
}

// A player who has left the team (traded or signed elsewhere) must not count as a teammate who is out.
func TestRosterExcludesDepartedPlayers(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	mm := commonIdx["miss_min"]
	// Without roster info the departed star (player 11) looks like a rotation player who is out.
	if r := b.Live(12, 1, 2, true, "2025-11-30", map[int]bool{11: true}); r.X[mm] == 0 {
		t.Fatal("setup: expected the star to count as out before the roster is known")
	}
	// Now the roster says player 11 plays for team 9 and the rest stay on team 1.
	b.SetRoster(map[int]int{11: 9, 12: 1, 13: 1, 14: 1, 15: 1})
	if r := b.Live(12, 1, 2, true, "2025-11-30", map[int]bool{11: true}); r.X[mm] != 0 {
		t.Errorf("a player now on another team is not a teammate who is out, got miss_min %v", r.X[mm])
	}
	// A teammate who is still on the team and ruled out still counts.
	if r := b.Live(12, 1, 2, true, "2025-11-30", map[int]bool{13: true}); r.X[mm] == 0 {
		t.Error("a current teammate who is out should still count")
	}
}

func TestReturningFromAbsence(t *testing.T) {
	// Player 15 plays ten games, then misses the next fifteen team games.
	var games []data.PlayerGame
	for _, g := range miniLeagueN(25) {
		if g.PlayerID == 15 && g.GameID >= 110 {
			continue
		}
		games = append(games, g)
	}
	b := NewBuilder(games, nil)
	r := b.Live(15, 1, 2, true, "2025-12-21", nil)
	if r == nil {
		t.Fatal("nil row")
	}
	if got := r.X[commonIdx["missed"]]; got != 15 {
		t.Errorf("missed = %v, want 15 team games since his last game", got)
	}
	if r.X[commonIdx["returning"]] != 1 {
		t.Error("fifteen missed games should flag a return from absence")
	}
	// A regular player who played every game has missed nothing.
	if got := b.Live(12, 1, 2, true, "2025-12-21", nil).X[commonIdx["missed"]]; got != 0 {
		t.Errorf("player who never missed a game: missed = %v", got)
	}
}

func TestHeadToHeadAndSplit(t *testing.T) {
	// Player 12 scores 14 against team 2 every game. Make him score 30 in games against team 3.
	var games []data.PlayerGame
	for _, g := range miniLeague() {
		games = append(games, g)
	}
	for i := range games {
		if games[i].PlayerID == 12 && games[i].GameID >= 100 && games[i].GameID <= 105 {
			games[i].Pts = 30
			games[i].HomeTeamID, games[i].VisitorTeamID = 1, 3 // these six games were against team 3
		}
	}
	b := NewBuilder(games, nil)
	pts := data.StatIndex(data.Points)
	vsThree := b.Live(12, 1, 3, true, "2025-11-30", nil).S[pts][statIdx[pts]["h2h"]]
	vsTwo := b.Live(12, 1, 2, true, "2025-11-30", nil).S[pts][statIdx[pts]["h2h"]]
	if !(vsThree > 0 && vsTwo < 0) {
		t.Errorf("he scores more against team 3 than team 2: h2h %v vs %v", vsThree, vsTwo)
	}
	if math.Abs(vsThree) > 0.25 {
		t.Errorf("h2h should be shrunk hard, got %v", vsThree)
	}
	if v := b.Live(12, 1, 9, true, "2025-11-30", nil).S[pts][statIdx[pts]["h2h"]]; !math.IsNaN(v) {
		t.Errorf("no meetings with team 9: h2h should be missing, got %v", v)
	}
}

func TestPosGroup(t *testing.T) {
	for pos, want := range map[string]int{"G": 0, "G-F": 1, "F-G": 1, "F": 2, "": 2, "F-C": 3, "C-F": 3, "C": 4, "g": 0, "PG": 0, "C-G": 4} {
		if got := PosGroup(pos); got != want {
			t.Errorf("PosGroup(%q) = %d, want %d", pos, got, want)
		}
	}
}

func TestSeasonPhase(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	// The 12th game is on the date itself, so 11 team games are before it: 1 - 11/15.
	r := b.Live(12, 1, 2, true, "2025-11-23", nil)
	if e := r.X[commonIdx["early"]]; e < 0.26 || e > 0.27 {
		t.Errorf("early = %v, want about 0.267", e)
	}
	// A new season with no games yet is fully "early".
	r = b.Live(12, 1, 2, true, "2026-10-25", nil)
	if r == nil {
		t.Fatal("no row")
	}
	if e := r.X[commonIdx["early"]]; e != 1 {
		t.Errorf("opener early = %v, want 1", e)
	}
}

func TestGameLineFeatures(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	r := b.Live(12, 1, 2, true, "2025-11-23", nil)
	if v := r.X[commonIdx["abs_spread"]]; !math.IsNaN(v) {
		t.Errorf("without a line the feature should be empty, got %v", v)
	}
	b.AddGameLine(data.DayNumber("2025-11-23"), 1, GameLine{Spread: -7.5, Total: 226.5})
	r = b.Live(12, 1, 2, true, "2025-11-23", nil)
	if r.X[commonIdx["abs_spread"]] != 7.5 || r.X[commonIdx["fav_spread"]] != -7.5 || r.X[commonIdx["game_total"]] != 226.5 {
		t.Errorf("line features wrong: %v %v %v", r.X[commonIdx["abs_spread"]], r.X[commonIdx["fav_spread"]], r.X[commonIdx["game_total"]])
	}
}

func TestChancesFeatures(t *testing.T) {
	w := synthworld.Generate(synthworld.Config{Seed: 7, Seasons: []int{2024}, Teams: 10, Days: 100})
	rows := NewBuilder(w.Games, w.Adv).Rows(0)
	reb := data.StatIndex(data.Rebounds)
	ast := data.StatIndex(data.Assists)
	filled := 0
	for _, r := range rows {
		for _, n := range []string{"rch_tr", "rch_conv", "rch_off"} {
			v := r.S[reb][statIdx[reb][n]]
			if math.IsInf(v, 0) {
				t.Fatalf("%s is infinite", n)
			}
			if !math.IsNaN(v) {
				filled++
				if math.Abs(v) > 2 {
					t.Fatalf("%s = %v is implausibly large for a shrunk log ratio", n, v)
				}
			}
		}
		// The synthetic feed has no secondary assists, so that feature must stay empty.
		if !math.IsNaN(r.S[ast][statIdx[ast]["sec_tr"]]) {
			t.Fatal("sec_tr should be NaN without secondary assist data")
		}
	}
	if filled == 0 {
		t.Fatal("no chance features were ever filled")
	}
	// Disabling the group removes the features from the model's inputs.
	names := StatNames(data.Rebounds, map[string]bool{GroupChances: true})
	for _, n := range names {
		if n == "rch_tr" || n == "rch_conv" || n == "rch_off" {
			t.Fatalf("%s still selected with the chances group off", n)
		}
	}
}

func TestRatioTrendShrinks(t *testing.T) {
	var q ratio
	for i := 0; i < 40; i++ {
		q.add(i >= 30, 10, 10) // steady rate of 1.0
	}
	if v := q.trend(40, 50, 100); math.Abs(v) > 1e-9 {
		t.Fatalf("a flat rate should have zero trend, got %v", v)
	}
	var s ratio
	for i := 0; i < 40; i++ {
		n := 10.0
		if i >= 30 {
			n = 20 // recent rate doubles
		}
		s.add(i >= 30, n, 10)
	}
	v := s.trend(40, 50, 100)
	if v <= 0 || v >= math.Log(2) {
		t.Fatalf("a doubled recent rate should give a positive trend below ln 2 after shrinkage, got %v", v)
	}
	if !math.IsNaN(s.trend(40, 500, 100)) {
		t.Fatal("too little recent data should give NaN")
	}
}

func TestAdjustBlowoutsRaisesOnlyShortfalls(t *testing.T) {
	mk := func(id int, min float64) data.PlayerGame { return data.PlayerGame{GameID: id, Minutes: min} }
	// Five normal 34-minute games, a 24-minute night in a rout, a 38-minute night in a rout.
	h := []data.PlayerGame{mk(1, 34), mk(2, 34), mk(3, 34), mk(4, 34), mk(5, 34), mk(6, 24), mk(7, 38)}
	margin := func(g data.PlayerGame) (float64, bool) {
		if g.GameID >= 6 {
			return -25, true
		}
		return 5, true
	}
	got, cut := adjustBlowouts(h, margin)
	if got[5].Minutes != 34 {
		t.Errorf("a short night in a blowout should be raised to 34, got %v", got[5].Minutes)
	}
	if got[6].Minutes != 38 {
		t.Errorf("a long night in a blowout should be left alone, got %v", got[6].Minutes)
	}
	if h[5].Minutes != 24 {
		t.Error("the input must not be modified")
	}
	if want := 10.0 / 7; math.Abs(cut-want) > 1e-9 {
		t.Errorf("average minutes added = %v, want %v", cut, want)
	}
}

func TestAdjustBlowoutsNeedsEnoughNormalGames(t *testing.T) {
	h := []data.PlayerGame{{GameID: 1, Minutes: 20}, {GameID: 2, Minutes: 22}}
	got, cut := adjustBlowouts(h, func(data.PlayerGame) (float64, bool) { return -30, true })
	if cut != 0 || got[0].Minutes != 20 || got[1].Minutes != 22 {
		t.Errorf("with no normal games nothing should change, got %v cut %v", got, cut)
	}
}

func TestMinutesShapeHelpers(t *testing.T) {
	h := []data.PlayerGame{{Minutes: 30}, {Minutes: 10}, {Minutes: 32}, {Minutes: 34}}
	if got := medianMinutes(h); got != 31 {
		t.Errorf("median = %v, want 31", got)
	}
	if got := shareBelow(h, 20); got != 0.25 {
		t.Errorf("share below 20 = %v, want 0.25", got)
	}
	if !math.IsNaN(medianMinutes(nil)) || !math.IsNaN(shareBelow(nil, 10)) {
		t.Error("empty history should give NaN")
	}
}

// The other team's missing players show up as features, from box scores and from the live out list.
func TestOpponentAbsenceFeatures(t *testing.T) {
	games := miniLeague()
	for i := range games {
		if games[i].PlayerID == 11 { // the star is a center with 2 blocks a game
			games[i].Pos = "C"
			games[i].Blk = 2
		}
	}
	b := NewBuilder(games, nil)
	var present, absent *Row
	rows := b.Rows(0)
	for i := range rows {
		r := &rows[i]
		if r.G.PlayerID == 22 && r.G.GameID == 110 {
			present = r // the star played for the other team
		}
		if r.G.PlayerID == 22 && r.G.GameID == 111 {
			absent = r // the star is out
		}
	}
	if present == nil || absent == nil {
		t.Fatal("expected rows for player 22 in games 110 and 111")
	}
	for _, name := range []string{"oppabs_min", "oppabs_big", "oppabs_blk", "oppabs_reb"} {
		if v := present.X[commonIdx[name]]; v != 0 {
			t.Errorf("%s should be 0 when the opposing star played, got %v", name, v)
		}
	}
	for name, want := range map[string]float64{"oppabs_min": 36, "oppabs_big": 36, "oppabs_blk": 2, "oppabs_reb": 6, "oppabs_starters": 1} {
		if got := absent.X[commonIdx[name]]; got != want {
			t.Errorf("%s with the opposing star out = %v, want %v", name, got, want)
		}
	}
	if absent.X[commonIdx["oppabs_pts"]] <= 0 {
		t.Error("oppabs_pts should count the missing star's scoring")
	}
	if !b.IsStarter(1, absent.G.Day, 11) {
		t.Error("the 36-minute star should count as a starter")
	}
	// The live path agrees: ruling him out for tomorrow's game gives the same numbers.
	live := b.Live(22, 2, 1, false, "2025-11-23", map[int]bool{11: true})
	if live == nil {
		t.Fatal("live row is nil")
	}
	if got := live.X[commonIdx["oppabs_big"]]; got != 36 {
		t.Errorf("live oppabs_big = %v, want 36", got)
	}
	// The minutes model must not see these features.
	for _, n := range MinutesNames(nil) {
		if n == "oppabs_min" || n == "oppabs_big" || n == "oppabs_blk" || n == "oppabs_reb" {
			t.Errorf("minutes model should not use %s", n)
		}
	}
}

func TestAdjustShortExits(t *testing.T) {
	mk := func(id int, min float64) data.PlayerGame { return data.PlayerGame{GameID: id, Minutes: min} }
	close := func(data.PlayerGame) (float64, bool) { return 4, true }
	// An isolated 9-minute night (an injury exit) among 32-minute games is filled in.
	h := []data.PlayerGame{mk(1, 32), mk(2, 33), mk(3, 31), mk(4, 32), mk(5, 9), mk(6, 32), mk(7, 33)}
	got := adjustShortExits(h, close)
	if got[4].Minutes != 32 {
		t.Errorf("an isolated short night should be raised to the median 32, got %v", got[4].Minutes)
	}
	if h[4].Minutes != 9 {
		t.Error("the input must not be modified")
	}
	// Two short nights in the last five is a lasting change and is left alone.
	h2 := []data.PlayerGame{mk(1, 32), mk(2, 33), mk(3, 31), mk(4, 32), mk(5, 12), mk(6, 11), mk(7, 33)}
	got2 := adjustShortExits(h2, close)
	if got2[4].Minutes != 12 || got2[5].Minutes != 11 {
		t.Errorf("repeated short nights must be kept, got %v %v", got2[4].Minutes, got2[5].Minutes)
	}
	// A short night in a blowout is not an exit (adjustBlowouts handles it).
	rout := func(g data.PlayerGame) (float64, bool) { return -25, g.GameID == 5 }
	if got3 := adjustShortExits(h, rout); got3[4].Minutes != 9 {
		t.Errorf("blowouts are left to adjustBlowouts, got %v", got3[4].Minutes)
	}
	// Bench players are not touched: low minutes are normal for them.
	bench := []data.PlayerGame{mk(1, 12), mk(2, 11), mk(3, 13), mk(4, 12), mk(5, 3), mk(6, 12), mk(7, 11)}
	if got4 := adjustShortExits(bench, close); got4[4].Minutes != 3 {
		t.Errorf("a bench player's short night should stay, got %v", got4[4].Minutes)
	}
}

func TestDefenseRank(t *testing.T) {
	b := NewBuilder(miniLeague(), nil)
	// Team 1's players outproduce team 2's, so team 2 allows more to forwards than team 1 does.
	r, ok := b.DefenseRank(11, 2, data.Points, "2025-11-24")
	if !ok {
		t.Fatal("expected a ranking")
	}
	if r.Rank != 1 || r.Of != 2 || r.Pct <= 0 || r.Early {
		t.Errorf("team 2 should allow the most: %+v", r)
	}
	r2, ok := b.DefenseRank(21, 1, data.Points, "2025-11-24")
	if !ok || r2.Rank != 2 || r2.Pct >= 0 {
		t.Errorf("team 1 should allow the fewest: %+v ok=%v", r2, ok)
	}
	if _, ok := b.DefenseRank(11, 0, data.Points, "2025-11-24"); ok {
		t.Error("no opponent means no ranking")
	}
	if PosName(4) != "centers" || PosName(0) != "guards" || PosName(2) != "forwards" {
		t.Error("position names")
	}
}

func TestExpectedMissesFeature(t *testing.T) {
	w := synthworld.Generate(synthworld.Config{Seed: 9, Seasons: []int{2024}, Teams: 10, Days: 100})
	rows := NewBuilder(w.Games, w.Adv).Rows(0)
	reb := data.StatIndex(data.Rebounds)
	idx := statIdx[reb]["exp_miss"]
	filled := 0
	for _, r := range rows {
		v := r.S[reb][idx]
		if math.IsInf(v, 0) {
			t.Fatal("exp_miss is infinite")
		}
		if !math.IsNaN(v) {
			filled++
			if math.Abs(v) > 0.5 {
				t.Fatalf("exp_miss = %v: a shrunk log ratio to a typical game should stay close to 0", v)
			}
		}
	}
	if filled == 0 {
		t.Fatal("expected-misses was never filled")
	}
	// Off unless asked for: the default feature set must not include it.
	if hasName(StatNames(data.Rebounds, map[string]bool{GroupMisses: true}), "exp_miss") {
		t.Fatal("exp_miss selected with the misses group off")
	}
	if !hasName(StatNames(data.Rebounds, map[string]bool{}), "exp_miss") {
		t.Fatal("exp_miss missing when the group is on")
	}
}

func hasName(names []string, n string) bool {
	for _, x := range names {
		if x == n {
			return true
		}
	}
	return false
}
