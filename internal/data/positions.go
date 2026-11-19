package data

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// Corrections require both provider identity and exact name, never height or
// production heuristics. Raw downloads and frozen forecasts stay unchanged.
//
//go:embed position_corrections.json
var positionCorrectionsJSON []byte

type PositionCorrection struct {
	PlayerID   int    `json:"player_id"`
	Name       string `json:"name"`
	Position   string `json:"position"`
	Source     string `json:"source"`
	VerifiedAt string `json:"verified_at"`
	Reason     string `json:"reason"`
}

func PositionCorrections() []PositionCorrection {
	var rows []PositionCorrection
	if err := json.Unmarshal(positionCorrectionsJSON, &rows); err != nil {
		panic(err)
	}
	return rows
}

var checkedPositions = PositionCorrections()

func CorrectPosition(id int, name, raw string) (string, string) {
	for _, c := range checkedPositions {
		if id == c.PlayerID && strings.EqualFold(strings.TrimSpace(name), c.Name) {
			return c.Position, c.Source
		}
	}
	return strings.TrimSpace(raw), ""
}
