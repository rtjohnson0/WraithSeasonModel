// Package marketperiod keeps partial-game markets out of the full-game model.
package marketperiod

import (
	"encoding/json"
	"strings"
)

// FullGame accepts absent period metadata for legacy feeds. Explicit unknown
// periods are rejected rather than silently treated as a full game.
func FullGame(period string) bool {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "", "game", "full_game", "full game", "full", "0":
		return true
	}
	return false
}

// Metadata validates every explicit period field on a raw feed object.
func Metadata(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	for _, key := range []string{"period", "period_id", "periodID"} {
		v, ok := obj[key]
		if !ok || string(v) == "null" {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			s = string(v)
		}
		if !FullGame(s) {
			return false
		}
	}
	return true
}

// Stat rejects suffixes such as points_1q and points_first3min.
func Stat(stat string) bool {
	switch stat {
	case "points", "rebounds", "assists", "threes", "pra", "pr", "pa", "ra":
		return true
	}
	return false
}
