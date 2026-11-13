package main

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
)

// Historical rows can diagnose bias without being admitted to the public record.
// Require matching cached settlement identity and recompute the outcome; never
// accept a missing actual, REVIEW row, or a comparison as a graded selection.
func auditRows(rows []picklog.Settled, legacy bool) []picklog.Settled {
	var eligible []picklog.Settled
	for _, r := range rows {
		if r.Tier != "COMBO" || !data.Stat(r.Stat).IsCombo() || !r.FullGame() || !r.Pregame() ||
			(r.Side != "OVER" && r.Side != "UNDER") || (r.Result != "WIN" && r.Result != "LOSS") ||
			r.SettlementVersion < 1 || r.SourceHash == "" || r.SourceFetchedAt == "" || r.SourceTeamID <= 0 ||
			r.SourcePlayerID != r.PlayerID || r.SourceGameID != r.GameID || r.SourceDate != r.Date ||
			!strings.EqualFold(strings.TrimSpace(r.SourcePlayer), strings.TrimSpace(r.Player)) ||
			math.IsNaN(r.Actual) || math.IsInf(r.Actual, 0) || r.Actual < 0 ||
			math.IsNaN(r.Prob) || math.IsInf(r.Prob, 0) || math.IsNaN(r.ProbRaw) || math.IsInf(r.ProbRaw, 0) ||
			r.Prob <= 0 || r.Prob >= 1 || r.ProbRaw <= 0 || r.ProbRaw >= 1 ||
			r.ProjMean <= 0 || math.IsNaN(r.ProjMean) || math.IsInf(r.ProjMean, 0) ||
			(!legacy && !r.Verified()) {
			continue
		}
		if picklog.Grade(r.Entry, r.Actual, 0, true).Result != r.Result {
			continue
		}
		eligible = append(eligible, r)
	}
	rows, _ = picklog.DedupeSettled(eligible)
	return rows
}

func comboAudit(w io.Writer, dir string, legacy bool) error {
	rows, err := picklog.LoadSettled(filepath.Join(dir, "combosettled.jsonl"))
	if err != nil {
		return err
	}
	rows = auditRows(rows, legacy)
	fmt.Fprintln(w, "# Combo diagnostics")
	fmt.Fprintf(w, "\n%d unique binary outcomes. Read-only: no models, selections or public records changed.\n", len(rows))
	if legacy {
		fmt.Fprintln(w, "Includes historical quotes with incomplete provenance. Cached settlement identities match; this is exploratory, not verified performance or evidence of a profitable strategy.")
	}
	fmt.Fprintln(w, "\nPositive actual-minus-projection bias means projections were too low. Raw confidence is the simulation before calibration; it is not a demonstrated hit rate. Selected plays alone cannot calibrate all available markets.")
	fmt.Fprintln(w, "\n| Combo / side | Outcomes | Dates | Hit rate | Published probability | Raw confidence | Actual minus projection | Stat MAE |")
	fmt.Fprintln(w, "| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, st := range []string{"all", "pra", "pr", "pa", "ra"} {
		for _, side := range []string{"all", "OVER", "UNDER"} {
			n, wins := 0, 0
			dates := map[string]bool{}
			var published, raw, bias, mae float64
			for _, r := range rows {
				if (st != "all" && r.Stat != st) || (side != "all" && r.Side != side) {
					continue
				}
				n++
				dates[r.Date] = true
				if r.Result == "WIN" {
					wins++
				}
				published += r.Prob
				raw += r.ProbRaw
				bias += r.Actual - r.ProjMean
				mae += math.Abs(r.Actual - r.ProjMean)
			}
			if n == 0 {
				continue
			}
			v := float64(n)
			fmt.Fprintf(w, "| %s / %s | %d | %d | %.1f%% | %.1f%% | %.1f%% | %+.2f | %.2f |\n", st, side, n, len(dates), 100*float64(wins)/v, 100*published/v, 100*raw/v, bias/v, mae/v)
		}
	}
	return nil
}
