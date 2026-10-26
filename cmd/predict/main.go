// Command predict scores a day's player props and posts the picks that clear 55% / 45%.
//
//	go run ./cmd/predict                    (today, US Eastern)
//	go run ./cmd/predict -date 2026-10-21
//
// Needs, in data/: stats_*.jsonl (history), model.json (go run ./cmd/train), calibration.json
// (go run ./cmd/backtest) and snapshots/props_DATE.jsonl (go run ./cmd/pull snapshot).
// Optional and used when present: snapshots/injuries_DATE.jsonl (who is out) and
// snapshots/sgo_DATE.jsonl (more sportsbooks, from go run ./cmd/sgo snapshot).
//
// Refresh recent stats first so projections include last night's games:
// go run ./cmd/pull stats -seasons 2026
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	_ "time/tzdata"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/model"
	"propsmodel/internal/picklog"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/sgo"
	"propsmodel/internal/team"
)

type scored struct {
	PlayerID int
	GameID   int
	Player   string
	Team     string // his team, e.g. CHI, so the prop is easy to find in the app
	Matchup  string // e.g. "CHI @ MIL" (visitor @ home)
	Stat     data.Stat
	Side     string
	Line     float64
	Prob     float64 // calibrated probability for the chosen side
	ProbRaw  float64
	Edge     float64 // Prob minus the break-even probability at the offered odds
	Units    float64
	Vendor   string
	Odds     int
	ProjMean float64
	ProjMin  float64
	Why      string
	Tier     string  // "PICK" cleared the thresholds; "CHECK" cleared them but the minutes need a look; "LEAN" fills a game
	Shift    float64 // projected minutes minus his recent (15-game) average
	Far      bool    // the line sits far from the projection: often a bad or alternate line, not an edge
	Maybe    string  // his own injury status when it is uncertain (questionable, doubtful, day-to-day)
}

