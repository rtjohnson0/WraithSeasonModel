package sgo

import (
	"encoding/json"
	"testing"

	"propsmodel/internal/data"
)

const fixture = `{"eventID":"e1","odds":{
 "points-JALEN_DUREN_1_NBA-game-ou-over":{"statID":"points","periodID":"game","betTypeID":"ou","sideID":"over","byBookmaker":{
   "draftkings":{"odds":"-115","overUnder":"17.5","available":true,"lastUpdatedAt":"2026-10-20T20:00:00Z"},
   "fanduel":{"odds":"-110","overUnder":18.5,"available":true},
   "betmgm":{"odds":"-105","overUnder":"16.5","available":false}}},
 "points-JALEN_DUREN_1_NBA-game-ou-under":{"byBookmaker":{
   "draftkings":{"odds":"-105","overUnder":"17.5","available":true},
   "fanduel":{"odds":"+100","overUnder":18.5,"available":true}}},
 "rebounds-JALEN_DUREN_1_NBA-game-ou-over":{"byBookmaker":{"draftkings":{"odds":"+120","overUnder":"10.5","available":true}}},
 "points-JALEN_DUREN_1_NBA-1h-ou-over":{"byBookmaker":{"draftkings":{"odds":"-110","overUnder":"8.5","available":true}}},
 "steals-JALEN_DUREN_1_NBA-game-ou-over":{"byBookmaker":{"draftkings":{"odds":"-110","overUnder":"0.5","available":true}}},
 "points-home-game-ml-home":{"byBookmaker":{"draftkings":{"odds":"-150","available":true}}}
}}`

func TestParseEvent(t *testing.T) {
	got := ParseEvent(json.RawMessage(fixture))
	byKey := map[string]PlayerLine{}
	for _, l := range got {
		byKey[string(l.Stat)+"/"+l.Book] = l
	}
	if len(got) != 3 {
		t.Fatalf("want 3 lines (2 points books + 1 rebounds), got %d: %+v", len(got), got)
	}
	dk := byKey["points/draftkings"]
	if dk.Line != 17.5 || dk.OverOdds != -115 || dk.UnderOdds != -105 || dk.Name != "Jalen Duren" {
		t.Errorf("draftkings points parsed wrong: %+v", dk)
	}
	fd := byKey["points/fanduel"]
	if fd.Line != 18.5 || fd.UnderOdds != 100 {
		t.Errorf("fanduel points parsed wrong: %+v", fd)
	}
	if byKey["rebounds/draftkings"].Stat != data.Rebounds {
		t.Error("missing rebounds line")
	}
}

func TestNames(t *testing.T) {
	if NormName("Jaren Jackson Jr.") != NormName(NameFromKey("JAREN_JACKSON_JR_1_NBA")) {
		t.Errorf("%q vs %q", NormName("Jaren Jackson Jr."), NormName(NameFromKey("JAREN_JACKSON_JR_1_NBA")))
	}
	if NormName("De'Aaron Fox") != "de aaron fox" {
		t.Errorf("got %q", NormName("De'Aaron Fox"))
	}
}
