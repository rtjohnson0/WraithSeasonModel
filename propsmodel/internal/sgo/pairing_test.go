package sgo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectMismatchedUnavailableAndInvalidPrices(t *testing.T) {
	for _, tc := range []struct {
		name, underLine, price, available string
		want                              int
	}{
		{"valid", "4.5", "-140", "true", 1},
		{"wrong threshold", "1.5", "-140", "true", 0},
		{"unavailable under", "4.5", "-140", "false", 0},
		{"invalid American odds", "4.5", "-40", "true", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := fmt.Sprintf(`{"eventID":"e","odds":{
   "assists-BROOK_LOPEZ_1_NBA-game-ou-over":{"byBookmaker":{"betmgm":{"overUnder":"4.5","odds":"+120","available":true}}},
   "assists-BROOK_LOPEZ_1_NBA-game-ou-under":{"byBookmaker":{"betmgm":{"overUnder":"%s","odds":"%s","available":%s}}}
   }}`, tc.underLine, tc.price, tc.available)
			if got := ParseEvent(json.RawMessage(raw)); len(got) != tc.want {
				t.Fatalf("quotes=%+v", got)
			}
		})
	}
}
func TestLegacySnapshotCannotSupplyUnverifiedPair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sgo.jsonl")
	body := `{"captured_at":"2026-11-10T12:00:00Z","PlayerKey":"P","Name":"Brook Lopez","Stat":"assists","Book":"betmgm","Line":4.5,"OverOdds":120,"UnderOdds":-140}`
	if err := os.WriteFile(path, []byte(body+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadSnapshot(path)
	if err != nil || len(rows) != 0 {
		t.Fatalf("legacy quote trusted: %+v %v", rows, err)
	}
}
