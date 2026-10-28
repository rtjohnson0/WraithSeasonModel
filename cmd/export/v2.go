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
	Strong   int                `json:"strong"`
	Lean     int                `json:"lean"`
	Check    int                `json:"check"`
	Dropped  V2Drops            `json:"dropped"`
	Gates    []V2Gate           `json:"gates"`
	GateMinN int                `json:"gate_min_n"`
	Watch    int                `json:"watch"`
	Keep     bool               `json:"keep_disagree"`
	EdgeBar  float64            `json:"edge_bar"`
	MaxJuice int                `json:"max_juice"`
}

// V2Drops counts the plays that cleared the bar and were left out, by reason.
type V2Drops struct {
	Thin         int `json:"thin"`
	Disagree     int `json:"disagree"`
	DisagreeOver int `json:"disagree_over"`
	Juiced       int `json:"juiced"`
	Failed       int `json:"failed"`
}

// V2Gate is V2's record on one stat and side over every graded line in the market log.
type V2Gate struct {
	Stat   string  `json:"stat"`
	Side   string  `json:"side"`
	N      int     `json:"n"`
	Hit    float64 `json:"hit"`
	Brier  float64 `json:"brier"`
	Base   float64 `json:"base"`
	State  string  `json:"state"`
	Reason string  `json:"reason"`
}

type v2File struct {
	Date      string          `json:"date"`
	At        string          `json:"at"`
	Plays     []picklog.Entry `json:"plays"`
	Combos    []picklog.Entry `json:"combos"`
	ComboInfo *ComboInfo      `json:"combo_info"`
	Info      *V2Info         `json:"info"`
	Overs     []OverWatch     `json:"overs"`
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
	site.Board.Overs = f.Overs
	if site.Info != nil && site.Info.Gates == nil {
		site.Info.Gates = []V2Gate{} // the page reads it as a list
	}
	attachCombos(&site.Board, moreFile{Date: f.Date, Combos: f.Combos, ComboInfo: f.ComboInfo}, form)
	attachCases(&site.Board, trackRecords(singles))
	return site
}
