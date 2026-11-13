// minutesaudit diagnoses minutes, stat rates and calibration on frozen pregame questions.
// It writes only private audit outputs; it never fits or installs production corrections.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/minutesaudit"
	"propsmodel/internal/picklog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/projlog"
	"propsmodel/internal/replay"
	"propsmodel/internal/settlement"
	"sort"
	"strings"
	"time"
)

type Report struct {
	CurrentSlate      *Slate                      `json:"latestUngradedSlate,omitempty"`
	Stages            []Stage                     `json:"ungradedReplayStages"`
	At                string                      `json:"at"`
	Before            string                      `json:"before"`
	Since             string                      `json:"since"`
	Through           string                      `json:"through,omitempty"`
	Note              string                      `json:"note"`
	Excluded          map[string]int              `json:"excluded"`
	Models            map[string]string           `json:"models"`
	Minutes           []minutesaudit.MinutesSlice `json:"minutes"`
	Stats             []minutesaudit.StatSlice    `json:"stats"`
	Observations      []minutesaudit.Observation  `json:"observations"`
	VerifiedQuestions int                         `json:"verifiedQuestions"`
	LegacyQuestions   int                         `json:"legacyQuestions"`
}

func readRows[T any](path string) ([]T, error) {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 65536), 1<<24)
	out := []T{}
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var r T
		if e = json.Unmarshal([]byte(strings.TrimPrefix(s.Text(), "\ufeff")), &r); e != nil {
			return nil, fmt.Errorf("%s: invalid JSON", filepath.Base(path))
		}
		out = append(out, r)
	}
	return out, s.Err()
}
func ptr(v float64) *float64 { return &v }
func supportedStat(st data.Stat) bool {
	for _, s := range data.Stats {
		if st == s {
			return true
		}
	}
	return false
}
func inWindow(date, since, before string) bool {
	_, e := time.Parse("2006-01-02", date)
	return e == nil && date >= since && date < before
}
func collect(dir, before, since string, legacy bool) (Report, error) {
	rep := Report{At: time.Now().UTC().Format(time.RFC3339), Before: before, Since: since, Excluded: map[string]int{}, Models: map[string]string{}, Observations: []minutesaudit.Observation{}, Note: "Read-only diagnostics. Minutes count one earliest available player/game, not one per prop. Stat error is actual minus projection; positive means underprediction. Its minutes/rate decomposition uses realized minutes after the game and is arithmetic, not causal or a new forecast. No selection thresholds, model parameters, calibration or public records are changed. Replay samples require frozen hash-checked pregame models, timing, period, quote and final-stat identity checks. Legacy projection samples have incomplete quote/training provenance and are exploratory only. DNPs are included in minutes diagnostics but excluded from stat/rate summaries. Over/Under mix is measured, never forced."}
	ix, e := settlement.Load(dir)
	if e != nil {
		return rep, e
	}
	models := map[string]*predictor.Model{}
	files, e := filepath.Glob(filepath.Join(dir, "replay", "decisions_*.jsonl"))
	if e != nil {
		return rep, e
	}
	sort.Strings(files)
	for _, file := range files {
		rows, e := readRows[replay.Record](file)
		if e != nil {
			return rep, e
		}
		for _, r := range rows {
			skip := func(s string) { rep.Excluded["replay: "+s]++ }
			if !inWindow(r.Date, since, before) {
				skip("outside completed-date window")
				continue
			}
			if !supportedStat(r.Stat) || r.Row.G.GameID <= 0 || r.Row.G.PlayerID <= 0 || r.Row.G.Date != r.Date {
				skip("identity/stat invalid")
				continue
			}
			at, ea := time.Parse(time.RFC3339Nano, r.At)
			tip, et := time.Parse(time.RFC3339Nano, r.Tip)
			q, eq := time.Parse(time.RFC3339Nano, r.QuoteAt)
			u, eu := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			if ea != nil || et != nil || eq != nil || eu != nil || !at.Before(tip) || q.After(at) || u.After(at) || at.Sub(q) > 30*time.Minute || at.Sub(u) > 6*time.Hour {
				skip("quote/timing invalid")
				continue
			}
			if r.Period != "game" || r.BookCount < 3 || r.CalibrationBefore == "" || r.CalibrationBefore >= r.Date || math.Abs(float64(r.OverOdds)) < 100 || math.Abs(float64(r.UnderOdds)) < 100 || r.Market <= 0 || r.Market >= 1 || math.IsNaN(r.Market) || math.Abs(r.Market-bets.MarketOver(r.OverOdds, r.UnderOdds)) > 1e-9 {
				skip("period/consensus/calibration cutoff unproven")
				continue
			}
			id, err := hex.DecodeString(r.ModelID)
			if err != nil || len(id) != 32 {
				skip("checkpoint invalid")
				continue
			}
			m := models[r.ModelID]
			if m == nil {
				path := filepath.Join(dir, "replay", "model_"+r.ModelID+".json")
				b, err := os.ReadFile(path)
				if err != nil {
					skip("checkpoint missing")
					continue
				}
				h := sha256.Sum256(b)
				if hex.EncodeToString(h[:]) != r.ModelID {
					return rep, fmt.Errorf("frozen checkpoint hash mismatch")
				}
				m, err = predictor.Load(path)
				if err != nil {
					return rep, err
				}
				models[r.ModelID] = m
			}
			if m.TrainedBefore == "" || m.TrainedBefore >= r.Date {
				skip("model training cutoff invalid")
				continue
			}
			frozen, e := replay.Recompute(m, r)
			if e != nil || math.IsNaN(frozen) || math.Abs(frozen-r.P1) > 1e-9 {
				skip("not exactly reproducible")
				continue
			}
			entry := picklog.Entry{Date: r.Date, LoggedAt: r.At, Tip: r.Tip, Period: "game", Player: r.Row.G.Name, Team: r.Row.G.TeamAbbr, GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Stat: string(r.Stat), Side: "OVER", Line: r.Line}
			grade := ix.Grade(entry, 0)
			mins, ok := ix.ActualMinutes(entry)
			if !ok || grade.Result == "REVIEW" {
				skip("final actuals missing or identity review")
				continue
			}
			actual, ok := ix.Actual(entry.GameID, entry.PlayerID)
			if !ok || actual.TeamID != r.Row.G.TeamID {
				skip("final team mismatch")
				continue
			}
			p := m.Forecast(r.Row.Features(), r.Policy)
			base := m.Predict(r.Row.Features())
			pr, e := p.ProbOver(r.Stat, r.Line)
			if e != nil {
				return rep, e
			}
			cohort := "verified replay"
			if r.SelectionPolicy == "value" {
				cohort = "experimental value replay"
			}
			o := minutesaudit.Observation{Cohort: cohort, Date: r.Date, At: r.At, GameID: entry.GameID, PlayerID: entry.PlayerID, Player: entry.Player, Stat: string(r.Stat), Line: r.Line, Minutes: p.Minutes, ActualMinutes: mins, Typical: r.Row.Features().TypicalMinutes(), Projection: p.Mean(r.Stat), Actual: grade.Actual, BeforeScale: ptr(p.Minutes * base.Rate[data.StatIndex(r.Stat)]), RawOver: ptr(pr.Over), FinalOver: ptr(r.P1), Context: r.AuditContext}
			if math.IsNaN(o.Typical) || math.IsInf(o.Typical, 0) {
				o.Typical = 0
			}
			neutral := r.Cal
			neutral.Intercept = 0
			o.ZeroInterceptOver = ptr(neutral.Apply(pr.Over, r.Market))
			if o.Typical > 0 {
				tp := p.WithMinutes(o.Typical)
				prob, err := tp.ProbOver(r.Stat, r.Line)
				if err == nil {
					o.TypicalOver = ptr(r.Cal.Apply(prob.Over, r.Market))
				}
			}
			rep.Models[r.ModelID] = m.TrainedBefore
			rep.Observations = append(rep.Observations, o)
		}
	}
	if legacy {
		if e = collectLegacy(dir, since, before, ix, &rep); e != nil {
			return rep, e
		}
	}
	rep.Observations = minutesaudit.Dedupe(rep.Observations)
	for _, o := range rep.Observations {
		if o.Cohort == "legacy projection diagnostic" {
			rep.LegacyQuestions++
		} else if o.Cohort == "verified replay" {
			rep.VerifiedQuestions++
		}
		if o.Date > rep.Through {
			rep.Through = o.Date
		}
	}
	rep.Minutes, rep.Stats = minutesaudit.Summary(rep.Observations)
	rep.CurrentSlate, e = loadSlate(dir, before)
	if e != nil {
		return rep, e
	}
	rep.Stages, e = stages(dir, since, before)
	if e != nil {
		return rep, e
	}
	return rep, nil
}
func collectLegacy(dir, since, before string, ix *settlement.Index, rep *Report) error {
	// A historical tip recovered from final metadata is not equivalent to frozen pregame quote provenance.
	type raw struct {
		At   string `json:"_stats_fetched_at"`
		Game struct {
			ID       int    `json:"id"`
			Date     string `json:"date"`
			Datetime string `json:"datetime"`
			Status   string `json:"status"`
		} `json:"game"`
	}
	tips := map[int]string{}
	dateOf := map[int]string{}
	files, e := filepath.Glob(filepath.Join(dir, "stats_*.jsonl"))
	if e != nil {
		return e
	}
	for _, f := range files {
		rows, e := readRows[raw](f)
		if e != nil {
			return e
		}
		for _, r := range rows {
			if strings.EqualFold(r.Game.Status, "Final") {
				if _, e := time.Parse(time.RFC3339Nano, r.Game.Datetime); e == nil {
					tips[r.Game.ID] = r.Game.Datetime
					dateOf[r.Game.ID] = data.NormalizeDate(r.Game.Date)
				}
			}
		}
	}
	rows, e := readRows[projlog.Entry](filepath.Join(dir, "projlog.jsonl"))
	if e != nil {
		return e
	}
	strict := map[string]bool{}
	for _, o := range rep.Observations {
		if o.Cohort == "verified replay" {
			strict[fmt.Sprintf("%s/%d/%d/%s", o.Date, o.GameID, o.PlayerID, o.Stat)] = true
		}
	}
	for _, r := range rows {
		skip := func(s string) { rep.Excluded["legacy: "+s]++ }
		if !inWindow(r.Date, since, before) {
			skip("outside completed-date window")
			continue
		}
		at, a := time.Parse(time.RFC3339Nano, r.At)
		tip, t := time.Parse(time.RFC3339Nano, tips[r.GameID])
		if a != nil || t != nil || !at.Before(tip) || dateOf[r.GameID] != r.Date {
			skip("tip missing, after tip or date mismatch")
			continue
		}
		for st, sp := range r.Stats {
			if !supportedStat(data.Stat(st)) || sp.Line <= 0 {
				skip("stat or historical line missing")
				continue
			}
			if strict[fmt.Sprintf("%s/%d/%d/%s", r.Date, r.GameID, r.PlayerID, st)] {
				continue
			}
			entry := picklog.Entry{Date: r.Date, LoggedAt: r.At, Tip: tips[r.GameID], Period: "game", GameID: r.GameID, PlayerID: r.PlayerID, Player: r.Player, Team: r.Team, Stat: st, Side: "OVER", Line: sp.Line}
			grade := ix.Grade(entry, 0)
			mins, ok := ix.ActualMinutes(entry)
			if !ok || grade.Result == "REVIEW" {
				skip("final actuals missing or identity review")
				continue
			}
			o := minutesaudit.Observation{Cohort: "legacy projection diagnostic", Date: r.Date, At: r.At, GameID: r.GameID, PlayerID: r.PlayerID, Player: r.Player, Stat: st, Line: sp.Line, Minutes: r.ProjMin, ActualMinutes: mins, Typical: r.TypMin, Projection: sp.Proj, Actual: grade.Actual, BeforeScale: ptr(sp.Raw)}
			// Old logs had no distinction between unavailable and zero absence counts; do not invent it.
			rep.Observations = append(rep.Observations, o)
		}
	}
	return nil
}
func main() {
	dir := flag.String("data", "data", "input data directory")
	out := flag.String("out", filepath.Join("run_evidence", "minutes-audit"), "private audit output directory")
	before := flag.String("before", "", "dates strictly before YYYY-MM-DD, default today Eastern")
	days := flag.Int("days", 60, "look-back in calendar days")
	legacy := flag.Bool("legacy", false, "include a separate exploratory cohort from old projection logs; never forward evidence")
	flag.Parse()
	loc, e := time.LoadLocation("America/New_York")
	if e != nil {
		fatal(e)
	}
	if *before == "" {
		*before = time.Now().In(loc).Format("2006-01-02")
	}
	cut, e := time.Parse("2006-01-02", *before)
	if e != nil || *days < 1 {
		fatal(fmt.Errorf("valid date and positive days required"))
	}
	since := cut.AddDate(0, 0, -*days).Format("2006-01-02")
	rep, e := collect(*dir, *before, since, *legacy)
	if e != nil {
		fatal(e)
	}
	if e = os.MkdirAll(*out, 0755); e != nil {
		fatal(e)
	}
	b, e := json.MarshalIndent(rep, "", "  ")
	if e != nil {
		fatal(e)
	}
	if e = os.WriteFile(filepath.Join(*out, "report.json"), b, 0644); e != nil {
		fatal(e)
	}
	if e = writeHTML(filepath.Join(*out, "report.html"), rep); e != nil {
		fatal(e)
	}
	fmt.Printf("Minutes/Under-bias audit: %d verified replay questions, %d legacy diagnostic questions; finals through %s.\nPrivate report: %s\nNo production files or picks changed.\n", rep.VerifiedQuestions, rep.LegacyQuestions, rep.Through, filepath.Join(*out, "report.html"))
	for _, m := range rep.Minutes {
		if m.Group == "all player-games" {
			fmt.Printf("%s: %d player-games / %d dates, minutes bias %+.2f, MAE %.2f\n", m.Cohort, m.N, m.Days, m.Bias, m.MAE)
		}
	}
	for _, s := range rep.Stats {
		fmt.Printf("%s %s: N=%d actual-minus-projection %+.2f = minutes %+.2f + rate %+.2f\n", s.Cohort, s.Stat, s.N, s.Bias, s.MinutesContribution, s.RateContribution)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
