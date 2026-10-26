package main

import (
	"encoding/json"
	"testing"

	"propsmodel/internal/picklog"
	"propsmodel/internal/runlog"
)

func e(player string, id int, stat, side, tier string, prob float64, odds int) picklog.Entry {
	return picklog.Entry{Date: "2026-10-24", GameID: id, PlayerID: id*100 + len(player), Player: player, Stat: stat, Side: side,
		Line: 5.5, Odds: odds, Prob: prob, Tier: tier, Units: 0, Matchup: "CHI @ MIL", Team: "CHI"}
}

func TestGradeCapsUnprovenPlays(t *testing.T) {
	if g := grade(0.67, "PICK"); g != "A+" {
		t.Errorf("staked 67%% = %s", g)
	}
	if g := grade(0.67, "PAPER"); g != "B+" {
		t.Errorf("a PAPER play must not earn above B+, got %s", g)
	}
	if g := grade(0.67, "CHECK"); g != "B+" {
		t.Errorf("a CHECK play must not earn above B+, got %s", g)
	}
	if g := grade(0.51, "LEAN"); g != "D" {
		t.Errorf("51%% = %s", g)
	}
	if g := grade(0.59, ""); g != "B+" {
		t.Errorf("an old untiered pick counts as PICK, got %s", g)
	}
}

func TestBoardDedupesAndGroups(t *testing.T) {
	a := e("Alpha", 1, "rebounds", "UNDER", "LEAN", 0.56, -110)
	a2 := e("Alpha", 1, "rebounds", "UNDER", "PAPER", 0.58, -120) // same play, better tier
	a2.PlayerID = a.PlayerID
	b := e("Beta", 2, "assists", "OVER", "PICK", 0.62, -110)
	b.Units = 0.5
	old := e("Gamma", 1, "points", "OVER", "PAPER", 0.54, -110)
	old.Date = "2026-10-23"
	board := buildBoard([]picklog.Entry{a, a2, b, old}, "2026-10-24", nil, nil)
	if board.Plays != 2 {
		t.Fatalf("expected 2 plays after collapsing the repeat and ignoring other days, got %d", board.Plays)
	}
	if board.Staked != 1 {
		t.Fatalf("expected 1 staked play, got %d", board.Staked)
	}
	if len(board.Games) != 2 || board.Games[0].ID != 1 {
		t.Fatalf("games out of order: %+v", board.Games)
	}
	if board.Games[0].Plays[0].Tier != "PAPER" {
		t.Fatalf("the repeat should keep the PAPER version, got %s", board.Games[0].Plays[0].Tier)
	}
}

func TestRecordTiersAndJSON(t *testing.T) {
	mk := func(tier, res string, prob, profit, flat float64, units float64, id int) picklog.Settled {
		return picklog.Settled{
			Entry:  picklog.Entry{Date: "2026-10-23", PlayerID: id, Player: "P", Stat: "rebounds", Side: "OVER", Tier: tier, Prob: prob, Units: units, Matchup: "A @ B"},
			Result: res, Profit: profit, Flat: flat, Actual: 7,
		}
	}
	in := []picklog.Settled{
		mk("PICK", "WIN", 0.6, 0.45, 0.91, 0.5, 1),
		mk("PICK", "LOSS", 0.6, -0.5, -1, 0.5, 2),
		mk("PAPER", "WIN", 0.56, 0.91, 0.91, 0, 3),
		mk("PAPER", "WIN", 0.56, 0.91, 0.91, 0, 3), // duplicate
		mk("LEAN", "VOID", 0.52, 0, 0, 0, 4),
	}
	rec := buildRecord(in, 10)
	if rec.Dups != 1 {
		t.Fatalf("expected 1 duplicate collapsed, got %d", rec.Dups)
	}
	if rec.Tiers[0].Name != "Max plays (staked)" || rec.Tiers[0].Wins != 1 || rec.Tiers[0].Losses != 1 {
		t.Fatalf("staked row wrong: %+v", rec.Tiers[0])
	}
	if u := rec.Tiers[0].Units; u < -0.06 || u > -0.04 {
		t.Fatalf("staked units should be the real profit (-0.05), got %v", u)
	}
	if _, err := json.Marshal(Site{Record: rec}); err != nil {
		t.Fatalf("record does not marshal: %v", err)
	}
	for _, r := range rec.Recent {
		if r.Result == "VOID" && r.Actual != 0 {
			t.Fatalf("a void play should carry no actual")
		}
	}
}

