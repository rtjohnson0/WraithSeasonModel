// Command train fits the full model on every game in data/ and saves it to data/model.json.
//
//	go run ./cmd/train
//	go run ./cmd/train -lineup     (include teammates-out features; needs injury snapshots when predicting)
//
// Retrain whenever you pull new stats (the daily scripts do this), so the model always reflects
// the latest games. It takes seconds.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"propsmodel/internal/data"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	lineup := flag.Bool("lineup", false, "include teammates-out features")
	missed := flag.Bool("missed", false, "add the expected-missed-shots rebound feature (backtest it first; the calibration must be refitted)")
	flag.Parse()
	pipeline.Missed = *missed

	d, err := pipeline.Load(*dir, true)
	if err != nil {
		fatal(err)
	}
	disabled := pipeline.DisabledFor(*lineup)
	m, err := predictor.Train(d.Rows, disabled)
	if err != nil {
		fatal(err)
	}
	out := filepath.Join(*dir, "model.json")
	if err := m.Save(out); err != nil {
		fatal(err)
	}
	fmt.Printf("trained on %d player-games through %s, feature set %q\n", m.Rows, m.TrainedBefore, pipeline.ConfigName(disabled))
	fmt.Printf("saved %s\n\n", out)

	fmt.Println("Minutes model, largest effects (minutes per standard deviation of the feature):")
	for i, n := range m.Minutes.Names {
		fmt.Printf("  %-14s %+.2f\n", n, m.Minutes.Coef[i])
	}
	for _, s := range data.Stats {
		fmt.Printf("\n%s model (dispersion alpha %.3f), effect on per-minute rate per standard deviation:\n", s, m.Stat[data.StatIndex(s)].Alpha)
		for _, c := range m.Importance(s) {
			fmt.Printf("  %-14s %+.3f  (%+.1f%%)\n", c.Name, c.Value, 100*math.Expm1(c.Value))
		}
	}
	fmt.Println("\nNext: go run ./cmd/predict")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
