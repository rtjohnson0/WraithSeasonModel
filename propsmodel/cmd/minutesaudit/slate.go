package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type SlateRow struct {
	Stat             string  `json:"stat"`
	Markets          int     `json:"markets"`
	RawOver          int     `json:"rawOver"`
	RawUnder         int     `json:"rawUnder"`
	FinalOver        int     `json:"finalOver"`
	FinalUnder       int     `json:"finalUnder"`
	OverProbBar      int     `json:"overProbBar"`
	UnderProbBar     int     `json:"underProbBar"`
	OverEdgeBar      int     `json:"overEdgeBar"`
	UnderEdgeBar     int     `json:"underEdgeBar"`
	PostedOver       int     `json:"postedOver"`
	PostedUnder      int     `json:"postedUnder"`
	AvgProjectionGap float64 `json:"avgProjectionGap"`
	AvgMinutesShift  float64 `json:"avgMinutesShift"`
}
type Slate struct {
	Date string     `json:"date"`
	At   string     `json:"at"`
	Rows []SlateRow `json:"rows"`
}

func loadSlate(dir, before string) (*Slate, error) {
	b, e := os.ReadFile(filepath.Join(dir, "side_audit.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var s Slate
	if e = json.Unmarshal(b, &s); e != nil {
		return nil, e
	}
	if _, e = time.Parse("2006-01-02", s.Date); e != nil || s.Date > before {
		return nil, nil
	}
	return &s, nil
}
