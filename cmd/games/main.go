// Command games handles the game markets: point spread, moneyline and game total.
//
//	go run ./cmd/pull gameodds -season 2025        (once: last season's opening lines)
//	go run ./cmd/games -backtest -season 2025      (tests the model against those lines; saves data/gamecal.json)
//	go run ./cmd/games                             (today's plays; needs `pull snapshot` first)
//
// Game lines are much sharper than player props. The backtest says whether the team model adds
// anything to the sportsbook's own number; the daily output is only worth following if it does.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	_ "time/tzdata"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/props"
	"propsmodel/internal/team"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	backtest := flag.Bool("backtest", false, "test against last season's opening lines instead of picking today's games")
	season := flag.Int("season", 2025, "season (start year) to backtest")
	date := flag.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	hi := flag.Float64("hi", 0.55, "post a play at or above this win probability (game lines have their own graded history, so this stays 0.55 until swept)")
	minEdge := flag.Float64("minedge", 0.01, "and at least this far above the break-even probability of the price")
	maxUnits := flag.Float64("maxunits", 0.5, "largest stake in units (1 unit = 1% of bankroll)")
	skipBooks := flag.String("skipbooks", "kalshi,polymarket", "comma-separated books to ignore (prediction markets are not sportsbooks: different fees, limits and prices)")
	allowUnproven := flag.Bool("allowunproven", false, "stake markets whose backtest showed no edge over the book (not recommended)")
	minGames := flag.Int("mingames", 6, "predict: games each team needs before its game is projected (an offseason counts as 4, so 6 is two games into the season; the model was fitted at 8)")
	minPrior := flag.Int("minprior", 150, "backtest: games needed before the second stage is fitted")
	flag.Parse()

	rows, err := data.LoadStatsDir(*dir)
	if err != nil {
		fatal(err)
	}
	games := team.Build(rows)
	samples, state := team.Samples(games)
	fmt.Printf("%d games of history\n", len(games))

	if *backtest {
		runBacktest(*dir, samples, *season, *hi, *minEdge, *minPrior)
		return
	}
	if *date == "" {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			loc = time.Local
		}
		*date = time.Now().In(loc).Format("2006-01-02")
	}
	if *minGames > 0 {
		state.MinGames = *minGames
	}
	if state.MinGames < team.DefaultMinGames {
		fmt.Printf("note: early season, ratings lean on last year's teams (min %d games, model fitted at %d); treat as paper\n", state.MinGames, team.DefaultMinGames)
	}
	runPredict(*dir, *date, samples, state, *hi, *minEdge, *maxUnits, *skipBooks, *allowUnproven)
}

// ---------------------------------------------------------------- backtest

type rec struct {
	g            team.Game
	c            team.Consensus
	projM, projT float64
}

type event struct {
	market string
	p, pm  float64 // model and market probability of side A (home cover / over / home win)
	y      float64 // 1 if side A won
	oA, oB int
}

