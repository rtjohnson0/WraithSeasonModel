package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/picklog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/projlog"
	"propsmodel/internal/props"
)

// V2 is a challenger that runs beside the real model on every run and is judged separately. It never
// changes a V1 pick, a V1 log or a V1 number; its plays are tracked (never staked) in data/v2log.jsonl,
// graded by `go run ./cmd/v2 settle`, and shown on the site's "V2 test" page, which looks like the
// main one.
//
// What V2 changes, and why (each one answers something seen on the live board):
//
//  1. Live correction. The model's projection is compared with what happened for every player it scored
//     on earlier days (data/projlog.jsonl). Players projected 32+ minutes get the minutes miss added, and
//     each stat gets its overall miss as a ratio. Both are shrunk hard toward no change while the sample
//     is small and capped, and only days BEFORE the run's date are used, so nothing leaks. The backtest
//     bias file was fitted on a different path from live; this one is fitted on live.
//
//  2. A model-forward blend. V1's calibration, fitted on the 2025 backtest, gives the model 3% of the
//     weight on points and the book 85%, so points plays are the book's number with the model barely
//     consulted. V2 blends model and book 50/50 (-v2weight) so the model's view shows, and a wrong view
//     shows up in the record.
//
//  3. Combos get the same blend, so points + rebounds + assists and the other combos with points appear.
//
//  4. Tiers and gates (tighter, never one-sided). A play that clears the posting bar is a STRONG play
//     only when the real model leans the same way (53% or more on that side) and V2's own record on
//     that stat and side, graded over every line in the market log, beats always saying the stat's
//     base rate on 150+ lines. Without that proof it is a LEAN: posted, tracked, not backed. It is a
//     CHECK when the model and the book are 15+ points apart (more often missing news than an edge) or
//     the player's status is uncertain. It is dropped when the real model leans the other way at 53%
//     or more, when fewer than 4 books list it or their lines are scattered, or when V2's record on
//     that stat and side has 150+ graded lines and does not beat the base rate. The same rules apply
//     to Overs and Unders; the Overs the rules leave out are listed on the V2 page as an Overs watch.
//
// V2 does not force Overs. If its Overs are scarce, that is what the price says.
type v2Fix struct {
	MinShift float64               // minutes added for players projected at the star level
	MinN     int                   // player-games behind it
	Ratio    map[data.Stat]float64 // live miss against the books' lines: actual / median line over earlier days, shrunk
	StatN    map[data.Stat]int     // player-games behind each ratio
	Anchor   map[data.Stat]float64 // today's level correction: the slate's average line / the model's average projection
	Days     int                   // distinct earlier days used
	Players  int                   // distinct player-days
}

const (
	v2StarMin     = 32.0 // projected minutes at which a player counts as a starter for the minutes correction
	v2MinShrink   = 40.0 // pseudo player-games of "no correction" for the minutes shift
	v2StatShrink  = 150.0
	v2MinShiftCap = 3.0
	v2StatCap     = 0.05
	v2AnchorLo    = 0.85
	v2AnchorHi    = 1.15

	v2GateMinN  = 150  // graded lines a stat and side needs before its record can promote or block a play
	v2MinBooks  = 4    // books that must list the player's stat
	v2GapMax    = 0.15 // model-to-book gap (probability points) beyond which a play is a CHECK
	v2AgreeBar  = 0.53 // the real model's probability for V2's side that counts as agreeing (tracking bar)
	v2SpreadMin = 1.5  // the widest high-low line spread always allowed; wider stats allow 12% of the line

	// Tiers V2 writes into its log.
	tierStrong = "STRONG"
	tierLean   = "V2LEAN"
	tierCheck  = "CHECK"
)

func newV2Fix() v2Fix {
	return v2Fix{Ratio: map[data.Stat]float64{}, StatN: map[data.Stat]int{}, Anchor: map[data.Stat]float64{}}
}

