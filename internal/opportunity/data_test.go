package opportunity

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/props"
	"testing"
	"time"
)

func TestTrackingScopeNullZeroFutureAndCoverage(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "context"), 0755)
	var games []data.PlayerGame
	var lines string
	for i := 1; i <= 5; i++ {
		g := data.PlayerGame{PlayerID: 1, GameID: i, TeamID: 1, Date: fmt.Sprintf("2026-11-%02d", i), SourceFinal: true, BoxScoreComplete: true, ShootingComplete: true, Minutes: 30, Pts: 10, Reb: 5, Ast: 5, Fg3a: 5, Fg3m: 2, Fga: 10, Fta: 2}
		games = append(games, g)
		row := fmt.Sprintf(`{"captured_at":"2026-11-05T10:00:00Z","row":{"player_id":1,"game_id":%d,"team_id":1,"type":"passing","stats":{"potential_ast":10}}}`, i)
		lines += row + "\n"
	}
	// A later capture must not become a pregame input; a same-day game is also excluded.
	lines += `{"captured_at":"2026-11-07T10:00:00Z","row":{"player_id":1,"game_id":2,"team_id":1,"type":"passing","stats":{"potential_ast":999}}}` + "\n"
	os.WriteFile(filepath.Join(dir, "context", "passing_2026.jsonl"), []byte(lines), 0600)
	ix, e := Load(dir, games)
	if e != nil {
		t.Fatal(e)
	}
	ix.AsOf, _ = time.Parse(time.RFC3339, "2026-11-05T18:00:00Z")
	s := ix.Signals(1, "2026-11-05", [4]float64{.3, .16, 1. / 6, .06})[2]
	if !s.Available || s.Games != 3 || s.Chances != 30 || s.Through != "2026-11-04" {
		t.Fatal(s)
	}
	zero := 0.
	o := Observation{Stats: map[string]*float64{"zero": &zero, "missing": nil}}
	if o.Value("zero") != 0 || !math.IsNaN(o.Value("missing")) {
		t.Fatal("null became zero")
	}
	// The source slice was not rewritten by the context loader.
	if games[0].Ast != 5 {
		t.Fatal("input changed")
	}
}
func TestPregameFeedDistinguishesExpectedAndRejectsPostTip(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-11-18T18:00:00Z")
	p := Projected{ID: 1, Game: 7, Team: 1, Status: "expected", Updated: "2026-11-18T17:00:00Z"}
	roster := map[int]int{}
	for i, slot := range []string{"PG", "SG", "SF", "PF", "C"} {
		s := Starter{Position: slot}
		s.Player.ID = i + 1
		p.Lineup = append(p.Lineup, s)
		roster[i+1] = 1
	}
	body, _ := json.Marshal(map[string]any{"captured_at": "2026-11-18T17:59:00Z", "row": p})
	path := filepath.Join(t.TempDir(), "projected.jsonl")
	os.WriteFile(path, body, 0600)
	games := map[int]props.GameInfo{7: {Home: 1, Visitor: 2, HomeAbbr: "BOS", Start: "2026-11-18T23:00:00Z"}}
	got, e := LoadPregame(path, games, roster, now)
	if e != nil || len(got) != 1 || got[1].Status != "expected" || got[1].TeamAbbr != "BOS" {
		t.Fatal(got, e)
	}
	later := now.Add(6 * time.Hour)
	got, e = LoadPregame(path, games, roster, later)
	if e != nil || len(got) != 0 {
		t.Fatal("post-tip lineup used", got, e)
	}
	p.Lineup[4].Player.ID = 1
	body, _ = json.Marshal(map[string]any{"captured_at": "2026-11-18T17:59:00Z", "row": p})
	os.WriteFile(path, body, 0600)
	got, _ = LoadPregame(path, games, roster, now)
	if len(got) != 0 {
		t.Fatal("duplicate starter accepted")
	}
}