func main() {
	date := flag.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	dir := flag.String("data", "data", "data directory")
	hi := flag.Float64("hi", model.PostOver, "post OVER at or above this probability; UNDER at or below 1 minus it")
	minEdge := flag.Float64("minedge", 0.01, "also require the probability to beat the offered odds break-even by this much")
	showAll := flag.Bool("all", false, "print every scored line, not just posted picks")
	doubtful := flag.Bool("doubtful", false, "treat 'doubtful' players as out")
	maxUnits := flag.Float64("maxunits", 0.5, "largest stake in units (1 unit = 1% of bankroll); the backtest edge is small, so the default is small")
	maxGame := flag.Int("maxgame", 4, "most picks to post from any one game (picks in one game are correlated)")
	maxPlayer := flag.Int("maxplayer", 2, "most picks to post on any one player")
	perGame := flag.Int("pergame", 3, "make sure every game has at least this many plays; extras below the 55% bar are marked LEAN, staked 0 units and not logged (0 turns this off)")
	maxGap := flag.Float64("maxgap", 1.0, "a line this far from the projection (as a share of max(projection, 2)) is marked CHECK, since it is more often a bad or alternate line than an edge; 0 turns the guard off")
	maxCut := flag.Float64("maxshift", 8, "a play whose projected minutes differ from his recent average by this many or more is marked CHECK (0 units) until you confirm the minutes; 0 turns the guard off")
	stakeStats := flag.String("stakestats", "rebounds", "stats that get a real stake; plays on other stats are tiered PAPER (0 units, still logged). Default follows the 2025 backtest: rebounds were profitable, assists lost money, points posted nothing")
	boardStats := flag.String("stats", "points,rebounds,assists,threes", "stats scored for the posted board and the log. Threes are included but never staked until a backtest supports them; -game always shows every stat")
	skewMax := flag.Float64("skew", 0.70, "if more than this share of the day's scored lines lean the same way (all unders, say), the model is probably biased that day, so plays on that side are marked CHECK, not staked (0 turns this off)")
	warmup := flag.Int("warmup", 8, "a team's first this-many games of a season are tiered PAPER (0 units): early-season inputs (last season's averages, empty rolling windows) are off the distribution the backtest was fitted on; 0 turns it off")
	also := flag.String("also", "points", "stats to always show the strongest lines for, even when none clear the edge rule (shown as PAPER, 0 units); empty to turn off")
	alsoTop := flag.Int("alsotop", 8, "how many lines to show per -also stat")
	allowStale := flag.Bool("allowstale", false, "run even if calibration.json does not match the model's feature set (probabilities will be unreliable)")
	gameQ := flag.String("game", "", "look at one game only: team abbreviations such as \"ORL CHA\" or \"ORL@CHA\", or a game id. Prints that game's best lines (any tier), writes data/game_<teams>_<date>.csv, and logs nothing")
	topN := flag.Int("top", 20, "with -game: how many lines to show")
	noLog := flag.Bool("nolog", false, "do not append picks to data/picklog.jsonl")
	flag.Parse()

	if *date == "" {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			loc = time.Local
		}
		*date = time.Now().In(loc).Format("2006-01-02")
	}

	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fatal(err)
	}
	b := d.Builder
	ix := b.Index()

	m, err := predictor.Load(filepath.Join(*dir, "model.json"))
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/train)", err))
	}
	config := pipeline.ConfigName(m.Disabled)
	cal, calOK := eval.LoadCalibration(filepath.Join(*dir, "calibration.json"))
	switch {
	case !calOK:
		fmt.Println("WARNING: no data/calibration.json, so these probabilities are RAW and overconfident.")
		fmt.Println("Run `go run ./cmd/backtest` first. Treat today's output as a dry run.")
		fmt.Println()
	case cal.Config != config:
		if *allowStale {
			fmt.Printf("WARNING: calibration.json was fitted for %q but model.json is %q; probabilities are NOT trustworthy.\n\n", cal.Config, config)
			break
		}
		fatal(fmt.Errorf("calibration.json was fitted for feature set %q but model.json is %q. The probabilities would be miscalibrated "+
			"(usually overconfident). Run: go run ./cmd/backtest -season 2025   then   go run ./cmd/train   (same -lineup setting for both)", cal.Config, config))
	}

	snapDir := filepath.Join(*dir, "snapshots")
	propsPath := filepath.Join(snapDir, fmt.Sprintf("props_%s.jsonl", *date))
	lines, err := props.LoadSnapshot(propsPath)
	if err != nil {
		fatal(fmt.Errorf("%w (run: go run ./cmd/pull snapshot)", err))
	}
	moves, _ := props.LineMoves(propsPath)
	games, err := props.LoadGames(propsPath)
	if err != nil {
		fatal(err)
	}

	gid := 0
	if *gameQ != "" {
		var err error
		if gid, err = findGame(*gameQ, games); err != nil {
			fatal(err)
		}
		*noLog = true
	}

	out := map[int]string{}
	injPath := filepath.Join(snapDir, fmt.Sprintf("injuries_%s.jsonl", *date))
	if o, err := props.LoadOut(injPath, *doubtful); err == nil {
		out = o
	} else if strings.HasPrefix(config, "lineup") {
		fmt.Println("WARNING: no injuries snapshot found, so the model assumes nobody is out.")
		fmt.Println()
	}

	statuses, _ := props.LoadStatuses(injPath)

	consensus := map[int]team.Consensus{}
	// Today's spread and total are inputs to the model (blowout risk, pace). They have to be in
	// place before any row is built.
	if gl, err := team.LoadOdds(filepath.Join(snapDir, fmt.Sprintf("odds_%s.jsonl", *date))); err == nil {
		var keep []team.Odds
		for _, o := range gl {
			if pipeline.IsSportsbook(o.Vendor) {
				keep = append(keep, o)
			}
		}
		today := data.DayNumber(*date)
		n := 0
		for id, c := range team.ConsensusOf(keep) {
			consensus[id] = c
			if g, ok := games[id]; ok && c.HasSpread && c.HasTotal {
				b.AddGameLine(today, g.Home, feat.GameLine{Spread: c.SpreadHome, Total: c.Total})
				b.AddGameLine(today, g.Visitor, feat.GameLine{Spread: -c.SpreadHome, Total: c.Total})
				n++
			}
		}
		fmt.Printf("game spread and total loaded for %d games\n", n)
	} else {
		fmt.Println("note: no game odds snapshot, so the spread and total features are empty today (run: go run ./cmd/pull snapshot)")
	}

	// A player's team today. Box scores only know his last team, which is wrong after a trade
	// or free agency, so prefer the current rosters from `pull players`.
	active, _ := props.LoadActiveTeams(filepath.Join(*dir, "players_active.jsonl"))
	if len(active) == 0 {
		fmt.Println("WARNING: no data/players_active.jsonl, so each player's team is taken from his last game.")
		fmt.Println("Anyone traded or signed since then may be matched to the wrong opponent. Run: go run ./cmd/pull players")
		fmt.Println()
	}
	b.SetRoster(active)
	teamOf := func(pid int) (int, bool) {
		if len(active) > 0 {
			t, ok := active[pid]
			return t, ok
		}
		return b.LastTeam(pid)
	}

	// Which game each player is in today, by his team.
	gameOf := map[int]props.GameInfo{}
	for _, g := range games {
		for _, pid := range b.PlayerIDs() {
			if t, ok := teamOf(pid); ok && (t == g.Home || t == g.Visitor) {
				gameOf[pid] = g
			}
		}
	}

	extra := sgoLines(*dir, *date, b, gameOf)
	if len(extra) > 0 {
		fmt.Printf("%d extra lines from SportsGameOdds merged in for line shopping\n", len(extra))
		lines = append(lines, extra...)
	}

	staked := map[data.Stat]bool{}
	for _, st := range strings.Split(*stakeStats, ",") {
		staked[data.Stat(strings.TrimSpace(st))] = true
	}
	// Where the books agree on a line, a book that is off it is either stale or shading: a
	// chance to take the better number that has nothing to do with the model.
	type marketKey struct {
		player, stat, game int
	}
	byMarket := map[marketKey][]float64{}
	for _, l := range lines {
		k := marketKey{l.PlayerID, data.StatIndex(l.Stat), l.GameID}
		byMarket[k] = append(byMarket[k], l.Line)
	}
	boardSet := map[data.Stat]bool{}
	for _, st := range strings.Split(*boardStats, ",") {
		boardSet[data.Stat(strings.TrimSpace(st))] = true
	}
	rowCache := map[int]*feat.Row{}
	var all, picks []scored
	skip := struct{ noGame, noTeam, noHistory, minutes, out, noPrice int }{}
	for _, l := range lines {
		if gid != 0 && l.GameID != gid {
			continue
		}
		if gid == 0 && !boardSet[l.Stat] {
			continue // not on the posted board (see -stats)
		}
		g, ok := games[l.GameID]
		if !ok {
			skip.noGame++
			continue
		}
		if _, isOut := out[l.PlayerID]; isOut {
			skip.out++
			continue
		}
		// A line with no price on one side cannot be devigged or priced: the calibration would
		// treat the missing side as a coin flip and produce a made-up edge.
		if l.OverOdds == 0 || l.UnderOdds == 0 {
			skip.noPrice++
			continue
		}
		team, ok := teamOf(l.PlayerID)
		if !ok || (team != g.Home && team != g.Visitor) {
			skip.noTeam++
			continue
		}
		row, cached := rowCache[l.PlayerID]
		if !cached {
			opp := g.Visitor
			if team == g.Visitor {
				opp = g.Home
			}
			outSet := map[int]bool{}
			for pid := range out {
				outSet[pid] = true
			}
			row = b.Live(l.PlayerID, team, opp, team == g.Home, *date, outSet)
			rowCache[l.PlayerID] = row
		}
		if row == nil {
			skip.noHistory++
			continue
		}
		teamAbbr, matchup := g.HomeAbbr, g.VisitorAbbr+" @ "+g.HomeAbbr
		if team == g.Visitor {
			teamAbbr = g.VisitorAbbr
		}
		if g.HomeAbbr == "" || g.VisitorAbbr == "" {
			teamAbbr, matchup = fmt.Sprintf("team %d", team), fmt.Sprintf("game %d", g.ID)
		}
		early := *warmup > 0 && b.SeasonGames(team, *date) < *warmup
		pred := m.Predict(row)
		if !model.Eligible(row.AvgMin15, pred.Minutes) {
			skip.minutes++
			continue
		}
		pr, err := pred.ProbOver(l.Stat, l.Line)
		if err != nil {
			continue
		}
		pOver := pr.Over
		if calOK {
			pOver = cal.Apply(l.Stat, pr.Over, bets.MarketOver(l.OverOdds, l.UnderOdds))
		}
		name := ix.Name(l.PlayerID)
		if name == "" {
			name = fmt.Sprintf("player %d", l.PlayerID)
		}
		side, prob, raw, odds := "OVER", pOver, pr.Over, l.OverOdds
		if pOver < 0.5 {
			side, prob, raw, odds = "UNDER", 1-pOver, 1-pr.Over, l.UnderOdds
		}
		s := scored{
			PlayerID: l.PlayerID, GameID: l.GameID, Player: name, Team: teamAbbr, Matchup: matchup, Stat: l.Stat, Side: side, Line: l.Line,
			Prob: prob, ProbRaw: raw, Edge: bets.Edge(prob, odds), Units: bets.Units(prob, odds, *maxUnits),
			Vendor: l.Vendor, Odds: odds, ProjMean: pred.Mean(l.Stat), ProjMin: pred.Minutes,
			Why:   why(b, ix, row, l.Stat, pred, team, map[bool]int{true: g.Home, false: g.Visitor}[team == g.Visitor], out, *date),
			Shift: pred.Minutes - row.AvgMin15,
		}
		if st, ok := statuses[l.PlayerID]; ok && props.IsUncertain(st) {
			s.Maybe = st
			s.Why = addWhy(s.Why, "HIS STATUS: "+st)
		}
		if *maxGap > 0 && math.Abs(s.Line-s.ProjMean)/math.Max(s.ProjMean, 2) > *maxGap {
			s.Far = true
			s.Why = addWhy(s.Why, fmt.Sprintf("line %.1f is far from projection %.1f: confirm the line", s.Line, s.ProjMean))
		}
		if ls := byMarket[marketKey{l.PlayerID, data.StatIndex(l.Stat), l.GameID}]; len(ls) >= 4 {
			sorted := append([]float64(nil), ls...)
			sort.Float64s(sorted)
			med := sorted[len(sorted)/2]
			if d := s.Line - med; math.Abs(d) >= 1.0 {
				kind := "against"
				if (s.Side == "UNDER" && d > 0) || (s.Side == "OVER" && d < 0) {
					kind = "in your favor"
				}
				s.Why = addWhy(s.Why, fmt.Sprintf("line %.1f is %.1f %s the market median %.1f across %d books: off the market, %s", s.Line, math.Abs(d), map[bool]string{true: "above", false: "below"}[d > 0], med, len(ls), kind))
			}
		}
		if mv, ok := moves[props.MoveKey{Game: l.GameID, Player: l.PlayerID, Stat: l.Stat}]; ok && mv.Moved() {
			s.Why = addWhy(s.Why, moveNote(mv, s.Side))
		}
		var shaky []string
		for _, pid := range b.RotationOf(team, row.G.Day) {
			if st, ok := statuses[pid]; ok && props.IsUncertain(st) && pid != l.PlayerID {
				shaky = append(shaky, ix.Name(pid)+" ("+strings.ToLower(st)+")")
			}
		}
		if len(shaky) > 0 {
			s.Why = addWhy(s.Why, "teammates uncertain: "+strings.Join(shaky, ", "))
		}
		s.Tier = "LEAN"
		all = append(all, s)
		if (pOver >= *hi || pOver <= 1-*hi) && s.Edge >= *minEdge {
			s.Tier = "PICK"
			if early {
				s.Tier, s.Units = "PAPER", 0
			} else if !staked[s.Stat] {
				s.Tier, s.Units = "PAPER", 0
			} else if s.Far {
				s.Tier, s.Units = "CHECK", 0
			} else if s.Maybe != "" {
				// He may not play, or may play fewer minutes: confirm his status at tip-off.
				s.Tier, s.Units = "CHECK", 0
			} else if *maxCut > 0 && math.Abs(s.Shift) >= *maxCut {
				// A big minutes change is where a wrong projection (a minutes limit, a role change,
				// a stale average) turns into a confident wrong pick. Verify before staking.
				s.Tier, s.Units = "CHECK", 0
			}
			picks = append(picks, s)
		}
	}

	// A board that leans the same way almost everywhere is a sign the model is off that day, not
	// that every game has the same edge. Plays on the crowded side are not staked until checked.
	if *skewMax > 0 && gid == 0 && len(all) >= 100 {
		unders := 0
		for _, s := range all {
			if s.Side == "UNDER" {
				unders++
			}
		}
		share := float64(unders) / float64(len(all))
		crowded := ""
		switch {
		case share >= *skewMax:
			crowded = "UNDER"
		case 1-share >= *skewMax:
			crowded = "OVER"
		}
		if crowded != "" {
			side := share
			if crowded == "OVER" {
				side = 1 - share
			}
			fmt.Printf("\nWARNING: %.0f%% of the %d scored lines lean %s. Books do not price a whole slate that one-sided, so the\n", 100*side, len(all), crowded)
			fmt.Println("model is probably biased that day (minutes or early-season inputs). Plays on that side are marked CHECK, not staked.")
			fmt.Println("Run `go run ./cmd/bias` to measure the bias.")
			fmt.Println()
			for i := range picks {
				if picks[i].Side == crowded && picks[i].Tier == "PICK" {
					picks[i].Tier, picks[i].Units = "CHECK", 0
					picks[i].Why = addWhy(picks[i].Why, fmt.Sprintf("%.0f%% of today's lines lean %s: model may be biased", 100*side, crowded))
				}
			}
		}
	}

	picks = bestLinePerPick(picks)
	// CHECK and PAPER plays are capped separately so they cannot crowd real picks out of a game.
	var firm, check []scored
	for _, s := range picks {
		if s.Tier == "CHECK" || s.Tier == "PAPER" {
			check = append(check, s)
		} else {
			firm = append(firm, s)
		}
	}
	picks = append(capCorrelated(firm, *maxGame, *maxPlayer), capCorrelated(check, *maxGame, *maxPlayer)...)
	picks = addWatchlist(picks, all, *also, *alsoTop, *maxPlayer)
	nReal := len(picks)
	if *perGame > 0 {
		picks = fillLeans(picks, all, games, *perGame, *maxPlayer)
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].GameID != picks[j].GameID {
			return picks[i].GameID < picks[j].GameID
		}
		if picks[i].Tier != picks[j].Tier {
			return tierRank(picks[i].Tier) < tierRank(picks[j].Tier)
		}
		return picks[i].Prob > picks[j].Prob
	})

	nFirm := 0
	for _, s := range picks {
		if s.Tier == "PICK" {
			nFirm++
		}
	}
	fmt.Printf("%s: %d lines read, %d scored; %d PICK, %d CHECK/PAPER (not staked), %d LEAN (fill-ins, so every game has %d)\n",
		*date, len(lines), len(all), nFirm, nReal-nFirm, len(picks)-nReal, *perGame)
	if gid != 0 {
		gameView(games[gid], consensus[gid], all, picks, staked, *topN, *hi, *minEdge, *date, *dir)
		return
	}
	statBreakdown(all, picks, *hi)
	boardChecks(picks, all)
	fmt.Printf("skipped: %d no game info, %d player not on a team playing today, %d too little history, %d not enough minutes, %d ruled out, %d with no price on one side\n\n",
		skip.noGame, skip.noTeam, skip.noHistory, skip.minutes, skip.out, skip.noPrice)

	show := picks
	if *showAll {
		show = append([]scored(nil), all...)
		for i := range show {
			// The all-lines view is for inspection: nothing in it is staked unless it is a posted pick.
			if show[i].Prob < *hi || show[i].Edge < *minEdge {
				show[i].Units = 0
			}
		}
		sort.Slice(show, func(i, j int) bool { return show[i].Prob > show[j].Prob })
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "game\ttier\tplayer\tteam\tstat\tpick\tline\tprob\tedge\tunits\tbook\todds\tproj\tmin\twhy")
	for _, s := range show {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%.1f\t%.1f%%\t%+.1f%%\t%.2f\t%s\t%s\t%.1f\t%.0f\t%s\n",
			s.Matchup, s.Tier, s.Player, s.Team, s.Stat, s.Side, s.Line, 100*s.Prob, 100*s.Edge, s.Units, s.Vendor, oddsStr(s.Odds), s.ProjMean, s.ProjMin, s.Why)
	}
	tw.Flush()

	csvPath := filepath.Join(*dir, fmt.Sprintf("picks_%s.csv", *date))
	if err := writeCSV(csvPath, picks); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved %d picks to %s.\n", len(picks), csvPath)

	if !*noLog && len(picks) > 0 {
		var entries []picklog.Entry
		for _, s := range picks {
			entries = append(entries, picklog.Entry{
				Date: *date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
				Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, ProbRaw: s.ProbRaw,
				ProjMean: s.ProjMean, ProjMin: s.ProjMin, Units: s.Units, Tier: s.Tier, Why: s.Why,
			})
		}
		n, err := picklog.Append(filepath.Join(*dir, "picklog.jsonl"), entries)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Logged %d new picks to %s (the permanent record; never edit it).\n", n, filepath.Join(*dir, "picklog.jsonl"))
	}
	fmt.Println("Re-run close to tip-off: lineups and injuries change, and the lines move.")
}