// fitV2 measures the live miss from earlier days. actual returns what a player did in a final game.
func fitV2(entries []projlog.Entry, before string, actual func(gameID, playerID int) (data.PlayerGame, bool)) v2Fix {
	f := newV2Fix()
	days := map[string]bool{}
	var minSum float64
	var minN int
	sumAct := map[data.Stat]float64{}
	sumLine := map[data.Stat]float64{}
	for _, e := range entries {
		if e.Date >= before || e.ProjMin <= 0 {
			continue
		}
		pg, ok := actual(e.GameID, e.PlayerID)
		if !ok {
			continue
		}
		days[e.Date] = true
		f.Players++
		if e.ProjMin >= v2StarMin {
			minSum += pg.Minutes - e.ProjMin
			minN++
		}
		for _, st := range data.Stats {
			sp, ok := e.Stats[string(st)]
			if !ok || sp.Line <= 0 {
				continue
			}
			sumAct[st] += pg.Value(st)
			sumLine[st] += sp.Line
			f.StatN[st]++
		}
	}
	f.Days = len(days)
	f.MinN = minN
	if minN > 0 {
		f.MinShift = clampF(minSum/(float64(minN)+v2MinShrink), -v2MinShiftCap, v2MinShiftCap)
	}
	for _, st := range data.Stats {
		n := float64(f.StatN[st])
		if n == 0 || sumLine[st] <= 0 {
			f.Ratio[st] = 1
			continue
		}
		ratio := sumAct[st] / sumLine[st]
		f.Ratio[st] = clampF(1+(ratio-1)*n/(n+v2StatShrink), 1-v2StatCap, 1+v2StatCap)
	}
	return f
}

func clampF(x, lo, hi float64) float64 { return math.Min(math.Max(x, lo), hi) }

// shiftMinutes adds the star-minutes correction to a projection that already puts the player at the
// star level.
func (f v2Fix) shiftMinutes(p predictor.Pred) predictor.Pred {
	if f.MinShift == 0 || p.Minutes < v2StarMin {
		return p
	}
	return p.WithMinutes(p.Minutes + f.MinShift)
}

// statFactor is the multiplier on one stat's mean: today's level anchor times the live miss against
// the lines (each 1 when nothing was measured).
func (f v2Fix) statFactor(st data.Stat) float64 {
	return orOne(f.Anchor[st]) * orOne(f.Ratio[st])
}

func orOne(x float64) float64 {
	if x > 0 {
		return x
	}
	return 1
}

// apply is the whole V2 correction for one stat of one player.
func (f v2Fix) apply(p predictor.Pred, st data.Stat) predictor.Pred {
	return f.shiftMinutes(p).Scaled(st, f.statFactor(st))
}

// blend is V2's probability: a plain mix of the model's P(over) and the book's devigged one.
func blend(pModel, pMarket, w float64) float64 {
	return clampF(w*pModel+(1-w)*pMarket, 0.01, 0.99)
}

// V2Info summarises the day's V2 run for the site.
type V2Info struct {
	At       string             `json:"at"`
	Weight   float64            `json:"weight"`
	Bar      float64            `json:"bar"`
	ComboBar float64            `json:"combo_bar"`
	MinShift float64            `json:"min_shift"`
	MinN     int                `json:"min_n"`
	Anchor   map[string]float64 `json:"anchor"`
	Ratio    map[string]float64 `json:"ratio"`
	StatN    map[string]int     `json:"stat_n"`
	Model    string             `json:"model"`
	Days     int                `json:"days"`
	Players  int                `json:"players"`
	Scored   int                `json:"scored"`
	Over     int                `json:"over"`
	Under    int                `json:"under"`
	Posted   int                `json:"posted"`
	Strong   int                `json:"strong"`
	Lean     int                `json:"lean"`
	Check    int                `json:"check"`
	Dropped  V2Drops            `json:"dropped"`
	Gates    []V2Gate           `json:"gates,omitempty"`
	GateMinN int                `json:"gate_min_n"`
	Watch    int                `json:"watch"`
}

// V2File is data/v2.json: the latest run's V2 plays for the day.
type V2File struct {
	Date      string          `json:"date"`
	At        string          `json:"at"`
	Plays     []picklog.Entry `json:"plays"`
	Combos    []picklog.Entry `json:"combos"`
	ComboInfo *ComboInfo      `json:"combo_info,omitempty"`
	Info      *V2Info         `json:"info,omitempty"`
	Overs     []OverWatch     `json:"overs,omitempty"`
}

