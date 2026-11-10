package props

import (
	"encoding/json"
	"strings"
	"time"
)

// ParseQuarterPoints reads only explicitly identified first-quarter points.
// It never infers a period from a small threshold.
func ParseQuarterPoints(body json.RawMessage, fallbackGame int) []Line {
	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return nil
	}
	var out []Line
	for _, raw := range resp.Data {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		var typ string
		_ = json.Unmarshal(obj["prop_type"], &typ)
		typ = strings.ToLower(typ)
		explicit := typ == "points_1q"
		if typ != "points" && !explicit {
			continue
		}
		valid := true
		check := func(m map[string]json.RawMessage) {
			for _, k := range []string{"period", "period_id", "periodID"} {
				if v, ok := m[k]; ok && string(v) != "null" {
					var p string
					if json.Unmarshal(v, &p) != nil {
						p = string(v)
					}
					p = strings.ToLower(strings.TrimSpace(p))
					if p != "1q" && p != "1" && p != "q1" {
						valid = false
					}
					explicit = true
					m[k] = json.RawMessage(`"game"`)
				}
			}
		}
		check(obj)
		var market map[string]json.RawMessage
		if json.Unmarshal(obj["market"], &market) != nil {
			continue
		}
		check(market)
		if !valid || !explicit {
			continue
		}
		obj["prop_type"] = json.RawMessage(`"points"`)
		obj["market"], _ = json.Marshal(market)
		row, _ := json.Marshal(obj)
		wrapped, _ := json.Marshal(map[string]any{"data": []json.RawMessage{row}})
		for _, l := range ParseResponse(wrapped, fallbackGame) {
			l.Period = "1q"
			out = append(out, l)
		}
	}
	return out
}
func LoadQuarterSnapshot(path string, asOf time.Time) ([]Line, error) {
	return loadSnapshotPeriod(path, false, true, asOf, 6*time.Hour, "1q")
}

// QuarterExecutable applies identical quote checks while preserving period isolation.
func QuarterExecutable(l Line, g GameInfo, now time.Time) bool {
	if l.Period != "1q" {
		return false
	}
	l.Period = "game"
	return Executable(l, g, now, 6*time.Hour, 30*time.Minute)
}
