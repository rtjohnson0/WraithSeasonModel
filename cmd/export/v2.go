package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
)

// V2Site is the challenger's half of the site: a board and a record built from the same code as the
// real ones, so the "V2 test" page looks like the main site. See cmd/predict/v2.go.
type V2Site struct {
	Board  Board       `json:"board"`
	Record Record      `json:"record"`
	Combos ComboRecord `json:"combos"`
	Info   *V2Info     `json:"info,omitempty"`
}

// V2Info mirrors the summary predict writes about the day's V2 run.
type V2Info struct {
	At       string             `json:"at"`
	Weight   float64            `json:"weight"`
	Bar      float64            `json:"bar"`
	ComboBar float64            `json:"combo_bar"`
	MinShift float64            `json:"min_shift"`
	MinN     int                `json:"min_n"`
	Anchor   map[string]float64 `json:"anchor"`
	Ratio    map[string]float64 `json:"ratio"`
	StatN    map[string]int     `json:"stat_n"`
	Model    string             `json:"model"`
	Days     int                `json:"days"`
	Players  int                `json:"players"`
	Scored   int                `json:"scored"`
	Over     int                `json:"over"`
	Under    int                `json:"under"`
	Posted   int                `json:"posted"`
}

type v2File struct {
	Date      string          `json:"date"`
	At        string          `json:"at"`
	Plays     []picklog.Entry `json:"plays"`
	Combos    []picklog.Entry `json:"combos"`
	ComboInfo *ComboInfo      `json:"combo_info"`
	Info      *V2Info         `json:"info"`
}

// buildV2 reads data/v2.json (the latest run) and data/v2settled.jsonl. It returns nil when V2 has
// never run, and the site then shows a "not run yet" note.
func buildV2(dir string, form Form) *V2Site {
	raw, err := os.ReadFile(filepath.Join(dir, "v2.json"))
	if err != nil {
		return nil
	}
	var f v2File
	if json.Unmarshal(raw, &f) != nil || f.Date == "" {
		return nil
	}
	settled, _ := picklog.LoadSettled(filepath.Join(dir, "v2settled.jsonl"))
	var singles, combos []picklog.Settled
	for _, s := range settled {
		if data.Stat(s.Stat).IsCombo() {
			combos = append(combos, s)
		} else {
			singles = append(singles, s)
		}
	}
	site := &V2Site{
		Board:  buildBoard(f.Plays, f.Date, form, nil),
		Record: buildRecord(singles, 600),
		Combos: buildComboRecord(combos, 120),
		Info:   f.Info,
	}
	attachCombos(&site.Board, moreFile{Date: f.Date, Combos: f.Combos, ComboInfo: f.ComboInfo}, form)
	attachCases(&site.Board, trackRecords(singles))
	return site
}
