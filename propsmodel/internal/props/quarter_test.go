package props

import (
	"encoding/json"
	"testing"
)

func TestExplicitQuarterPointsStaySeparate(t *testing.T) {
	raw := json.RawMessage(`{"data":[
 {"player_id":7,"prop_type":"points_1q","line_value":"3.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points","period":"1q","line_value":"4.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points","line_value":"24.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}},
 {"player_id":7,"prop_type":"points_1q","period":"game","line_value":"5.5","market":{"type":"over_under","over_odds":-110,"under_odds":-110}}
 ]}`)
	q := ParseQuarterPoints(raw, 9)
	if len(q) != 2 || q[0].Period != "1q" || q[0].Line != 3.5 {
		t.Fatalf("quarter lines %+v", q)
	}
	full := ParseResponse(raw, 9)
	if len(full) != 1 || full[0].Line != 24.5 {
		t.Fatalf("quarter mixed with full game %+v", full)
	}
}
