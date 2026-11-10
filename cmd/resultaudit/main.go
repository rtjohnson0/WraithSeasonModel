// Command resultaudit reconciles every saved full-game result with final stats.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"propsmodel/internal/picklog"
	"propsmodel/internal/settlement"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	flag.Parse()
	for _, pair := range [][2]string{{"picklog.jsonl", "settled.jsonl"}, {"combolog.jsonl", "combosettled.jsonl"}, {"v2log.jsonl", "v2settled.jsonl"}} {
		_, a := os.Stat(filepath.Join(*dir, pair[0]))
		_, b := os.Stat(filepath.Join(*dir, pair[1]))
		if os.IsNotExist(a) && os.IsNotExist(b) {
			continue
		}
		entries, e := picklog.Load(filepath.Join(*dir, pair[0]))
		if e != nil {
			fatal(e)
		}
		r, e := settlement.Reconcile(*dir, pair[1], entries, nil)
		if e != nil {
			fatal(e)
		}
		fmt.Printf("%s: %d checked, %d confirmed, %d for review; %d changes.\n", pair[1], r.Checked, r.Confirmed, r.Review, len(r.Changes))
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