// statBreakdown shows, per stat, how many lines were scored and how many cleared each hurdle, so a
// missing stat can be traced to its cause: no lines pulled, probabilities hugging 50%, or the
// edge rule rejecting them.
func statBreakdown(all, picks []scored, hi float64) {
	fmt.Println("by stat:   lines scored | prob >= bar | + edge ok (posted before caps) | on board | strongest prob | avg (projection - line) | share of lines where projection is under the line")
	for _, st := range data.Stats {
		var n, bar, board int
		best := 0.0
		for _, s := range all {
			if s.Stat != st {
				continue
			}
			n++
			if s.Prob >= hi {
				bar++
			}
			if s.Prob > best {
				best = s.Prob
			}
		}
		for _, s := range picks {
			if s.Stat == st && s.Tier != "LEAN" {
				board++
			}
		}
		edgeOK := 0
		for _, s := range all {
			if s.Stat == st && s.Prob >= hi && s.Edge >= 0.01 {
				edgeOK++
			}
		}
		var gap float64
		var below, seen int
		pl := map[[2]int]bool{}
		for _, s := range all {
			k := [2]int{s.PlayerID, data.StatIndex(s.Stat)}
			if s.Stat != st || pl[k] {
				continue
			}
			pl[k] = true
			gap += s.ProjMean - s.Line
			seen++
			if s.ProjMean < s.Line {
				below++
			}
		}
		avgGap, shareBelow := 0.0, 0.0
		if seen > 0 {
			avgGap, shareBelow = gap/float64(seen), 100*float64(below)/float64(seen)
		}
		fmt.Printf("  %-9s %5d | %4d | %4d | %3d | %.1f%% | %+.2f | %.0f%%\n", st, n, bar, edgeOK, board, 100*best, avgGap, shareBelow)
	}
	fmt.Println()
}

