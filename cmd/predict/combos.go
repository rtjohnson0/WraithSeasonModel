package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"propsmodel/internal/bets"
	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/picklog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
)

// comboTier marks a combined-market play. It is its own tier on purpose: it is not PICK, PAPER, CHECK
// or LEAN, so nothing that builds the pick record, the staked board or the sweep counts it. The site
// shows combos in their own section and `go run ./cmd/combos settle` grades them in their own file.
const comboTier = "COMBO"

// comboDeps is what scoring a combined market needs from main: the same rows, rosters and
// corrections the single stats use.
type comboDeps struct {
	b        *feat.Builder
	ix       interface{ Name(int) string }
	m        *predictor.Model
	games    map[int]props.GameInfo
	out      map[int]string
	statuses map[int]string
	teamOf   func(int) (int, bool)
	rowFor   func(pid int, g props.GameInfo, team int) *feat.Row
	adjust   func(predictor.Pred, *feat.Row) predictor.Pred
	// scale is the rate correction the single stat gets (bias file, anchor, with-and-without).
	scale func(st data.Stat, row *feat.Row) float64

	corr combo.Corr
	// cal is the single-stat calibration. A combo's final probability uses the calibration its parts
	// earned (weighted by how much of the sum each part is), unless weight is 0 or more, which
	// forces a plain model/book blend with that share from the model.
	cal       eval.Calibration
	calOK     bool
	weight    float64
	hi        float64
	minEdge   float64
	maxGap    float64
	maxTotal  int
	perPlayer int
	date      string
}

// comboStats summarises a day's combo scoring for the printout.
type comboStats struct {
	Scored, Over, Under, Cleared int
	Basis                        string
}

// calibrate gives a combo's final P(over). The single stats have a calibration fitted on graded
// lines (how much to trust the model against the book, and a shift); combos have no graded lines
// yet, so each combo borrows the calibration of its parts, weighted by each part's share of the
// projected total (a PRA is mostly points, so it gets mostly the points calibration). That keeps the
// combos exactly as humble as the singles have earned. weight >= 0 overrides it with a plain blend.
func (d comboDeps) calibrate(st data.Stat, pred predictor.Pred, pModel, pMarket float64) float64 {
	if d.weight >= 0 || !d.calOK {
		w := d.weight
		if w < 0 {
			w = 0.25
		}
		return w*pModel + (1-w)*pMarket
	}
	var total float64
	for _, c := range st.Parts() {
		total += math.Max(pred.Mean(c), 0.01)
	}
	var mix eval.Cal
	for _, c := range st.Parts() {
		share := math.Max(pred.Mean(c), 0.01) / total
		cal, ok := d.cal.ByStat[string(c)]
		if !ok {
			cal = d.cal.Global
		}
		mix.Intercept += share * cal.Intercept
		mix.Slope += share * cal.Slope
		mix.Market += share * cal.Market
	}
	return mix.Apply(pModel, pMarket)
}

