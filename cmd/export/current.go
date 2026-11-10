package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/boardstate"
	"propsmodel/internal/picklog"
	"propsmodel/internal/runlog"
	"sort"
)

// currentBoard never reconstructs an executable board from the permanent log.
// An empty successful run must clear old plays, even if those old plays had a better edge.
func currentBoard(snapshot *boardstate.Snapshot, date string, form Form, runs []runlog.Run) Board {
	var entries []picklog.Entry
	if snapshot != nil && snapshot.Date == date {
		entries = snapshot.Plays
	}
	board := buildBoard(entries, date, form, runs)
	if board.Games == nil {
		board.Games = []Game{}
	}
	if snapshot == nil || snapshot.Date != date {
		return board
	}
	have := map[int]bool{}
	for _, g := range board.Games {
		have[g.ID] = true
	}
	for _, g := range snapshot.Games {
		if !have[g.ID] {
			board.Games = append(board.Games, Game{ID: g.ID, Matchup: g.Matchup, Tip: g.Tip, Plays: []Play{}})
		}
	}
	sort.SliceStable(board.Games, func(i, j int) bool { return board.Games[i].Tip < board.Games[j].Tip })
	return board
}

type ExcludedQuote struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}
type ArchiveData struct {
	Record   Record          `json:"record"`
	Excluded []ExcludedQuote `json:"excluded,omitempty"`
}

func buildArchive(dir string, all []picklog.Settled) *ArchiveData {
	var legacy []picklog.Settled
	for _, r := range all {
		if !r.Verified() {
			legacy = append(legacy, r)
		}
	}
	out := &ArchiveData{Record: buildRecord(legacy, 300)}
	out.Record.ProvenanceNote = "Historical quotes or actual results without current validation. For audit only; excluded from verified performance."
	path := filepath.Join(dir, "exclusions.jsonl")
	b, err := os.ReadFile(path)
	if err == nil {
		for _, line := range splitJSONLines(b) {
			var r ExcludedQuote
			if json.Unmarshal(line, &r) == nil {
				out.Excluded = append(out.Excluded, r)
			}
		}
	}
	if len(legacy) == 0 && len(out.Excluded) == 0 {
		return nil
	}
	return out
}
func splitJSONLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, v := range b {
		if v == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}