// addWatchlist adds the strongest lines for stats that rarely clear the edge rule (points, in the
// backtest), so they can still be seen and tracked. They are PAPER: the price already reflects
// the lean, so they are not bets.
func addWatchlist(picks, all []scored, stats string, top, perPlayer int) []scored {
	if strings.TrimSpace(stats) == "" || top <= 0 {
		return picks
	}
	have := map[[3]int]bool{}
	perPlr := map[int]int{}
	for _, s := range picks {
		have[[3]int{s.PlayerID, data.StatIndex(s.Stat), 0}] = true
		perPlr[s.PlayerID]++
	}
	cands := bestLinePerPick(all)
	sort.Slice(cands, func(i, j int) bool { return cands[i].Prob > cands[j].Prob })
	for _, name := range strings.Split(stats, ",") {
		st := data.Stat(strings.TrimSpace(name))
		n := 0
		for _, c := range cands {
			if c.Stat != st || n >= top || have[[3]int{c.PlayerID, data.StatIndex(c.Stat), 0}] || perPlr[c.PlayerID] >= perPlayer {
				continue
			}
			have[[3]int{c.PlayerID, data.StatIndex(c.Stat), 0}] = true
			perPlr[c.PlayerID]++
			c.Tier, c.Units = "PAPER", 0
			picks = append(picks, c)
			n++
		}
	}
	return picks
}

