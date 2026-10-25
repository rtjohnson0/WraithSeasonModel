package main

import (
	"strings"
	"testing"

	"propsmodel/internal/props"
)

func TestFindGame(t *testing.T) {
	games := map[int]props.GameInfo{
		1: {ID: 1, HomeAbbr: "ORL", VisitorAbbr: "CHA"},
		2: {ID: 2, HomeAbbr: "BOS", VisitorAbbr: "NYK"},
	}
	for q, want := range map[string]int{"ORL": 1, "cha": 1, "ORL@CHA": 1, "cha vs orl": 1, "NYK BOS": 2, "2": 2} {
		got, err := findGame(q, games)
		if err != nil || got != want {
			t.Errorf("%q: got %d, %v; want %d", q, got, err, want)
		}
	}
	if _, err := findGame("LAL", games); err == nil || !strings.Contains(err.Error(), "ORL") {
		t.Errorf("an unknown team should list the slate, got %v", err)
	}
	if _, err := findGame("ORL BOS", games); err == nil {
		t.Error("two teams from different games should not match")
	}
}
