package main

import (
	"propsmodel/internal/boardstate"
	"propsmodel/internal/picklog"
	"propsmodel/internal/runlog"
	"testing"
)

func TestCurrentBoardNeverSelectsBestHistoricalQuote(t *testing.T) {
	old := picklog.Entry{Date: "2026-11-10", GameID: 9, PlayerID: 7, Player: "P", Stat: "rebounds", Side: "UNDER", Line: 8.5, Book: "draftkings", Odds: -110, Prob: 0.8, Edge: 0.28, Tier: "PICK"}
	current := old
	current.Line = 6.5
	current.Prob = 0.57
	current.Edge = 0.04
	current.Book = "fanduel"
	history := []runlog.Run{{Date: old.Date, Plays: []runlog.Play{{PlayerID: 7, Stat: "rebounds", Side: "UNDER"}}}}
	// The day log still contains old. Only the explicit current snapshot may supply the board.
	snapshot := &boardstate.Snapshot{Date: old.Date, Plays: []picklog.Entry{current}}
	got := currentBoard(snapshot, old.Date, nil, history)
	if got.Plays != 1 || got.Games[0].Plays[0].Line != 6.5 || got.Games[0].Plays[0].Book != "fanduel" {
		t.Fatalf("historical quote leaked %+v", got)
	}
}
func TestZeroPickRunClearsPreviousBoard(t *testing.T) {
	s := &boardstate.Snapshot{Date: "2026-11-10", Games: []boardstate.Game{{ID: 9, Matchup: "A @ B"}}}
	got := currentBoard(s, s.Date, nil, nil)
	if got.Plays != 0 || len(got.Games) != 1 || len(got.Games[0].Plays) != 0 {
		t.Fatalf("zero-run carried forward picks %+v", got)
	}
	missing := currentBoard(nil, s.Date, nil, nil)
	if missing.Games == nil || missing.Plays != 0 {
		t.Fatalf("missing snapshot needs an empty board %+v", missing)
	}
}
func TestExportCarriesQuoteProvenanceAndBreakEven(t *testing.T) {
	p := makePlay(picklog.Entry{QuoteAt: "2026-11-10T12:00:00Z", SourceUpdatedAt: "2026-11-10T11:59:00Z", Line: 6.5, Odds: -200, MarketMedian: 6.5, BookCount: 3}, "2026-11-10", nil, 0, nil)
	if p.QuoteAt == "" || p.BreakEven < 0.666 || p.BreakEven > 0.667 || p.BookCount != 3 {
		t.Fatalf("missing quote facts %+v", p)
	}
}

func TestPartialGameCannotEnterPublishedFullGameBoard(t *testing.T) {
	partial := picklog.Entry{Date: "2026-11-10", GameID: 9, PlayerID: 7, Stat: "points", Period: "1q", Side: "OVER", Line: 3.5, Tier: "PICK"}
	snapshot := &boardstate.Snapshot{Date: partial.Date, Plays: []picklog.Entry{partial}}
	got := currentBoard(snapshot, partial.Date, nil, nil)
	if got.Plays != 0 {
		t.Fatal("quarter pick leaked into full-game board")
	}
	full := partial
	full.Period = "game"
	full.Line = 24.5
	p := makePlay(full, full.Date, nil, 0, nil)
	if p.Period != "game" {
		t.Fatal("period lost in export")
	}
}

func TestCurrentSnapshotDisplaysLeansEvenWhenRunLogOnlyTracksPicks(t *testing.T) {
	e := picklog.Entry{Date: "2026-11-10", Period: "game", GameID: 9, PlayerID: 7, Player: "P", Stat: "points", Side: "OVER", Line: 20.5, Odds: -110, Prob: .54, Edge: .016, Tier: "LEAN"}
	snap := &boardstate.Snapshot{Date: e.Date, Plays: []picklog.Entry{e}}
	board := currentBoard(snap, e.Date, nil, []runlog.Run{{Date: e.Date, Plays: []runlog.Play{}}})
	if board.Plays != 1 || len(board.Games) != 1 || board.Games[0].Plays[0].Tier != "LEAN" {
		t.Fatalf("lean lost from snapshot %+v", board)
	}
}
