// rotationaudit evaluates immutable rotation-v1 decisions against later final
// actuals. It compares the same model/book blend with and without team allocation.
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
	"propsmodel/internal/lineup"
	"propsmodel/internal/picklog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"propsmodel/internal/settlement"
	"sort"
	"strings"
	"time"
)

type Slice struct {
	Stat                                                   string
	N, Days                                                int
	BaselineBrier, RotationBrier, BaselineMAE, RotationMAE float64
	ProtectedBrier, ProtectedMAE                           float64
	MinutesContribution, RateContribution                  float64
}
type GameContext struct {
	PlayerGames int
	MinutesBias float64
	MinutesMAE  float64
}
type Report struct {
	GameContexts                           map[string]*GameContext
	Cohort                                 string
	ModelVersions                          map[int]int
	Policy, Note, Through                  string
	Questions, PlayerGames, Days           int
	BaselineMinutesMAE, RotationMinutesMAE float64
	ProtectedMinutesMAE                    float64
	Slices                                 []Slice
	Excluded                               map[string]int
}

func collect(dir, before string) (Report, error) { return collectCohort(dir, before, "all") }

func collectCohort(dir, before, cohort string) (Report, error) {
	return collectPolicy(dir, before, cohort, rotation.Policy)
}

func collectPolicy(dir, before, cohort, policy string) (Report, error) {
	out := Report{GameContexts: map[string]*GameContext{}, Cohort: cohort, ModelVersions: map[int]int{}, Policy: policy, Note: "Read-only paired forward diagnostics. Earliest eligible immutable question per player/game/stat. Same frozen model, raw per-minute rates, quote and model/book weight; captured research minutes and production policy differs. Pushes/DNPs are excluded from binary scores. Minutes count each player/game once. No performance certification or automatic promotion; no live files changed.", Excluded: map[string]int{}}
	ix, e := settlement.Load(dir)
	if e != nil {
		return out, e
	}
	models := map[string]*predictor.Model{}
	chosen := map[string]replay.Record{}
	files, _ := filepath.Glob(filepath.Join(dir, rotation.EvidenceDirectory(policy), "replay", "decisions_*.jsonl"))
	sort.Strings(files)
	for _, file := range files {
		f, e := os.Open(file)
		if e != nil {
			return out, e
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 65536), 1<<24)
		for sc.Scan() {
			var r replay.Record
			if e = json.Unmarshal(sc.Bytes(), &r); e != nil {
				f.Close()
				return out, e
			}
			if r.Date >= before {
				out.Excluded["date not completed"]++
				continue
			}
			if r.SelectionPolicy != policy || r.ProbabilityMethod != "linear" || r.Rotation == nil || r.Row.G.GameID <= 0 || r.Row.G.PlayerID <= 0 || r.Row.G.Date != r.Date || r.Row.G.Name == "" || r.Period != "game" || r.BookCount < 3 {
				out.Excluded["capture identity or policy metadata"]++
				continue
			}
			at, ea := time.Parse(time.RFC3339Nano, r.At)
			tip, et := time.Parse(time.RFC3339Nano, r.Tip)
			q, eq := time.Parse(time.RFC3339Nano, r.QuoteAt)
			u, eu := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			if ea != nil || et != nil || eq != nil || eu != nil || !at.Before(tip) || q.After(at) || u.After(at) || at.Sub(q) > 30*time.Minute || at.Sub(u) > 6*time.Hour || math.Abs(float64(r.OverOdds)) < 100 || math.Abs(float64(r.UnderOdds)) < 100 || math.Abs(r.Market-bets.MarketOver(r.OverOdds, r.UnderOdds)) > 1e-9 {
				out.Excluded["quote/timing"]++
				continue
			}
			m := models[r.ModelID]
			if m == nil {
				id, er := hex.DecodeString(r.ModelID)
				if er != nil || len(id) != 32 {
					out.Excluded["checkpoint"]++
					continue
				}
				path := filepath.Join(dir, rotation.EvidenceDirectory(policy), "replay", "model_"+r.ModelID+".json")
				b, er := os.ReadFile(path)
				if er != nil {
					out.Excluded["checkpoint"]++
					continue
				}
				h := sha256.Sum256(b)
				if hex.EncodeToString(h[:]) != r.ModelID {
					f.Close()
					return out, fmt.Errorf("checkpoint hash mismatch")
				}
				m, er = predictor.Load(path)
				if er != nil {
					f.Close()
					return out, er
				}
				models[r.ModelID] = m
			}
			if m.TrainedBefore == "" || m.TrainedBefore >= r.Date {
				out.Excluded["training cutoff"]++
				continue
			}
			p, er := replay.Recompute(m, r)
			if er != nil || math.IsNaN(p) || math.Abs(p-r.P1) > 1e-9 {
				out.Excluded["not reproducible"]++
				continue
			}
			k := fmt.Sprintf("%s/%d/%d/%s", r.Date, r.Row.G.GameID, r.Row.G.PlayerID, r.Stat)
			old, exists := chosen[k]
			oldAt, _ := time.Parse(time.RFC3339Nano, old.At)
			if !exists || at.Before(oldAt) {
				chosen[k] = r
			}
		}
		e = sc.Err()
		f.Close()
		if e != nil {
			return out, e
		}
	}
	keys := []string{}
	for k := range chosen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	players := map[string]bool{}
	dates := map[string]bool{}
	stats := map[string]*Slice{}
	statDates := map[string]map[string]bool{}
	for _, k := range keys {
		r := chosen[k]
		if rotation.ProductionPolicy(policy) && !validProductionCapture(r) {
			out.Excluded["production provenance unavailable"]++
			continue
		}
		group, valid := absenceCohort(r)
		if !valid {
			out.Excluded["absence timestamp/status unavailable"]++
			continue
		}
		if cohort != "all" && cohort != group {
			out.Excluded["different absence cohort"]++
			continue
		}
		entry := picklog.Entry{Date: r.Date, LoggedAt: r.At, Tip: r.Tip, Period: "game", GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Player: r.Row.G.Name, Team: r.Row.G.TeamAbbr, Stat: string(r.Stat), Side: "OVER", Line: r.Line}
		grade := ix.Grade(entry, 0)
		actual, ok := ix.Actual(entry.GameID, entry.PlayerID)
		mins, okm := ix.ActualMinutes(entry)
		if grade.Result == "REVIEW" || !ok || !okm || actual.TeamID != r.Row.G.TeamID {
			out.Excluded["final identity/source unavailable"]++
			continue
		}
		m := models[r.ModelID]
		base := m.Forecast(r.Row.Features(), predictor.Adjustments{})
		p := m.Forecast(r.Row.Features(), r.Policy)
		protected := base
		if group == "supported-joint-absence" || rotation.ProductionPolicy(policy) {
			protected = p
		}
		pk := fmt.Sprintf("%s/%d/%d", r.Date, entry.GameID, entry.PlayerID)
		if !players[pk] {
			players[pk] = true
			out.BaselineMinutesMAE += math.Abs(base.Minutes - mins)
			out.RotationMinutesMAE += math.Abs(p.Minutes - mins)
			out.ProtectedMinutesMAE += math.Abs(protected.Minutes - mins)
			context := spreadGroup(r.Row.Features().AbsSpread())
			if out.GameContexts[context] == nil {
				out.GameContexts[context] = &GameContext{}
			}
			c := out.GameContexts[context]
			c.PlayerGames++
			c.MinutesBias += mins - p.Minutes
			c.MinutesMAE += math.Abs(mins - p.Minutes)
		}
		dates[r.Date] = true
		if r.Date > out.Through {
			out.Through = r.Date
		}
		if grade.Result == "PUSH" || grade.Result == "VOID" {
			out.Excluded["push/DNP binary score"]++
			continue
		}
		raw, er := base.ProbOver(r.Stat, r.Line)
		if er != nil {
			return out, er
		}
		pb := math.Max(.01, math.Min(.99, r.BlendWeight*raw.Over+(1-r.BlendWeight)*r.Market))
		protectedRaw, er := protected.ProbOver(r.Stat, r.Line)
		if er != nil {
			return out, er
		}
		protectedProb := math.Max(.01, math.Min(.99, r.BlendWeight*protectedRaw.Over+(1-r.BlendWeight)*r.Market))
		y := 0.
		if grade.Result == "WIN" {
			y = 1
		}
		st := string(r.Stat)
		s := stats[st]
		if s == nil {
			s = &Slice{Stat: st}
			stats[st] = s
			statDates[st] = map[string]bool{}
		}
		s.N++
		out.Questions++
		out.ModelVersions[m.Version]++
		statDates[st][r.Date] = true
		s.BaselineBrier += (pb - y) * (pb - y)
		s.RotationBrier += (r.P1 - y) * (r.P1 - y)
		s.BaselineMAE += math.Abs(base.Mean(r.Stat) - grade.Actual)
		s.RotationMAE += math.Abs(p.Mean(r.Stat) - grade.Actual)
		s.ProtectedBrier += (protectedProb - y) * (protectedProb - y)
		s.ProtectedMAE += math.Abs(protected.Mean(r.Stat) - grade.Actual)
		if p.Minutes > 0 {
			rate := p.Mean(r.Stat) / p.Minutes
			s.MinutesContribution += (mins - p.Minutes) * rate
			s.RateContribution += grade.Actual - mins*rate
		}
	}
	for _, c := range out.GameContexts {
		c.MinutesBias /= float64(c.PlayerGames)
		c.MinutesMAE /= float64(c.PlayerGames)
	}
	out.PlayerGames = len(players)
	out.Days = len(dates)
	if out.PlayerGames > 0 {
		out.BaselineMinutesMAE /= float64(out.PlayerGames)
		out.RotationMinutesMAE /= float64(out.PlayerGames)
		out.ProtectedMinutesMAE /= float64(out.PlayerGames)
	}
	names := []string{}
	for st := range stats {
		names = append(names, st)
	}
	sort.Strings(names)
	for _, st := range names {
		s := stats[st]
		n := float64(s.N)
		s.Days = len(statDates[st])
		s.BaselineBrier /= n
		s.RotationBrier /= n
		s.BaselineMAE /= n
		s.RotationMAE /= n
		s.ProtectedBrier /= n
		s.ProtectedMAE /= n
		s.MinutesContribution /= n
		s.RateContribution /= n
		out.Slices = append(out.Slices, *s)
	}
	return out, nil
}
func main() {
	dir := flag.String("data", "data", "input directory")
	dest := flag.String("out", filepath.Join("run_evidence", "rotation-audit"), "private output directory")
	before := flag.String("before", "", "today Eastern; only earlier dates are graded")
	cohort := flag.String("cohort", "all", "all, no-confirmed-out, irrelevant-outs, supported-joint-absence, or unsupported-fallback")
	policy := flag.String("policy", rotation.Policy, "rotation-v1, rotation-v2 or rotation-v3; evidence and results remain separate")
	flag.Parse()
	if !rotation.KnownPolicy(*policy) {
		fatal(fmt.Errorf("unknown rotation policy %q", *policy))
	}
	if !validCohort(*cohort) {
		fatal(fmt.Errorf("unknown absence cohort %q", *cohort))
	}
	if *before == "" {
		loc, _ := time.LoadLocation("America/New_York")
		*before = time.Now().In(loc).Format("2006-01-02")
	}
	if _, e := time.Parse("2006-01-02", *before); e != nil {
		fatal(e)
	}
	r, e := collectPolicy(*dir, *before, *cohort, *policy)
	if e != nil {
		fatal(e)
	}
	if e = os.MkdirAll(*dest, 0755); e != nil {
		fatal(e)
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	if e = os.WriteFile(filepath.Join(*dest, "report.json"), b, 0644); e != nil {
		fatal(e)
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# V2 rotation comparison\n\n%s\n\n", r.Note)
	fmt.Fprintf(&md, "Cohort: %s. Model versions (binary questions): %v.\n\n", r.Cohort, r.ModelVersions)
	fmt.Fprintln(&md, "For rotation-v1, protected columns apply a retrospective supported-only minutes rule selected after inspecting earlier results; they are not prospective validation. For rotation-v2/v3, protected columns equal the captured candidate, including production changes.")
	if r.PlayerGames == 0 {
		fmt.Fprintln(&md, "No completed eligible rotation forecasts yet. Accuracy is unavailable, not zero.")
	} else {
		fmt.Fprintf(&md, "%d player-games across %d dates; %d binary prop questions. Minutes MAE: baseline %.2f, rotation %.2f, protected %.2f.\n\n| Stat | Questions / dates | Baseline Brier | Rotation Brier | Protected Brier | Baseline stat MAE | Rotation stat MAE | Protected stat MAE |\n|---|---:|---:|---:|---:|---:|---:|---:|\n", r.PlayerGames, r.Days, r.Questions, r.BaselineMinutesMAE, r.RotationMinutesMAE, r.ProtectedMinutesMAE)
		for _, s := range r.Slices {
			fmt.Fprintf(&md, "| %s | %d / %d | %.5f | %.5f | %.5f | %.2f | %.2f | %.2f |\n", s.Stat, s.N, s.Days, s.BaselineBrier, s.RotationBrier, s.ProtectedBrier, s.BaselineMAE, s.RotationMAE, s.ProtectedMAE)
		}
	}
	fmt.Fprintln(&md, "\n## Pregame competitiveness and minutes\n\nGrouped by the captured pregame spread, never the final margin. Positive bias means projected minutes were too low. This diagnoses an association; it does not fit a blowout probability.\n\n| Pregame spread | Player-games | Minutes bias | Minutes MAE |\n|---|---:|---:|---:|")
	for _, name := range []string{"close (0–3.5)", "competitive (3.5–7)", "wide (7–10)", "blowout risk (10+)", "unknown"} {
		if c := r.GameContexts[name]; c != nil {
			fmt.Fprintf(&md, "| %s | %d | %+.2f | %.2f |\n", name, c.PlayerGames, c.MinutesBias, c.MinutesMAE)
		}
	}
	fmt.Fprintln(&md, "\n## Signed stat-error decomposition\n\nPositive means actual exceeded projected. Minutes contribution = (actual minutes - projected minutes) × projected per-minute rate. Rate contribution = actual stat - actual minutes × projected rate. Their sum is actual minus projected; these are accounting diagnostics, not causal injury effects.\n\n| Stat | Minutes contribution | Rate contribution |\n|---|---:|---:|")
	for _, s := range r.Slices {
		fmt.Fprintf(&md, "| %s | %+.3f | %+.3f |\n", s.Stat, s.MinutesContribution, s.RateContribution)
	}

	if e = os.WriteFile(filepath.Join(*dest, "report.md"), []byte(md.String()), 0644); e != nil {
		fatal(e)
	}
	fmt.Printf("Rotation audit: %d completed player-games / %d dates; %d questions. %s\n", r.PlayerGames, r.Days, r.Questions, filepath.Join(*dest, "report.md"))
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }

// Classify only after selecting the earliest eligible pregame question, so a
// later status cannot change which forecast is evaluated. Supported here means
// enough matching rotations, not demonstrated predictive improvement.
func validCohort(s string) bool {
	switch s {
	case "all", "no-confirmed-out", "irrelevant-outs", "supported-joint-absence", "unsupported-fallback":
		return true
	}
	return false
}
func absenceCohort(r replay.Record) (string, bool) {
	if r.Rotation == nil {
		return "", false
	}
	if len(r.Rotation.ConfirmedOut) == 0 {
		return "no-confirmed-out", true
	}
	at, err := time.Parse(time.RFC3339Nano, r.At)
	if err != nil {
		return "", false
	}
	for _, status := range r.Rotation.ConfirmedOut {
		reported, err := time.Parse(time.RFC3339Nano, status.At)
		if status.PlayerID <= 0 || !strings.EqualFold(strings.TrimSpace(status.Status), "out") || err != nil || reported.After(at) || at.Sub(reported) > 24*time.Hour {
			return "", false
		}
	}
	if strings.Contains(r.Rotation.Reason, "no recent rotation minutes") {
		return "irrelevant-outs", true
	}
	if r.Rotation.Supported && r.Rotation.MatchedGames >= 3 {
		return "supported-joint-absence", true
	}
	return "unsupported-fallback", true
}

// Production effects retain the fresh confirmed reports and bounded scales
// used at decision time, independently of the player's own minutes cohort.
func validProductionCapture(r replay.Record) bool {
	if r.Rotation == nil {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, r.At)
	if err != nil {
		return false
	}
	for _, impact := range r.Rotation.Production {
		applied := false
		seen := map[string]bool{}
		for _, e := range impact.Effects {
			if !e.Applied {
				continue
			}
			applied = true
			key := string(e.Stat)
			if e.Role != nil && e.Role.Before != r.Date {
				return false
			}
			if seen[key] || !validEffectEvidence(e, impact, r.SelectionPolicy) || math.IsNaN(e.Scale) || math.IsInf(e.Scale, 0) || e.Scale < .95 || e.Scale > 1.05 {
				return false
			}
			seen[key] = true
		}
		if !applied {
			continue
		}
		if len(impact.Missing) == 0 || impact.Team <= 0 {
			return false
		}
		for _, p := range impact.Missing {
			stamp, err := time.Parse(time.RFC3339Nano, p.At)
			if p.PlayerID <= 0 || !strings.EqualFold(strings.TrimSpace(p.Status), "Out") || err != nil || stamp.After(at) || at.Sub(stamp) > 24*time.Hour {
				return false
			}
		}
	}
	for k, x := range lineup.Scale(r.Rotation.Production) {
		if math.IsNaN(r.Policy.Scale[k]) || math.Abs(x-r.Policy.Scale[k]) > 1e-9 {
			return false
		}
	}
	return true
}

// A fallback cannot borrow joint sample counts. Verify its component evidence,
// neutral remainder and captured scale independently before grading it.
func validEffectEvidence(e lineup.Effect, impact lineup.Impact, policy string) bool {
	if e.Method == "pooled-lineup" {
		if policy != rotation.FallbackPolicy || e.Role == nil || e.Role.Games < 500 || e.Role.Days < 30 || len(e.Role.Features) != 14 || len(e.Role.Coefficients) != 14 || len(e.Role.Baseline) != 7 || len(e.Role.Projected) != 7 {
			return false
		}
		delta := 0.
		offset := 0
		if impact.Kind == "opponent-allowance" {
			offset = 7
		} else if impact.Kind != "teammate-production" {
			return false
		}
		floors := []float64{40, 20, 10, 5, 3, 48, 48}
		for k := 0; k < 7; k++ {
			b, u := e.Role.Baseline[k], e.Role.Projected[k]
			if math.IsNaN(b) || math.IsNaN(u) || math.IsInf(b, 0) || math.IsInf(u, 0) || b < 0 || u < 0 {
				return false
			}
			expected := math.Max(-1, math.Min(1, (u-b)/math.Max(floors[k], b)))
			if math.Abs(e.Role.Features[offset+k]-expected) > 1e-9 || e.Role.Features[(7-offset)+k] != 0 {
				return false
			}
		}
		for i, x := range e.Role.Features {
			b := e.Role.Coefficients[i]
			if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1 || math.IsNaN(b) || math.IsInf(b, 0) {
				return false
			}
			delta += x * b
		}
		return math.Abs(e.Scale-math.Max(.95, math.Min(1.05, 1+delta))) < 1e-9
	}
	if e.Method != "individual-mixture" {
		return e.Without >= 5 && e.With >= 5
	}
	if policy != rotation.FallbackPolicy || len(e.Evidence) == 0 {
		return false
	}
	missing := map[int]bool{}
	for _, p := range impact.Missing {
		missing[p.PlayerID] = true
	}
	seen := map[int]bool{}
	weight, delta := 0., 0.
	for _, c := range e.Evidence {
		if !missing[c.PlayerID] || seen[c.PlayerID] || c.Without < 5 || c.With < 5 || math.IsNaN(c.Weight) || math.IsInf(c.Weight, 0) || c.Weight <= 0 || c.Weight > 1 || math.IsNaN(c.Scale) || c.Scale < .975 || c.Scale > 1.025 {
			return false
		}
		seen[c.PlayerID] = true
		weight += c.Weight
		delta += c.Weight * (c.Scale - 1)
	}
	return weight <= 1+1e-9 && math.Abs(e.Scale-math.Max(.975, math.Min(1.025, 1+delta))) < 1e-9
}

func spreadGroup(spread float64) string {
	switch {
	case math.IsNaN(spread) || math.IsInf(spread, 0):
		return "unknown"
	case math.Abs(spread) <= 3.5:
		return "close (0–3.5)"
	case math.Abs(spread) <= 7:
		return "competitive (3.5–7)"
	case math.Abs(spread) < 10:
		return "wide (7–10)"
	default:
		return "blowout risk (10+)"
	}
}
