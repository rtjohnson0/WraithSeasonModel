// Command diagnose explains a saved slate without changing its fitted model.
package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"sort"
	"strings"
	"time"
)

type row struct {
	Stat                    string  `json:"stat"`
	N                       int     `json:"n"`
	RawUnder                int     `json:"rawUnder"`
	CalibratedUnder         int     `json:"calibratedUnder"`
	NoInterceptUnder        int     `json:"noInterceptUnder"`
	TypicalMinutesUnder     int     `json:"typicalMinutesUnder"`
	OverQualified           int     `json:"overQualified"`
	UnderQualified          int     `json:"underQualified"`
	MeanMinutesShift        float64 `json:"meanMinutesShift"`
	MeanProjectionGap       float64 `json:"meanProjectionGap"`
	CalibrationIntercept    float64 `json:"calibrationIntercept"`
	CalibrationModelWeight  float64 `json:"calibrationModelWeight"`
	CalibrationMarketWeight float64 `json:"calibrationMarketWeight"`
}
type report struct {
	Examples           []map[string]any `json:"examples,omitempty"`
	ExcludedNonPregame int              `json:"excludedNonPregame"`
	Records            int              `json:"records"`
	ExactlyReproduced  int              `json:"exactlyReproduced"`
	MissingGameID      int              `json:"missingGameId"`
	Rows               []row            `json:"rows"`
	Note               string           `json:"note"`
}

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "saved decision date (default: latest captured date)")
	player := flag.Int("player", 0, "optional player ID for a context example")
	out := flag.String("out", "", "report filename")
	flag.Parse()
	if *date == "" {
		files, err := filepath.Glob(filepath.Join(*dir, "replay", "decisions_*.jsonl"))
		if err != nil || len(files) == 0 {
			fatal(fmt.Errorf("no saved decision captures"))
		}
		sort.Strings(files)
		*date = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(files[len(files)-1]), "decisions_"), ".jsonl")
	}
	path := filepath.Join(*dir, "replay", "decisions_"+*date+".jsonl")
	f, e := os.Open(path)
	if e != nil {
		fatal(e)
	}
	defer f.Close()
	models := map[string]*predictor.Model{}
	byStat := map[data.Stat]*row{}
	rep := report{Note: "Matched saved pregame questions. Typical-minutes and zero-intercept scenarios are sensitivity checks, not fitted replacements or validated recommendations. No model, calibration or live picks are changed."}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 65536), 1<<24)
	for sc.Scan() {
		var r replay.Record
		if e = json.Unmarshal(sc.Bytes(), &r); e != nil {
			fatal(e)
		}
		id, err := hex.DecodeString(r.ModelID)
		if err != nil || len(id) != 32 {
			fatal(fmt.Errorf("invalid model checkpoint ID"))
		}
		at, ea := time.Parse(time.RFC3339Nano, r.At)
		tip, et := time.Parse(time.RFC3339Nano, r.Tip)
		if ea != nil || et != nil || !at.Before(tip) {
			rep.ExcludedNonPregame++
			continue
		}
		m := models[r.ModelID]
		if m == nil {
			m, e = predictor.Load(filepath.Join(*dir, "replay", "model_"+r.ModelID+".json"))
			if e != nil {
				fatal(e)
			}
			models[r.ModelID] = m
		}
		p := m.Forecast(r.Row.Features(), r.Policy)
		pr, e := p.ProbOver(r.Stat, r.Line)
		if e != nil {
			fatal(e)
		}
		final := r.Cal.Apply(pr.Over, r.Market)
		if *player > 0 && r.Row.G.PlayerID == *player {
			rep.Examples = append(rep.Examples, map[string]any{"playerId": *player, "stat": r.Stat, "line": r.Line, "projection": p.Mean(r.Stat), "minutes": p.Minutes, "typicalMinutes": r.Policy.Typical, "rawUnder": 1 - pr.Over, "finalUnder": 1 - final, "context": m.ExplainContext(r.Row.Features(), r.Stat)})
		}
		rep.Records++
		if math.Abs(final-r.P1) < 1e-9 {
			rep.ExactlyReproduced++
		}
		if r.Row.G.GameID == 0 {
			rep.MissingGameID++
		}
		v := byStat[r.Stat]
		if v == nil {
			v = &row{Stat: string(r.Stat), CalibrationIntercept: r.Cal.Intercept, CalibrationModelWeight: r.Cal.Slope, CalibrationMarketWeight: r.Cal.Market}
			byStat[r.Stat] = v
		}
		v.N++
		if pr.Over < .5 {
			v.RawUnder++
		}
		if final < .5 {
			v.CalibratedUnder++
		}
		neutral := r.Cal
		neutral.Intercept = 0
		if neutral.Apply(pr.Over, r.Market) < .5 {
			v.NoInterceptUnder++
		}
		typical := r.Policy.Typical
		if typical > 0 {
			tp := p.WithMinutes(typical)
			tpr, _ := tp.ProbOver(r.Stat, r.Line)
			if r.Cal.Apply(tpr.Over, r.Market) < .5 {
				v.TypicalMinutesUnder++
			}
			v.MeanMinutesShift += p.Minutes - typical
		}
		v.MeanProjectionGap += p.Mean(r.Stat) - r.Line
		if final >= .55 && bets.Edge(final, r.OverOdds) >= .01 {
			v.OverQualified++
		}
		if final <= .45 && bets.Edge(1-final, r.UnderOdds) >= .01 {
			v.UnderQualified++
		}
	}
	if e = sc.Err(); e != nil {
		fatal(e)
	}
	for _, st := range data.Stats {
		if r := byStat[st]; r != nil {
			r.MeanMinutesShift /= float64(r.N)
			r.MeanProjectionGap /= float64(r.N)
			rep.Rows = append(rep.Rows, *r)
		}
	}
	b, e := json.MarshalIndent(rep, "", "  ")
	if e != nil {
		fatal(e)
	}
	fmt.Println(string(b))
	if *out != "" {
		if e = os.WriteFile(*out, b, 0644); e != nil {
			fatal(e)
		}
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