// scoreCombos prices every combined-market line and returns the plays that clear the bar. The
// probability is calibrated like the single stats (see calibrate), so a combo is only as confident
// as the model has earned on its parts. Nothing here is staked.
func scoreCombos(d comboDeps, lines []props.Line) ([]scored, comboStats) {
	var cs comboStats
	type simKey struct{ pid, game int }
	type sim struct {
		pred predictor.Pred
		dist *combo.Dist
		row  *feat.Row
		team int
		g    props.GameInfo
	}
	sims := map[simKey]*sim{}
	var cands []scored
	for _, l := range lines {
		g, ok := d.games[l.GameID]
		if !ok || !l.Stat.IsCombo() || l.OverOdds == 0 || l.UnderOdds == 0 {
			continue
		}
		if _, isOut := d.out[l.PlayerID]; isOut {
			continue
		}
		team, ok := d.teamOf(l.PlayerID)
		if !ok || (team != g.Home && team != g.Visitor) {
			continue
		}
		k := simKey{l.PlayerID, l.GameID}
		sm := sims[k]
		if sm == nil {
			row := d.rowFor(l.PlayerID, g, team)
			if row == nil {
				sims[k] = &sim{}
				continue
			}
			pred := d.adjust(d.m.Predict(row), row)
			if !model.Eligible(row.AvgMin15, pred.Minutes) {
				sims[k] = &sim{}
				continue
			}
			for _, st := range []data.Stat{data.Points, data.Rebounds, data.Assists} {
				pred = pred.Scaled(st, d.scale(st, row))
			}
			sm = &sim{pred: pred, row: row, team: team, g: g,
				dist: combo.Simulate(pred, d.corr, 2000, int64(l.PlayerID)*1000003+int64(l.GameID))}
			sims[k] = sm
		}
		if sm.dist == nil {
			continue
		}
		pr, err := sm.dist.ProbOver(l.Stat, l.Line)
		if err != nil {
			continue
		}
		market := bets.MarketOver(l.OverOdds, l.UnderOdds)
		pOver := d.calibrate(l.Stat, sm.pred, pr.Over, market)
		cs.Scored++
		if pOver >= 0.5 {
			cs.Over++
		} else {
			cs.Under++
		}
		name := d.ix.Name(l.PlayerID)
		if name == "" {
			name = fmt.Sprintf("player %d", l.PlayerID)
		}
		teamAbbr, matchup := sm.g.HomeAbbr, sm.g.VisitorAbbr+" @ "+sm.g.HomeAbbr
		if team == sm.g.Visitor {
			teamAbbr = sm.g.VisitorAbbr
		}
		if sm.g.HomeAbbr == "" || sm.g.VisitorAbbr == "" {
			teamAbbr, matchup = fmt.Sprintf("team %d", team), fmt.Sprintf("game %d", sm.g.ID)
		}
		side, prob, raw, odds := "OVER", pOver, pr.Over, l.OverOdds
		if pOver < 0.5 {
			side, prob, raw, odds = "UNDER", 1-pOver, 1-pr.Over, l.UnderOdds
		}
		proj := sm.dist.Mean(l.Stat)
		oppTeam := map[bool]int{true: sm.g.Home, false: sm.g.Visitor}[team == sm.g.Visitor]
		parts := []string{componentNote(l.Stat, sm.pred)}
		parts = append(parts, whyContext(d.b, d.ix, sm.row, data.Points, sm.pred, team, oppTeam, d.out)...)
		s := scored{
			PlayerID: l.PlayerID, GameID: l.GameID, Player: name, Team: teamAbbr, Matchup: matchup, Stat: l.Stat, Side: side, Line: l.Line,
			Prob: prob, ProbRaw: raw, Edge: bets.Edge(prob, odds), Vendor: l.Vendor, Odds: odds, ProjMean: proj, ProjMin: sm.pred.Minutes,
			Shift: sm.pred.Minutes - sm.row.TypicalMinutes(), Tip: sm.g.Start, Tier: comboTier,
			Dist: comboDist(sm.dist, l.Stat),
		}
		s.Why = strings.Join(parts, "; ")
		s.Why = addWhy(s.Why, fmt.Sprintf("model alone %.0f%% over, book %.0f%%, calibrated %.0f%%", 100*pr.Over, 100*market, 100*pOver))
		if st, ok := d.statuses[l.PlayerID]; ok && props.IsUncertain(st) {
			s.Maybe = st
			s.Why = addWhy(s.Why, "HIS STATUS: "+st)
		}
		if d.maxGap > 0 && math.Abs(s.Line-s.ProjMean)/math.Max(s.ProjMean, 2) > d.maxGap {
			s.Far = true
		}
		if !s.Far && (pOver >= d.hi || pOver <= 1-d.hi) && s.Edge >= d.minEdge {
			cands = append(cands, s)
		}
		if !s.Far && (pOver >= 0.55 || pOver <= 0.45) && s.Edge >= d.minEdge {
			cs.Cleared++
		}
	}
	cands = bestLinePerPick(cands)
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Edge > cands[j].Edge })
	perP := map[int]int{}
	var out []scored
	for _, c := range cands {
		if perP[c.PlayerID] >= d.perPlayer || (d.maxTotal > 0 && len(out) >= d.maxTotal) {
			continue
		}
		perP[c.PlayerID]++
		out = append(out, c)
	}
	cs.Basis = "the calibration its parts earned"
	if d.weight >= 0 || !d.calOK {
		cs.Basis = "a plain model/book blend"
	}
	return out, cs
}

