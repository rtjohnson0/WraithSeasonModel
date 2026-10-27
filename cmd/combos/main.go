// Command combos is the toolbox for the combined markets: points + rebounds + assists (PRA),
// points + rebounds (PR), points + assists (PA) and rebounds + assists (RA).
//
//	go run ./cmd/combos count            how many combo lines are in your snapshots (and from which books)
//	go run ./cmd/combos fit              measure the correlation between the stats, save data/combocorr.json
//	go run ./cmd/combos check            test the combo distributions against this season's games
//	go run ./cmd/combos settle           grade the combo plays predict logged (data/combolog.jsonl)
//
// Combo plays are tracked here, apart from the pick record, until they have earned a place in it.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/props"
	"propsmodel/internal/sgo"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dir := fs.String("data", "data", "data directory")
	switch cmd {
	case "count":
		days := fs.Int("days", 3, "how many of the latest snapshot days to count")
		date := fs.String("date", "", "count one day only (YYYY-MM-DD)")
		fs.Parse(args)
		count(*dir, *date, *days)
	case "fit":
		season := fs.Int("season", 0, "season start year (default: the current season)")
		write := fs.Bool("write", true, "save data/combocorr.json")
		fs.Parse(args)
		fit(*dir, seasonOr(*season), *write)
	case "check":
		season := fs.Int("season", 0, "season start year (default: the current season)")
		maxN := fs.Int("n", 1500, "most player-games to simulate")
		fs.Parse(args)
		check(*dir, seasonOr(*season), *maxN)
	case "settle":
		fs.Parse(args)
		settle(*dir)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: go run ./cmd/combos count|fit|check|settle [flags]")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func seasonOr(s int) int {
	if s != 0 {
		return s
	}
	now := time.Now()
	if now.Month() < 8 {
		return now.Year() - 1
	}
	return now.Year()
}

// ---- count ----------------------------------------------------------------------------------

func snapshotDates(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "snapshots", "props_*.jsonl"))
	var out []string
	for _, f := range files {
		b := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "props_"), ".jsonl")
		out = append(out, b)
	}
	sort.Strings(out)
	return out
}

// propTypes counts the prop_type values in a raw BALLDONTLIE snapshot, so it is clear which markets
// the feed really sends.
func propTypes(path string) map[string]int {
	out := map[string]int{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		var row struct {
			Response struct {
				Data []struct {
					PropType string `json:"prop_type"`
				} `json:"data"`
			} `json:"response"`
		}
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		for _, p := range row.Response.Data {
			out[p.PropType]++
		}
	}
	return out
}

