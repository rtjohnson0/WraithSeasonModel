// Command sweep reads graded plays (data/settled.jsonl) and shows, for each side and each
// edge cutoff, how many plays would have been posted, the hit rate against break-even, and flat units.
// Use it to choose the posting cutoff from live results instead of guessing.
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"propsmodel/internal/picklog"
)

func breakEven(odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	if odds < 0 {
		return float64(-odds) / float64(-odds+100)
	}
	return 100 / float64(odds+100)
}

func main() {
	dir := flag.String("dir", "data", "data directory")
	stat := flag.String("stat", "", "limit to one stat (points, rebounds, assists, threes)")
	minN := flag.Int("minn", 30, "flag cutoffs with fewer graded plays than this as thin")
	flag.Parse()

	all, err := picklog.LoadSettled(filepath.Join(*dir, "settled.jsonl"))
	if err != nil {
		fmt.Println("could not read settled.jsonl:", err)
		return
	}
	var rows []picklog.Settled
	for _, s := range all {
		if s.Result != "WIN" && s.Result != "LOSS" {
			continue
		}
		if *stat != "" && s.Stat != *stat {
			continue
		}
		rows = append(rows, s)
	}
	fmt.Printf("%d graded plays (pushes and voids excluded)\n", len(rows))

	cuts := []float64{0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.08}
	for _, side := range []string{"UNDER", "OVER"} {
		var sub []picklog.Settled
		days := map[string]bool{}
		for _, s := range rows {
			if s.Side == side {
				sub = append(sub, s)
				days[s.Date] = true
			}
		}
		fmt.Printf("\n%s  (%d plays over %d days)\n", side, len(sub), len(days))
		if len(sub) == 0 {
			fmt.Println("  none graded, so no cutoff can be chosen for this side yet")
			continue
		}
		fmt.Println("  edge>=   plays  /day   hit%   breakeven%  model%   units   note")
		for _, c := range cuts {
			var n, w int
			var be, pm, u float64
			d := map[string]bool{}
			for _, s := range sub {
				edge := s.Prob - breakEven(s.Odds)
				if edge < c {
					continue
				}
				n++
				d[s.Date] = true
				be += breakEven(s.Odds)
				pm += s.Prob
				u += s.Flat
				if s.Result == "WIN" {
					w++
				}
			}
			if n == 0 {
				continue
			}
			f := float64(n)
			note := ""
			if n < *minN {
				note = "thin"
			} else if float64(w)/f > be/f {
				note = "beats break-even"
			}
			fmt.Printf("  %5.2f  %6d  %5.1f  %5.1f   %8.1f  %6.1f  %+7.1f   %s\n",
				c, n, f/float64(len(d)), 100*float64(w)/f, 100*be/f, 100*pm/f, u, note)
		}
	}

	// calibration by predicted probability, both sides together
	fmt.Println("\nBY MODEL PROBABILITY (chosen side)")
	bins := []float64{0.45, 0.50, 0.55, 0.60, 0.65, 0.70, 1.01}
	sort.Float64s(bins)
	for i := 0; i+1 < len(bins); i++ {
		var n, w int
		var pm float64
		for _, s := range rows {
			if s.Prob >= bins[i] && s.Prob < bins[i+1] {
				n++
				pm += s.Prob
				if s.Result == "WIN" {
					w++
				}
			}
		}
		if n > 0 {
			fmt.Printf("  %.2f-%.2f  n=%-4d model %.1f%%  actual %.1f%%\n", bins[i], bins[i+1], n, 100*pm/float64(n), 100*float64(w)/float64(n))
		}
	}
}
