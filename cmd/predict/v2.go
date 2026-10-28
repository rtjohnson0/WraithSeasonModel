package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
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
//  2. A model-forward blend. V1's calibration, fitted on the 2025 backtest, gives the model 3% of the
//     weight on points and the book 85%, so points plays are the book's number with the model barely
//     consulted. V2 blends model and book 50/50 (-v2weight) so the model's view shows, and a wrong view
//     shows up in the record.
//  3. Combos get the same blend, so points + rebounds + assists and the other combos with points appear.
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
}

// V2File is data/v2.json: the latest run's V2 plays for the day.
type V2File struct {
	Date      string          `json:"date"`
	At        string          `json:"at"`
	Plays     []picklog.Entry `json:"plays"`
	Combos    []picklog.Entry `json:"combos"`
	ComboInfo *ComboInfo      `json:"combo_info,omitempty"`
	Info      *V2Info         `json:"info,omitempty"`
}

func v2Info(f v2Fix, weight, bar, comboBar float64, model string, all, posted []scored) *V2Info {
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
	return in
}

func v2Entries(date string, plays []scored, tier string) []picklog.Entry {
	var out []picklog.Entry
	for _, s := range plays {
		out = append(out, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, ProbRaw: s.ProbRaw,
			ProjMean: s.ProjMean, ProjMin: s.ProjMin, Units: 0, Tier: tier, Why: s.Why, Tip: s.Tip, Dist: s.Dist,
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
	fmt.Printf("  posted: %d Over, %d Under (%v)\n\n", overs, len(plays)-overs, names)
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

// v2Select picks the plays V2 posts: the same bar, edge rule and per-game and per-player caps as the
// real board, so the two boards are comparable, and never staked.
func v2Select(all []scored, hi, minEdge float64, maxGame, maxPlayer int) []scored {
	var picks []scored
	for _, s := range all {
		if s.Prob >= hi && s.Edge >= minEdge {
			picks = append(picks, s)
		}
	}
	picks = bestLinePerPick(picks)
	picks = capCorrelated(picks, maxGame, maxPlayer)
	sort.SliceStable(picks, func(i, j int) bool {
		if picks[i].GameID != picks[j].GameID {
			return picks[i].GameID < picks[j].GameID
		}
		return picks[i].Prob > picks[j].Prob
	})
	return picks
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