// componentNote shows what the single-stat projections add up to.
func componentNote(st data.Stat, p predictor.Pred) string {
	short := map[data.Stat]string{data.Points: "pts", data.Rebounds: "reb", data.Assists: "ast"}
	var bits []string
	var sum float64
	for _, c := range st.Parts() {
		m := p.Mean(c)
		sum += m
		bits = append(bits, fmt.Sprintf("%.1f %s", m, short[c]))
	}
	return fmt.Sprintf("projects %s (single-stat model, shared minutes)", strings.Join(bits, " + "))
}

// comboDist is the combined stat's probability of 0, 1, 2... for the site's outcome curve.
func comboDist(d *combo.Dist, st data.Stat) []float64 {
	pm := d.Support(st)
	end := len(pm)
	for end > 1 && pm[end-1] < 0.0005 {
		end--
	}
	pm = pm[:end]
	for i := range pm {
		pm[i] = math.Round(pm[i]*1000) / 1000
	}
	return pm
}

// comboEntries turns the plays into log entries (tier COMBO, never staked).
func comboEntries(date string, plays []scored) []picklog.Entry {
	var out []picklog.Entry
	for _, s := range plays {
		out = append(out, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, ProbRaw: s.ProbRaw,
			ProjMean: s.ProjMean, ProjMin: s.ProjMin, Units: 0, Tier: comboTier, Why: s.Why, Tip: s.Tip, Dist: s.Dist,
		})
	}
	return out
}

// printCombos lists the combined-market plays.
func printCombos(plays []scored, cs comboStats, bar float64) {
	if cs.Scored == 0 {
		return
	}
	fmt.Printf("COMBO LINES (points+rebounds+assists and so on): %d priced, %d lean Over, %d lean Under; %d reach the 55%%/45%% bar, %d plays tracked at the %.0f%%/%.0f%% bar.\n",
		cs.Scored, cs.Over, cs.Under, cs.Cleared, len(plays), 100*bar, 100*(1-bar))
	fmt.Printf("Probabilities use %s. Tracked in data/combolog.jsonl, never staked, not in the pick record; `go run ./cmd/combos settle` grades them.\n", cs.Basis)
	if len(plays) == 0 {
		fmt.Println()
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "game\tplayer\tteam\tstat\tpick\tline\tprob\tedge\tbook\todds\tproj\tmin\twhy")
	for _, s := range plays {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%.1f\t%.1f%%\t%+.1f%%\t%s\t%s\t%.1f\t%.0f\t%s\n",
			s.Matchup, s.Player, s.Team, strings.ToUpper(string(s.Stat)), s.Side, s.Line, 100*s.Prob, 100*s.Edge, s.Vendor, oddsStr(s.Odds), s.ProjMean, s.ProjMin, s.Why)
	}
	tw.Flush()
	overs := 0
	for _, s := range plays {
		if s.Side == "OVER" {
			overs++
		}
	}
	fmt.Printf("%d combo plays: %d Over, %d Under\n\n", len(plays), overs, len(plays)-overs)
}

// comboInfo summarises the day's combo pricing for data/more.json (nil when combos were off).
func comboInfo(cs comboStats, tracked int, bar float64) *ComboInfo {
	if cs.Scored == 0 {
		return nil
	}
	return &ComboInfo{Priced: cs.Scored, Over: cs.Over, Under: cs.Under, Cleared: cs.Cleared, Tracked: tracked, Bar: bar, Basis: cs.Basis,
		At: time.Now().UTC().Format(time.RFC3339)}
}