func v2Info(f v2Fix, weight, bar, comboBar float64, model string, all, posted []scored, drops V2Drops, gates map[string]V2Gate) *V2Info {
	in := &V2Info{At: time.Now().UTC().Format(time.RFC3339), Weight: weight, Bar: bar, ComboBar: comboBar, MinShift: f.MinShift, MinN: f.MinN,
		Anchor: map[string]float64{}, Ratio: map[string]float64{}, StatN: map[string]int{}, Model: model, Days: f.Days, Players: f.Players, Scored: len(all), Posted: len(posted)}
	for _, st := range data.Stats {
		in.Anchor[string(st)] = orOne(f.Anchor[st])
		in.Ratio[string(st)] = orOne(f.Ratio[st])
		in.StatN[string(st)] = f.StatN[st]
	}
	for _, s := range all {
		if s.Side == "OVER" {
			in.Over++
		} else {
			in.Under++
		}
	}
	for _, s := range posted {
		switch s.Tier {
		case tierStrong:
			in.Strong++
		case tierCheck:
			in.Check++
		default:
			in.Lean++
		}
	}
	in.Dropped, in.GateMinN, in.Gates = drops, v2GateMinN, gateList(gates)
	return in
}

// v2Entries turns plays into log entries. tier names the tier for every entry; empty means each
// play's own tier.
func v2Entries(date string, plays []scored, tier string) []picklog.Entry {
	var out []picklog.Entry
	for _, s := range plays {
		t := tier
		if t == "" {
			t = s.Tier
		}
		out = append(out, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, ProbRaw: s.ProbRaw,
			ProjMean: s.ProjMean, ProjMin: s.ProjMin, Units: 0, Tier: t, Why: s.Why, Tip: s.Tip, Dist: s.Dist,
		})
	}
	return out
}

func writeV2(dir string, f V2File) error {
	b, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "v2.json"), b, 0o644)
}

// printV2 shows the challenger's board next to the real one.
func printV2(f v2Fix, info *V2Info, plays, combos []scored) {
	fmt.Printf("V2 TEST (challenger, tracked apart from the picks, never staked): fitted on %d player-games over %d earlier day(s).\n", f.Players, f.Days)
	fmt.Printf("  level anchor (average line / average projection)")
	for _, st := range data.Stats {
		fmt.Printf(" %s x%.3f", st, orOne(f.Anchor[st]))
	}
	fmt.Printf("; projections from %s\n", info.Model)
	if f.Players == 0 {
		fmt.Println("  no earlier days to learn from yet, so the live miss is zero today; V2 differs from V1 in the level anchor and the model/book blend.")
	} else {
		fmt.Printf("  star minutes %+.2f (%d player-games); live miss against the lines", f.MinShift, f.MinN)
		for _, st := range data.Stats {
			fmt.Printf(" %s x%.3f", st, orOne(f.Ratio[st]))
		}
		fmt.Println()
	}
	fmt.Printf("  blend %.0f%% model / %.0f%% book: %d lines scored, %d lean Over, %d lean Under; %d plays posted, %d combo plays.\n",
		100*info.Weight, 100*(1-info.Weight), info.Scored, info.Over, info.Under, len(plays), len(combos))
	overs := 0
	for _, s := range plays {
		if s.Side == "OVER" {
			overs++
		}
	}
	byStat := map[data.Stat]int{}
	for _, s := range plays {
		byStat[s.Stat]++
	}
	var names []string
	for st, n := range byStat {
		names = append(names, fmt.Sprintf("%s %d", st, n))
	}
	sort.Strings(names)
	fmt.Printf("  posted: %d Over, %d Under (%v)\n", overs, len(plays)-overs, names)
	fmt.Printf("  tiers: %d STRONG, %d CHECK, %d LEAN. Left out: %d thin or scattered market, %d the real model disagrees with (%d of them Overs), %d on a stat and side that failed its gate.\n",
		info.Strong, info.Check, info.Lean, info.Dropped.Thin, info.Dropped.Disagree, info.Dropped.DisagreeOver, info.Dropped.Failed)
	if info.Dropped.Disagree > 0 && info.Dropped.DisagreeOver*2 > info.Dropped.Disagree {
		fmt.Println("  note: most of what the real model's disagreement removed are Overs. V2's level anchor lifts projections the real model leaves low, so that rule can undo it; -v2keep turns the drop into a Lean.")
	}
	var parts []string
	for _, g := range info.Gates {
		parts = append(parts, fmt.Sprintf("%s %s %s (%d)", g.Stat, strings.ToLower(g.Side), g.State, g.N))
	}
	fmt.Printf("  gates (V2's own record on graded lines, %d needed): %s\n\n", info.GateMinN, strings.Join(parts, ", "))
}

