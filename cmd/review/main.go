// Command review grades a day's logged plays against what actually happened and writes a CSV
// that says what hit, what missed, and the most likely reason for each.
//
//	go run ./cmd/pull stats -new -seasons 2026      (so the games are in the stats)
//	go run ./cmd/review                             (yesterday)
//	go run ./cmd/review -date 2026-10-22
//	go run ./cmd/review -all                        (every logged day, one CSV)
//
// It reads data/picklog.jsonl, so it covers every tier (PICK, CHECK, PAPER, LEAN), and it grades
// from the box scores itself, so it does not depend on `settle` having run. The reason column is a
// diagnosis from the box score (minutes, blowout, how far the stat was from the projection), not
// proof; a single game cannot tell skill from luck, so the summary counts reasons across plays.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "day to review as YYYY-MM-DD (default: yesterday)")
	all := flag.Bool("all", false, "review every logged day")
	flag.Parse()

	if *date == "" {
		*date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	entries, err := picklog.Load(filepath.Join(*dir, "picklog.jsonl"))
	if err != nil {
		fatal(err)
	}
	var day []picklog.Entry
	for _, e := range entries {
		if *all || e.Date == *date {
			day = append(day, e)
		}
	}
	if len(day) == 0 {
		fmt.Printf("No plays are logged for %s. Run `go run ./cmd/predict` on a game day first (it logs every play).\n", *date)
		return
	}
	day, dups := dedupe(day)

	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fatal(err)
	}
	ix := d.Builder.Index()

	// Final team scores per game, from the box scores, to spot blowouts.
	type tg struct{ game, team int }
	score := map[tg]float64{}
	for _, g := range d.Games {
		score[tg{g.GameID, g.TeamID}] += g.Pts
	}

	var outs []outcome
	waiting := 0
	for _, e := range day {
		if _, known := ix.GameDate(e.GameID); !known {
			waiting++
			continue
		}
		act, played := ix.Actual(e.GameID, e.PlayerID)
		o := outcome{E: e, Played: played}
		if played {
			o.Actual = act.Value(data.Stat(e.Stat))
			o.Minutes = act.Minutes
			own := score[tg{e.GameID, act.TeamID}]
			opp := 0.0
			if act.HomeTeamID != 0 {
				other := act.HomeTeamID
				if act.TeamID == act.HomeTeamID {
					other = act.VisitorTeamID
				}
				opp = score[tg{e.GameID, other}]
			}
			if own > 0 && opp > 0 {
				o.Margin, o.HasMar = own-opp, true
			}
		}
		diagnose(&o)
		outs = append(outs, o)
	}
	if len(outs) == 0 {
		fmt.Printf("None of the %d plays are final in your stats yet (%d waiting). Run `go run ./cmd/pull stats -new -seasons 2026` and try again.\n", len(day), waiting)
		return
	}
	sortOutcomes(outs)

	name := *date
	if *all {
		name = "all"
	}
	path := filepath.Join(*dir, fmt.Sprintf("review_%s.csv", name))
	if err := writeCSV(path, outs); err != nil {
		fatal(err)
	}

	label := *date
	if *all {
		label = "all logged days"
	}
	fmt.Printf("REVIEW %s: %d plays graded (%d duplicate log lines collapsed, %d still waiting for final stats)\n\n", label, len(outs), dups, waiting)
	summary(outs)
	fmt.Printf("\nSaved %s\n", path)
}

func writeCSV(path string, outs []outcome) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"date", "result", "tier", "player", "team", "stat", "pick", "line", "actual", "past_line", "prob_pct", "odds", "book",
		"units", "proj_stat", "proj_min", "actual_min", "min_diff", "team_margin", "reason_type", "reason", "why_picked"})
	f1 := func(x float64) string { return strconv.FormatFloat(x, 'f', 1, 64) }
	for _, o := range outs {
		e := o.E
		tier := e.Tier
		if tier == "" {
			tier = "PICK"
		}
		actual, am, md, mar, past := "", "", "", "", ""
		if o.Played {
			actual, am, md = f1(o.Actual), f1(o.Minutes), f1(o.Minutes-e.ProjMin)
			past = f1(o.pastLine())
			if o.HasMar {
				mar = fmt.Sprintf("%+.0f", o.Margin)
			}
		}
		odds := ""
		if e.Odds != 0 {
			odds = strconv.Itoa(e.Odds)
		}
		w.Write([]string{e.Date, o.Result, tier, e.Player, e.Team, e.Stat, e.Side, f1(e.Line), actual, past, f1(100 * e.Prob), odds, e.Book,
			strconv.FormatFloat(o.Units(), 'f', 2, 64), f1(e.ProjMean), f1(e.ProjMin), am, md, mar, o.Category, o.Reason, e.Why})
	}
	w.Flush()
	return w.Error()
}

