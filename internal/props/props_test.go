package props

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"propsmodel/internal/data"
)

const sample = `{"data":[
 {"id":1,"game_id":50,"player_id":7,"vendor":"draftkings","prop_type":"points","line_value":"24.5","market":{"type":"over_under","over_odds":-115,"under_odds":-105},"updated_at":"2026-10-20T20:00:00Z"},
 {"id":2,"game_id":50,"player_id":7,"vendor":"fanduel","prop_type":"rebounds","line_value":8.5,"market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"id":3,"game_id":50,"player_id":7,"vendor":"fanduel","prop_type":"points","line_value":"20","market":{"type":"milestone","odds":150}},
 {"id":4,"game_id":50,"player_id":7,"vendor":"fanduel","prop_type":"steals","line_value":"2.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}
]}`

func TestParseResponse(t *testing.T) {
	got := ParseResponse(json.RawMessage(sample), 0)
	if len(got) != 2 {
		t.Fatalf("want 2 over/under points+rebounds lines, got %d: %+v", len(got), got)
	}
	if got[0].Stat != data.Points || got[0].Line != 24.5 || got[0].OverOdds != -115 || got[0].Vendor != "draftkings" {
		t.Errorf("unexpected first line: %+v", got[0])
	}
	if got[1].Stat != data.Rebounds || got[1].Line != 8.5 {
		t.Errorf("unexpected second line: %+v", got[1])
	}
}

func TestLoadSnapshotKeepsLatestCapture(t *testing.T) {
	dir := t.TempDir()
	first := `{"captured_at":"t1","game_id":50,"game":{},"response":{"data":[{"game_id":50,"player_id":7,"vendor":"draftkings","prop_type":"points","line_value":"24.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}]}}`
	second := `{"captured_at":"t2","game_id":50,"game":{},"response":{"data":[{"game_id":50,"player_id":7,"vendor":"draftkings","prop_type":"points","line_value":"25.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}]}}`
	p := filepath.Join(dir, "props_2026-10-20.jsonl")
	if err := os.WriteFile(p, []byte(first+"\n"+second+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSnapshot(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Line != 25.5 {
		t.Fatalf("want the latest capture only (25.5), got %+v", got)
	}
}

func TestLoadOpeningUsesWrapperGameID(t *testing.T) {
	dir := t.TempDir()
	row := `{"_game_id":77,"response":{"data":[{"player_id":3,"vendor":"betmgm","prop_type":"assists","line_value":"6.5","market":{"type":"over_under","over_odds":-120,"under_odds":100}}]}}`
	p := filepath.Join(dir, "opening_props_2025.jsonl")
	if err := os.WriteFile(p, []byte(row+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOpening(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GameID != 77 || got[0].Stat != data.Assists {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestLoadGamesAndOut(t *testing.T) {
	dir := t.TempDir()
	gp := dir + "/props.jsonl"
	os.WriteFile(gp, []byte(`{"captured_at":"x","game_id":7,"game":{"id":7,"date":"2026-10-20","home_team":{"id":3},"visitor_team":{"id":9}},"response":{"data":[]}}
{"captured_at":"x","game_id":8,"game":{"id":8,"home_team_id":4,"visitor_team_id":5},"response":{"data":[]}}
{"captured_at":"x","game_id":9,"game":{},"response":{"data":[]}}
`), 0o644)
	g, err := LoadGames(gp)
	if err != nil {
		t.Fatal(err)
	}
	if g[7].Home != 3 || g[7].Visitor != 9 || g[8].Home != 4 || g[8].Visitor != 5 || len(g) != 2 {
		t.Errorf("games parsed wrong: %+v", g)
	}

	ip := dir + "/inj.jsonl"
	os.WriteFile(ip, []byte(`{"captured_at":"a","row":{"status":"Out","player":{"id":1}}}
{"captured_at":"a","row":{"status":"Day-To-Day","player":{"id":2}}}
{"captured_at":"a","row":{"status":"Doubtful","player":{"id":3}}}
{"captured_at":"b","row":{"status":"Day-To-Day","player":{"id":1}}}
{"captured_at":"b","row":{"status":"Out For Season","player":{"id":4}}}
`), 0o644)
	out, err := LoadOut(ip, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out[1]; ok {
		t.Error("player 1's latest status is Day-To-Day, so should not be out")
	}
	if _, ok := out[4]; !ok || len(out) != 1 {
		t.Errorf("want only player 4 out, got %v", out)
	}
	out, _ = LoadOut(ip, true)
	if _, ok := out[3]; !ok {
		t.Error("includeDoubtful should include player 3")
	}
}

func TestLoadActiveTeams(t *testing.T) {
	p := filepath.Join(t.TempDir(), "players.jsonl")
	os.WriteFile(p, []byte(`{"id":5,"team":{"id":21},"first_name":"A"}
{"id":6,"team_id":9}
{"id":7}
`), 0o644)
	m, err := LoadActiveTeams(p)
	if err != nil || m[5] != 21 || m[6] != 9 || len(m) != 2 {
		t.Errorf("got %v err %v", m, err)
	}
}

func TestThreesLinesAreParsed(t *testing.T) {
	body := []byte(`{"data":[{"game_id":50,"player_id":7,"vendor":"fanduel","prop_type":"threes","line_value":"2.5","market":{"type":"over_under","over_odds":-120,"under_odds":100}}]}`)
	lines := ParseResponse(body, 0)
	if len(lines) != 1 || lines[0].Stat != data.Threes || lines[0].Line != 2.5 {
		t.Fatalf("want one threes line at 2.5, got %+v", lines)
	}
}

func TestLineMoves(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/props.jsonl"
	row := func(at string, lines ...string) string {
		s := `{"captured_at":"` + at + `","game_id":50,"response":{"data":[`
		for i, l := range lines {
			if i > 0 {
				s += ","
			}
			s += l
		}
		return s + `]}}` + "\n"
	}
	prop := func(vendor, line string) string {
		return `{"game_id":50,"player_id":7,"vendor":"` + vendor + `","prop_type":"points","line_value":"` + line + `","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}`
	}
	body := row("2026-10-26T13:17:00Z", prop("a", "24.5"), prop("b", "24.5"), prop("c", "24.5")) +
		row("2026-10-26T22:17:00Z", prop("a", "25.5"), prop("b", "25.5"), prop("c", "24.5"))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	moves, err := LineMoves(path)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := moves[MoveKey{50, 7, data.Points}]
	if !ok {
		t.Fatal("expected a move for the player")
	}
	if m.From != 24.5 || m.To != 25.5 || m.Books != 3 || m.Up != 2 || m.Down != 0 {
		t.Errorf("unexpected move %+v", m)
	}
	if !m.Moved() {
		t.Error("two of three books moved a full point: that is a move")
	}
	// A single capture has nothing to compare.
	if err := os.WriteFile(path, []byte(row("2026-10-26T13:17:00Z", prop("a", "24.5"), prop("b", "24.5"), prop("c", "24.5"))), 0o644); err != nil {
		t.Fatal(err)
	}
	if moves, _ := LineMoves(path); len(moves) != 0 {
		t.Errorf("one capture should give no moves, got %v", moves)
	}
}
