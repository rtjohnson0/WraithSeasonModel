package main

import (
	"propsmodel/internal/props"
	"testing"
)

func TestSlateCountsGamesWithoutQuotesOrKnownTipTimes(t *testing.T) {
	games := map[int]props.GameInfo{1: {ID: 1, HomeAbbr: "H", VisitorAbbr: "A", Start: "2026-11-13T00:00:00Z"}, 2: {ID: 2, HomeAbbr: "H2", VisitorAbbr: "A2", Start: "2026-11-13T01:00:00Z"}, 3: {ID: 3, HomeAbbr: "H3", VisitorAbbr: "A3"}, 4: {ID: 4, HomeAbbr: "H4", VisitorAbbr: "A4", Start: "2026-11-12T15:00:00Z"}}
	got := slateGames(games)
	if len(got) != 4 || got[0].ID != 4 || got[3].ID != 3 {
		t.Fatalf("games dropped from slate: %+v", got)
	}
}