func summary(outs []outcome) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "slice\tW-L-P\thit rate\tpredicted\tunits")
	row := func(name string, rs []outcome) {
		var w, l, p int
		var prob, units float64
		for _, o := range rs {
			switch o.Result {
			case "WIN":
				w++
			case "LOSS":
				l++
			case "PUSH":
				p++
			default:
				continue
			}
			prob += o.E.Prob
			units += o.Units()
		}
		if w+l+p == 0 {
			return
		}
		hit := 0.0
		if w+l > 0 {
			hit = 100 * float64(w) / float64(w+l)
		}
		fmt.Fprintf(tw, "%s\t%d-%d-%d\t%.1f%%\t%.1f%%\t%+.1f\n", name, w, l, p, hit, 100*prob/float64(w+l+p), units)
	}
	row("all plays", outs)
	for _, t := range []string{"PICK", "CHECK", "PAPER", "LEAN"} {
		var sub []outcome
		for _, o := range outs {
			tt := o.E.Tier
			if tt == "" {
				tt = "PICK"
			}
			if tt == t {
				sub = append(sub, o)
			}
		}
		row(t, sub)
	}
	for _, st := range data.Stats {
		var sub []outcome
		for _, o := range outs {
			if o.E.Stat == string(st) {
				sub = append(sub, o)
			}
		}
		row(string(st), sub)
	}
	for _, side := range []string{"OVER", "UNDER"} {
		var sub []outcome
		for _, o := range outs {
			if o.E.Side == side {
				sub = append(sub, o)
			}
		}
		row(side, sub)
	}
	tw.Flush()
	fmt.Println("(units: logged stake for staked picks, flat 1u for everything else)")

	// Why the losses lost, and how the minutes and stat projections did overall.
	counts := map[string]int{}
	losses := 0
	for _, o := range outs {
		if o.Result == "LOSS" {
			counts[o.Category]++
			losses++
		}
	}
	if losses > 0 {
		fmt.Println("\nWHY THE LOSSES LOST")
		var keys []string
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
		meaning := map[string]string{
			"minutes":         "minutes went the wrong way (blowout, injury, role)",
			"close_call":      "missed by a point or less: the read was fine",
			"projection_miss": "stat far from projection with minutes on track: the model misread him",
			"variance":        "projection was close; just game-to-game noise",
		}
		for _, k := range keys {
			fmt.Printf("  %2d  %-16s %s\n", counts[k], k, meaning[k])
		}
	}

	var dMin, nMin, bias, nBias float64
	voids := 0
	for _, o := range outs {
		if !o.Played {
			voids++
			continue
		}
		if o.E.ProjMin > 0 {
			dMin += o.Minutes - o.E.ProjMin
			nMin++
		}
		if o.E.ProjMean > 0 {
			bias += o.Actual - o.E.ProjMean
			nBias++
		}
	}
	fmt.Println("\nHOW THE PROJECTIONS DID (all graded plays)")
	if nMin > 0 {
		fmt.Printf("  minutes: actual averaged %+.1f vs projected (negative means the model projected too many)\n", dMin/nMin)
	}
	if nBias > 0 {
		fmt.Printf("  stat:    actual averaged %+.2f vs projected\n", bias/nBias)
	}
	if voids > 0 {
		fmt.Printf("  %d play(s) never played (void)\n", voids)
	}
	// Bias by side tells you whether the one-sided board (all unders) was right to lean that way.
	for _, side := range []string{"OVER", "UNDER"} {
		var sum, n float64
		for _, o := range outs {
			if o.Played && o.E.Side == side && o.E.ProjMean > 0 {
				sum += o.Actual - o.E.ProjMean
				n++
			}
		}
		if n >= 5 {
			fmt.Printf("  %s plays: stat averaged %+.2f vs projected over %d plays\n", side, sum/n, int(n))
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
