// Package feat turns raw box scores into model features.
//
// Every feature for a game is computed from information that existed BEFORE that game's date,
// with one deliberate exception: the teammates-out features. For history they are derived from
// who actually played, which is what a live run would know from the injury report and lineups
// near tip-off. A backtest against OPENING lines therefore flatters those features (the market
// had not yet seen the late scratches), which is why the backtest reports them separately.
//
// Missing values are NaN; the regression treats them as an average value.
package feat

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"propsmodel/internal/data"
	"propsmodel/internal/proj"
)

// Spec names a feature and the group it belongs to. Groups are the unit of ablation testing:
// the backtest can switch a whole group off and measure what it was worth.
type Spec struct{ Name, Group string }

// Feature groups.
const (
	GroupMinutes   = "minutes"   // minutes history (always on)
	GroupBase      = "base"      // baseline per-minute rate (always on)
	GroupForm      = "form"      // recent form vs long run
	GroupSlump     = "slump"     // shooting efficiency vs the player's norm
	GroupOpp       = "opp"       // opponent defense and game pace
	GroupGame      = "game"      // strength gap (blowout / game script proxy)
	GroupRest      = "rest"      // back-to-backs, schedule density
	GroupHome      = "home"      // home court
	GroupTeammates = "teammates" // rotation players who are out
	GroupUsage     = "usage"     // advanced-stat trends: usage, assist and rebound share, passes
	GroupH2H       = "h2h"       // this player's history against this opponent (shrunk hard)
	GroupSplit     = "split"     // this player's home/away split (shrunk hard)
	GroupLayoff    = "layoff"    // games missed since his last game: returning from an absence
	GroupMatchup   = "matchup"   // opponent defense against this player's position
	GroupMarket    = "market"    // the game's betting spread and total, known before tip-off (blowout risk, pace)
	GroupSeason    = "season"    // how far into the season: lets early-season games trust history differently
	GroupChances   = "chances"   // tracking opportunities: rebound chances, passes, touches, and how well they convert
)

// OptionalGroups are the groups that can be switched off for ablation.
var OptionalGroups = []string{GroupForm, GroupSlump, GroupOpp, GroupGame, GroupRest, GroupHome, GroupTeammates, GroupUsage, GroupH2H, GroupSplit, GroupLayoff, GroupMatchup, GroupSeason, GroupMarket, GroupChances}

// CommonSpecs are features shared by every model. The minutes model uses all of them; the stat
// models skip the ones in the "minutes" group because they already condition on minutes played.
var CommonSpecs = []Spec{
	{"m_ewma", GroupMinutes}, {"m_last", GroupMinutes}, {"m_avg5", GroupMinutes},
	{"m_avg30", GroupMinutes}, {"m_start", GroupMinutes}, {"m_blowcut", GroupMinutes},
	{"m_med10", GroupMinutes}, {"m_low10", GroupMinutes},
	{"rest", GroupRest}, {"b2b", GroupRest}, {"g7", GroupRest}, {"opp_b2b", GroupRest},
	{"missed", GroupLayoff}, {"returning", GroupLayoff},
	{"early", GroupSeason}, {"early_m", GroupSeason},
	{"home", GroupHome},
	{"abs_spread", GroupMarket}, {"fav_spread", GroupMarket}, {"game_total", GroupMarket},
	{"star_spread", GroupMarket},
	{"net_diff", GroupGame},
	{"pace", GroupOpp},
	{"miss_min", GroupTeammates}, {"miss_starters", GroupTeammates},
}

// StatSpecs are the extra features for each stat, indexed by data.StatIndex.
var StatSpecs = [data.NStats][]Spec{
	{ // points
		{"base_rate", GroupBase}, {"form_tr", GroupForm}, {"fga_tr", GroupForm}, {"fg_dev", GroupSlump},
		{"opp_allow", GroupOpp}, {"miss_s", GroupTeammates},
		{"usage_tr", GroupUsage}, {"usage_lvl", GroupUsage},
		{"touch_tr", GroupChances},
		{"h2h", GroupH2H}, {"venue_split", GroupSplit}, {"pos5_allow", GroupMatchup},
	},
	{ // rebounds
		{"base_rate", GroupBase}, {"form_tr", GroupForm},
		{"opp_allow", GroupOpp}, {"miss_s", GroupTeammates},
		{"reb_tr", GroupUsage},
		{"rch_tr", GroupChances}, {"rch_conv", GroupChances}, {"rch_off", GroupChances},
		{"h2h", GroupH2H}, {"venue_split", GroupSplit}, {"pos5_allow", GroupMatchup},
	},
	{ // assists
		{"base_rate", GroupBase}, {"form_tr", GroupForm},
		{"opp_allow", GroupOpp}, {"miss_s", GroupTeammates},
		{"ast_tr", GroupUsage}, {"pass_tr", GroupUsage}, {"usage_tr", GroupUsage},
		{"pass_conv", GroupChances}, {"touch_tr", GroupChances}, {"sec_tr", GroupChances},
		{"h2h", GroupH2H}, {"venue_split", GroupSplit}, {"pos5_allow", GroupMatchup},
	},
	{ // threes made
		{"base_rate", GroupBase}, {"form_tr", GroupForm}, {"tpa_tr", GroupForm}, {"tp_dev", GroupSlump},
		{"opp_allow", GroupOpp}, {"miss_s", GroupTeammates},
		{"h2h", GroupH2H}, {"venue_split", GroupSplit}, {"pos5_allow", GroupMatchup},
	},
}

