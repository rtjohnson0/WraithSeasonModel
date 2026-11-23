// Command gamepace publishes a separate game challenger and its own record.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/gameboard"
	"propsmodel/internal/gamepace"
	"propsmodel/internal/props"
	"propsmodel/internal/rotation"
	"propsmodel/internal/team"
	"sort"
	"time"
)

type evidence struct {
	Policy, Date, At string
	Teams            map[int]rotation.Allocation
	Scenarios        map[int]map[int]rotation.Allocation
}

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "slate date, Eastern")
	audit := flag.Bool("audit", false, "write monthly pregame point-error study only")
	out := flag.String("out", "run_evidence/game-pace", "study output directory")
	flag.Parse()
	loc, _ := time.LoadLocation("America/New_York")
	if *date == "" {
		*date = time.Now().In(loc).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", *date); err != nil {
		fail(fmt.Errorf("invalid slate date: %w", err))
	}
	rows, err := data.LoadStatsDir(*dir)
	if err != nil {
		fail(err)
	}
	games, excluded := gamepace.Build(rows)
	samples, state := gamepace.Samples(games, *date)
	study := gamepace.MonthlyReplay(samples, *date)
	report := map[string]any{"study": study, "excluded": excluded, "note": "Possessions are estimated from complete box scores. Monthly fits use earlier dates and earlier projected workloads. This study does not reconstruct historical injuries or validate sportsbook edge."}
	baseline := pairedBaseline(rows, study.Replays)
	report["pairedBaseline"] = baseline
	report["residualStudy"] = gamepace.StudyResiduals(study.Replays)
	if err = os.MkdirAll(*out, 0755); err != nil {
		fail(err)
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(*out, "report.json"), b, 0644); err != nil {
		fail(err)
	}
	fmt.Printf("Game pace: %d eligible training games; %d monthly OOF forecasts / %d dates; margin MAE %.2f, total MAE %.2f\n", len(samples), study.Games, study.Dates, study.MarginMAE, study.TotalMAE)
	fmt.Println("Possession coverage exclusions:", excluded)
	if *audit {
		fmt.Println("Read-only audit; no boards or records changed.")
		return
	}
	model, err := gamepace.Fit(samples, *date)
	if err != nil {
		fail(err)
	}
	if study.Games >= 300 && study.Dates >= 30 {
		model.MarginSigma = math.Max(8, study.MarginSigma)
		model.TotalSigma = math.Max(12, study.TotalSigma)
	}
	asof := time.Now().UTC()
	old, err := gameboard.Load(*dir, *date)
	if err != nil || old == nil {
		fail(fmt.Errorf("current game-v2 snapshot required before challenger: %v", err))
	}
	results, err := gameboard.LoadPolicyResults(*dir, gamepace.Policy)
	if err != nil {
		fail(err)
	}
	outcomes := []gamepace.Outcome{}
	seen := map[string]bool{}
	for _, r := range results {
		key := fmt.Sprintf("%s/%d/%s", r.Date, r.GameID, r.Market)
		if seen[key] || r.Policy != gamepace.Policy || r.Context == nil || r.Context.Date != r.Date || r.Context.Model == nil || r.Context.Model.Before != r.Date || r.SourceAt == "" || (r.Result != "WIN" && r.Result != "LOSS") {
			continue
		}
		tip, et := time.Parse(time.RFC3339Nano, r.Tip)
		at, ea := time.Parse(time.RFC3339Nano, r.At)
		source, es := time.Parse(time.RFC3339Nano, r.SourceAt)
		if et != nil || ea != nil || es != nil || !at.Before(tip) || !source.After(tip) {
			continue
		}
		seen[key] = true
		y := 0.
		if r.Result == "WIN" {
			y = 1
		}
		outcomes = append(outcomes, gamepace.Outcome{Date: r.Date, Market: r.Market, Raw: r.RawProb, Book: r.MarketProb, Y: y})
	}
	fits := gamepace.FitMarkets(outcomes, *date)
	evaluation := gamepace.EvaluateMarkets(outcomes, *date)
	report["forwardProbabilityEvaluation"] = evaluation
	b, _ = json.MarshalIndent(report, "", "  ")
	if err = os.WriteFile(filepath.Join(*out, "report.json"), b, 0644); err != nil {
		fail(err)
	}
	snapshot := gameboard.Snapshot{Policy: gamepace.Policy, Date: *date, At: asof.Format(time.RFC3339Nano), Z: map[string]float64{}, Calibration: fits, Note: "Separate research estimates from possessions × efficiency and projected rotations. No demonstrated market edge; no stakes. Calibration is separate by market and uses only earlier frozen decisions. Missing lineup evidence retains baseline workload."}
	snapshot.Evaluation = evaluation
	ev := loadEvidence(*dir, *date, asof)
	roster, _ := props.LoadActiveTeams(filepath.Join(*dir, "players_active.jsonl"))
	names := map[int]string{}
	for _, r := range rows {
		if r.Date < *date && r.Name != "" {
			names[r.PlayerID] = r.Name
		}
	}
	for _, g := range old.Games {
		tip, err := time.Parse(time.RFC3339Nano, g.Tip)
		if err != nil || !asof.Before(tip) {
			continue
		}
		in := state.Features(g.HomeID, g.AwayID, *date, nil, nil)
		if in.HomeGames < 6 || in.AwayGames < 6 || in.HomeCoverage < .8 || in.AwayCoverage < .8 {
			continue
		}
		ctx := &gamepace.Context{AsOf: snapshot.At, Date: *date, Model: model, Baseline: in, Used: in, Calibration: fits, Reason: "Earlier team workload; no complete fresh V2 rotation override"}
		hm, am := validAllocation(ev.Teams[g.HomeID], g.HomeID, *date, roster), validAllocation(ev.Teams[g.AwayID], g.AwayID, *date, roster)
		if hm != nil || am != nil {
			trial := state.Features(g.HomeID, g.AwayID, *date, hm, am)
			if trial.HomeCoverage >= .8 && trial.AwayCoverage >= .8 {
				ctx.Used = trial
				ctx.LineupApplied = true
				ctx.LineupAt = ev.At
				ctx.Reason = "Fresh V2 projected full-game rotation; jointly fitted earlier player-composition associations, bounded ±3 points per team. Not a causal injury effect."
			}
		}
		f := gamepace.LineupForecast(model, in, ctx.Used)
		ctx.Forecast = f
		// Each alternative removes only the named uncertain teammate alongside confirmed outs.
		for _, tid := range []int{g.HomeID, g.AwayID} {
			ids := []int{}
			for pid := range ev.Scenarios[tid] {
				ids = append(ids, pid)
			}
			sort.Ints(ids)
			for _, pid := range ids {
				alt := validAllocation(ev.Scenarios[tid][pid], tid, *date, roster)
				if alt == nil {
					continue
				}
				h, a := hm, am
				if tid == g.HomeID {
					h = alt
				} else {
					a = alt
				}
				trial := state.Features(g.HomeID, g.AwayID, *date, h, a)
				if trial.HomeCoverage < .8 || trial.AwayCoverage < .8 {
					continue
				}
				af := gamepace.LineupForecast(model, in, trial)
				name := names[pid]
				if name == "" {
					continue
				}
				ctx.Scenarios = append(ctx.Scenarios, gamepace.Scenario{PlayerID: pid, Player: name, Team: tid, Margin: af.Margin, Total: af.Total})
			}
		}
		g.PaceContext = ctx
		g.HomeScore = f.HomeScore
		g.AwayScore = f.AwayScore
		g.Margin = f.Margin
		g.Total = f.Total
		g.TotalLow = f.Total - 1.28155*model.TotalSigma
		g.TotalHigh = f.Total + 1.28155*model.TotalSigma
		quotes := []gameboard.Quote{}
		for _, q := range g.Quotes {
			if !fresh(q, asof, tip) || len(q.Sides) != 2 {
				continue
			}
			var ph, pa, push float64
			switch q.Market {
			case "spread":
				ph, pa, push = gameboard.TotalProb(f.Margin, model.MarginSigma, -q.Line)
			case "total":
				ph, pa, push = gameboard.TotalProb(f.Total, model.TotalSigma, q.Line)
			case "moneyline":
				ph = team.NormCDF(f.Margin / model.MarginSigma)
				pa = 1 - ph
			default:
				continue
			}
			if math.IsNaN(ph) || math.IsNaN(pa) || math.IsInf(ph, 0) || math.IsInf(pa, 0) || ph <= 0 || pa <= 0 || push < 0 || push >= 1 {
				continue
			}
			labels := [2]string{q.Sides[0].Label, q.Sides[1].Label}
			odds := [2]int{q.Sides[0].Odds, q.Sides[1].Odds}
			q.Sides = gameboard.Pair(labels, odds, [2]float64{ph, pa}, push)
			for i := range q.Sides {
				s := &q.Sides[i]
				s.Raw = s.Conditional
				if fit := fits[q.Market]; fit.Ready {
					s.Conditional = fit.Cal.Apply(s.Raw, s.Market)
					s.Win = (1 - push) * s.Conditional
					s.Edge = s.Conditional - s.BreakEven
					payout := 100 / math.Abs(float64(s.Odds))
					if s.Odds > 0 {
						payout = float64(s.Odds) / 100
					}
					s.EV = s.Win*payout - (1 - push - s.Win)
				}
			}
			quotes = append(quotes, q)
		}
		g.Quotes = quotes
		if len(quotes) > 0 {
			snapshot.Games = append(snapshot.Games, g)
		}
	}
	if err = snapshot.SaveNamed(*dir, "gamechallenger.json"); err != nil {
		fail(err)
	}
	if err = gameboard.Log(*dir, snapshot, .55, .01); err != nil {
		fail(err)
	}
	fmt.Printf("Published separate %s estimates for %d games; earlier outcome counts: %v. Original game-v2 board unchanged.\n", gamepace.Policy, len(snapshot.Games), fits)
}
func fresh(q gameboard.Quote, now, tip time.Time) bool {
	quoteTip, et := time.Parse(time.RFC3339Nano, q.Tip)
	cap, ec := time.Parse(time.RFC3339Nano, q.QuoteAt)
	updated, eu := time.Parse(time.RFC3339Nano, q.SourceUpdatedAt)
	return et == nil && quoteTip.Equal(tip) && now.Before(tip) && ec == nil && eu == nil && !cap.After(now) && !updated.After(now) && cap.Before(tip) && updated.Before(tip) && now.Sub(cap) <= 30*time.Minute && now.Sub(updated) <= 6*time.Hour && q.BookCount >= 3
}
func loadEvidence(dir, date string, now time.Time) evidence {
	var e evidence
	b, err := os.ReadFile(filepath.Join(dir, rotation.EvidenceDirectory(rotation.ContextPolicy), "rotation_"+date+".json"))
	if err != nil || json.Unmarshal(b, &e) != nil {
		return evidence{}
	}
	at, err := time.Parse(time.RFC3339Nano, e.At)
	if err != nil || at.After(now) || now.Sub(at) > 30*time.Minute || e.Date != date || e.Policy != rotation.ContextPolicy {
		return evidence{}
	}
	return e
}
func validAllocation(a rotation.Allocation, team int, date string, roster map[int]int) map[int]float64 {
	if !a.Valid || a.Team != team || a.Date != date || !a.RoleProjected || len(roster) == 0 {
		return nil
	}
	total := 0.
	for pid, m := range a.Minutes {
		if math.IsNaN(m) || math.IsInf(m, 0) || m < 0 || m > 48 || roster[pid] != team {
			return nil
		}
		total += m
	}
	if math.Abs(total-240) > .01 {
		return nil
	}
	return a.Minutes
}
func pairedBaseline(rows []data.PlayerGame, replays []gamepace.Replay) map[string]any {
	samples, _ := team.ResearchSamples(team.BuildVerified(rows))
	models := map[string]*team.Model{}
	by := map[int]team.Sample{}
	for _, s := range samples {
		if s.Ready {
			by[s.G.ID] = s
		}
	}
	n := 0
	oldM, oldT, newM, newT := 0., 0., 0., 0.
	for _, r := range replays {
		s, ok := by[r.GameID]
		if !ok {
			continue
		}
		month := r.Date[:7]
		m, ok := models[month]
		if !ok {
			prior := []team.Sample{}
			for _, s := range samples {
				if s.G.Date < month+"-01" && s.Ready {
					prior = append(prior, s)
				}
			}
			m, _ = team.Fit(prior)
			models[month] = m
		}
		if m == nil {
			continue
		}
		margin, total := m.Project(s.X)
		n++
		oldM += math.Abs(r.ActualMargin - margin)
		oldT += math.Abs(r.ActualTotal - total)
		newM += math.Abs(r.ActualMargin - r.Margin)
		newT += math.Abs(r.ActualTotal - r.Total)
	}
	if n == 0 {
		return map[string]any{"games": 0, "note": "no paired forecasts"}
	}
	return map[string]any{"games": n, "baselineMarginMAE": oldM / float64(n), "challengerMarginMAE": newM / float64(n), "baselineTotalMAE": oldT / float64(n), "challengerTotalMAE": newT / float64(n), "note": "Same-game monthly raw point projections. Not market-blended probabilities, not sportsbook-edge evidence; live injury overrides were not reconstructed."}
}
func fail(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
