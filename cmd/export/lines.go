package main

import (
	"fmt"
	"os"

	"propsmodel/internal/lineslog"
	"propsmodel/internal/pipeline"
)

// LinesRecord is the graded market log: every line the model scored (not only the plays it posted),
// and the lines that sat off the market. See internal/lineslog.
type LinesRecord struct {
	Since    string       `json:"since,omitempty"`
	Until    string       `json:"until,omitempty"`
	Days     int          `json:"days"`
	Logged   int          `json:"logged"`
	Graded   int          `json:"graded"`
	Model    float64      `json:"brierModel,omitempty"` // Brier score of the model's P(over) at the median line
	Base     float64      `json:"brierBase,omitempty"`  // Brier score of always saying the day's base rate
	Bins     []LinesBin   `json:"bins,omitempty"`
	Slices   []LinesSlice `json:"slices,omitempty"`
	Off      []LinesOff   `json:"off,omitempty"`
	OffCount int          `json:"offGraded"`
}

// LinesBin is one band of the model's P(over) and how often those lines went over.
type LinesBin struct {
	Label     string  `json:"label"`
	N         int     `json:"n"`
	Predicted float64 `json:"predicted"`
	Actual    float64 `json:"actual"`
}

// LinesSlice is a situation and its Over rate against the model's.
type LinesSlice struct {
	Name      string  `json:"name"`
	N         int     `json:"n"`
	OverRate  float64 `json:"overRate"`
	ModelOver float64 `json:"modelOver"`
}

// LinesOff is the record of one group of off-market lines.
type LinesOff struct {
	Name      string  `json:"name"`
	N         int     `json:"n"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	Pushes    int     `json:"pushes"`
	HitRate   float64 `json:"hitRate"`
	Predicted float64 `json:"predicted"`
	Units     float64 `json:"units"`
	ROI       float64 `json:"roi"`
}

// buildLinesRecord grades data/lines against the stats. It returns an empty record (and the site
// shows nothing) when no lines have been logged or no stats are available.
func buildLinesRecord(dir string) LinesRecord {
	var rec LinesRecord
	mk, err := lineslog.LoadMarkets(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "note: could not read the lines log:", err)
		return rec
	}
	off, _ := lineslog.LoadOff(dir)
	if len(mk) == 0 && len(off) == 0 {
		return rec
	}
	d, err := pipeline.Load(dir, false)
	if err != nil {
		return rec
	}
	return summariseLines(mk, off, lineslog.Actualer(d.Builder.Index().Actual))
}

func summariseLines(mk []lineslog.Market, off []lineslog.Off, act lineslog.Actualer) LinesRecord {
	var rec LinesRecord
	rec.Logged = len(mk)
	days := map[string]bool{}
	gm := lineslog.GradeMarkets(mk, act)
	rec.Graded = len(gm)
	for _, g := range gm {
		days[g.Date] = true
		if rec.Since == "" || g.Date < rec.Since {
			rec.Since = g.Date
		}
		if g.Date > rec.Until {
			rec.Until = g.Date
		}
	}
	rec.Days = len(days)
	for _, b := range lineslog.CalBins(gm) {
		rec.Bins = append(rec.Bins, LinesBin{b.Label, b.N, b.Predicted, b.Actual})
	}
	rec.Model, rec.Base, _ = lineslog.Skill(gm)
	for _, s := range lineslog.Slices(gm) {
		rec.Slices = append(rec.Slices, LinesSlice{s.Name, s.N, s.OverRate, s.ModelOver})
	}
	go2 := lineslog.GradeOff(off, act)
	rec.OffCount = len(go2)
	for _, r := range lineslog.OffSummary(go2) {
		rec.Off = append(rec.Off, LinesOff{r.Name, r.N, r.Wins, r.Losses, r.Pushes, r.HitRate, r.Predicted, r.Units, r.ROI()})
	}
	return rec
}
