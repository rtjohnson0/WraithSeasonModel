package main

import (
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/props"
)

func TestLinesLogRowsOpenAndStartedGames(t *testing.T) {
	rows := map[marketKey]lineslog.Market{
		{1, 0, 10}: {GameID: 10, PlayerID: 1, Stat: "points", Line: 21.5},
		{2, 0, 11}: {GameID: 11, PlayerID: 2, Stat: "points", Line: 12.5}, // game 11 has already tipped
	}
	series := map[props.MoveKey][]props.SeriesPoint{
		{Game: 10, Player: 1, Stat: data.Points}: {{Line: 20.5}, {Line: 21.5}},
	}
	games := map[int]props.GameInfo{10: {Start: "2026-10-28T23:00:00Z"}, 11: {Start: "2026-10-28T20:00:00Z"}}
	off := []offRow{
		{PlayerID: 1, Player: "A", Stat: data.Points, Side: "UNDER", Line: 23.5, Median: 21.5, Books: 8, Odds: -110, Prob: 0.55},
		{PlayerID: 2, Player: "B", Stat: data.Points, Side: "OVER", Line: 10.5, Median: 12.5, Books: 8, Odds: -110, Prob: 0.5},
	}
	mkt, offs := linesLogRows("2026-10-28", rows, series, off, "2026-10-28T21:00:00Z", games)
	if len(mkt) != 1 || mkt[0].PlayerID != 1 || mkt[0].Open != 20.5 {
		t.Fatalf("started game must be left out and the opening median filled in: %+v", mkt)
	}
	if len(offs) != 1 || offs[0].GameID != 10 || offs[0].Side != "UNDER" || offs[0].PSide != 0.55 {
		t.Errorf("off-market rows wrong: %+v", offs)
	}
}