var (
	commonIdx = map[string]int{}
	statIdx   [data.NStats]map[string]int
)

func init() {
	for i, s := range CommonSpecs {
		commonIdx[s.Name] = i
	}
	for k := range StatSpecs {
		statIdx[k] = map[string]int{}
		for i, s := range StatSpecs[k] {
			statIdx[k][s.Name] = i
		}
	}
}

func on(group string, disabled map[string]bool) bool { return !disabled[group] }

// MinutesNames lists the features of the minutes model with the given groups switched off.
func MinutesNames(disabled map[string]bool) []string {
	var out []string
	for _, s := range CommonSpecs {
		if s.Name == "pace" {
			continue
		}
		if on(s.Group, disabled) {
			out = append(out, s.Name)
		}
	}
	return out
}

// StatNames lists the features of one stat model with the given groups switched off.
func StatNames(stat data.Stat, disabled map[string]bool) []string {
	var out []string
	for _, s := range CommonSpecs {
		if s.Group != GroupMinutes && s.Name != "early_m" && s.Name != "star_spread" && on(s.Group, disabled) {
			out = append(out, s.Name)
		}
	}
	for _, s := range StatSpecs[data.StatIndex(stat)] {
		if on(s.Group, disabled) {
			out = append(out, s.Name)
		}
	}
	return out
}

// Row holds the features of one player in one game.
type Row struct {
	G        data.PlayerGame        // the actual game when known (labels); identity fields only for a live row
	X        []float64              // common features, in CommonSpecs order
	S        [data.NStats][]float64 // per-stat features, in StatSpecs order
	AvgMin15 float64                // average minutes over the last 15 games, for the eligibility rule
	Games    int                    // games of history behind the row
}