// v2Score re-prices one line with the V2 correction and blend. s is the V1 scoring of the same line:
// V2 keeps its context (player, game, price, notes that do not depend on the side) and replaces the
// probability, side, projection and edge.
func v2Score(s scored, baseWhy string, pred predictor.Pred, l props.Line, f v2Fix, w float64) (scored, bool) {
	pv := f.apply(pred, l.Stat)
	pr, err := pv.ProbOver(l.Stat, l.Line)
	if err != nil {
		return scored{}, false
	}
	market := bets.MarketOver(l.OverOdds, l.UnderOdds)
	pOver := blend(pr.Over, market, w)
	side, prob, raw, odds := "OVER", pOver, pr.Over, l.OverOdds
	if pOver < 0.5 {
		side, prob, raw, odds = "UNDER", 1-pOver, 1-pr.Over, l.UnderOdds
	}
	out := s
	out.V1Prob, out.Mkt = s.Prob, market
	if s.Side != side {
		out.V1Prob = 1 - s.Prob
	}
	if side == "UNDER" {
		out.Mkt = 1 - market
	}
	out.Side, out.Prob, out.ProbRaw, out.Odds = side, prob, raw, odds
	out.Edge, out.Units, out.Tier = bets.Edge(prob, odds), 0, "PAPER"
	out.ProjMean, out.ProjMin = pv.Mean(l.Stat), pv.Minutes
	out.Dist = pv.Dist(l.Stat, distMax(l.Stat, l.Line, out.ProjMean))
	out.Why = addWhy(baseWhy, fmt.Sprintf("V2: model %.0f%% Over, book %.0f%%, blended %.0f%%; projection %.1f (V1 %.1f)",
		100*pr.Over, 100*market, 100*pOver, out.ProjMean, s.ProjMean))
	if s.Maybe != "" {
		out.Why = addWhy(out.Why, "HIS STATUS: "+s.Maybe)
	}
	return out, true
}

// V2Gate is V2's record on one stat and side over every graded line in the market log.
type V2Gate struct {
	Stat   string  `json:"stat"`
	Side   string  `json:"side"`
	N      int     `json:"n"`
	Hit    float64 `json:"hit"`
	Brier  float64 `json:"brier"`
	Base   float64 `json:"base"`
	State  string  `json:"state"` // "proven", "failed" or "unproven"
	Reason string  `json:"reason"`
}

const (
	gateProven   = "proven"
	gateFailed   = "failed"
	gateUnproven = "unproven"
)

func gateKey(stat, side string) string { return stat + "|" + side }

// v2Gates turns the graded cells into a gate per stat and side. A cell with too few graded lines is
// unproven (its plays are LEAN at best); one with enough lines is proven when V2's probability beats
// the stat's base rate and its leans are right more often than not, and failed otherwise (tracking
// only, nothing posted).
func v2Gates(cells []lineslog.Cell, minN int) map[string]V2Gate {
	out := map[string]V2Gate{}
	for _, c := range cells {
		g := V2Gate{Stat: c.Stat, Side: c.Side, N: c.N, Hit: c.Hit, Brier: c.Brier, Base: c.Base}
		switch {
		case c.N < minN:
			g.State, g.Reason = gateUnproven, fmt.Sprintf("%d of %d graded lines", c.N, minN)
		case c.Brier < c.Base && c.Hit >= 0.5:
			g.State, g.Reason = gateProven, fmt.Sprintf("beats the base rate (%.3f against %.3f) and %.0f%% of leans were right", c.Brier, c.Base, 100*c.Hit)
		default:
			g.State, g.Reason = gateFailed, fmt.Sprintf("does not beat the base rate (%.3f against %.3f); %.0f%% of leans were right", c.Brier, c.Base, 100*c.Hit)
		}
		out[gateKey(c.Stat, c.Side)] = g
	}
	return out
}