func count(dir, date string, days int) {
	dates := snapshotDates(dir)
	if date != "" {
		dates = []string{date}
	} else if len(dates) > days {
		dates = dates[len(dates)-days:]
	}
	if len(dates) == 0 {
		fmt.Println("no snapshots/props_DATE.jsonl files found. Run: go run ./cmd/pull snapshot")
		return
	}
	for _, dt := range dates {
		path := filepath.Join(dir, "snapshots", fmt.Sprintf("props_%s.jsonl", dt))
		fmt.Printf("=== %s ===\n", dt)
		types := propTypes(path)
		var names []string
		for k := range types {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Print("BALLDONTLIE prop types in the raw snapshot:")
		for _, k := range names {
			fmt.Printf(" %s=%d", k, types[k])
		}
		fmt.Println()

		single, _ := props.LoadSnapshot(path)
		combos, _ := props.LoadSnapshotCombos(path)
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "source\tstat\tlines\tplayers\tbooks\tmarkets with 4+ books")
		report := func(src string, stat data.Stat, keys, players, books map[string]bool, perMarket map[string]map[string]bool) {
			four := 0
			for _, bs := range perMarket {
				if len(bs) >= 4 {
					four++
				}
			}
			n := 0
			for _, bs := range perMarket {
				n += len(bs)
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d of %d\n", src, stat, n, len(players), len(books), four, len(perMarket))
		}
		for _, stat := range append(append([]data.Stat(nil), data.Stats...), data.Combos...) {
			src := single
			if stat.IsCombo() {
				src = combos
			}
			players, books := map[string]bool{}, map[string]bool{}
			per := map[string]map[string]bool{}
			for _, l := range src {
				if l.Stat != stat {
					continue
				}
				players[fmt.Sprint(l.PlayerID)] = true
				books[l.Vendor] = true
				k := fmt.Sprintf("%d|%d", l.GameID, l.PlayerID)
				if per[k] == nil {
					per[k] = map[string]bool{}
				}
				per[k][l.Vendor] = true
			}
			if len(per) > 0 || stat.IsCombo() {
				report("balldontlie", stat, nil, players, books, per)
			}
		}
		sg, _ := sgo.LoadSnapshot(filepath.Join(dir, "snapshots", fmt.Sprintf("sgo_%s.jsonl", dt)))
		sgc, _ := sgo.LoadSnapshotCombos(filepath.Join(dir, "snapshots", fmt.Sprintf("sgo_%s.jsonl", dt)))
		if len(sg)+len(sgc) > 0 {
			for _, stat := range append(append([]data.Stat(nil), data.Stats...), data.Combos...) {
				src := sg
				if stat.IsCombo() {
					src = sgc
				}
				players, books := map[string]bool{}, map[string]bool{}
				per := map[string]map[string]bool{}
				for _, l := range src {
					if l.Stat != stat {
						continue
					}
					players[l.PlayerKey] = true
					books[l.Book] = true
					if per[l.PlayerKey] == nil {
						per[l.PlayerKey] = map[string]bool{}
					}
					per[l.PlayerKey][l.Book] = true
				}
				if len(per) > 0 || stat.IsCombo() {
					report("sportsgameodds", stat, nil, players, books, per)
				}
			}
		}
		tw.Flush()
		fmt.Println()
	}
	fmt.Println("If a combo row shows 0 lines, that feed did not send that market on those days (or, for sportsgameodds,")
	fmt.Println("the snapshot was taken before combos were added to the request: run `go run ./cmd/sgo snapshot` again).")
}

// ---- fit and check -----------------------------------------------------------------------------

func samplesFor(dir string, season int) []eval.BiasSample {
	d, err := pipeline.Load(dir, true)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Projecting every eligible player-game of the %d season with a model trained on earlier months only...\n\n", season)
	feat.BlowoutAdjust, feat.ShortExitAdjust = true, true
	s, err := eval.BiasCheck(d.Rows, season, pipeline.DisabledFor(false))
	if err != nil {
		fatal(err)
	}
	if len(s) == 0 {
		fmt.Println("No games to check for that season.")
		os.Exit(0)
	}
	return s
}

// residuals are each stat's miss in standard deviations, with the minutes he really played held fixed.
// Whatever they share is the link between the stats that minutes do not explain.
func residuals(s eval.BiasSample) (r [3]float64, ok bool) {
	for j, st := range []data.Stat{data.Points, data.Rebounds, data.Assists} {
		k := data.StatIndex(st)
		mean := s.ActualMin * s.Pred0.Rate[k]
		v := mean * (1 + s.Pred0.Alpha[k]*mean)
		if mean <= 0.05 || v <= 0 {
			return r, false
		}
		r[j] = (s.Actual[k] - mean) / math.Sqrt(v)
	}
	return r, true
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	var mx, my float64
	for i := range x {
		mx += x[i] / n
		my += y[i] / n
	}
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	if sxx <= 0 || syy <= 0 {
		return 0
	}
	return sxy / math.Sqrt(sxx*syy)
}

func fit(dir string, season int, write bool) {
	samples := samplesFor(dir, season)
	var pts, reb, ast []float64
	for _, s := range samples {
		if r, ok := residuals(s); ok {
			pts, reb, ast = append(pts, r[0]), append(reb, r[1]), append(ast, r[2])
		}
	}
	c := combo.Corr{PR: pearson(pts, reb), PA: pearson(pts, ast), RA: pearson(reb, ast), Samples: len(pts), Season: season, Updated: time.Now().Format("2006-01-02")}
	se := 1 / math.Sqrt(float64(len(pts)))
	fmt.Printf("Correlation between the stats once minutes are fixed (%d player-games, standard error about %.3f):\n", len(pts), se)
	fmt.Printf("  points-rebounds  %+.3f   (default %+.2f)\n", c.PR, combo.Default.PR)
	fmt.Printf("  points-assists   %+.3f   (default %+.2f)\n", c.PA, combo.Default.PA)
	fmt.Printf("  rebounds-assists %+.3f   (default %+.2f)\n", c.RA, combo.Default.RA)
	// Shrink toward the default so a thin sample cannot swing the width of every combo.
	w := float64(len(pts)) / (float64(len(pts)) + 400)
	c.PR = w*c.PR + (1-w)*combo.Default.PR
	c.PA = w*c.PA + (1-w)*combo.Default.PA
	c.RA = w*c.RA + (1-w)*combo.Default.RA
	fmt.Printf("\nUsed (shrunk toward the defaults by %.0f%% for the small sample): PR %+.3f, PA %+.3f, RA %+.3f\n", 100*(1-w), c.PR, c.PA, c.RA)
	if write {
		path := filepath.Join(dir, "combocorr.json")
		if err := c.Save(path); err != nil {
			fatal(err)
		}
		fmt.Println("Saved", path)
	}
}

func check(dir string, season, maxN int) {
	samples := samplesFor(dir, season)
	corr := combo.Load(filepath.Join(dir, "combocorr.json"))
	fmt.Printf("Correlation used: PR %+.3f, PA %+.3f, RA %+.3f (%s)\n\n", corr.PR, corr.PA, corr.RA, map[bool]string{true: "fitted", false: "default; run `combos fit` first"}[corr.Fitted()])
	stride := 1
	if len(samples) > maxN {
		stride = (len(samples) + maxN - 1) / maxN
	}
	type acc struct {
		n          int
		err, errSq float64
		sdSum      float64
		pit        [10]int
		logIndep   float64
		logCorr    float64
	}
	res := map[data.Stat]*acc{}
	for _, s := range data.Combos {
		res[s] = &acc{}
	}
	rng := rand.New(rand.NewSource(7))
	used := 0
	for i := 0; i < len(samples); i += stride {
		smp := samples[i]
		dist := combo.Simulate(smp.Pred0, corr, 300, int64(i)+1)
		indep := combo.Simulate(smp.Pred0, combo.Corr{}, 300, int64(i)+1)
		used++
		for _, st := range data.Combos {
			var actual float64
			for _, p := range st.Parts() {
				actual += smp.Actual[data.StatIndex(p)]
			}
			a := res[st]
			a.n++
			m := dist.Mean(st)
			a.err += actual - m
			a.errSq += (actual - m) * (actual - m)
			a.sdSum += dist.SD(st)
			k := int(actual)
			lo, hi := 0.0, dist.CDF(st, k)
			if k > 0 {
				lo = dist.CDF(st, k-1)
			}
			u := lo + rng.Float64()*(hi-lo)
			b := int(u * 10)
			if b > 9 {
				b = 9
			}
			a.pit[b]++
			floor := func(p float64) float64 { return math.Max(p, 1e-4) }
			a.logCorr += math.Log(floor(hi - lo))
			ilo, ihi := 0.0, indep.CDF(st, k)
			if k > 0 {
				ilo = indep.CDF(st, k-1)
			}
			a.logIndep += math.Log(floor(ihi - ilo))
		}
	}
	fmt.Printf("%d player-games simulated. A good model has: bias near 0, spread ratio near 1, and each tenth of the\n", used)
	fmt.Println("probability scale holding about 10% of the results (the lowest and highest tenths hold the tails).")
	fmt.Println()
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "combo\tn\tbias (actual-proj)\tspread ratio\tPIT tenths (% of results)\t10-90 band holds\tlog score vs independent")
	for _, st := range data.Combos {
		a := res[st]
		if a.n == 0 {
			continue
		}
		rmse := math.Sqrt(a.errSq / float64(a.n))
		ratio := rmse / (a.sdSum / float64(a.n))
		var tenths []string
		for _, c := range a.pit {
			tenths = append(tenths, fmt.Sprintf("%.0f", 100*float64(c)/float64(a.n)))
		}
		inBand := 100 * float64(a.n-a.pit[0]-a.pit[9]) / float64(a.n)
		fmt.Fprintf(tw, "%s\t%d\t%+.2f\t%.2f\t%s\t%.0f%%\t%+.3f per game\n", st, a.n, a.err/float64(a.n), ratio, strings.Join(tenths, " "), inBand, (a.logCorr-a.logIndep)/float64(a.n))
	}
	tw.Flush()
	fmt.Println()
	fmt.Println("spread ratio above 1: the model is too sure of itself (combos will look better than they are); below 1: too cautious.")
	fmt.Println("bias is the model's level, not its width: it is the sum of the single-stat biases `go run ./cmd/bias` reports.")
}

