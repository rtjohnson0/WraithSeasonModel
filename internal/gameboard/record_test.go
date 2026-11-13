package gameboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func frozen(market, side string, line float64) Decision {
	return Decision{Date: "2026-11-12", At: "2026-11-12T23:00:00Z", GameID: 7, HomeID: 1, AwayID: 2, Home: "BOS", Away: "NYK", Tip: "2026-11-13T00:00:00Z", Market: market, Side: side, Line: line, Odds: -110, Book: "draftkings", QuoteAt: "2026-11-12T22:59:00Z", UpdatedAt: "2026-11-12T22:58:00Z", Prob: .58, Tier: "PICK"}
}
func TestGradeFrozenLineFinalScoresAndIdentity(t *testing.T) {
	dir := t.TempDir()
	rows := []Decision{frozen("moneyline", "NYK", 0), frozen("spread", "BOS", -3), frozen("total", "Under", 217)}
	if e := writeJSONL(filepath.Join(dir, "gamepicklog.jsonl"), rows); e != nil {
		t.Fatal(e)
	}
	// All three are different outcomes on the same actual score; whole-number spread pushes.
	raw := map[string]any{"_stats_endpoint": "/nba/v1/stats", "_stats_fetched_at": "2026-11-13T03:00:00Z", "game": map[string]any{"id": 7, "date": "2026-11-12", "status": "Final", "home_team_id": 1, "visitor_team_id": 2, "home_team_score": 110, "visitor_team_score": 107}}
	b, _ := json.Marshal(raw)
	path := filepath.Join(dir, "stats_2026.jsonl")
	os.WriteFile(path, append(b, '\n'), 0644)
	if e := Settle(dir); e != nil {
		t.Fatal(e)
	}
	got, e := LoadResults(dir)
	if e != nil {
		t.Fatal(e)
	}
	by := map[string]Result{}
	for _, r := range got {
		by[r.Market] = r
	}
	if by["moneyline"].Result != "LOSS" || by["spread"].Result != "PUSH" || by["total"].Result != "PUSH" {
		t.Fatal(got)
	}
	raw["game"].(map[string]any)["visitor_team_id"] = 3
	b, _ = json.Marshal(raw)
	os.WriteFile(path, append(b, '\n'), 0644)
	Settle(dir)
	got, _ = LoadResults(dir)
	for _, r := range got {
		if r.Result != "REVIEW" || r.Units != 0 {
			t.Fatal("identity mismatch graded", r)
		}
	}
}
func TestDoNotGradePartialQuarterOrUnfetchedScores(t *testing.T) {
	for _, extra := range []map[string]any{{"period": 1}, {"_stats_endpoint": "/other"}, {"_stats_fetched_at": ""}} {
		dir := t.TempDir()
		writeJSONL(filepath.Join(dir, "gamepicklog.jsonl"), []Decision{frozen("total", "Over", 210.5)})
		raw := map[string]any{"_stats_endpoint": "/nba/v1/stats", "_stats_fetched_at": "2026-11-13T03:00:00Z", "game": map[string]any{"id": 7, "date": "2026-11-12", "status": "Final", "home_team_id": 1, "visitor_team_id": 2, "home_team_score": 110, "visitor_team_score": 107}}
		for k, v := range extra {
			raw[k] = v
		}
		b, _ := json.Marshal(raw)
		os.WriteFile(filepath.Join(dir, "stats_2026.jsonl"), append(b, '\n'), 0644)
		if e := Settle(dir); e != nil {
			t.Fatal(e)
		}
		got, _ := LoadResults(dir)
		if len(got) != 1 || got[0].Result != "PENDING" {
			t.Fatal(got)
		}
	}
}
func TestFirstPublishedDecisionCannotMoveToWinningLine(t *testing.T) {
	dir := t.TempDir()
	q := Quote{Market: "spread", Book: "dk", Line: -5.5, QuoteAt: "2026-11-12T22:59:00Z", SourceUpdatedAt: "2026-11-12T22:58:00Z", Tip: "2026-11-13T00:00:00Z", BookCount: 3, Sides: Pair([2]string{"BOS", "NYK"}, [2]int{-110, -110}, [2]float64{.6, .4}, 0)}
	s := Snapshot{Date: "2026-11-12", At: "2026-11-12T23:00:00Z", Z: map[string]float64{"spread": 3}, Games: []Game{{ID: 7, HomeID: 1, AwayID: 2, Home: "BOS", Away: "NYK", Tip: q.Tip, Quotes: []Quote{q}}}}
	if e := Log(dir, s, .55, .01); e != nil {
		t.Fatal(e)
	}
	s.Games[0].Quotes[0].Line = -1.5
	s.At = "2026-11-12T23:05:00Z"
	Log(dir, s, .55, .01)
	got, _ := readJSONL[Decision](filepath.Join(dir, "gamepicklog.jsonl"))
	if len(got) != 1 || got[0].Line != -5.5 || got[0].Tier != "PICK" {
		t.Fatal(got)
	}
	s.Date = "2026-11-13"
	s.At = "2026-11-13T00:01:00Z"
	Log(dir, s, .55, .01)
	got, _ = readJSONL[Decision](filepath.Join(dir, "gamepicklog.jsonl"))
	if len(got) != 1 {
		t.Fatal("live pick logged", got)
	}
}
