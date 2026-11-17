// Command audit checks the prop-line DATA for problems that could make a board lean one way,
// before blaming the model. For one day's snapshot it reports:
//
//  1. alternate lines: books that list several lines for one player and stat (the loader keeps the
//     main one; if the feed sends alternates, taking the wrong one tilts everything under or over)
//
//  2. vendor offsets: whether one book's lines sit above or below the other books
//
//  3. price lean: whether the books price overs or unders as more likely
//
//  4. lines against the player's own form: how far the line sits above or below his last-10 average
//
//     go run ./cmd/audit                  (today, US Eastern)
//     go run ./cmd/audit -date 2026-10-26
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/props"
)

type mkey struct {
	game, player int
	stat         data.Stat
}

func median(x []float64) float64 {
	c := append([]float64(nil), x...)
	sort.Float64s(c)
	n := len(c)
	if n == 0 {
		return math.NaN()
	}
	if n%2 == 0 {
		return (c[n/2-1] + c[n/2]) / 2
	}
	return c[n/2]
}

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "snapshot day YYYY-MM-DD (default: today, US Eastern)")
	flag.Parse()
	if *date == "" {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			loc = time.Local
		}
		*date = time.Now().In(loc).Format("2006-01-02")
	}
	path := filepath.Join(*dir, "snapshots", fmt.Sprintf("props_%s.jsonl", *date))
	all, err := props.LoadAllLines(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err, "(run: go run ./cmd/pull snapshot)")
		os.Exit(1)
	}
	main, err := props.LoadSnapshot(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d line entries in the feed, %d after choosing one main line per book, player and stat\n\n", *date, len(all), len(main))

	// 1. Alternate lines within one capture.
	type vk struct{ vendor string }
	type rk struct {
		row, game, player int
		vendor            string
		stat              data.Stat
	}
	seen := map[rk]map[float64]bool{}
	for _, l := range all {
		k := rk{l.Row, l.GameID, l.PlayerID, l.Vendor, l.Stat}
		if seen[k] == nil {
			seen[k] = map[float64]bool{}
		}
		seen[k][l.Line] = true
	}
	multiBy := map[string]int{}
	totBy := map[string]int{}
	for k, ls := range seen {
		totBy[k.vendor]++
		if len(ls) > 1 {
			multiBy[k.vendor]++
		}
	}
	fmt.Println("1. ALTERNATE LINES (same book, player and stat, more than one line in one capture)")
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "book\tmarkets\twith alternates\tshare")
	var vendors []string
	for v := range totBy {
		vendors = append(vendors, v)
	}
	sort.Strings(vendors)
	anyAlt := false
	for _, v := range vendors {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.0f%%\n", v, totBy[v], multiBy[v], 100*float64(multiBy[v])/float64(totBy[v]))
		if multiBy[v] > 0 {
			anyAlt = true
		}
	}
	tw.Flush()
	if anyAlt {
		fmt.Println("Alternates exist. The loader now keeps the line with the most even over/under prices, so they no longer tilt the board.")
	} else {
		fmt.Println("No alternate lines in this feed, so line choice is not the cause.")
	}
	fmt.Println()

	// Median main line per market across books, and each book's offset from it.
	groups := map[mkey][]props.Line{}
	for _, l := range main {
		k := mkey{l.GameID, l.PlayerID, l.Stat}
		groups[k] = append(groups[k], l)
	}
	med := map[mkey]float64{}
	for k, ls := range groups {
		xs := make([]float64, len(ls))
		for i, l := range ls {
			xs[i] = l.Line
		}
		med[k] = median(xs)
	}

	// 2. Vendor offsets.
	type acc struct {
		sum float64
		n   int
	}
	off := map[string]*acc{}
	for k, ls := range groups {
		if len(ls) < 3 {
			continue
		}
		for _, l := range ls {
			a := off[l.Vendor]
			if a == nil {
				a = &acc{}
				off[l.Vendor] = a
			}
			a.sum += l.Line - med[k]
			a.n++
		}
	}
	fmt.Println("2. EACH BOOK'S LINES VS THE MEDIAN OF ALL BOOKS (markets with 3+ books)")
	tw = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "book\tmarkets\tavg line minus median")
	for _, v := range vendors {
		if a := off[v]; a != nil && a.n >= 20 {
			fmt.Fprintf(tw, "%s\t%d\t%+.2f\n", v, a.n, a.sum/float64(a.n))
		}
	}
	tw.Flush()
	fmt.Println("A book far from 0 here posts different lines from the rest; one book dominating the board can tilt it.")
	fmt.Println()

	// 3. Price lean.
	type lean struct {
		n                     int
		overShort, underShort int
		pOver                 float64
	}
	leans := map[data.Stat]*lean{}
	for _, l := range main {
		if l.OverOdds == 0 || l.UnderOdds == 0 {
			continue
		}
		a := leans[l.Stat]
		if a == nil {
			a = &lean{}
			leans[l.Stat] = a
		}
		a.n++
		a.pOver += bets.MarketOver(l.OverOdds, l.UnderOdds)
		switch {
		case l.OverOdds < l.UnderOdds:
			a.overShort++
		case l.UnderOdds < l.OverOdds:
			a.underShort++
		}
	}
	fmt.Println("3. WHICH SIDE THE BOOKS PRICE AS MORE LIKELY (vig removed)")
	tw = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "stat\tlines\tavg chance of over\tover favoured\tunder favoured")
	for _, s := range data.Stats {
		if a := leans[s]; a != nil && a.n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%.1f%%\t%.0f%%\t%.0f%%\n", s, a.n, 100*a.pOver/float64(a.n),
				100*float64(a.overShort)/float64(a.n), 100*float64(a.underShort)/float64(a.n))
		}
	}
	tw.Flush()
	fmt.Println("If the books themselves favour unders on most lines, the lean is in the market, not the model.")
	fmt.Println()

	// 4. Lines against form.
	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fmt.Println("(no stats history, so the form check is skipped:", err, ")")
		return
	}
	ix := d.Builder.Index()
	type form struct {
		n                     int
		gap10, gapSeason      float64
		above10, aboveSeasonN int
		seasonN               int
		aboveSeason           int
	}
	forms := map[data.Stat]*form{}
	for k, m := range med {
		hist := ix.History(k.player)
		var prior []data.PlayerGame
		for _, g := range hist {
			if g.Date < *date {
				prior = append(prior, g)
			}
		}
		if len(prior) < 10 {
			continue
		}
		last := prior[len(prior)-10:]
		var s10 float64
		for _, g := range last {
			s10 += g.Value(k.stat)
		}
		s10 /= 10
		f := forms[k.stat]
		if f == nil {
			f = &form{}
			forms[k.stat] = f
		}
		f.n++
		f.gap10 += m - s10
		if m > s10 {
			f.above10++
		}
		season := prior[len(prior)-1].Season
		var ss float64
		var sn int
		for _, g := range prior {
			if g.Season == season {
				ss += g.Value(k.stat)
				sn++
			}
		}
		if sn >= 3 {
			ss /= float64(sn)
			f.seasonN++
			f.gapSeason += m - ss
			if m > ss {
				f.aboveSeason++
			}
		}
	}
	fmt.Println("4. THE LINE VS THE PLAYER'S OWN NUMBERS (median line across books; players with 10+ games on file)")
	tw = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "stat\tlines\tline minus last-10 avg\tline above last-10 avg\tline minus this-season avg\tline above this-season avg")
	for _, s := range data.Stats {
		f := forms[s]
		if f == nil || f.n == 0 {
			continue
		}
		ss, sa := "n/a", "n/a"
		if f.seasonN > 0 {
			ss = fmt.Sprintf("%+.2f", f.gapSeason/float64(f.seasonN))
			sa = fmt.Sprintf("%.0f%%", 100*float64(f.aboveSeason)/float64(f.seasonN))
		}
		fmt.Fprintf(tw, "%s\t%d\t%+.2f\t%.0f%%\t%s\t%s\n", s, f.n, f.gap10/float64(f.n), 100*float64(f.above10)/float64(f.n), ss, sa)
	}
	tw.Flush()
	fmt.Println()
	fmt.Println("How to read section 4: books normally set lines near a player's recent average, so about half of the lines should sit above it.")
	fmt.Println("If most lines sit well ABOVE his last-10 and season averages, the books are expecting more than the recent games show")
	fmt.Println("(early-season lines lean on last year), and an all-unders board is the market's doing. If the lines sit near his averages")
	fmt.Println("but the model still says under, the model is the one that is off.")
}