func runBacktest(dir string, samples []team.Sample, season int, hi, minEdge float64, minPrior int) {
	path := filepath.Join(dir, fmt.Sprintf("opening_games_%d.jsonl", season))
	odds, err := team.LoadOdds(path)
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/pull gameodds -season %d)", err, season))
	}
	cons := team.ConsensusOf(odds)
	fmt.Printf("%d games with opening odds\n", len(cons))

	byMonth := map[string][]team.Sample{}
	for _, s := range samples {
		if s.G.Season != season || !s.Ready {
			continue
		}
		if _, ok := cons[s.G.ID]; ok {
			byMonth[s.G.Date[:7]] = append(byMonth[s.G.Date[:7]], s)
		}
	}
	months := make([]string, 0, len(byMonth))
	for m := range byMonth {
		months = append(months, m)
	}
	sort.Strings(months)

	var recs []rec
	for _, m := range months {
		var train []team.Sample
		for _, s := range samples {
			if s.G.Date < m+"-01" {
				train = append(train, s)
			}
		}
		if len(train) < 500 {
			continue
		}
		mdl, err := team.Fit(train)
		if err != nil {
			fatal(err)
		}
		for _, s := range byMonth[m] {
			pm, pt := mdl.Project(s.X)
			recs = append(recs, rec{g: s.G, c: cons[s.G.ID], projM: pm, projT: pt})
		}
	}
	if len(recs) == 0 {
		fatal(fmt.Errorf("no games to test: need history before season %d and its opening odds", season))
	}

	// Second stage walk-forward: for each month, fit on the earlier games only.
	var evs []event
	for _, m := range months {
		var prior []rec
		var test []rec
		for _, r := range recs {
			switch {
			case r.g.Date < m+"-01":
				prior = append(prior, r)
			case r.g.Date[:7] == m:
				test = append(test, r)
			}
		}
		mixM, mixT := fitMixes(prior)
		if len(prior) < minPrior || mixM == nil || mixT == nil {
			continue
		}
		for _, r := range test {
			evs = append(evs, eventsFor(r, mixM, mixT)...)
		}
	}
	if len(evs) == 0 {
		fatal(fmt.Errorf("not enough games to fit the second stage (%d games; need %d first)", len(recs), minPrior))
	}

	fmt.Printf("\nBacktest of season %d opening lines: %d games, %d graded bets, second stage fitted only on earlier games\n\n", season, len(recs), len(evs))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "market\tn\tmodel logloss\tbook logloss\tgain/bet\tz\tposted\thit rate\tbreak-even\tROI")
	for _, mk := range []string{"spread", "total", "moneyline"} {
		report(tw, mk, evs, hi, minEdge)
	}
	tw.Flush()

	mixM, mixT := fitMixes(recs)
	if mixM == nil || mixT == nil {
		fatal(fmt.Errorf("could not fit the final blend"))
	}
	bm, pm := mixM.Weights()
	bt, pt := mixT.Weights()
	fmt.Printf("\nWhat the fit says about the ratings model (1.0 on the market and 0.0 on the model means it adds nothing):\n")
	fmt.Printf("  margin: weight on market line %.2f, on ratings model %.2f, typical miss %.1f pts\n", bm, pm, mixM.Sigma)
	fmt.Printf("  total:  weight on market line %.2f, on ratings model %.2f, typical miss %.1f pts\n", bt, pt, mixT.Sigma)
	zs := map[string]float64{}
	for _, mk := range []string{"spread", "total", "moneyline"} {
		zs[mk] = marketZ(evs, mk)
	}
	for _, mk := range []string{"spread", "total", "moneyline"} {
		if zs[mk] < 2 {
			fmt.Printf("  %s: no demonstrated edge over the book (z = %+.1f), so predict will not stake it.\n", mk, zs[mk])
		}
	}
	out := filepath.Join(dir, "gamecal.json")
	if err := (team.Calibration{Margin: mixM, Total: mixT, Z: zs}).Save(out); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved %s. Now run: go run ./cmd/games\n", out)
	fmt.Println("Read the 'gain/bet' and 'z' columns first: a z under about 2 means the model has not shown it beats the book.")
}

func fitMixes(rs []rec) (*team.Mix, *team.Mix) {
	var mr, tr []team.Rec
	for _, r := range rs {
		if r.c.HasSpread {
			mr = append(mr, team.Rec{Market: -r.c.SpreadHome, Proj: r.projM, Actual: r.g.Margin()})
		}
		if r.c.HasTotal {
			tr = append(tr, team.Rec{Market: r.c.Total, Proj: r.projT, Actual: r.g.Total()})
		}
	}
	if len(mr) < 50 || len(tr) < 50 {
		return nil, nil
	}
	a, err1 := team.FitMix(mr)
	b, err2 := team.FitMix(tr)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	return a, b
}