// gateOf is the gate for a stat and side; a cell with no graded lines at all is unproven.
func gateOf(gates map[string]V2Gate, stat, side string) V2Gate {
	if g, ok := gates[gateKey(stat, side)]; ok {
		return g
	}
	return V2Gate{Stat: stat, Side: side, State: gateUnproven, Reason: "no graded lines yet"}
}

// gateList is every stat and side in a fixed order, including the cells with no data yet.
func gateList(gates map[string]V2Gate) []V2Gate {
	var out []V2Gate
	for _, st := range data.Stats {
		if st.IsCombo() {
			continue
		}
		for _, side := range []string{"OVER", "UNDER"} {
			out = append(out, gateOf(gates, string(st), side))
		}
	}
	return out
}

// V2Drops counts the plays that cleared the bar and were left out, by reason.
type V2Drops struct {
	Thin         int `json:"thin"`          // fewer than 4 books, or their lines are scattered
	Disagree     int `json:"disagree"`      // the real model leans the other way at 53% or more
	DisagreeOver int `json:"disagree_over"` // of those, how many were Overs: the level anchor moves V2's Overs against the real model's Unders, so this shows what the rule costs
	Failed       int `json:"failed"`        // V2's record on that stat and side does not beat the base rate
}

// v2Rules are the knobs of v2Select.
type v2Rules struct {
	Hi, MinEdge        float64
	MaxGame, MaxPlayer int
	Gates              map[string]V2Gate
	MinBooks           int  // books that must list the stat (0 means the default, 4)
	KeepDisagree       bool // a play the real model opposes at 53%+ is a Lean instead of dropped
}

// spreadOK reports whether a market is deep and tight enough: 4+ books, and a high-low spread no wider
// than the larger of 1.5 and 12% of the line.
func spreadOK(books, minBooks int, spread, line float64) bool {
	if minBooks <= 0 {
		minBooks = v2MinBooks
	}
	return books >= minBooks && spread <= math.Max(v2SpreadMin, 0.12*line)
}

// v2Tier decides one play that cleared the bar: its tier, or "" with a reason to drop it.
func v2Tier(s scored, r v2Rules) (tier, drop string) {
	gates := r.Gates
	if !spreadOK(s.Books, r.MinBooks, s.Spread, s.Line) {
		return "", "thin"
	}
	if s.V1Prob > 0 && s.V1Prob <= 1-v2AgreeBar && !r.KeepDisagree {
		return "", "disagree"
	}
	g := gateOf(gates, string(s.Stat), s.Side)
	if g.State == gateFailed {
		return "", "failed"
	}
	gap := s.ProbRaw - s.Mkt
	switch {
	case s.Maybe != "" || (s.Mkt > 0 && gap >= v2GapMax):
		return tierCheck, ""
	case g.State == gateProven && s.V1Prob >= v2AgreeBar:
		return tierStrong, ""
	}
	return tierLean, ""
}

// v2Select picks the plays V2 posts: the same bar, edge rule and per-game and per-player caps as the
// real board, so the two boards are comparable, then tiers them (STRONG, CHECK, LEAN) and drops the
// plays the gates rule out. Never staked. Overs and Unders go through exactly the same rules.
func v2Select(all []scored, r v2Rules) ([]scored, V2Drops) {
	var drops V2Drops
	var picks []scored
	for _, s := range all {
		if s.Prob < r.Hi || s.Edge < r.MinEdge {
			continue
		}
		t, why := v2Tier(s, r)
		switch why {
		case "thin":
			drops.Thin++
			continue
		case "disagree":
			drops.Disagree++
			if s.Side == "OVER" {
				drops.DisagreeOver++
			}
			continue
		case "failed":
			drops.Failed++
			continue
		}
		s.Tier, s.Units = t, 0
		switch t {
		case tierStrong:
			s.Why = addWhy(s.Why, "V2 STRONG: the real model leans the same way and V2 has proven this stat and side on graded lines")
		case tierCheck:
			if s.Maybe == "" {
				s.Why = addWhy(s.Why, fmt.Sprintf("V2 CHECK: the model alone says %.0f%% and the book %.0f%%, a gap this big is more often missing news than an edge", 100*s.ProbRaw, 100*s.Mkt))
			}
		default:
			s.Why = addWhy(s.Why, v2LeanWhy(s, r.Gates))
		}
		picks = append(picks, s)
	}
	picks = bestLinePerPick(picks)
	// Each tier is capped on its own, most trusted first, so a Lean never crowds out a Strong play.
	var out []scored
	for _, t := range []string{tierStrong, tierCheck, tierLean} {
		var group []scored
		for _, s := range picks {
			if s.Tier == t {
				group = append(group, s)
			}
		}
		out = append(out, capCorrelated(group, r.MaxGame, r.MaxPlayer)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		if out[i].Tier != out[j].Tier {
			return picklog.TierRank(out[i].Tier) < picklog.TierRank(out[j].Tier)
		}
		return out[i].Prob > out[j].Prob
	})
	return out, drops
}