// moveNote describes how the market line has moved since the first snapshot of the day, and whether
// that is toward or away from the pick. The feed has no bet counts, so the reason is not known:
// several books moving together is usually news, one book alone is usually stale or shaded.
func moveNote(mv props.Move, side string) string {
	t := "the first snapshot"
	if at, err := time.Parse(time.RFC3339, mv.FirstAt); err == nil {
		loc, lerr := time.LoadLocation("America/New_York")
		if lerr != nil {
			loc = time.Local
		}
		t = at.In(loc).Format("3:04 pm")
	}
	dir := "no net change"
	switch {
	case mv.To > mv.From:
		dir = "up"
	case mv.To < mv.From:
		dir = "down"
	}
	rel := "market line moved"
	if (side == "OVER" && mv.To > mv.From) || (side == "UNDER" && mv.To < mv.From) {
		rel = "market line moved with the pick"
	} else if mv.To != mv.From {
		rel = "market line moved against the pick"
	}
	return fmt.Sprintf("%s %.1f to %.1f since %s (%d of %d books changed, %d up %d down; %s)", rel, mv.From, mv.To, t, mv.Up+mv.Down, mv.Books, mv.Up, mv.Down, dir)
}

func addWhy(why, more string) string {
	if why == "" {
		return more
	}
	return why + "; " + more
}

func tierRank(t string) int {
	switch t {
	case "PICK":
		return 0
	case "CHECK":
		return 1
	case "PAPER":
		return 2
	}
	return 3
}

// boardChecks warns when the slate looks one-sided or the minutes projections look shifted as a
// whole. Neither is proof of a problem, but both are what a wrong shared input looks like (stale
// averages early in the season, say), and a board built on one wrong input loses together.
func boardChecks(picks, all []scored) {
	var under, firm int
	for _, s := range picks {
		if s.Tier == "LEAN" {
			continue
		}
		firm++
		if s.Side == "UNDER" {
			under++
		}
	}
	if firm >= 8 {
		share := float64(under) / float64(firm)
		if share >= 0.75 || share <= 0.25 {
			fmt.Printf("WARNING: %d of %d posted plays are on the same side (%s). A board this one-sided usually means one shared\n", max(under, firm-under), firm, map[bool]string{true: "UNDER", false: "OVER"}[share >= 0.5])
			fmt.Println("input is off (often minutes). Spot-check the biggest minutes cuts before posting, and stake smaller.")
		}
	}
	if len(all) >= 40 {
		seen := map[int]bool{}
		var sum float64
		var n int
		for _, s := range all {
			if seen[s.PlayerID] {
				continue
			}
			seen[s.PlayerID] = true
			sum += s.Shift
			n++
		}
		if mean := sum / float64(n); mean <= -2.5 || mean >= 2.5 {
			fmt.Printf("WARNING: projected minutes average %+.1f vs players' recent averages across %d players. Real games rarely shift\n", mean, n)
			fmt.Println("that much as a group (about -1 in a typical opening week), so the minutes model may be off today.")
		}
	}
	fmt.Println()
}

