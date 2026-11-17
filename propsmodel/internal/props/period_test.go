package props

import (
	"encoding/json"
	"testing"
)

func TestPartialPeriodsNeverCompeteWithFullGame(t *testing.T) {
	body := json.RawMessage(`{"data":[
 {"player_id":7,"prop_type":"points","line_value":"24.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points_1q","line_value":"6.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points","period":"1q","line_value":"6.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points","line_value":"6.5","market":{"period_id":"1h","type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points_rebounds","period":1,"line_value":"8.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}
 ]}`)
	lines := ParseResponse(body, 50)
	if len(lines) != 1 || lines[0].Line != 24.5 {
		t.Fatalf("full-game lines: %+v", lines)
	}
	if lines := ParseCombos(body, 50); len(lines) != 0 {
		t.Fatalf("partial combo admitted: %+v", lines)
	}
}