func TestBoardFormAndHits(t *testing.T) {
	a := e("Alpha", 1, "rebounds", "UNDER", "PICK", 0.60, -110)
	a.Line = 6.5
	form := func(pid int, stat, before string) ([]Game5, float64, int) {
		vals := []float64{4, 8, 5, 7, 6}
		var out []Game5
		for i, v := range vals {
			out = append(out, Game5{Date: "2026-10-2" + string(rune('0'+i)), Opp: "AAA", Value: v, Minutes: 30})
		}
		return out, 6.2, 12
	}
	board := buildBoard([]picklog.Entry{a}, "2026-10-24", form, nil)
	p := board.Games[0].Plays[0]
	if p.L5Hits != 3 { // 4, 5 and 6 are under 6.5
		t.Fatalf("an under on 6.5 should have hit 3 of the last 5, got %d", p.L5Hits)
	}
	if p.L5Avg != 6 || p.MinAvg5 != 30 || p.SeasonGP != 12 {
		t.Fatalf("form summary wrong: %+v", p)
	}
}

func TestDailySeriesIsCumulative(t *testing.T) {
	mk := func(date, res string, id int, units, profit, flat float64) picklog.Settled {
		return picklog.Settled{Entry: picklog.Entry{Date: date, PlayerID: id, Stat: "rebounds", Side: "OVER", Tier: "PICK", Units: units},
			Result: res, Profit: profit, Flat: flat}
	}
	rec := buildRecord([]picklog.Settled{
		mk("2026-10-22", "WIN", 1, 0.5, 0.45, 0.91),
		mk("2026-10-23", "LOSS", 2, 0.5, -0.5, -1),
		mk("2026-10-23", "WIN", 3, 0.5, 0.45, 0.91),
	}, 10)
	if len(rec.Daily) != 2 {
		t.Fatalf("expected 2 days, got %d", len(rec.Daily))
	}
	if d := rec.Daily[1]; d.StakedUnits < 0.39 || d.StakedUnits > 0.41 || d.Wins != 1 || d.Losses != 1 {
		t.Fatalf("second day wrong: %+v", d)
	}
}

func TestLabelsDescribeEvidence(t *testing.T) {
	cases := []struct {
		tier string
		prob float64
		want string
	}{
		{"PICK", 0.57, "Max play"}, {"", 0.60, "Max play"}, {"CHECK", 0.66, "Risky play"},
		{"PAPER", 0.64, "Great play"}, {"PAPER", 0.55, "Lean"}, {"LEAN", 0.60, "Lean"},
	}
	for _, c := range cases {
		if got := label(c.tier, c.prob); got != c.want {
			t.Errorf("label(%q, %.2f) = %s, want %s", c.tier, c.prob, got, c.want)
		}
	}
}

func has(bs []Badge, key, tone string) bool {
	for _, b := range bs {
		if b.Key == key && (tone == "" || b.Tone == tone) {
			return true
		}
	}
	return false
}

func TestBadgesAreFromThePicksPointOfView(t *testing.T) {
	soft := Play{Side: "OVER", Why: "back-to-back; soft defense +8%; out: A, B"}
	b := badgesFor(soft)
	if !has(b, "weak-defense", "good") || !has(b, "back-to-back", "bad") || !has(b, "teammates-out", "note") {
		t.Fatalf("over badges wrong: %+v", b)
	}
	under := Play{Side: "UNDER", Why: "soft defense +8%"}
	if !has(badgesFor(under), "weak-defense", "bad") {
		t.Fatalf("a weak defense should be a bad sign for an under")
	}
	hot := Play{Side: "UNDER", SeasonGP: 10, SeasonAvg: 6, L5Avg: 8, ProjMin: 30, MinAvg5: 30,
		Last5: make([]Game5, 5), L5Hits: 0}
	b = badgesFor(hot)
	if !has(b, "hot", "bad") || !has(b, "trend-against", "bad") {
		t.Fatalf("hot streak against an under should warn: %+v", b)
	}
	cold := Play{Side: "UNDER", SeasonGP: 10, SeasonAvg: 8, L5Avg: 5, ProjMin: 20, MinAvg5: 30,
		Last5: make([]Game5, 5), L5Hits: 5}
	b = badgesFor(cold)
	if !has(b, "cold", "good") || !has(b, "minutes-down", "good") || !has(b, "trend-for", "good") {
		t.Fatalf("cold streak and minutes cut should back an under: %+v", b)
	}
	if len(badgesFor(Play{Side: "OVER"})) != 0 {
		t.Fatal("a play with no signals should have no badges")
	}
}

