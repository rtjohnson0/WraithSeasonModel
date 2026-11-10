// Package boardstate separates the current board from the immutable pick history.
package boardstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/picklog"
	"time"
)

type Game struct {
	ID      int    `json:"id"`
	Matchup string `json:"matchup"`
	Tip     string `json:"tip"`
}
type Feed struct {
	Name   string `json:"name"`
	Quotes int    `json:"quotes"`
}
type Status struct {
	ScoredAt             string  `json:"scoredAt"`
	QuotesRead           int     `json:"quotesRead"`
	UsableQuotes         int     `json:"usableQuotes"`
	UnsafeOrDuplicate    int     `json:"unsafeOrDuplicate"`
	ConflictingBooks     int     `json:"conflictingBooks"`
	OffConsensus         int     `json:"offConsensus"`
	CaptureMaxAgeSeconds int     `json:"captureMaxAgeSeconds"`
	PriceMaxAgeSeconds   int     `json:"priceMaxAgeSeconds"`
	MinBooks             int     `json:"minBooks"`
	MaxMarketGap         float64 `json:"maxMarketGap"`
	Feeds                []Feed  `json:"feeds"`
}
type Snapshot struct {
	Date   string          `json:"date"`
	At     string          `json:"at"`
	Plays  []picklog.Entry `json:"plays"`
	Games  []Game          `json:"games"`
	Status Status          `json:"status"`
}

func Load(dir string) (*Snapshot, error) {
	b, err := os.ReadFile(filepath.Join(dir, "board.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
func Save(dir string, s Snapshot) error {
	if s.Plays == nil {
		s.Plays = []picklog.Entry{}
	}
	if s.At == "" {
		s.At = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "board.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
