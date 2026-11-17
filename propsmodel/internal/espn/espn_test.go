package espn

import "testing"

const sample = `{"injuries":[{"displayName":"Atlanta Hawks","id":"1","injuries":[
{"status":"Day-To-Day","date":"2026-10-27T21:07Z","shortComment":"Wiggins is questionable for Wednesday's game.","athlete":{"displayName":"Aaron Wiggins","team":{"abbreviation":"ATL"}}},
{"status":"Out","date":"2026-10-20T10:00Z","shortComment":"Out for the season.","athlete":{"displayName":"Some Player","team":{"abbreviation":"ATL"}}},
{"status":"Out","athlete":{"displayName":""}}]}]}`

func TestParse(t *testing.T) {
	in, err := Parse([]byte(sample))
	if err != nil || len(in) != 2 {
		t.Fatalf("parse: %v %d", err, len(in))
	}
	if in[0].Name != "Aaron Wiggins" || in[0].Team != "ATL" || !in[0].IsGameTime() || in[0].IsOut() {
		t.Errorf("first wrong: %+v", in[0])
	}
	if !in[1].IsOut() || in[1].IsGameTime() {
		t.Errorf("second wrong: %+v", in[1])
	}
	if _, err := Parse([]byte(`{"injuries":[]}`)); err == nil {
		t.Error("an empty list should be an error so a format change is noticed")
	}
	if _, err := Parse([]byte(`not json`)); err == nil {
		t.Error("bad json should error")
	}
}