func eventsFor(r rec, mixM, mixT *team.Mix) []event {
	var out []event
	margin, total := r.g.Margin(), r.g.Total()
	if r.c.HasSpread {
		mu := mixM.Mean(-r.c.SpreadHome, r.projM)
		p := team.NormCDF((mu + r.c.SpreadHome) / mixM.Sigma)
		if v := margin + r.c.SpreadHome; v != 0 {
			out = append(out, event{"spread", p, bets.MarketOver(r.c.SpreadHomeOdds, r.c.SpreadAwayOdds), b2f(v > 0), r.c.SpreadHomeOdds, r.c.SpreadAwayOdds})
		}
		if r.c.MLHome != 0 && r.c.MLAway != 0 && margin != 0 {
			pw := team.NormCDF(mu / mixM.Sigma)
			out = append(out, event{"moneyline", pw, bets.MarketOver(r.c.MLHome, r.c.MLAway), b2f(margin > 0), r.c.MLHome, r.c.MLAway})
		}
	}
	if r.c.HasTotal && total != r.c.Total {
		mu := mixT.Mean(r.c.Total, r.projT)
		p := 1 - team.NormCDF((r.c.Total-mu)/mixT.Sigma)
		out = append(out, event{"total", p, bets.MarketOver(r.c.Over, r.c.Under), b2f(total > r.c.Total), r.c.Over, r.c.Under})
	}
	return out
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func ll(p, y float64) float64 {
	p = math.Min(math.Max(p, 1e-6), 1-1e-6)
	return -(y*math.Log(p) + (1-y)*math.Log(1-p))
}

func payout(odds int) float64 {
	if odds > 0 {
		return float64(odds) / 100
	}
	if odds == 0 {
		odds = -110
	}
	return 100 / -float64(odds)
}

func report(tw *tabwriter.Writer, mk string, evs []event, hi, minEdge float64) {
	var n int
	var lm, lb float64
	var d []float64
	var posted, wins int
	var be, roi float64
	for _, e := range evs {
		if e.market != mk {
			continue
		}
		n++
		a, b := ll(e.p, e.y), ll(e.pm, e.y)
		lm += a
		lb += b
		d = append(d, b-a)
		side, p, odds, won := "", e.p, e.oA, e.y == 1
		switch {
		case e.p >= hi && bets.Edge(e.p, e.oA) >= minEdge:
			side = "A"
		case e.p <= 1-hi && bets.Edge(1-e.p, e.oB) >= minEdge:
			side, p, odds, won = "B", 1-e.p, e.oB, e.y == 0
		}
		if side == "" {
			continue
		}
		_ = p
		posted++
		be += bets.Implied(odds)
		if won {
			wins++
			roi += payout(odds)
		} else {
			roi--
		}
	}
	if n == 0 {
		return
	}
	mean, z := meanZ(d)
	hit, brk, r := "-", "-", "-"
	if posted > 0 {
		hit = fmt.Sprintf("%.1f%%", 100*float64(wins)/float64(posted))
		brk = fmt.Sprintf("%.1f%%", 100*be/float64(posted))
		r = fmt.Sprintf("%+.1f%%", 100*roi/float64(posted))
	}
	fmt.Fprintf(tw, "%s\t%d\t%.4f\t%.4f\t%+.4f\t%+.1f\t%d\t%s\t%s\t%s\n", mk, n, lm/float64(n), lb/float64(n), mean, z, posted, hit, brk, r)
}

func meanZ(d []float64) (mean, z float64) {
	n := float64(len(d))
	if n < 2 {
		return 0, 0
	}
	for _, v := range d {
		mean += v
	}
	mean /= n
	var ss float64
	for _, v := range d {
		ss += (v - mean) * (v - mean)
	}
	se := math.Sqrt(ss/(n-1)) / math.Sqrt(n)
	if se == 0 {
		return mean, 0
	}
	return mean, mean / se
}

// ---------------------------------------------------------------- daily plays

type play struct {
	GameID                int
	Matchup, Market, Pick string
	Prob, Edge, Units     float64
	Vendor                string
	Odds                  int
	Tier                  string
	Model                 string
}

func runPredict(dir, date string, samples []team.Sample, state *team.State, hi, minEdge, maxUnits float64, skipBooks string, allowUnproven bool) {
	cal, ok := team.LoadCalibration(filepath.Join(dir, "gamecal.json"))
	if !ok {
		fatal(fmt.Errorf("no data/gamecal.json (run: go run ./cmd/games -backtest -season 2025)"))
	}
	mdl, err := team.Fit(samples)
	if err != nil {
		fatal(err)
	}
	snap := filepath.Join(dir, "snapshots")
	gm, err := props.LoadGames(filepath.Join(snap, fmt.Sprintf("props_%s.jsonl", date)))
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/pull snapshot)", err))
	}
	odds, err := team.LoadOdds(filepath.Join(snap, fmt.Sprintf("odds_%s.jsonl", date)))
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/pull snapshot)", err))
	}
	skip := map[string]bool{}
	for _, b := range strings.Split(skipBooks, ",") {
		if b = strings.ToLower(strings.TrimSpace(b)); b != "" {
			skip[b] = true
		}
	}
	byGame := map[int][]team.Odds{}
	for _, o := range odds {
		if skip[strings.ToLower(o.Vendor)] {
			continue
		}
		byGame[o.GameID] = append(byGame[o.GameID], o)
	}
	day := data.DayNumber(date)

	ids := make([]int, 0, len(gm))
	for id := range gm {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var plays []play
	droppedRows := 0
	for _, id := range ids {
		g := gm[id]
		rowsO := byGame[id]
		if len(rowsO) == 0 {
			fmt.Printf("note: no game odds yet for game %d\n", id)
			continue
		}
		if !state.Ready(g.Home, g.Visitor) {
			fmt.Printf("note: %s @ %s skipped: only %d and %d games of history (need %d each; lower with -mingames)\n",
				abbr(g.VisitorAbbr, g.Visitor), abbr(g.HomeAbbr, g.Home),
				state.Games(g.Visitor), state.Games(g.Home), state.MinGames)
			continue
		}
		pm, pt := mdl.Project(state.Vec(g.Home, g.Visitor, day))
		c := team.ConsensusOf(rowsO)[id]
		if !c.HasSpread || !c.HasTotal {
			fmt.Printf("note: game %d is missing a spread or total, skipped\n", id)
			continue
		}
		h, a := abbr(g.HomeAbbr, g.Home), abbr(g.VisitorAbbr, g.Visitor)
		kept, dropped := cleanRows(rowsO, c)
		droppedRows += dropped
		rowsO = kept
		muM := cal.Margin.Mean(-c.SpreadHome, pm)
		muT := cal.Total.Mean(c.Total, pt)
		match := fmt.Sprintf("%s @ %s", a, h)
		model := fmt.Sprintf("%s by %.1f, total %.1f", h, muM, muT)
		if muM < 0 {
			model = fmt.Sprintf("%s by %.1f, total %.1f", a, -muM, muT)
		}
		fav := h
		if c.SpreadHome > 0 {
			fav = a
		}
		model += fmt.Sprintf(" | books: %s %.1f, total %.1f", fav, -math.Abs(c.SpreadHome), c.Total)
		// The model has far less information than the market (no injuries, no lineups), so a big
		// disagreement is more likely a gap in the model than an opportunity.
		farMargin := math.Abs(muM-(-c.SpreadHome)) > maxDisagreeMargin
		farTotal := math.Abs(muT-c.Total) > maxDisagreeTotal

		type cand struct {
			market, pick string
			p            float64
			odds         int
			vendor       string
			side         string
		}
		best := map[string]cand{} // market+side -> best edge across books
		add := func(market, side, pick string, p float64, odds int, vendor string) {
			if odds == 0 {
				return
			}
			k := market + "/" + side
			if cur, ok := best[k]; !ok || bets.Edge(p, odds) > bets.Edge(cur.p, cur.odds) {
				best[k] = cand{market, pick, p, odds, vendor, side}
			}
		}
		for _, o := range rowsO {
			if o.HasSpread {
				ph := team.NormCDF((muM + o.SpreadHome) / cal.Margin.Sigma)
				add("spread", "home", fmt.Sprintf("%s %+.1f", h, o.SpreadHome), ph, o.SpreadHomeOdds, o.Vendor)
				pa := team.NormCDF((o.SpreadAway - muM) / cal.Margin.Sigma)
				add("spread", "away", fmt.Sprintf("%s %+.1f", a, o.SpreadAway), pa, o.SpreadAwayOdds, o.Vendor)
			}
			if o.HasTotal {
				po := 1 - team.NormCDF((o.Total-muT)/cal.Total.Sigma)
				add("total", "over", fmt.Sprintf("OVER %.1f", o.Total), po, o.Over, o.Vendor)
				add("total", "under", fmt.Sprintf("UNDER %.1f", o.Total), 1-po, o.Under, o.Vendor)
			}
			if o.MLHome != 0 && o.MLAway != 0 {
				pw := team.NormCDF(muM / cal.Margin.Sigma)
				add("moneyline", "home", h+" ML", pw, o.MLHome, o.Vendor)
				add("moneyline", "away", a+" ML", 1-pw, o.MLAway, o.Vendor)
			}
		}
		for _, mk := range []string{"spread", "total", "moneyline"} {
			var top *cand
			for _, side := range []string{"home", "away", "over", "under"} {
				cd, ok := best[mk+"/"+side]
				if !ok {
					continue
				}
				if top == nil || bets.Edge(cd.p, cd.odds) > bets.Edge(top.p, top.odds) {
					cc := cd
					top = &cc
				}
			}
			if top == nil {
				continue
			}
			e := bets.Edge(top.p, top.odds)
			pl := play{GameID: id, Matchup: match, Market: mk, Pick: top.pick, Prob: top.p, Edge: e,
				Vendor: top.vendor, Odds: top.odds, Tier: "LEAN", Model: model}
			if z, ok := cal.Z[mk]; (!ok || z < 2) && !allowUnproven {
				// The backtest never showed this market beats the book, so nothing here is staked.
				pl.Tier, pl.Units = "PAPER", 0
			} else if top.p >= hi && e >= minEdge {
				pl.Tier, pl.Units = "PICK", bets.Units(top.p, top.odds, maxUnits)
				if (mk == "total" && farTotal) || (mk != "total" && farMargin) {
					pl.Tier, pl.Units = "CHECK", 0
					pl.Model += " | FAR FROM THE BOOKS: check injuries and lineups first"
				}
			}
			plays = append(plays, pl)
		}
	}

	if droppedRows > 0 {
		fmt.Printf("ignored %d odds rows that looked wrong (stale, off-grid, one-off lines, or far from the other books)\n", droppedRows)
	}
	var unproven []string
	for _, mk := range []string{"spread", "total", "moneyline"} {
		if z, ok := cal.Z[mk]; !ok || z < 2 {
			unproven = append(unproven, mk)
		}
	}
	if len(unproven) > 0 && !allowUnproven {
		fmt.Printf("NOTE: the backtest did not show an edge over the book for: %s. Those plays are PAPER (0 units); treat them as content, not bets.\n\n",
			strings.Join(unproven, ", "))
	}
	nPick := 0
	for _, p := range plays {
		if p.Tier == "PICK" {
			nPick++
		}
	}
	fmt.Printf("\n%s: %d games, %d full picks and %d leans (one best play per game per market)\n\n", date, len(ids), nPick, len(plays)-nPick)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "game\ttier\tmarket\tpick\tprob\tedge\tunits\tbook\todds\tmodel view")
	for _, p := range plays {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.1f%%\t%+.1f%%\t%.2f\t%s\t%s\t%s\n",
			p.Matchup, p.Tier, p.Market, p.Pick, 100*p.Prob, 100*p.Edge, p.Units, p.Vendor, oddsStr(p.Odds), p.Model)
	}
	tw.Flush()

	path := filepath.Join(dir, fmt.Sprintf("gamepicks_%s.csv", date))
	if err := writeCSV(path, plays); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved %s. LEAN rows are staked 0 units: they are what the model likes most, not what it is confident in.\n", path)
}