// ---- settle -----------------------------------------------------------------------------------

func settle(dir string) {
	entries, err := picklog.Load(filepath.Join(dir, "combolog.jsonl"))
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fmt.Println("data/combolog.jsonl is empty. Run `go run ./cmd/predict` first; it logs the combo plays that clear the bar.")
		return
	}
	settledPath := filepath.Join(dir, "combosettled.jsonl")
	done, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	have := map[string]bool{}
	key := func(date string, pid int, stat, side, book string, line float64) string {
		return fmt.Sprintf("%s|%d|%s|%s|%s|%.1f", date, pid, stat, side, book, line)
	}
	for _, s := range done {
		have[key(s.Date, s.PlayerID, s.Stat, s.Side, s.Book, s.Line)] = true
	}
	d, err := pipeline.Load(dir, false)
	if err != nil {
		fatal(err)
	}
	ix := d.Builder.Index()
	var fresh []picklog.Settled
	pending := 0
	for _, e := range entries {
		if have[key(e.Date, e.PlayerID, e.Stat, e.Side, e.Book, e.Line)] {
			continue
		}
		if _, known := ix.GameDate(e.GameID); !known {
			pending++
			continue
		}
		actual, played := ix.Actual(e.GameID, e.PlayerID)
		fresh = append(fresh, picklog.Grade(e, actual.Value(data.Stat(e.Stat)), 0, played))
	}
	n, err := picklog.AppendSettled(settledPath, fresh)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("settled %d new combo plays (%d still waiting for final stats)\n\n", n, pending)
	all, err := picklog.LoadSettled(settledPath)
	if err != nil {
		fatal(err)
	}
	every, _ := picklog.DedupeSettled(all)
	if len(every) == 0 {
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "slice\tplays\tW-L-P\thit rate\tpredicted\tflat 1u\tROI at the odds")
	row := func(name string, rs []picklog.Settled) {
		s := picklog.Summarize(rs)
		if s.Picks-s.Voids == 0 {
			return
		}
		roi := 0.0
		if s.Wins+s.Losses > 0 {
			roi = s.Flat / float64(s.Wins+s.Losses)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d-%d-%d\t%.1f%%\t%.1f%%\t%+.1f\t%+.1f%%\n", name, s.Picks-s.Voids, s.Wins, s.Losses, s.Pushes, 100*s.HitRate(), 100*s.AvgProb, s.Flat, 100*roi)
	}
	row("all combos", every)
	for _, st := range data.Combos {
		var sub []picklog.Settled
		for _, r := range every {
			if r.Stat == string(st) {
				sub = append(sub, r)
			}
		}
		row(strings.ToUpper(string(st)), sub)
	}
	for _, side := range []string{"OVER", "UNDER"} {
		var sub []picklog.Settled
		for _, r := range every {
			if r.Side == side {
				sub = append(sub, r)
			}
		}
		row(side, sub)
	}
	for _, band := range [][2]float64{{0.55, 0.60}, {0.60, 0.65}, {0.65, 1.01}} {
		var sub []picklog.Settled
		for _, r := range every {
			if r.Prob >= band[0] && r.Prob < band[1] {
				sub = append(sub, r)
			}
		}
		row(fmt.Sprintf("model %.0f-%.0f%%", 100*band[0], math.Min(100, 100*band[1])), sub)
	}
	tw.Flush()
	fmt.Println()
	fmt.Println("These are paper plays at flat 1 unit. Wait for about 100 graded before reading anything into the hit rate;")
	fmt.Println("until then the \"predicted\" column against the hit rate is the number to watch.")
}
