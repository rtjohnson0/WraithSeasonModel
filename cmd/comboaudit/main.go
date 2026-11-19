package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/comboaudit"
	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/rotation"
)

func main() {
	dir := flag.String("dir", "data", "data directory")
	policy := flag.String("policy", rotation.ContextPolicy, "V2 policy to inspect")
	date := flag.String("date", "", "optional slate date")
	dest := flag.String("out", "run_evidence/combo-audit/report.json", "diagnostic JSON path")
	flag.Parse()
	if !rotation.KnownPolicy(*policy) {
		fail(fmt.Errorf("unknown policy %q", *policy))
	}
	_, name := rotation.Files(*policy)
	settled, err := picklog.LoadSettled(filepath.Join(*dir, name))
	if err != nil {
		fail(err)
	}
	finals, err := data.LoadStatsResearchDir(*dir)
	if err != nil {
		fail(err)
	}
	report := comboaudit.Build(settled, finals, *date)
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	if err = os.MkdirAll(filepath.Dir(*dest), 0755); err != nil {
		fail(err)
	}
	if err = os.WriteFile(*dest, b, 0644); err != nil {
		fail(err)
	}
	fmt.Printf("%s combo audit: %d matched forecasts; %d duplicates collapsed\n", *policy, len(report.Rows), report.Duplicates)
	fmt.Println("slice  N  hit%  estimate%  actual-projection  minutes bias  minutes component  rate component")
	for _, s := range report.Slices {
		fmt.Printf("%-12s %3d %5.1f %8.1f %+18.2f %+13.2f %+18.2f %+15.2f\n", s.Name, s.N, 100*s.HitRate, 100*s.Predicted, s.Bias, s.MinutesBias, s.MinutesComponent, s.RateComponent)
	}
	fmt.Println(report.Note)
	fmt.Println("Saved", *dest)
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