// v2LeanWhy says why a play is a Lean and not a Strong play.
func v2LeanWhy(s scored, gates map[string]V2Gate) string {
	g := gateOf(gates, string(s.Stat), s.Side)
	var parts []string
	if s.V1Prob < v2AgreeBar {
		parts = append(parts, fmt.Sprintf("the real model gives this side %.0f%%, short of the %.0f%% that counts as agreeing", 100*s.V1Prob, 100*v2AgreeBar))
	}
	if g.State != gateProven {
		parts = append(parts, fmt.Sprintf("V2 has not proven %ss on %s yet (%s)", strings.ToLower(g.Side), s.Stat, g.Reason))
	}
	if len(parts) == 0 {
		return "V2 LEAN"
	}
	return "V2 LEAN, not Strong: " + strings.Join(parts, "; ")
}

// v2Anchor is the level correction for one stat: the slate's average median line over the model's
// average projection, within bounds. The books are unbiased on average (see cmd/linecheck), so a model
// whose projections sit far below the lines is off in level, not in who it likes; this moves only
// the level.
func v2Anchor(sumLine, sumProj float64) float64 {
	if sumProj <= 0 || sumLine <= 0 {
		return 1
	}
	return clampF(sumLine/sumProj, v2AnchorLo, v2AnchorHi)
}

// v2OverNote says why an Over that cleared V2's bar is not on its board, or "" when it is just short
// of the bar (the Overs watch status already says that).
func v2OverNote(s scored, r v2Rules) string {
	if s.Prob < r.Hi || s.Edge < r.MinEdge {
		return ""
	}
	_, why := v2Tier(s, r)
	switch why {
	case "thin":
		return fmt.Sprintf("V2 left it out: only %d book(s) list it, or their lines are scattered (%.1f apart)", s.Books, s.Spread)
	case "disagree":
		return fmt.Sprintf("V2 left it out: the real model gives the Over only %.0f%%", 100*s.V1Prob)
	case "failed":
		g := gateOf(r.Gates, string(s.Stat), s.Side)
		return "V2 left it out: V2's Overs on " + string(s.Stat) + " " + g.Reason
	}
	return ""
}

// v2OversWatch lists the Overs V2 scored that are not on its board: the closest to the bar, and the
// ones the rules left out, each with the reason. Overs that fail the depth floor are not listed
// unless they cleared the bar, since a thin market is not worth chasing.
func v2OversWatch(all, posted []scored, r v2Rules, limit int) []OverWatch {
	var pool []scored
	for _, s := range all {
		if s.Side == "OVER" && (spreadOK(s.Books, r.MinBooks, s.Spread, s.Line) || (s.Prob >= r.Hi && s.Edge >= r.MinEdge)) {
			pool = append(pool, s)
		}
	}
	rows := overWatchRows(pool, posted, nil, r.Hi, r.MinEdge, limit)
	byKey := map[[2]string]scored{}
	for _, s := range pool {
		k := [2]string{s.Player, string(s.Stat)}
		if cur, ok := byKey[k]; !ok || s.Edge > cur.Edge {
			byKey[k] = s
		}
	}
	for i := range rows {
		if s, ok := byKey[[2]string{rows[i].Player, rows[i].Stat}]; ok {
			if note := v2OverNote(s, r); note != "" {
				rows[i].Status = note
			}
		}
	}
	return rows
}