// sgoLines converts the SportsGameOdds snapshot, if any, into props.Line values by matching
// player names to players who have history and whose team plays today.
func sgoLines(dir, date string, b *feat.Builder, gameOf map[int]props.GameInfo) []props.Line {
	path := filepath.Join(dir, "snapshots", fmt.Sprintf("sgo_%s.jsonl", date))
	pl, err := sgo.LoadSnapshot(path)
	if err != nil || len(pl) == 0 {
		return nil
	}
	ix := b.Index()
	byName := map[string]int{}
	dup := map[string]bool{}
	for pid := range gameOf {
		n := sgo.NormName(ix.Name(pid))
		if _, seen := byName[n]; seen {
			dup[n] = true
		}
		byName[n] = pid
	}
	var out []props.Line
	for _, l := range pl {
		n := sgo.NormName(l.Name)
		pid, ok := byName[n]
		if !ok || dup[n] {
			continue
		}
		out = append(out, props.Line{
			GameID: gameOf[pid].ID, PlayerID: pid, Vendor: "sgo/" + l.Book, Stat: l.Stat,
			Line: l.Line, OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, UpdatedAt: l.UpdatedAt,
		})
	}
	return out
}

// why gives a short human reason for the projection.
func why(b *feat.Builder, ix interface{ Name(int) string }, row *feat.Row, stat data.Stat, pred predictor.Pred, team, oppTeam int, out map[int]string, date string) string {
	var parts []string
	var names []string
	for _, pid := range b.RotationOf(team, row.G.Day) {
		if _, isOut := out[pid]; isOut {
			names = append(names, ix.Name(pid))
		}
	}
	if len(names) > 0 {
		parts = append(parts, "out: "+strings.Join(names, ", "))
	}
	var oppNames []string
	for _, pid := range b.RotationOf(oppTeam, row.G.Day) {
		if _, isOut := out[pid]; isOut {
			n := ix.Name(pid)
			if b.IsStarter(oppTeam, row.G.Day, pid) {
				n += " (starter)"
			}
			oppNames = append(oppNames, n)
		}
	}
	if len(oppNames) > 0 {
		parts = append(parts, "opponent out: "+strings.Join(oppNames, ", "))
	}
	ci, si := feat.FeatureIndex("b2b", stat)
	_ = si
	if ci >= 0 && row.X[ci] == 1 {
		parts = append(parts, "back-to-back")
	}
	if d := pred.Minutes - row.AvgMin15; math.Abs(d) >= 3 {
		parts = append(parts, fmt.Sprintf("minutes %+.0f vs recent", d))
	}
	if _, pi := feat.FeatureIndex("opp_allow", stat); pi >= 0 {
		if v := row.S[data.StatIndex(stat)][pi]; !math.IsNaN(v) {
			switch {
			case v > 0.04:
				parts = append(parts, fmt.Sprintf("soft defense %+.0f%%", 100*math.Expm1(v)))
			case v < -0.04:
				parts = append(parts, fmt.Sprintf("tough defense %+.0f%%", 100*math.Expm1(v)))
			}
		}
	}
	return strings.Join(parts, "; ")
}