// Vec picks the named features out of a row, in order.
func (r *Row) Vec(stat data.Stat, names []string) []float64 {
	k := data.StatIndex(stat)
	out := make([]float64, len(names))
	for i, n := range names {
		if j, ok := commonIdx[n]; ok {
			out[i] = r.X[j]
		} else if j, ok := statIdx[k][n]; ok {
			out[i] = r.S[k][j]
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

const (
	minPrior    = 8  // games of history needed before a row is built
	teamWindow  = 20 // games used for team pace, defense and strength
	teamShrink  = 6  // pseudo-games of league-average behaviour
	rotWindow   = 10 // team games used to decide who is in the rotation
	rotMinApps  = 5  // appearances in those games to count as rotation
	rotMinMin   = 14 // average minutes to count as rotation
	starterMins = 28 // average minutes that make an absent player a "starter"
)

type teamGame struct {
	Day, GameID, Opp int
	Season           int
	HasOpp           bool
	For, Against     [data.NStats]float64
	PosFor           [NPos][data.NStats]float64 // [position group][stat] produced by this team's players
	PosAgainst       [NPos][data.NStats]float64 // what the opponent's players produced against this team
	Players          map[int]float64
}

type teamKey struct{ team, game int }

type leagueCum struct {
	days   []int
	forSum [][data.NStats]float64
	totSum []float64
	posSum [][NPos][data.NStats]float64
}

// Builder computes features from the full history it is given. Rows only ever use data from
// before their own date.
type Builder struct {
	ix       *proj.Index
	all      []data.PlayerGame
	byPlayer map[int][]data.PlayerGame
	adv      map[data.AdvKey]data.AdvRow
	team     map[int][]*teamGame
	tgByKey  map[teamKey]*teamGame
	league   map[int]*leagueCum
	rot      map[[2]int][]rotMember
	roster   map[int]int // current team of each active player, when known
	lines    map[[2]int]GameLine
}

// GameLine is a game's betting market, from one team's point of view. Spread is negative for
// the favourite. Where it is missing the features are left empty and the model treats them as
// average.
type GameLine struct {
	Spread float64
	Total  float64
}

// SetGameLines supplies known betting lines keyed by (day number, team id). Call it before
// building rows; Rows and Live then add the spread and total as features.
func (b *Builder) SetGameLines(m map[[2]int]GameLine) { b.lines = m }

// AddGameLine adds one team's line for one day (used for today's games).
func (b *Builder) AddGameLine(day, team int, l GameLine) {
	if b.lines == nil {
		b.lines = map[[2]int]GameLine{}
	}
	b.lines[[2]int{day, team}] = l
}

type rotMember struct {
	pid  int
	min  float64
	prod [data.NStats]float64
}

// NewBuilder indexes the history. games must be sorted by date (data.LoadStatsDir does that).
// adv may be nil or empty.
func NewBuilder(games []data.PlayerGame, adv map[data.AdvKey]data.AdvRow) *Builder {
	b := &Builder{
		ix:       proj.NewIndex(games),
		all:      games,
		byPlayer: map[int][]data.PlayerGame{},
		adv:      adv,
		team:     map[int][]*teamGame{},
		tgByKey:  map[teamKey]*teamGame{},
		league:   map[int]*leagueCum{},
		rot:      map[[2]int][]rotMember{},
	}
	for _, g := range games {
		b.byPlayer[g.PlayerID] = append(b.byPlayer[g.PlayerID], g)
		k := teamKey{g.TeamID, g.GameID}
		tg := b.tgByKey[k]
		if tg == nil {
			tg = &teamGame{Day: g.Day, GameID: g.GameID, Season: g.Season, Players: map[int]float64{}}
			b.tgByKey[k] = tg
			b.team[g.TeamID] = append(b.team[g.TeamID], tg)
		}
		tg.Players[g.PlayerID] = g.Minutes
		tg.For[0] += g.Pts
		tg.For[1] += g.Reb
		tg.For[2] += g.Ast
		pg := PosGroup(g.Pos)
		tg.PosFor[pg][0] += g.Pts
		tg.PosFor[pg][1] += g.Reb
		tg.PosFor[pg][2] += g.Ast
	}
	// Pair each team game with the opponent's side of the same game.
	byGame := map[int][]int{}
	for k := range b.tgByKey {
		byGame[k.game] = append(byGame[k.game], k.team)
	}
	for game, teams := range byGame {
		if len(teams) != 2 {
			continue
		}
		a, c := b.tgByKey[teamKey{teams[0], game}], b.tgByKey[teamKey{teams[1], game}]
		a.Opp, c.Opp = teams[1], teams[0]
		a.Against, c.Against = c.For, a.For
		a.PosAgainst, c.PosAgainst = c.PosFor, a.PosFor
		a.HasOpp, c.HasOpp = true, true
	}
	for _, list := range b.team {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Day != list[j].Day {
				return list[i].Day < list[j].Day
			}
			return list[i].GameID < list[j].GameID
		})
	}
	b.buildLeague()
	return b
}

// SetRoster gives the builder each active player's CURRENT team (from `pull players`). A player
// only counts as part of a team's rotation if he is still on that team, so players who were
// traded or signed elsewhere since the last box score are not treated as teammates who are out.
func (b *Builder) SetRoster(roster map[int]int) {
	b.roster = roster
	b.rot = map[[2]int][]rotMember{}
}

// Index returns the baseline projection index built on the same history.
func (b *Builder) Index() *proj.Index { return b.ix }

func (b *Builder) buildLeague() {
	var list []*teamGame
	for _, tgs := range b.team {
		for _, tg := range tgs {
			if tg.HasOpp {
				list = append(list, tg)
			}
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Day < list[j].Day })
	for _, tg := range list {
		c := b.league[tg.Season]
		if c == nil {
			c = &leagueCum{}
			b.league[tg.Season] = c
		}
		var f [data.NStats]float64
		var tot float64
		var ps [NPos][data.NStats]float64
		if n := len(c.days); n > 0 {
			f, tot, ps = c.forSum[n-1], c.totSum[n-1], c.posSum[n-1]
		}
		for k := 0; k < data.NStats; k++ {
			f[k] += tg.For[k]
			for g := 0; g < NPos; g++ {
				ps[g][k] += tg.PosFor[g][k]
			}
		}
		tot += tg.For[0] + tg.Against[0]
		c.days = append(c.days, tg.Day)
		c.forSum = append(c.forSum, f)
		c.totSum = append(c.totSum, tot)
		c.posSum = append(c.posSum, ps)
	}
}

// leagueMeans returns the per-team-game league means (points, rebounds, assists, total points
// in a game, and the same three stats by position group) from games before `day`, using the
// previous season when the current one has too few games.
func (b *Builder) leagueMeans(season, day int) (f [data.NStats]float64, tot float64, pos [NPos][data.NStats]float64, ok bool) {
	at := func(c *leagueCum, before int) ([data.NStats]float64, float64, [NPos][data.NStats]float64, int) {
		i := sort.SearchInts(c.days, before)
		if i == 0 {
			return [data.NStats]float64{}, 0, [NPos][data.NStats]float64{}, 0
		}
		return c.forSum[i-1], c.totSum[i-1], c.posSum[i-1], i
	}
	fill := func(s [data.NStats]float64, t float64, ps [NPos][data.NStats]float64, n int) {
		for k := range s {
			f[k] = s[k] / float64(n)
		}
		tot = t / float64(n)
		for g := range ps {
			for k := range ps[g] {
				pos[g][k] = ps[g][k] / float64(n)
			}
		}
	}
	if c := b.league[season]; c != nil {
		if s, t, ps, n := at(c, day); n >= 150 {
			fill(s, t, ps, n)
			return f, tot, pos, true
		}
	}
	if c := b.league[season-1]; c != nil {
		if s, t, ps, n := at(c, math.MaxInt32); n >= 150 {
			fill(s, t, ps, n)
			return f, tot, pos, true
		}
	}
	return f, 0, pos, false
}

// teamBefore returns the team's games strictly before day.
func (b *Builder) teamBefore(team, day int) []*teamGame {
	list := b.team[team]
	i := sort.Search(len(list), func(i int) bool { return list[i].Day >= day })
	return list[:i]
}

// SeasonGames is how many games the team has already played in the season that contains date.
func (b *Builder) SeasonGames(team int, date string) int {
	day := data.DayNumber(date)
	start := data.DayNumber(fmt.Sprintf("%d-08-01", data.SeasonOf(date)))
	n := 0
	for _, g := range b.teamBefore(team, day) {
		if g.Day >= start {
			n++
		}
	}
	return n
}

func lastN(list []*teamGame, n int) []*teamGame {
	if len(list) <= n {
		return list
	}
	return list[len(list)-n:]
}

func tailGames(h []data.PlayerGame, n int) []data.PlayerGame {
	if len(h) <= n {
		return h
	}
	return h[len(h)-n:]
}

// BlowoutAdjust switches the blowout correction to the minutes averages on or off. It is on in
// real use; the backtest's -noblowout flag turns it off to measure what the correction adds.
var BlowoutAdjust = true

// Blowout handling. Starters are pulled early in lopsided games, so a low minutes total in a 25-point
// game says nothing about how much that player will play next time. Before the minutes averages are
// taken, a short night in a blowout is raised to the player's normal minutes (the average of his
// non-blowout games), because the coach took the minutes away, not the player's role.
const (
	blowoutMargin  = 18.0 // final margin at which the starters usually sit (same cutoff the review uses)
	blowoutMinGood = 3    // non-blowout games needed before anything is adjusted
)

// adjustBlowouts returns a copy of h with blowout shortfalls in Minutes filled in, plus the average
// minutes added across the last ten games (0 when nothing was adjusted). margin reports a game's
// final margin and whether it is known. Only minutes below the player's normal are raised: a
// starter who still played 38 in a rout keeps his 38.
func adjustBlowouts(h []data.PlayerGame, margin func(data.PlayerGame) (float64, bool)) ([]data.PlayerGame, float64) {
	out := make([]data.PlayerGame, len(h))
	copy(out, h)
	blow := make([]bool, len(h))
	var sum float64
	var good int
	for i, g := range h {
		if m, ok := margin(g); ok && math.Abs(m) >= blowoutMargin {
			blow[i] = true
			continue
		}
		sum += g.Minutes
		good++
	}
	if good < blowoutMinGood {
		return out, 0
	}
	normal := sum / float64(good)
	var added float64
	for i := range out {
		if blow[i] && out[i].Minutes < normal {
			if i >= len(out)-10 {
				added += normal - out[i].Minutes
			}
			out[i].Minutes = normal
		}
	}
	n := 10
	if len(out) < n {
		n = len(out)
	}
	return out, added / float64(n)
}

func (b *Builder) gameMargin(g data.PlayerGame) (float64, bool) {
	tg := b.tgByKey[teamKey{g.TeamID, g.GameID}]
	if tg == nil || !tg.HasOpp {
		return 0, false
	}
	return tg.For[0] - tg.Against[0], true
}

func medianMinutes(h []data.PlayerGame) float64 {
	if len(h) == 0 {
		return math.NaN()
	}
	m := make([]float64, len(h))
	for i, g := range h {
		m[i] = g.Minutes
	}
	sort.Float64s(m)
	if n := len(m); n%2 == 0 {
		return (m[n/2-1] + m[n/2]) / 2
	}
	return m[len(m)/2]
}

// shareBelow is the fraction of games with fewer minutes than the cutoff (NaN when there are none).
func shareBelow(h []data.PlayerGame, cutoff float64) float64 {
	if len(h) == 0 || math.IsNaN(cutoff) {
		return math.NaN()
	}
	n := 0.0
	for _, g := range h {
		if g.Minutes < cutoff {
			n++
		}
	}
	return n / float64(len(h))
}

func ewma(h []data.PlayerGame, halfLife float64) float64 {
	var num, den float64
	for i, g := range h {
		w := math.Pow(0.5, float64(len(h)-1-i)/halfLife)
		num += w * g.Minutes
		den += w
	}
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func meanMinutes(h []data.PlayerGame) float64 {
	if len(h) == 0 {
		return math.NaN()
	}
	var s float64
	for _, g := range h {
		s += g.Minutes
	}
	return s / float64(len(h))
}

// rotation returns the players who were in the team's rotation going into `day`.
func (b *Builder) rotation(team, day int) []rotMember {
	key := [2]int{team, day}
	if r, ok := b.rot[key]; ok {
		return r
	}
	apps := map[int]int{}
	mins := map[int]float64{}
	for _, tg := range lastN(b.teamBefore(team, day), rotWindow) {
		for pid, m := range tg.Players {
			apps[pid]++
			mins[pid] += m
		}
	}
	var out []rotMember
	for pid, n := range apps {
		avg := mins[pid] / float64(n)
		if n < rotMinApps || avg < rotMinMin {
			continue
		}
		hist := b.history(pid, day)
		if len(hist) == 0 || hist[len(hist)-1].TeamID != team {
			continue // traded away
		}
		if len(b.roster) > 0 {
			if cur, ok := b.roster[pid]; !ok || cur != team {
				continue // no longer on this team (or not active)
			}
		}
		rm := rotMember{pid: pid, min: avg}
		recent := tailGames(hist, 10)
		for _, g := range recent {
			rm.prod[0] += g.Pts
			rm.prod[1] += g.Reb
			rm.prod[2] += g.Ast
		}
		for k := range rm.prod {
			rm.prod[k] /= float64(len(recent))
		}
		out = append(out, rm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].pid < out[j].pid })
	b.rot[key] = out
	return out
}

// history returns a player's games strictly before day.
func (b *Builder) history(pid, day int) []data.PlayerGame {
	g := b.byPlayer[pid]
	i := sort.Search(len(g), func(i int) bool { return g[i].Day >= day })
	return g[:i]
}

// RotationOf lists the players currently in a team's rotation, for reports.
func (b *Builder) RotationOf(team, day int) []int {
	var out []int
	for _, r := range b.rotation(team, day) {
		out = append(out, r.pid)
	}
	return out
}

// absentees returns rotation players who are not playing. played reports who played (history)
// or out reports who is ruled out (live).
func (b *Builder) absentees(team, day, self int, isAbsent func(pid int) bool) []rotMember {
	var out []rotMember
	for _, r := range b.rotation(team, day) {
		if r.pid != self && isAbsent(r.pid) {
			out = append(out, r)
		}
	}
	return out
}

// Rows builds a feature row for every historical game that has enough history, in order.
// minAvg is the smallest 15-game average minutes a player needs to be included (training focus).
func (b *Builder) Rows(minAvg float64) []Row {
	var rows []Row
	for _, g := range b.all {
		hist := b.history(g.PlayerID, g.Day)
		if len(hist) < minPrior {
			continue
		}
		if meanMinutes(tailGames(hist, 15)) < minAvg {
			continue
		}
		tg := b.tgByKey[teamKey{g.TeamID, g.GameID}]
		var abs []rotMember
		if tg != nil {
			abs = b.absentees(g.TeamID, g.Day, g.PlayerID, func(pid int) bool {
				_, played := tg.Players[pid]
				return !played
			})
		}
		home := math.NaN()
		if g.HomeTeamID != 0 {
			home = 0
			if g.HomeTeamID == g.TeamID {
				home = 1
			}
		}
		opp := 0
		if tg != nil && tg.HasOpp {
			opp = tg.Opp
		}
		r := b.build(hist, g.TeamID, opp, home, g.Day, g.Date, abs)
		if r == nil {
			continue
		}
		r.G = g
		rows = append(rows, *r)
	}
	return rows
}

// Live builds the row for an upcoming game. out is the set of player ids ruled out.
func (b *Builder) Live(playerID, teamID, oppID int, home bool, date string, out map[int]bool) *Row {
	day := data.DayNumber(date)
	hist := b.history(playerID, day)
	if len(hist) < minPrior {
		return nil
	}
	abs := b.absentees(teamID, day, playerID, func(pid int) bool { return out[pid] })
	h := 0.0
	if home {
		h = 1
	}
	r := b.build(hist, teamID, oppID, h, day, date, abs)
	if r == nil {
		return nil
	}
	r.G = data.PlayerGame{PlayerID: playerID, TeamID: teamID, Date: date, Day: day, Season: data.SeasonOf(date)}
	return r
}

// PosGroup maps a position string (G, F, C, G-F, C-F...) to 0 guard, 1 forward, 2 center, by its
// first letter. Unknown positions count as forwards.
func PosGroup(pos string) int {
	p := strings.ToUpper(strings.TrimSpace(pos))
	switch p {
	case "G", "PG", "SG":
		return 0 // guard
	case "G-F", "F-G":
		return 1 // wing: guard-forward hybrids
	case "F", "SF", "PF", "":
		return 2 // forward (also the default when the feed has no position)
	case "F-C", "C-F":
		return 3 // big forward: forward-center hybrids
	case "C":
		return 4 // center
	}
	switch p[0] {
	case 'G':
		return 0
	case 'C':
		return 4
	}
	return 2
}

// NPos is the number of position groups used for the matchup features. The feed only has G, F, C
// and hybrids, so the five groups are guard, wing (G-F), forward, big forward (F-C) and center.
const NPos = 5

// oppOf returns the opponent of a past game for the player's team (0 if unknown).
func oppOf(g data.PlayerGame) int {
	switch {
	case g.HomeTeamID == 0 || g.VisitorTeamID == 0:
		return 0
	case g.TeamID == g.HomeTeamID:
		return g.VisitorTeamID
	}
	return g.HomeTeamID
}

func isHomeGame(g data.PlayerGame) (home, ok bool) {
	if g.HomeTeamID == 0 {
		return false, false
	}
	return g.TeamID == g.HomeTeamID, true
}

func logRatio(num, den float64) float64 {
	if num <= 0 || den <= 0 {
		return math.NaN()
	}
	return math.Log(num / den)
}

// shrunkMean averages x over games, blended with `prior` as if teamShrink extra games had
// that value.
func shrunkMean(games []*teamGame, prior float64, x func(*teamGame) float64) float64 {
	sum := float64(teamShrink) * prior
	n := float64(teamShrink)
	for _, g := range games {
		sum += x(g)
		n++
	}
	return sum / n
}

func (b *Builder) build(hist []data.PlayerGame, team, opp int, home float64, day int, date string, abs []rotMember) *Row {
	pj, ok := b.ix.Project(hist[len(hist)-1].PlayerID, date)
	if !ok {
		return nil
	}
	season := data.SeasonOf(date)
	r := &Row{
		X:        make([]float64, len(CommonSpecs)),
		AvgMin15: meanMinutes(tailGames(hist, 15)),
		Games:    len(hist),
	}
	for k := range r.S {
		r.S[k] = make([]float64, len(StatSpecs[k]))
		for i := range r.S[k] {
			r.S[k][i] = math.NaN()
		}
	}
	setC := func(name string, v float64) { r.X[commonIdx[name]] = v }
	for i := range r.X {
		r.X[i] = math.NaN()
	}

	// Minutes history.
	// Minutes are averaged after blowout shortfalls are filled in, so a night on the bench in a
	// 25-point game does not make the model expect fewer minutes next time.
	adj, cut := tailGames(hist, 30), 0.0
	if BlowoutAdjust {
		adj, cut = adjustBlowouts(adj, b.gameMargin)
	}
	setC("m_ewma", ewma(tailGames(adj, 10), 4))
	setC("m_last", adj[len(adj)-1].Minutes)
	setC("m_avg5", meanMinutes(tailGames(adj, 5)))
	setC("m_avg30", meanMinutes(adj))
	setC("m_blowcut", cut)
	// A median is not dragged around by one injury exit or one garbage-time night, and the share
	// of recent games well below his normal says how often his minutes have been cut short.
	setC("m_med10", medianMinutes(tailGames(adj, 10)))
	setC("m_low10", shareBelow(tailGames(adj, 10), 0.6*meanMinutes(adj)))
	started := 0.0
	last10 := tailGames(hist, 10)
	for _, g := range last10 {
		if g.Minutes >= starterMins {
			started++
		}
	}
	setC("m_start", started/float64(len(last10)))

	// Schedule.
	own := b.teamBefore(team, day)
	rest, g7 := 3.0, 0.0
	if n := len(own); n > 0 {
		if gap := day - own[n-1].Day; gap <= 20 {
			rest = math.Min(float64(gap), 7)
		}
		for i := n - 1; i >= 0 && day-own[i].Day <= 7; i-- {
			g7++
		}
	}
	setC("rest", rest)
	if gl, ok := b.lines[[2]int{day, team}]; ok {
		setC("abs_spread", math.Abs(gl.Spread))
		setC("fav_spread", gl.Spread)
		setC("game_total", gl.Total)
		// A lopsided spread costs a starter minutes (he is pulled early in the rout) far more than
		// it costs a bench player, so the spread is scaled by how big a part of the game he plays.
		if m30 := meanMinutes(tailGames(hist, 30)); !math.IsNaN(m30) {
			setC("star_spread", math.Abs(gl.Spread)*m30/36)
		}
	}
	// Season phase: 1 at the opener, falling to 0 after 15 team games. Early in a season the
	// "recent" windows are mostly last season's games and the rolling team numbers are nearly
	// empty, so the model needs to be able to weigh history differently then.
	sg := 0.0
	seasonStart := data.DayNumber(fmt.Sprintf("%d-08-01", season))
	for i := len(own) - 1; i >= 0 && own[i].Day >= seasonStart; i-- {
		sg++
	}
	early := math.Max(0, 1-sg/15)
	setC("early", early)
	setC("early_m", early*meanMinutes(tailGames(hist, 30)))
	b2b := 0.0
	if rest <= 1 {
		b2b = 1
	}
	setC("b2b", b2b)
	setC("g7", g7)
	setC("home", home)

	// Team strength gap, game pace and opponent defense.
	lf, ltot, lpos, lok := b.leagueMeans(season, day)
	pg := PosGroup(hist[len(hist)-1].Pos)
	oppG := b.teamBefore(opp, day)
	if opp != 0 && lok {
		ownW, oppW := lastN(onlyWithOpp(own), teamWindow), lastN(onlyWithOpp(oppG), teamWindow)
		margin := func(g *teamGame) float64 { return g.For[0] - g.Against[0] }
		setC("net_diff", shrunkMean(ownW, 0, margin)-shrunkMean(oppW, 0, margin))
		tot := func(g *teamGame) float64 { return g.For[0] + g.Against[0] }
		setC("pace", logRatio((shrunkMean(ownW, ltot, tot)+shrunkMean(oppW, ltot, tot))/2, ltot))
		for k := 0; k < data.NStats; k++ {
			k := k
			allowed := shrunkMean(oppW, lf[k], func(g *teamGame) float64 { return g.Against[k] })
			r.S[k][statIdx[k]["opp_allow"]] = logRatio(allowed, lf[k])
			// The same for production against this player's position group, over a longer window.
			posW := lastN(onlyWithOpp(oppG), 30)
			posAllowed := shrunkMean(posW, lpos[pg][k], func(g *teamGame) float64 { return g.PosAgainst[pg][k] })
			r.S[k][statIdx[k]["pos5_allow"]] = logRatio(posAllowed, lpos[pg][k])
		}
	}
	oppB2B := 0.0
	if n := len(oppG); n > 0 && day-oppG[n-1].Day == 1 {
		oppB2B = 1
	}
	if opp != 0 {
		setC("opp_b2b", oppB2B)
	}

	// Availability: team games played since this player's last game. Zero across the offseason
	// (nobody played), so it separates a real injury return from a normal season opener.
	lastDay := hist[len(hist)-1].Day
	missed := 0.0
	for i := len(own) - 1; i >= 0 && own[i].Day > lastDay; i-- {
		missed++
	}
	missed = math.Min(missed, 30)
	setC("missed", missed)
	returning := 0.0
	if missed >= 8 {
		returning = 1
	}
	setC("returning", returning)

	// Teammates out.
	var missMin, missStarters float64
	var missS [data.NStats]float64
	for _, a := range abs {
		missMin += a.min
		if a.min >= starterMins {
			missStarters++
		}
		for k := range missS {
			missS[k] += a.prod[k]
		}
	}
	setC("miss_min", missMin)
	setC("miss_starters", missStarters)

	// Baseline rates, form, and usage trends per stat.
	for _, s := range data.Stats {
		k := data.StatIndex(s)
		if pj.ProjMinutes > 0 && pj.Mean[s] > 0 {
			r.S[k][statIdx[k]["base_rate"]] = math.Log(pj.Mean[s] / pj.ProjMinutes)
		}
		r.S[k][statIdx[k]["miss_s"]] = missS[k]

		// Form: last 10 games per-minute rate vs the long run, shrunk by 40 pseudo-minutes.
		var s10, m10, sl, ml float64
		for _, g := range tailGames(hist, 10) {
			s10 += g.Value(s)
			m10 += g.Minutes
		}
		for _, g := range tailGames(hist, 120) {
			sl += g.Value(s)
			ml += g.Minutes
		}
		if ml > 0 && sl > 0 {
			rl := sl / ml
			r.S[k][statIdx[k]["form_tr"]] = math.Log(((s10 + 40*rl) / (m10 + 40)) / rl)

			// Against this opponent: up to his last 8 meetings, shrunk by 200 pseudo-minutes
			// because a few games against one team are mostly noise.
			if opp != 0 {
				var sh, mh float64
				meetings := 0
				for i := len(hist) - 1; i >= 0 && meetings < 8; i-- {
					if oppOf(hist[i]) == opp {
						sh += hist[i].Value(s)
						mh += hist[i].Minutes
						meetings++
					}
				}
				if meetings > 0 {
					r.S[k][statIdx[k]["h2h"]] = math.Log(((sh + 200*rl) / (mh + 200)) / rl)
				}
			}
			// Same venue (home or away) over his last 60 games, shrunk the same way.
			if !math.IsNaN(home) {
				var sv, mv float64
				for _, g := range tailGames(hist, 60) {
					if h, ok := isHomeGame(g); ok && h == (home == 1) {
						sv += g.Value(s)
						mv += g.Minutes
					}
				}
				if mv > 0 {
					r.S[k][statIdx[k]["venue_split"]] = math.Log(((sv + 200*rl) / (mv + 200)) / rl)
				}
			}
		}
	}

	b.shooting(r, hist)
	b.threePointers(r, hist)
	b.advanced(r, hist)
	b.chances(r, hist)
	return r
}

func onlyWithOpp(list []*teamGame) []*teamGame {
	out := make([]*teamGame, 0, len(list))
	for _, g := range list {
		if g.HasOpp {
			out = append(out, g)
		}
	}
	return out
}

// shooting fills the points-only volume and efficiency features: how a player's recent shot
// volume and shooting percentage compare with their own norm.
func (b *Builder) shooting(r *Row, hist []data.PlayerGame) {
	k := data.StatIndex(data.Points)
	var f10, m10, g10, fl, ml, gl float64
	for _, g := range tailGames(hist, 10) {
		f10 += g.Fga
		g10 += g.Fgm
		m10 += g.Minutes
	}
	for _, g := range tailGames(hist, 60) {
		fl += g.Fga
		gl += g.Fgm
		ml += g.Minutes
	}
	if fl < 150 || ml <= 0 {
		return // no shot data in this feed, or too little of it
	}
	rate := fl / ml
	r.S[k][statIdx[k]["fga_tr"]] = math.Log(((f10 + 40*rate) / (m10 + 40)) / rate)
	if f10 >= 40 {
		r.S[k][statIdx[k]["fg_dev"]] = g10/f10 - gl/fl
	}
}

// threePointers fills the threes-only features: how his three-point attempt volume and his
// three-point percentage over the last 10 games compare with his own norm. Attempts are the
// stable part (they follow his role); the percentage is mostly luck, so it is kept as a
// deviation the model can shrink.
func (b *Builder) threePointers(r *Row, hist []data.PlayerGame) {
	k := data.StatIndex(data.Threes)
	var a10, m10, x10, al, ml, xl float64
	for _, g := range tailGames(hist, 10) {
		a10 += g.Fg3a
		x10 += g.Fg3m
		m10 += g.Minutes
	}
	for _, g := range tailGames(hist, 60) {
		al += g.Fg3a
		xl += g.Fg3m
		ml += g.Minutes
	}
	if al < 60 || ml <= 0 {
		return // no three-point data in this feed, or too little of it
	}
	rate := al / ml
	r.S[k][statIdx[k]["tpa_tr"]] = math.Log(((a10 + 40*rate) / (m10 + 40)) / rate)
	if a10 >= 20 {
		r.S[k][statIdx[k]["tp_dev"]] = x10/a10 - xl/al
	}
}

// advanced fills the usage-style trends from the advanced stats feed, when it is available.
func (b *Builder) advanced(r *Row, hist []data.PlayerGame) {
	if len(b.adv) == 0 {
		return
	}
	type acc struct{ sum, n float64 }
	var u10, u40, a10, a40, r10, r40 acc
	var p10, pm10, p40, pm40 float64
	add := func(a *acc, v float64) {
		if !math.IsNaN(v) {
			a.sum += v
			a.n++
		}
	}
	recent := tailGames(hist, 40)
	for i, g := range recent {
		row, ok := b.adv[data.AdvKey{Player: g.PlayerID, Game: g.GameID}]
		if !ok {
			continue
		}
		isRecent := i >= len(recent)-10
		add(&u40, row.Usage)
		add(&a40, row.AstPct)
		add(&r40, row.RebPct)
		if !math.IsNaN(row.Passes) {
			p40 += row.Passes
			pm40 += g.Minutes
		}
		if isRecent {
			add(&u10, row.Usage)
			add(&a10, row.AstPct)
			add(&r10, row.RebPct)
			if !math.IsNaN(row.Passes) {
				p10 += row.Passes
				pm10 += g.Minutes
			}
		}
	}
	trend := func(short, long acc) float64 {
		if short.n < 4 || long.n < 15 {
			return math.NaN()
		}
		return short.sum/short.n - long.sum/long.n
	}
	pts, reb, ast := data.StatIndex(data.Points), data.StatIndex(data.Rebounds), data.StatIndex(data.Assists)
	ut := trend(u10, u40)
	r.S[pts][statIdx[pts]["usage_tr"]] = ut
	r.S[ast][statIdx[ast]["usage_tr"]] = ut
	if u40.n >= 15 {
		r.S[pts][statIdx[pts]["usage_lvl"]] = u40.sum / u40.n
	}
	r.S[reb][statIdx[reb]["reb_tr"]] = trend(r10, r40)
	r.S[ast][statIdx[ast]["ast_tr"]] = trend(a10, a40)
	if pm10 >= 100 && pm40 >= 300 && p40 > 0 {
		rl := p40 / pm40
		r.S[ast][statIdx[ast]["pass_tr"]] = math.Log(((p10 + 40*rl) / (pm10 + 40)) / rl)
	}
}

// ratio accumulates a numerator and a denominator over a player's last 10 and last 40 games, so
// a recent rate can be compared with his own longer-run rate.
type ratio struct{ n10, d10, n40, d40 float64 }

// add counts one game. Games missing either number are skipped entirely.
func (q *ratio) add(recent bool, num, den float64) {
	if math.IsNaN(num) || math.IsNaN(den) || den <= 0 {
		return
	}
	q.n40 += num
	q.d40 += den
	if recent {
		q.n10 += num
		q.d10 += den
	}
}

// trend is log(recent rate / long-run rate), with the recent rate pulled toward the long-run
// rate by k pseudo-units of denominator. It is NaN until both windows hold enough data.
func (q ratio) trend(k, min10, min40 float64) float64 {
	if q.d10 < min10 || q.d40 < min40 || q.n40 <= 0 {
		return math.NaN()
	}
	rl := q.n40 / q.d40
	return math.Log(((q.n10 + k*rl) / (q.d10 + k)) / rl)
}

// chances fills the opportunity features from player tracking: how many rebound chances, passes
// and touches a player has been getting per minute, and how well he has been converting them.
// A rebound total can fall because chances dried up (a real change in role) or because the
// ball just did not come to him (which tends to correct), and the model needs to tell those apart.
func (b *Builder) chances(r *Row, hist []data.PlayerGame) {
	if len(b.adv) == 0 {
		return
	}
	var chPerMin, conv, offShare, passConv, touchPerMin, secPerMin ratio
	recent := tailGames(hist, 40)
	for i, g := range recent {
		row, ok := b.adv[data.AdvKey{Player: g.PlayerID, Game: g.GameID}]
		if !ok {
			continue
		}
		isRecent := i >= len(recent)-10
		chPerMin.add(isRecent, row.RebCh, g.Minutes)
		conv.add(isRecent, g.Value(data.Rebounds), row.RebCh)
		offShare.add(isRecent, row.RebChOff, row.RebCh)
		passConv.add(isRecent, g.Value(data.Assists), row.Passes)
		touchPerMin.add(isRecent, row.Touches, g.Minutes)
		if !math.IsNaN(row.SecAst) {
			ft := row.FtAst
			if math.IsNaN(ft) {
				ft = 0
			}
			secPerMin.add(isRecent, row.SecAst+ft, g.Minutes)
		}
	}
	pts, reb, ast := data.StatIndex(data.Points), data.StatIndex(data.Rebounds), data.StatIndex(data.Assists)
	r.S[reb][statIdx[reb]["rch_tr"]] = chPerMin.trend(40, 100, 300)
	r.S[reb][statIdx[reb]["rch_conv"]] = conv.trend(60, 60, 200)
	r.S[reb][statIdx[reb]["rch_off"]] = offShare.trend(60, 60, 200)
	r.S[ast][statIdx[ast]["pass_conv"]] = passConv.trend(150, 150, 500)
	tp := touchPerMin.trend(40, 100, 300)
	r.S[ast][statIdx[ast]["touch_tr"]] = tp
	r.S[pts][statIdx[pts]["touch_tr"]] = tp
	r.S[ast][statIdx[ast]["sec_tr"]] = secPerMin.trend(40, 100, 300)
}

// LastTeam returns the team a player most recently played for.
func (b *Builder) LastTeam(pid int) (int, bool) {
	g := b.byPlayer[pid]
	if len(g) == 0 {
		return 0, false
	}
	return g[len(g)-1].TeamID, true
}

// PlayerIDs lists every player with history.
func (b *Builder) PlayerIDs() []int {
	out := make([]int, 0, len(b.byPlayer))
	for pid := range b.byPlayer {
		out = append(out, pid)
	}
	sort.Ints(out)
	return out
}

// LastDay is the day number of a player's most recent game (0 if none).
func (b *Builder) LastDay(pid int) int {
	g := b.byPlayer[pid]
	if len(g) == 0 {
		return 0
	}
	return g[len(g)-1].Day
}

// FeatureIndex returns the position of a named feature in Row.X and in Row.S[stat].
func FeatureIndex(name string, stat data.Stat) (common, perStat int) {
	common, perStat = -1, -1
	if i, ok := commonIdx[name]; ok {
		common = i
	}
	if i, ok := statIdx[data.StatIndex(stat)][name]; ok {
		perStat = i
	}
	return
}
