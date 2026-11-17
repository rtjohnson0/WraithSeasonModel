// Command linecheck asks whether the BOOKS are running high or low this season, using every prop
// line in the saved snapshots and what the player actually did. It needs no model and no picks, so
// nothing is selected: if actual results sit well under the lines for everyone, an all-unders
// board is the market being early-season high, not only a model bias.
//
//	go run ./cmd/linecheck
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"text/tabwriter"

	"propsmodel/internal/data"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/props"
)

type key struct {
	game, player int
	stat         data.Stat
}

type tally struct {
	n, over, under     int
	gap, gapSq         float64 // actual minus line
	sumLine, sumActual float64
}

func (t *tally) add(line, actual float64) {
	if actual == line {
		return // a push says nothing about direction
	}
	t.n++
	if actual > line {
		t.over++
	} else {
		t.under++
	}
	d := actual - line
	t.gap += d
	t.gapSq += d * d
	t.sumLine += line
	t.sumActual += actual
}

func main() {
	dir := "data"
	d, err := pipeline.Load(dir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	ix := d.Builder.Index()
	files, _ := filepath.Glob(filepath.Join(dir, "snapshots", "props_*.jsonl"))
	sort.Strings(files)
	re := regexp.MustCompile(`props_(\d{4}-\d{2}-\d{2})\.jsonl$`)

	byStat := map[data.Stat]*tally{}
	byDay := map[string]*tally{}
	used := map[key]bool{}
	for _, f := range files {
		m := re.FindStringSubmatch(f)
		if m == nil {
			continue
		}
		lines, err := props.LoadSnapshot(f)
		if err != nil {
			continue
		}
		groups := map[key][]float64{}
		for _, l := range lines {
			k := key{l.GameID, l.PlayerID, l.Stat}
			groups[k] = append(groups[k], l.Line)
		}
		for k, ls := range groups {
			if used[k] {
				continue
			}
			if _, known := ix.GameDate(k.game); !known {
				continue // not played or not in the stats yet
			}
			actual, played := ix.Actual(k.game, k.player)
			if !played {
				continue
			}
			sort.Float64s(ls)
			line := ls[len(ls)/2]
			used[k] = true
			v := actual.Value(k.stat)
			if byStat[k.stat] == nil {
				byStat[k.stat] = &tally{}
			}
			if byDay[m[1]] == nil {
				byDay[m[1]] = &tally{}
			}
			byStat[k.stat].add(line, v)
			byDay[m[1]].add(line, v)
		}
	}
	if len(used) == 0 {
		fmt.Println("No finished games with saved prop lines yet (needs data/snapshots/props_*.jsonl and their box scores).")
		return
	}

	z := func(t *tally) float64 { // share over against 50%
		if t.n < 2 {
			return 0
		}
		p := float64(t.over) / float64(t.n)
		return (p - 0.5) / math.Sqrt(0.25/float64(t.n))
	}
	show := func(title string, rows [][2]interface{}) {
		fmt.Println(title)
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "\tlines\tavg line\tavg actual\tactual minus line\tWent over\tWent under\tz (over share vs 50%)")
		for _, r := range rows {
			t := r[1].(*tally)
			if t.n == 0 {
				continue
			}
			n := float64(t.n)
			fmt.Fprintf(tw, "%v\t%d\t%.2f\t%.2f\t%+.2f\t%.0f%%\t%.0f%%\t%+.1f\n", r[0], t.n, t.sumLine/n, t.sumActual/n, t.gap/n,
				100*float64(t.over)/n, 100*float64(t.under)/n, z(t))
		}
		tw.Flush()
		fmt.Println()
	}
	var sr [][2]interface{}
	all := &tally{}
	for _, s := range data.Stats {
		if t := byStat[s]; t != nil {
			sr = append(sr, [2]interface{}{s, t})
			all.n += t.n
			all.over += t.over
			all.under += t.under
			all.gap += t.gap
			all.sumLine += t.sumLine
			all.sumActual += t.sumActual
		}
	}
	sr = append([][2]interface{}{{"all stats", all}}, sr...)
	show("BOOKS VS RESULTS, BY STAT (median line across books, every line, no selection)", sr)
	var days []string
	for k := range byDay {
		days = append(days, k)
	}
	sort.Strings(days)
	var dr [][2]interface{}
	for _, k := range days {
		dr = append(dr, [2]interface{}{k, byDay[k]})
	}
	show("BY DAY", dr)
	fmt.Println("How to read this: if 'Went under' is well above 50% (z of -2 or lower), the market itself is running high")
	fmt.Println("early in the season, and an all-unders board is partly real. If it sits near 50% while the model leans under,")
	fmt.Println("the model is the one that is off. Compare with `go run ./cmd/bias`, which measures the model against results.")
}
