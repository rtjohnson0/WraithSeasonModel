// compare screens fixed challenger policies using historical walk-forward models.
// The current checkpoint is frozen as evidence, never applied to its own past.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"propsmodel/internal/backtest"
	"propsmodel/internal/bets"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/research"
	"reflect"
	"sort"
	"time"
)

type key struct{ game, player int }

func main() {
	dir := flag.String("data", "data", "data directory")
	season := flag.Int("season", 2025, "season start year to screen")
	out := flag.String("out", "research", "isolated research output directory")
	flag.Parse()
	run := filepath.Join(*out, time.Now().UTC().Format("20060102T150405.000000000Z"))
	if e := os.MkdirAll(filepath.Join(run, "frozen"), 0755); e != nil {
		fatal(e)
	}
	hashes := map[string]string{}
	for _, name := range []string{"model.json", "calibration.json"} {
		b, e := os.ReadFile(filepath.Join(*dir, name))
		if e != nil {
			fatal(e)
		}
		h := sha256.Sum256(b)
		hashes[name] = hex.EncodeToString(h[:])
		if e = os.WriteFile(filepath.Join(run, "frozen", name), b, 0644); e != nil {
			fatal(e)
		}
	}
	manifest := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "season": *season, "frozenSHA256": hashes, "variants": research.Specs, "note": "Cached historical screening only. The current checkpoint is not applied to past games it trained on. Monthly models train strictly before prediction month. No realized absence features. Calibration fits on earliest 60% of questions by full dates; next 20% selects a fixed challenger; last 20% evaluates it once. Re-running or redesigning after viewing the test makes that test exploratory. Source cache and historical quote provenance require independent verification; passing this screen does not certify a launch hit rate. Production files are never modified."}
	inputs := inputHashes(*dir, *season)
	manifest["inputSHA256"] = inputs
	save(filepath.Join(run, "manifest.json"), manifest)
	fmt.Println("Frozen current model and calibration. Loading historical features...")
	pipeline.Missed = false
	d, e := pipeline.Load(*dir, true)
	if e != nil {
		fatal(e)
	}
	lines, e := props.LoadOpening(filepath.Join(*dir, fmt.Sprintf("opening_props_%d.jsonl", *season)))
	if e != nil {
		fatal(e)
	}
	byKey := map[key]*feat.Row{}
	for i := range d.Rows {
		r := &d.Rows[i]
		byKey[key{r.G.GameID, r.G.PlayerID}] = r
	}
	byMonth := map[string][]props.Line{}
	for _, l := range backtest.MedianLines(lines) {
		r := byKey[key{l.GameID, l.PlayerID}]
		if r == nil || r.G.Season != *season || !props.ValidOdds(l.OverOdds) || !props.ValidOdds(l.UnderOdds) {
			continue
		}
		byMonth[r.G.Date[:7]] = append(byMonth[r.G.Date[:7]], l)
	}
	months := []string{}
	for m := range byMonth {
		months = append(months, m)
	}
	sort.Strings(months)
	var questions []research.Question
	for _, month := range months {
		n := sort.Search(len(d.Rows), func(i int) bool { return d.Rows[i].G.Date >= month+"-01" })
		if n < 500 {
			continue
		}
		fmt.Printf("Walk-forward month %s: train %d earlier player-games; score %d matched markets\n", month, n, len(byMonth[month]))
		m, e := predictor.Train(d.Rows[:n], pipeline.DisabledFor(false))
		if e != nil {
			fatal(e)
		}
		for _, l := range byMonth[month] {
			r := byKey[key{l.GameID, l.PlayerID}]
			base := m.Forecast(r, predictor.Adjustments{})
			if !model.Eligible(r.AvgMin15, base.Minutes) {
				continue
			}
			q := research.Question{ActualMinutes: r.G.Minutes}
			for _, spec := range research.Specs {
				p := base.BlendMinutes(r.TypicalMinutes(), spec.MinutesBlend)
				for k := range p.Alpha {
					p.Alpha[k] *= spec.DispersionScale
				}
				pr, e := p.ProbOver(l.Stat, l.Line)
				if e != nil {
					fatal(e)
				}
				q.Results = append(q.Results, backtest.Result{Date: r.G.Date, GameID: r.G.GameID, PlayerID: r.G.PlayerID, Stat: l.Stat, Line: l.Line, POver: pr.Over, Actual: r.G.Value(l.Stat), OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, Market: bets.MarketOver(l.OverOdds, l.UnderOdds)})
				q.Means = append(q.Means, p.Mean(l.Stat))
				q.Minutes = append(q.Minutes, p.Minutes)
			}
			questions = append(questions, q)
		}
	}
	if !reflect.DeepEqual(inputs, inputHashes(*dir, *season)) {
		fatal(fmt.Errorf("input files changed during comparison; no result accepted"))
	}
	study, e := research.Run(questions)
	if e != nil {
		fatal(e)
	}
	save(filepath.Join(run, "study.json"), study)
	save(filepath.Join(run, "questions.json"), questions)
	fmt.Printf("Research complete: %s\nFit through %s; validation through %s; final test %s to %s\nSelected challenger %s; final log-loss gain vs current method %+.5f (z %.2f), vs book %+.5f (z %.2f); passed screen=%t\nProduction model and calibration unchanged.\n", run, study.FitThrough, study.ValidationThrough, study.TestFrom, study.TestThrough, study.Selected, study.GainVsReference, study.ZVsReference, study.GainVsBook, study.ZVsBook, study.Passed)
}
func save(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		fatal(e)
	}
	if e = os.WriteFile(path, b, 0644); e != nil {
		fatal(e)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }

func inputHashes(dir string, season int) map[string]string {
	hashes := map[string]string{}
	patterns := []string{"stats_*.jsonl", "advanced_*.jsonl", "opening_games_*.jsonl", fmt.Sprintf("opening_props_%d.jsonl", season)}
	for _, pat := range patterns {
		files, e := filepath.Glob(filepath.Join(dir, pat))
		if e != nil {
			fatal(e)
		}
		for _, path := range files {
			f, e := os.Open(path)
			if e != nil {
				fatal(e)
			}
			h := sha256.New()
			_, e = io.Copy(h, f)
			f.Close()
			if e != nil {
				fatal(e)
			}
			hashes[filepath.Base(path)] = hex.EncodeToString(h.Sum(nil))
		}
	}
	return hashes
}
