// lineupstudy evaluates league-pooled role associations on later observed
// rotations. It is a conditional diagnostic, never a pregame betting backtest.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/lineup"
	"propsmodel/internal/opportunity"
	"time"
)

func main() {
	dir := flag.String("data", "data", "historical final stats")
	before := flag.String("before", "2026-01-01", "fit only dates strictly before this cutoff")
	through := flag.String("through", "2026-07-01", "evaluate earlier dates, beginning at cutoff")
	dest := flag.String("out", "run_evidence/lineup-study", "output directory")
	contextual := flag.Bool("context", false, "include earlier opportunity inputs in the conditional study")
	flag.Parse()
	for _, d := range []string{*before, *through} {
		if _, e := time.Parse("2006-01-02", d); e != nil {
			panic(e)
		}
	}
	if *through <= *before {
		panic("through must follow cutoff")
	}
	rows, e := data.LoadStatsResearchDir(*dir)
	if e != nil {
		panic(e)
	}
	ix := lineup.NewRoles(rows)
	m := ix.FitRoles(*before)
	if *contextual {
		op, e := opportunity.Load(*dir, rows)
		if e != nil {
			panic(e)
		}
		m = ix.FitContext(*before, op)
	}
	report := struct {
		Note, FitBefore, Through       string
		TrainingPlayerGames            [data.NStats]int
		TrainingDates                  [data.NStats]int
		OpportunityTrainingPlayerGames [data.NStats]int
		OpportunityTrainingDates       [data.NStats]int
		Stats                          []lineup.RoleStudy
	}{"Held-out conditional study uses observed later rotation minutes. NOT a pregame backtest, injury-effect validation, or evidence of pick accuracy. Live uses projected rotations; error there is not measured here.", *before, *through, m.N, m.Days, m.OpportunityN, m.OpportunityDays, ix.StudyRoles(m, *through)}
	b, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.MkdirAll(*dest, 0755); e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(*dest, "report.json"), b, 0644); e != nil {
		panic(e)
	}
	fmt.Println(string(b))
}
