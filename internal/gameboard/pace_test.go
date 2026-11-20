package gameboard

import (
	"os"
	"path/filepath"
	"propsmodel/internal/gamepace"
	"testing"
)

func TestPaceChallengerKeepsItsOwnFrozenRecordAndNeverSelectsStakes(t *testing.T) {
	dir := t.TempDir()
	old := []byte("original board")
	os.WriteFile(filepath.Join(dir, "gameboard.json"), old, 0600)
	q := Quote{Market: "moneyline", Book: "test", QuoteAt: "2026-11-12T22:59:00Z", SourceUpdatedAt: "2026-11-12T22:58:00Z", Tip: "2026-11-13T00:00:00Z", BookCount: 3, Sides: Pair([2]string{"BOS", "NYK"}, [2]int{-110, -110}, [2]float64{.8, .2}, 0)}
	q.Sides[0].Raw = .8
	q.Sides[1].Raw = .2
	s := Snapshot{Policy: gamepace.Policy, Date: "2026-11-12", At: "2026-11-12T23:00:00Z", Z: map[string]float64{"moneyline": 99}, Games: []Game{{ID: 7, HomeID: 1, AwayID: 2, Home: "BOS", Away: "NYK", Tip: q.Tip, Quotes: []Quote{q}, PaceContext: &gamepace.Context{Date: "2026-11-12"}}}}
	if err := s.SaveNamed(dir, "gamechallenger.json"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "gameboard.json")); string(got) != string(old) {
		t.Fatal("baseline board replaced")
	}
	if err := Log(dir, s, .55, .01); err != nil {
		t.Fatal(err)
	}
	rows, err := readJSONL[Decision](filepath.Join(dir, "gamepace_picklog.jsonl"))
	if err != nil || len(rows) != 1 || rows[0].Tier == "PICK" || rows[0].RawProb != .8 || rows[0].Context == nil || rows[0].BookCount != 3 {
		t.Fatal(rows, err)
	}
	s.Games[0].Quotes[0].Sides[0].Conditional = .9
	if err = Log(dir, s, .55, .01); err != nil {
		t.Fatal(err)
	}
	again, _ := readJSONL[Decision](filepath.Join(dir, "gamepace_picklog.jsonl"))
	if len(again) != 1 || again[0].Prob != rows[0].Prob {
		t.Fatal("first decision changed")
	}
	if _, err = os.Stat(filepath.Join(dir, "v2_gamepicklog.jsonl")); !os.IsNotExist(err) {
		t.Fatal("current game record was modified")
	}
}
