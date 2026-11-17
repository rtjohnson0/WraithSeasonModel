package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/team"
	"sort"
	"strconv"
	"strings"
)

type auditForecast struct {
	Date                            string
	GameID                          int
	ActualMargin, ActualTotal       float64
	LegacyMargin, LegacyTotal       float64
	CandidateMargin, CandidateTotal float64
	Early                           bool
}
type auditMetric struct {
	Market                                                                        string
	Games                                                                         int
	LegacyMAE, CandidateMAE, LegacyRMSE, CandidateRMSE, LegacyBias, CandidateBias float64
}

func measureForecasts(rs []auditForecast) []auditMetric {
	out := []auditMetric{}
	for _, market := range []string{"spread margin", "game total"} {
		m := auditMetric{Market: market, Games: len(rs)}
		for _, r := range rs {
			actual, old, new := r.ActualMargin, r.LegacyMargin, r.CandidateMargin
			if market == "game total" {
				actual, old, new = r.ActualTotal, r.LegacyTotal, r.CandidateTotal
			}
			a, b := actual-old, actual-new
			m.LegacyMAE += math.Abs(a)
			m.CandidateMAE += math.Abs(b)
			m.LegacyRMSE += a * a
			m.CandidateRMSE += b * b
			m.LegacyBias += a
			m.CandidateBias += b
		}
		if m.Games > 0 {
			n := float64(m.Games)
			m.LegacyMAE /= n
			m.CandidateMAE /= n
			m.LegacyRMSE = math.Sqrt(m.LegacyRMSE / n)
			m.CandidateRMSE = math.Sqrt(m.CandidateRMSE / n)
			m.LegacyBias /= n
			m.CandidateBias /= n
		}
		out = append(out, m)
	}
	return out
}
func auditForecasts(rows []data.PlayerGame, season int) ([]auditForecast, map[string]int, error) {
	oldGames := team.Build(rows)
	newGames := team.BuildVerified(rows)
	oldSamples, _ := team.Samples(oldGames)
	newSamples, _ := team.ResearchSamples(newGames)
	oldByID := map[int]team.Sample{}
	for _, s := range oldSamples {
		oldByID[s.G.ID] = s
	}
	months := map[string]bool{}
	start := int(^uint(0) >> 1)
	for _, s := range newSamples {
		if s.G.Season == season {
			months[s.G.Date[:7]] = true
			if s.G.Day < start {
				start = s.G.Day
			}
		}
	}
	ordered := []string{}
	for m := range months {
		ordered = append(ordered, m)
	}
	sort.Strings(ordered)
	out := []auditForecast{}
	excluded := map[string]int{}
	for _, month := range ordered {
		cutoff := month + "-01"
		var a, b []team.Sample
		for _, s := range oldSamples {
			if s.G.Date < cutoff {
				a = append(a, s)
			}
		}
		for _, s := range newSamples {
			if s.G.Date < cutoff {
				b = append(b, s)
			}
		}
		if len(a) < 500 || len(b) < 500 {
			continue
		}
		legacy, err := team.Fit(a)
		if err != nil {
			return nil, nil, err
		}
		candidate, err := team.Fit(b)
		if err != nil {
			return nil, nil, err
		}
		for _, s := range newSamples {
			if s.G.Season != season || s.G.Date[:7] != month {
				continue
			}
			old, ok := oldByID[s.G.ID]
			if !ok || !old.Ready || !s.Ready {
				excluded["not ready under both policies"]++
				continue
			}
			om, ot := legacy.Project(old.X)
			nm, nt := candidate.Project(s.X)
			out = append(out, auditForecast{Date: s.G.Date, GameID: s.G.ID, ActualMargin: s.G.Margin(), ActualTotal: s.G.Total(), LegacyMargin: om, LegacyTotal: ot, CandidateMargin: nm, CandidateTotal: nt, Early: s.G.Day-start < 21})
		}
	}
	return out, excluded, nil
}
func runGameAudit(dir, dest string, rows []data.PlayerGame, season int) error {
	forecasts, excluded, err := auditForecasts(rows, season)
	if err != nil {
		return err
	}
	if len(forecasts) == 0 {
		return fmt.Errorf("no paired eligible games for season %d; accuracy is unavailable", season)
	}
	early := []auditForecast{}
	for _, r := range forecasts {
		if r.Early {
			early = append(early, r)
		}
	}
	// Quotes are deliberately not synthesized when the historical file is absent.
	opening := filepath.Join(dir, fmt.Sprintf("opening_games_%d.jsonl", season))
	_, quoteErr := os.Stat(opening)
	marketStatus := "Opening odds available; use the separate legacy and -research backtests for probability comparison."
	if quoteErr != nil {
		marketStatus = "Opening odds unavailable. These point-error results do not measure sportsbook edge or validate spread/total probabilities."
	}
	legacyGames, verifiedGames := team.Build(rows), team.BuildVerified(rows)
	official := map[int]team.Game{}
	for _, g := range verifiedGames {
		official[g.ID] = g
	}
	mismatches := 0
	for _, g := range legacyGames {
		if v, ok := official[g.ID]; ok && (g.HPts != v.HPts || g.APts != v.APts) {
			mismatches++
		}
	}
	report := map[string]any{"season": season, "historyQuality": map[string]int{"legacyGames": len(legacyGames), "verifiedGames": len(verifiedGames), "summedScoreDisagreements": mismatches}, "policy": "verified-finals-schedule-season-reset-v1", "status": "research only; no live files changed", "metrics": measureForecasts(forecasts), "earlySeason": measureForecasts(early), "excluded": excluded, "marketComparison": marketStatus, "games": len(forecasts), "method": "Same-game monthly walk-forward forecasts; each model fitted only on earlier months. Candidate uses explicit finals, opponent-adjusted scoring ratings and expires offseason ratings before readiness/feature construction. Actuals for both are explicit final scores. Error bias is actual minus forecast. This audit does not validate injury or possession features."}
	if err = os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dest, "report.json"), b, 0644); err != nil {
		return err
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Spread and total research\n\nSeason %d, %d paired monthly walk-forward games. No live forecasts, calibration or records changed.\n\n%s\n\n| Market | Games | Legacy MAE | Candidate MAE | Legacy RMSE | Candidate RMSE | Legacy bias | Candidate bias |\n|---|---:|---:|---:|---:|---:|---:|---:|\n", season, len(forecasts), marketStatus)
	for _, m := range measureForecasts(forecasts) {
		fmt.Fprintf(&md, "| %s | %d | %.3f | %.3f | %.3f | %.3f | %+.3f | %+.3f |\n", m.Market, m.Games, m.LegacyMAE, m.CandidateMAE, m.LegacyRMSE, m.CandidateRMSE, m.LegacyBias, m.CandidateBias)
	}
	fmt.Fprintln(&md, "\nThese are raw point forecasts, not calibrated probabilities or pick hit rates. Lower point error alone does not establish stronger betting picks. The candidate must clear a separate quote-matched probability evaluation before promotion.")
	if err = os.WriteFile(filepath.Join(dest, "report.md"), []byte(md.String()), 0644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dest, "forecasts.csv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"date", "game_id", "actual_margin", "actual_total", "legacy_margin", "legacy_total", "candidate_margin", "candidate_total"})
	for _, r := range forecasts {
		row := []string{r.Date, strconv.Itoa(r.GameID)}
		for _, v := range []float64{r.ActualMargin, r.ActualTotal, r.LegacyMargin, r.LegacyTotal, r.CandidateMargin, r.CandidateTotal} {
			row = append(row, fmt.Sprintf("%.8f", v))
		}
		w.Write(row)
	}
	fmt.Printf("Game research: %d paired forecasts; report %s\n", len(forecasts), filepath.Join(dest, "report.md"))
	w.Flush()
	return w.Error()
}
