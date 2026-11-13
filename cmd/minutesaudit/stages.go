package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"sort"
	"time"
)

// These rows diagnose the frozen probability transformation only. They have no
// actuals attached and cannot be used as performance or calibration evidence.
type Stage struct {
	Stat                 string  `json:"stat"`
	N                    int     `json:"n"`
	MissingGameID        int     `json:"missingGameId"`
	IncompleteProvenance int     `json:"incompleteProvenance"`
	RawUnder             int     `json:"rawUnder"`
	FinalUnder           int     `json:"finalUnder"`
	ZeroInterceptUnder   int     `json:"zeroInterceptUnder"`
	TypicalUnder         int     `json:"typicalUnder"`
	RawUnderAboveMean    int     `json:"rawUnderDespiteMeanAtOrAboveLine"`
	OverThreshold        int     `json:"overThreshold"`
	UnderThreshold       int     `json:"underThreshold"`
	MinutesShift         float64 `json:"meanMinutesMinusTypical"`
	TypicalN             int     `json:"typicalN"`
}

func stages(dir, since, before string) ([]Stage, error) {
	files, e := filepath.Glob(filepath.Join(dir, "replay", "decisions_*.jsonl"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	models := map[string]*predictor.Model{}
	byStat := map[data.Stat]*Stage{}
	seen := map[string]bool{}
	for _, f := range files {
		rows, e := readRows[replay.Record](f)
		if e != nil {
			return nil, e
		}
		sort.SliceStable(rows, func(i, j int) bool {
			a, _ := time.Parse(time.RFC3339Nano, rows[i].At)
			b, _ := time.Parse(time.RFC3339Nano, rows[j].At)
			return a.Before(b)
		})
		for _, r := range rows {
			if !inWindow(r.Date, since, before) || !supportedStat(r.Stat) || r.Row.G.PlayerID <= 0 {
				continue
			}
			at, a := time.Parse(time.RFC3339Nano, r.At)
			tip, b := time.Parse(time.RFC3339Nano, r.Tip)
			if a != nil || b != nil || !at.Before(tip) {
				continue
			}
			id, e := hex.DecodeString(r.ModelID)
			if e != nil || len(id) != 32 {
				continue
			}
			m := models[r.ModelID]
			if m == nil {
				path := filepath.Join(dir, "replay", "model_"+r.ModelID+".json")
				bytes, e := os.ReadFile(path)
				if os.IsNotExist(e) {
					continue
				}
				if e != nil {
					return nil, e
				}
				h := sha256.Sum256(bytes)
				if hex.EncodeToString(h[:]) != r.ModelID {
					return nil, fmt.Errorf("checkpoint hash mismatch")
				}
				m, e = predictor.Load(path)
				if e != nil {
					return nil, e
				}
				models[r.ModelID] = m
			}
			if m.TrainedBefore == "" || m.TrainedBefore >= r.Date {
				continue
			}
			final, e := replay.Recompute(m, r)
			if e != nil || math.IsNaN(final) || math.Abs(final-r.P1) > 1e-9 {
				continue
			}
			key := fmt.Sprintf("%s/%d/%d/%s", r.Date, r.Row.G.GameID, r.Row.G.PlayerID, r.Stat)
			if seen[key] {
				continue
			}
			seen[key] = true
			p := m.Forecast(r.Row.Features(), r.Policy)
			raw, e := p.ProbOver(r.Stat, r.Line)
			if e != nil {
				continue
			}
			s := byStat[r.Stat]
			if s == nil {
				s = &Stage{Stat: string(r.Stat)}
				byStat[r.Stat] = s
			}
			s.N++
			if r.Row.G.GameID <= 0 {
				s.MissingGameID++
			}
			if r.Period != "game" || r.BookCount < 3 || r.CalibrationBefore == "" || r.CalibrationBefore >= r.Date {
				s.IncompleteProvenance++
			}
			if raw.Over < .5 {
				s.RawUnder++
				if p.Mean(r.Stat) >= r.Line {
					s.RawUnderAboveMean++
				}
			}
			if final < .5 {
				s.FinalUnder++
			}
			neutral := r.Cal
			neutral.Intercept = 0
			if neutral.Apply(raw.Over, r.Market) < .5 {
				s.ZeroInterceptUnder++
			}
			typical := r.Row.Features().TypicalMinutes()
			if typical > 0 && !math.IsNaN(typical) {
				s.TypicalN++
				s.MinutesShift += p.Minutes - typical
				v, e := p.WithMinutes(typical).ProbOver(r.Stat, r.Line)
				if e == nil && r.Cal.Apply(v.Over, r.Market) < .5 {
					s.TypicalUnder++
				}
			}
			if final >= .55 && bets.Edge(final, r.OverOdds) >= .01 {
				s.OverThreshold++
			}
			if final <= .45 && bets.Edge(1-final, r.UnderOdds) >= .01 {
				s.UnderThreshold++
			}
		}
	}
	out := []Stage{}
	for _, st := range data.Stats {
		if s := byStat[st]; s != nil {
			if s.TypicalN > 0 {
				s.MinutesShift /= float64(s.TypicalN)
			}
			out = append(out, *s)
		}
	}
	return out, nil
}