// bestLinePerPick keeps, for each player, stat and side, the book offering the best price.
func bestLinePerPick(picks []scored) []scored {
	type key struct {
		player int
		stat   data.Stat
		side   string
	}
	best := map[key]scored{}
	var order []key
	for _, s := range picks {
		k := key{s.PlayerID, s.Stat, s.Side}
		cur, ok := best[k]
		if !ok {
			order = append(order, k)
		}
		if !ok || s.Edge > cur.Edge {
			best[k] = s
		}
	}
	out := make([]scored, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

// fillLeans tops up every game that has fewer than min plays with its best remaining lines
// (largest edge first, one line per player and stat, at most perPlayer per player). They are
// weaker than real picks, so they get 0 units and are labelled LEAN.
func fillLeans(picks, all []scored, games map[int]props.GameInfo, min, perPlayer int) []scored {
	cands := bestLinePerPick(all)
	sort.Slice(cands, func(i, j int) bool { return cands[i].Edge > cands[j].Edge })
	perGame, perPlr := map[int]int{}, map[int]int{}
	used := map[[2]int]bool{}
	for _, s := range picks {
		perGame[s.GameID]++
		perPlr[s.PlayerID]++
		used[[2]int{s.PlayerID, int(data.StatIndex(s.Stat))}] = true
	}
	for _, c := range cands {
		if perGame[c.GameID] >= min || perPlr[c.PlayerID] >= perPlayer {
			continue
		}
		k := [2]int{c.PlayerID, int(data.StatIndex(c.Stat))}
		if used[k] {
			continue
		}
		used[k] = true
		c.Tier, c.Units = "LEAN", 0
		perGame[c.GameID]++
		perPlr[c.PlayerID]++
		picks = append(picks, c)
	}
	for id := range games {
		if perGame[id] < min {
			fmt.Printf("note: game %d only has %d play(s); too few eligible lines for a player-prop market there yet\n", id, perGame[id])
		}
	}
	return picks
}

// capCorrelated limits picks per game and per player, keeping the largest edges. Picks in the
// same game or on the same player tend to win and lose together, so posting everything
// concentrates risk.
func capCorrelated(picks []scored, perGame, perPlayer int) []scored {
	sort.Slice(picks, func(i, j int) bool { return picks[i].Edge > picks[j].Edge })
	games, players := map[int]int{}, map[int]int{}
	var out []scored
	for _, s := range picks {
		if games[s.GameID] >= perGame || players[s.PlayerID] >= perPlayer {
			continue
		}
		games[s.GameID]++
		players[s.PlayerID]++
		out = append(out, s)
	}
	return out
}

func oddsStr(o int) string {
	switch {
	case o == 0:
		return "-"
	case o > 0:
		return fmt.Sprintf("+%d", o)
	}
	return fmt.Sprintf("%d", o)
}

// writeCSV saves the board for a spreadsheet: strongest probability first, with percentages as
// plain numbers (62.3 means 62.3%) so Excel can sort and filter them.
func writeCSV(path string, in []scored) error {
	picks := append([]scored(nil), in...)
	sort.SliceStable(picks, func(i, j int) bool { return picks[i].Prob > picks[j].Prob })
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"rank", "prob_pct", "tier", "player", "team", "matchup", "stat", "pick", "line", "edge_pct", "units", "book", "odds", "proj_mean", "proj_minutes", "game", "why"})
	for i, s := range picks {
		w.Write([]string{
			fmt.Sprint(i + 1), fmt.Sprintf("%.1f", 100*s.Prob), s.Tier, s.Player, s.Team, s.Matchup, string(s.Stat), s.Side,
			fmt.Sprintf("%.1f", s.Line), fmt.Sprintf("%.1f", 100*s.Edge), fmt.Sprintf("%.2f", s.Units),
			s.Vendor, oddsStr(s.Odds),
			fmt.Sprintf("%.2f", s.ProjMean), fmt.Sprintf("%.1f", s.ProjMin), fmt.Sprint(s.GameID), s.Why,
		})
	}
	w.Flush()
	return w.Error()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// findGame resolves "-game" to one of today's games: a game id, or team abbreviations (any of
// "ORL", "ORL CHA", "ORL@CHA", "orl vs cha"). One abbreviation is enough if that team plays once.
func findGame(q string, games map[int]props.GameInfo) (int, error) {
	if id, err := strconv.Atoi(strings.TrimSpace(q)); err == nil {
		if _, ok := games[id]; ok {
			return id, nil
		}
		return 0, fmt.Errorf("game id %d is not on today's slate\n%s", id, slate(games))
	}
	tokens := strings.FieldsFunc(strings.ToUpper(q), func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
	var hits []int
	for id, g := range games {
		ok := len(tokens) > 0
		for _, t := range tokens {
			if t == "VS" || t == "AT" {
				continue
			}
			if t != strings.ToUpper(g.HomeAbbr) && t != strings.ToUpper(g.VisitorAbbr) {
				ok = false
			}
		}
		if ok {
			hits = append(hits, id)
		}
	}
	sort.Ints(hits)
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return 0, fmt.Errorf("no game today matches %q\n%s", q, slate(games))
	}
	return 0, fmt.Errorf("%q matches more than one game; name both teams\n%s", q, slate(games))
}

func slate(games map[int]props.GameInfo) string {
	var ids []int
	for id := range games {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var b strings.Builder
	b.WriteString("Today's games:\n")
	for _, id := range ids {
		g := games[id]
		v, h := g.VisitorAbbr, g.HomeAbbr
		if v == "" || h == "" {
			v, h = fmt.Sprintf("team %d", g.Visitor), fmt.Sprintf("team %d", g.Home)
		}
		fmt.Fprintf(&b, "  %s @ %s   (game id %d)\n", v, h, id)
	}
	return b.String()
}

// gameView prints one game's strongest lines: the best price for each player, stat and side,
// ranked by probability, with the tier it would get on the main board. Lines below the bar are
// marked LOOK: the model leans that way but not enough to post it.
func gameView(g props.GameInfo, c team.Consensus, all, picks []scored, staked map[data.Stat]bool, top int, hi, minEdge float64, date, dir string) {
	title := fmt.Sprintf("%s @ %s", g.VisitorAbbr, g.HomeAbbr)
	fmt.Printf("%s   %s   (game id %d)\n", title, date, g.ID)
	if c.HasSpread && c.HasTotal {
		fmt.Printf("market: %s %+.1f, total %.1f   (context only: the game-line model has no proven edge)\n", g.HomeAbbr, c.SpreadHome, c.Total)
	}
	type key struct {
		player int
		stat   data.Stat
		side   string
	}
	tier := map[key]string{}
	for _, p := range picks {
		tier[key{p.PlayerID, p.Stat, p.Side}] = p.Tier
	}
	cands := bestLinePerPick(all)
	sort.Slice(cands, func(i, j int) bool { return cands[i].Prob > cands[j].Prob })
	for i := range cands {
		if t, ok := tier[key{cands[i].PlayerID, cands[i].Stat, cands[i].Side}]; ok {
			cands[i].Tier = t
		} else {
			cands[i].Tier, cands[i].Units = "LOOK", 0
		}
	}
	posted := 0
	for _, s := range cands {
		if s.Tier != "LOOK" && s.Tier != "LEAN" {
			posted++
		}
	}
	fmt.Printf("%d lines scored; %d clear the %.0f%% bar and the %.0f%% edge rule (PICK/PAPER/CHECK); the rest are LOOK.\n",
		len(cands), posted, 100*hi, 100*minEdge)
	fmt.Println("Tiers: PICK staked | PAPER unproven stat or early-season team | CHECK confirm minutes/status/line | LOOK below the bar")
	fmt.Println()
	if len(cands) > top {
		cands = cands[:top]
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "tier\tplayer\tteam\tstat\tpick\tline\tprob\tedge\tunits\tbook\todds\tproj\tmin\twhy")
	for _, s := range cands {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%.1f\t%.1f%%\t%+.1f%%\t%.2f\t%s\t%s\t%.1f\t%.0f\t%s\n",
			s.Tier, s.Player, s.Team, s.Stat, s.Side, s.Line, 100*s.Prob, 100*s.Edge, s.Units, s.Vendor, oddsStr(s.Odds), s.ProjMean, s.ProjMin, s.Why)
	}
	tw.Flush()
	path := filepath.Join(dir, fmt.Sprintf("game_%s-%s_%s.csv", g.VisitorAbbr, g.HomeAbbr, date))
	if err := writeCSV(path, cands); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved %s.\n", path)

	projectionTable(g, all, picks, staked, hi, minEdge, date, dir)
	fmt.Println("\nNothing from this view is logged; run the normal predict to log the day's plays.")
}

// projectionTable shows every player's projection against the market line with no gates: no
// 55% bar, no edge rule, no warm-up or stat restrictions. The probabilities are the same
// calibrated ones the board uses (pulled toward the book price), so a projection far above the
// line does not turn into a big number unless the history supports it. Each row says how much
// to trust it.
func projectionTable(g props.GameInfo, all, picks []scored, staked map[data.Stat]bool, hi, minEdge float64, date, dir string) {
	type key struct {
		player int
		stat   data.Stat
	}
	groups := map[key][]scored{}
	for _, s := range all {
		k := key{s.PlayerID, s.Stat}
		groups[k] = append(groups[k], s)
	}
	tier := map[key]string{}
	for _, p := range picks {
		if p.Tier == "LEAN" {
			continue
		}
		tier[key{p.PlayerID, p.Stat}] = p.Tier
	}
	var rows []scored
	for k, v := range groups {
		// Use the consensus line (the median across books) and the version of it closest to it.
		ls := make([]float64, len(v))
		for i, s := range v {
			ls[i] = s.Line
		}
		sort.Float64s(ls)
		med := ls[len(ls)/2]
		best := v[0]
		for _, s := range v[1:] {
			if math.Abs(s.Line-med) < math.Abs(best.Line-med) || (math.Abs(s.Line-med) == math.Abs(best.Line-med) && s.Edge > best.Edge) {
				best = s
			}
		}
		label := "coin flip: no lean"
		switch {
		case tier[k] != "":
			label = tier[k] + " (clears the bar)"
		case best.Prob >= hi && best.Edge >= minEdge:
			label = "clears the bar, left off the board by the per-game/per-player cap"
		case best.Prob >= hi:
			label = "clears the bar, but the price is not good enough"
		case best.Prob >= 0.52:
			label = "slight lean: inside the noise, not an edge"
		}
		if !staked[best.Stat] {
			label += "; stat unproven in backtest"
		}
		best.Tier = label
		rows = append(rows, best)
	}
	statRank := map[data.Stat]int{data.Points: 0, data.Rebounds: 1, data.Assists: 2, data.Threes: 3}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ProjMin != b.ProjMin {
			return a.ProjMin > b.ProjMin
		}
		if a.Player != b.Player {
			return a.Player < b.Player
		}
		return statRank[a.Stat] < statRank[b.Stat]
	})
	fmt.Printf("\nPROJECTIONS, NO RESTRICTIONS: %s @ %s (one row per player and stat, at the consensus line, starters first)\n", g.VisitorAbbr, g.HomeAbbr)
	fmt.Println("prob is the calibrated chance of the leaning side. 'edge' compares it with the price you would pay.")
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "player\tteam\tstat\tproj\tline\tlean\tprob\tedge\tmin\tbook\todds\tread")
	for _, s := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%.1f\t%s\t%.1f%%\t%+.1f%%\t%.0f\t%s\t%s\t%s\n",
			s.Player, s.Team, s.Stat, s.ProjMean, s.Line, s.Side, 100*s.Prob, 100*s.Edge, s.ProjMin, s.Vendor, oddsStr(s.Odds), s.Tier)
	}
	tw.Flush()
	fmt.Println(`
HOW TO READ THIS HONESTLY
  - A projection above the line is not an edge. Books see the same stats, injuries and matchups; the
    backtest shows this model beats the book price by only a hair on average (about +0.0005 log loss
    per line), and its picks above 55% have not been proven profitable.
  - Only rebounds had a profitable backtest. Points and assists rows are shown for information; they
    are marked "stat unproven" and are never staked.
  - A 51-54% lean is inside the noise. Do not bet it because it is on this list.
  - Check minutes and status before betting anything: a wrong minutes projection is the most common cause of a miss.
  - Small stakes, and only on plays you would still like at the price you can actually get.`)
	path := filepath.Join(dir, fmt.Sprintf("game_%s-%s_%s_projections.csv", g.VisitorAbbr, g.HomeAbbr, date))
	if err := writeCSV(path, rows); err != nil {
		fatal(err)
	}
	fmt.Printf("\nSaved %s.\n", path)
}