func abbr(a string, id int) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return fmt.Sprintf("team%d", id)
}

func oddsStr(o int) string {
	if o > 0 {
		return fmt.Sprintf("+%d", o)
	}
	return fmt.Sprintf("%d", o)
}

func writeCSV(path string, plays []play) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"game", "tier", "market", "pick", "prob", "edge", "units", "book", "odds", "model_view"})
	for _, p := range plays {
		w.Write([]string{p.Matchup, p.Tier, p.Market, p.Pick, fmt.Sprintf("%.4f", p.Prob), fmt.Sprintf("%.4f", p.Edge),
			fmt.Sprintf("%.2f", p.Units), p.Vendor, oddsStr(p.Odds), p.Model})
	}
	w.Flush()
	return w.Error()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// How far the model may sit from the market before a play needs a human check, in points.
const (
	maxDisagreeMargin = 4.0
	maxDisagreeTotal  = 7.0
)

func onHalfGrid(x float64) bool { return math.Abs(x*2-math.Round(x*2)) < 1e-9 }

func saneOdds(o int) bool { return o >= 100 || o <= -100 }

// cleanRows drops sportsbook rows whose numbers do not look like real, current lines: lines off
// the half-point grid, prices that make no sense, lines far from the median, lines no second
// book posts, and rows much older than the newest in the game. It returns the rows that
// survive and how many it threw away (a row that loses only its spread or total is kept for
// the rest).
func cleanRows(rows []team.Odds, c team.Consensus) ([]team.Odds, int) {
	newest := time.Time{}
	stamp := func(s string) (time.Time, bool) {
		t, err := time.Parse(time.RFC3339, s)
		return t, err == nil
	}
	for _, o := range rows {
		if t, ok := stamp(o.Stamp); ok && t.After(newest) {
			newest = t
		}
	}
	spreadSeen, totalSeen := map[float64]int{}, map[float64]int{}
	for _, o := range rows {
		if o.HasSpread {
			spreadSeen[o.SpreadHome]++
		}
		if o.HasTotal {
			totalSeen[o.Total]++
		}
	}
	few := len(rows) < 3
	var out []team.Odds
	dropped := 0
	for _, o := range rows {
		if t, ok := stamp(o.Stamp); ok && !newest.IsZero() && newest.Sub(t) > 4*time.Hour {
			dropped++
			continue
		}
		bad := false
		if o.HasSpread {
			if math.Abs(o.SpreadHome+o.SpreadAway) > 0.01 || !onHalfGrid(o.SpreadHome) ||
				math.Abs(o.SpreadHome-c.SpreadHome) > 1.5 || !saneOdds(o.SpreadHomeOdds) || !saneOdds(o.SpreadAwayOdds) ||
				math.Abs(float64(o.SpreadHomeOdds)) > 200 || math.Abs(float64(o.SpreadAwayOdds)) > 200 ||
				(!few && spreadSeen[o.SpreadHome] < 2) {
				o.HasSpread = false
				bad = true
			}
		}
		if o.HasTotal {
			if !onHalfGrid(o.Total) || math.Abs(o.Total-c.Total) > 2.5 || !saneOdds(o.Over) || !saneOdds(o.Under) ||
				math.Abs(float64(o.Over)) > 200 || math.Abs(float64(o.Under)) > 200 || (!few && totalSeen[o.Total] < 2) {
				o.HasTotal = false
				bad = true
			}
		}
		if o.MLHome != 0 || o.MLAway != 0 {
			okML := saneOdds(o.MLHome) && saneOdds(o.MLAway)
			if okML {
				sum := bets.Implied(o.MLHome) + bets.Implied(o.MLAway)
				okML = sum >= 1.0 && sum <= 1.15
			}
			// The favourite on the moneyline has to be the favourite on the spread.
			if okML && c.SpreadHome != 0 && (o.MLHome < o.MLAway) != (c.SpreadHome < 0) {
				okML = false
			}
			if !okML {
				o.MLHome, o.MLAway = 0, 0
				bad = true
			}
		}
		if bad {
			dropped++
		}
		if o.HasSpread || o.HasTotal || o.MLHome != 0 {
			out = append(out, o)
		}
	}
	return out, dropped
}

// marketZ is the paired gain of the model over the book's own price for one market, in standard
// errors (positive means the model scored better).
func marketZ(evs []event, mk string) float64 {
	var d []float64
	for _, e := range evs {
		if e.market == mk {
			d = append(d, ll(e.pm, e.y)-ll(e.p, e.y))
		}
	}
	_, z := meanZ(d)
	return z
}