func TestOffMarketLineBadge(t *testing.T) {
	good := badgesFor(Play{Side: "UNDER", Why: "line 3.5 is 1.0 above the market median 2.5 across 8 books: off the market, in your favor; line 3.5 is far from projection 1.0: confirm the line"})
	n := 0
	for _, b := range good {
		if b.Key == "odd-line" {
			n++
			if b.Tone != "good" {
				t.Errorf("a soft line in the pick's favor should be good, got %q", b.Tone)
			}
		}
	}
	if n != 1 {
		t.Errorf("want exactly one odd-line badge, got %d", n)
	}
}

func TestBoardUsesLatestRunAndMarksRepeats(t *testing.T) {
	mk := func(id int, player string) picklog.Entry {
		return picklog.Entry{Date: "2026-10-26", GameID: 1, PlayerID: id, Player: player, Stat: "points", Side: "OVER",
			Line: 20.5, Prob: 0.58, Odds: -110, Tier: "PAPER", Matchup: "AAA @ BBB"}
	}
	rp := func(id int, player string) runlog.Play {
		return runlog.Play{PlayerID: id, Player: player, Stat: "points", Side: "OVER", Tier: "PAPER", Prob: 0.58}
	}
	day := []runlog.Run{
		{Date: "2026-10-26", Label: "9 AM", Plays: []runlog.Play{rp(1, "Stays"), rp(2, "Leaves")}},
		{Date: "2026-10-26", Label: "3 PM", Prev: "9 AM", Plays: []runlog.Play{rp(1, "Stays"), rp(3, "Arrives")},
			Dropped: []runlog.Dropped{{Play: rp(2, "Leaves"), Reason: "line moved 20.5 to 22.5"}}},
	}
	board := buildBoard([]picklog.Entry{mk(1, "Stays"), mk(2, "Leaves"), mk(3, "Arrives")}, "2026-10-26", nil, day)
	names := map[string]Play{}
	for _, g := range board.Games {
		for _, p := range g.Plays {
			names[p.Player] = p
		}
	}
	if _, ok := names["Leaves"]; ok {
		t.Error("a play the latest run dropped must not be on the live board")
	}
	if p := names["Stays"]; !p.Multi || p.Runs != 2 || len(p.SeenIn) != 2 {
		t.Errorf("repeat play = %+v", p)
	}
	if p := names["Arrives"]; p.Multi || p.Runs != 1 {
		t.Errorf("new play = %+v", p)
	}
	if len(board.RunsToday) != 2 || len(board.RunsToday[1].Dropped) != 1 || board.RunsToday[1].New != 1 {
		t.Errorf("runs = %+v", board.RunsToday)
	}
}

func TestMarkShortIgnoresOneCutShortNight(t *testing.T) {
	gs := []Game5{{Minutes: 33}, {Minutes: 31}, {Minutes: 8}, {Minutes: 34}, {Minutes: 32}}
	if got := markShort(gs); got != 32.5 {
		t.Errorf("typical minutes = %v, want 32.5", got)
	}
	if !gs[2].Short || gs[0].Short {
		t.Errorf("only the 8-minute night should be flagged: %+v", gs)
	}
	// A new starter: minutes jumped and stayed up. Nothing is cut short; the median follows him.
	role := []Game5{{Minutes: 14}, {Minutes: 16}, {Minutes: 30}, {Minutes: 31}, {Minutes: 32}}
	if got := markShort(role); got != 30 {
		t.Errorf("role change typical = %v, want 30", got)
	}
	for _, g := range role {
		if g.Short {
			t.Error("a lasting role change must not be flagged as cut short")
		}
	}
}

func TestParseDefense(t *testing.T) {
	d := parseDefense("minutes +3 vs his normal; defense vs centers: 3rd-most rebounds allowed (+9%, of 30 teams)", "OVER")
	if d == nil || d.Rank != 3 || d.Kind != "most" || d.Pos != "centers" || d.Pct != 9 || d.Of != 30 || !d.Helps {
		t.Fatalf("parsed %+v", d)
	}
	if d.Text != "Allows the 3rd-most rebounds to centers (+9%)" {
		t.Errorf("text %q", d.Text)
	}
	u := parseDefense("defense vs guards: 2nd-fewest 3-pointers allowed (-7%, of 30 teams, early season)", "OVER")
	if u == nil || u.Helps || !u.Early || u.Kind != "fewest" {
		t.Fatalf("parsed %+v", u)
	}
	if parseDefense("out: A. Duren", "OVER") != nil {
		t.Error("no defense note means nil")
	}
	p := Play{Side: "UNDER", Defense: parseDefense("defense vs guards: 2nd-fewest 3-pointers allowed (-7%, of 30 teams)", "UNDER")}
	found := false
	for _, b := range badgesFor(p) {
		if b.Key == "tough-defense" && b.Tone == "good" {
			found = true
		}
	}
	if !found {
		t.Error("an under against a top defense should get a good tough-defense badge")
	}
}
