package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/availability"
	"propsmodel/internal/bets"
	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/gamecontext"
	"propsmodel/internal/lineup"
	"propsmodel/internal/model"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"propsmodel/internal/sgo"
	"sort"
	"strings"
	"time"
)

type rotationForecast struct {
	learnedOwn  bool
	learnedOpp  bool
	strict      bool
	fallback    bool
	contextual  bool
	gameMinutes *gamecontext.Model
	primary     *predictor.Model
	pregame     map[int]opportunity.Projected
	roleModel   *lineup.RoleModel
	positions   map[int]string
	impacts     *lineup.Index
	impactCache map[int][]lineup.Impact
	roster      map[int]int
	opponents   map[int]int
	date        string
	teams       map[int]rotation.Allocation
	scenarios   map[int]map[int]rotation.Allocation
	statuses    map[int]availability.Status
}

// Numerical rotation changes require fresh timestamped reports. The existing
// V1 ESPN merge is left untouched; this path additionally checks capture time,
// an unambiguous name match and current team membership.
func freshRotationStatuses(injuryPath, snapDir, date string, now time.Time, b *feat.Builder, roster map[int]int, games map[int]props.GameInfo) map[int]availability.Status {
	statuses := availability.FreshStatuses(injuryPath, now, 30*time.Minute)
	for pid, st := range statuses {
		if st.Name == "" {
			st.Name = b.Index().Name(pid)
		}
		if strings.Contains(strings.ToLower(st.Status), "conflicting") {
			st.Status = "Questionable (conflicting reports)"
		}
		statuses[pid] = st
	}
	body, e := os.ReadFile(filepath.Join(snapDir, "espn_injuries_"+date+".json"))
	if e != nil {
		return statuses
	}
	var meta struct {
		Timestamp string `json:"timestamp"`
		Status    string `json:"status"`
	}
	if json.Unmarshal(body, &meta) != nil || meta.Status != "success" {
		return statuses
	}
	at, e := time.Parse(time.RFC3339Nano, meta.Timestamp)
	if e != nil || at.After(now) || now.Sub(at) > 30*time.Minute {
		return statuses
	}
	names := map[string][]int{}
	for _, pid := range b.PlayerIDs() {
		if roster[pid] > 0 {
			n := sgo.NormName(b.Index().Name(pid))
			names[n] = append(names[n], pid)
		}
	}
	teams := map[int]string{}
	for _, g := range games {
		teams[g.Home] = g.HomeAbbr
		teams[g.Visitor] = g.VisitorAbbr
	}
	for _, inj := range loadESPN(snapDir, date) {
		ids := names[sgo.NormName(inj.Name)]
		if len(ids) != 1 {
			continue
		}
		pid := ids[0]
		if teams[roster[pid]] == "" || teams[roster[pid]] != inj.Team {
			continue
		}
		st := ""
		if inj.IsOut() {
			st = "Out"
		} else if inj.IsGameTime() {
			st = "Questionable"
		}
		if st == "" {
			continue
		}
		old, exists := statuses[pid]
		if exists {
			prev, err := time.Parse(time.RFC3339Nano, old.At)
			if err == nil && prev.After(at) {
				continue
			}
			if err == nil && prev.Equal(at) && !strings.EqualFold(old.Status, st) {
				st = "Questionable (conflicting reports)"
			}
		}
		statuses[pid] = availability.Status{PlayerID: pid, Name: inj.Name, Status: st, At: meta.Timestamp}
	}
	return statuses
}

func newRotationForecast(rows []data.PlayerGame, roster map[int]int, fresh map[int]availability.Status, statuses map[int]string, games map[int]props.GameInfo, date string, strict ...bool) *rotationForecast {
	f := &rotationForecast{teams: map[int]rotation.Allocation{}, scenarios: map[int]map[int]rotation.Allocation{}, statuses: fresh}
	f.strict = len(strict) > 0 && strict[0]
	if f.strict {
		f.impacts = lineup.New(rows)
		f.impactCache = map[int][]lineup.Impact{}
		f.roster = roster
		f.date = date
		f.opponents = map[int]int{}
		for _, g := range games {
			f.opponents[g.Home] = g.Visitor
			f.opponents[g.Visitor] = g.Home
		}
	}
	ix := rotation.New(rows)
	out := map[int]bool{}
	for pid, s := range fresh {
		if strings.EqualFold(strings.TrimSpace(s.Status), "out") {
			out[pid] = true
		}
	}
	for _, g := range games {
		for _, team := range []int{g.Home, g.Visitor} {
			a := ix.Allocate(team, date, roster, out)
			// A known absence from a stale or conflicting source must not silently
			// become a healthy player in the rotation calculation.
			for pid, s := range statuses {
				if roster[pid] == team && strings.Contains(strings.ToLower(s), "out") && !out[pid] {
					a.Valid = false
					a.Supported = false
					a.Reason = "known absence lacks a fresh, unambiguous confirmed-out report"
				}
			}
			f.teams[team] = a
			f.scenarios[team] = map[int]rotation.Allocation{}
			for pid, s := range fresh {
				if roster[pid] != team || !props.IsUncertain(s.Status) {
					continue
				}
				scenario := map[int]bool{}
				for p, v := range out {
					scenario[p] = v
				}
				scenario[pid] = true
				f.scenarios[team][pid] = ix.Allocate(team, date, roster, scenario)
			}
		}
	}
	return f
}
func (f *rotationForecast) policy() string {
	if f.contextual {
		return rotation.ContextPolicy
	}
	if f.fallback {
		return rotation.FallbackPolicy
	}
	if f.strict {
		return rotation.StrictPolicy
	}
	return rotation.Policy
}
func (f *rotationForecast) target(a rotation.Allocation, pid int) (float64, bool) {
	if f.fallback {
		if !a.Valid || len(a.RelevantOut) == 0 || (!a.RoleProjected && a.MatchedGames < 3) {
			return 0, false
		}
		for _, p := range a.Out {
			if p == pid {
				return 0, false
			}
		}
		target, ok := a.Minutes[pid]
		return target, ok && target > 0 && target <= 48
	}
	if f.strict {
		return rotation.SupportedTarget(a, pid)
	}
	target, ok := a.Minutes[pid]
	return target, a.Valid && ok
}

// Avoid stacking research rates on an already fitted absence-feature model.
func (f *rotationForecast) configureProduction(m *predictor.Model) {
	f.primary = m
	if !f.strict {
		return
	}
	ownDisabled, oppDisabled := false, false
	for _, group := range m.Disabled {
		ownDisabled = ownDisabled || group == feat.GroupTeammates
		oppDisabled = oppDisabled || group == feat.GroupOppOut
	}
	f.learnedOwn = !ownDisabled
	f.learnedOpp = !oppDisabled
	f.impactCache = map[int][]lineup.Impact{}
}
func (f *rotationForecast) estimateImpact(team, player int, date string, roster map[int]int, outs []int, opponent bool) lineup.Impact {
	if f.fallback {
		a := f.teams[team]
		if a.Valid {
			return f.impacts.RoleImpact(f.roleModel, team, player, date, roster, outs, opponent, a.Baseline, a.Minutes)
		}
	}
	return f.impacts.Estimate(team, player, date, roster, outs, opponent)
}
func (f *rotationForecast) rateImpacts(row *feat.Row) []lineup.Impact {
	if !f.strict || f.impacts == nil {
		return nil
	}
	if found, ok := f.impactCache[row.G.PlayerID]; ok {
		return found
	}
	result := []lineup.Impact{}
	team := row.G.TeamID
	if a := f.teams[team]; a.Valid {
		result = append(result, f.estimateImpact(team, row.G.PlayerID, f.date, f.roster, a.Out, false))
	}
	opp := f.opponents[team]
	if a := f.teams[opp]; a.Valid {
		result = append(result, f.estimateImpact(opp, row.G.PlayerID, f.date, f.roster, a.Out, true))
	}
	if f.contextual {
		result = append(result, f.impacts.OpportunityImpact(f.roleModel, row.G.PlayerID, f.date))
	}
	for i := range result {
		if (result[i].Kind == "teammate-production" && f.learnedOwn) || (result[i].Kind == "opponent-allowance" && f.learnedOpp) {
			result[i].Reason = "research rates disabled: model already has learned absence features"
			for j := range result[i].Effects {
				e := &result[i].Effects[j]
				e.Applied = false
				e.Scale = 1
				e.Reason = result[i].Reason
			}
		}
		for j := range result[i].Missing {
			p := &result[i].Missing[j]
			st := f.statuses[p.PlayerID]
			p.Status = st.Status
			p.At = st.At
		}
	}
	f.impactCache[row.G.PlayerID] = result
	return result
}
func (f *rotationForecast) adjustments(row *feat.Row) predictor.Adjustments {
	a := predictor.Adjustments{}
	if target, ok := f.target(f.teams[row.G.TeamID], row.G.PlayerID); ok {
		a.TargetMinutes = &target
	}
	if f.strict {
		a.Scale = f.rateScale(f.rateImpacts(row))
	}
	if f.contextual && f.primary != nil && f.gameMinutes != nil {
		e := f.gameEstimate(row)
		if e.Applied {
			target := f.primary.Forecast(row, predictor.Adjustments{}).Minutes
			if a.TargetMinutes != nil {
				target = *a.TargetMinutes
			}
			target = math.Max(1, math.Min(48, target+e.Shift))
			a.TargetMinutes = &target
		}
	}
	return a
}
func (f *rotationForecast) adjust(p predictor.Pred, row *feat.Row) predictor.Pred {
	a := f.adjustments(row)
	if a.TargetMinutes != nil {
		p = p.WithTargetMinutes(*a.TargetMinutes)
	}
	for k, st := range data.Stats {
		if a.Scale[k] > 0 {
			p = p.Scaled(st, a.Scale[k])
		}
	}
	return p
}
func (f *rotationForecast) context(s scored, base predictor.Pred, row *feat.Row, market, weight float64, corr combo.Corr) *rotation.Context {
	a := f.teams[row.G.TeamID]
	c := &rotation.Context{OwnStatus: s.Maybe, Policy: f.policy(), RelevantOut: append([]int(nil), a.RelevantOut...), BaselineMinutes: base.Minutes, AllocationMinutes: base.Minutes, UsedMinutes: s.ProjMin, TeamGames: a.TeamGames, MatchedGames: a.MatchedGames, Supported: a.Valid && a.Supported, Reason: a.Reason}
	if f.fallback {
		c.OwnReplacements = f.replacements(row.G.TeamID)
		c.OpponentReplacements = f.replacements(f.opponents[row.G.TeamID])
	}
	if f.contextual {
		c.Opportunities = f.impacts.SignalsFor(row.G.PlayerID, f.date)
		c.TrackingProfile = f.impacts.StylesFor(row.G.PlayerID, f.date)
		e := f.gameEstimate(row)
		c.GameContext = &e
		c.WorkloadScenarios = f.workloadScenarios(s, base, row, market, weight)
		for _, team := range []int{row.G.TeamID, f.opponents[row.G.TeamID]} {
			if p, ok := f.pregame[team]; ok {
				c.PregameLineups = append(c.PregameLineups, p)
			}
		}
	}
	c.Production = f.rateImpacts(row)
	c.RateScale = f.rateScale(c.Production)
	_, c.AllocationApplied = f.target(a, s.PlayerID)
	if f.strict && !c.AllocationApplied {
		c.Supported = false
		c.Reason = "baseline minutes retained: no supported joint-absence target; " + a.Reason
		if f.fallback {
			c.Reason = "baseline minutes retained: no usable projected replacement target; " + a.Reason
		}
	}
	if a.Valid {
		if target, ok := a.Minutes[s.PlayerID]; ok {
			c.AllocationMinutes = target
			for _, m := range a.Minutes {
				c.TeamMinutes += m
			}
		} else {
			c.Supported = false
			c.Reason = "player has no observed current-team rotation workload"
		}
	}
	for _, pid := range a.Out {
		st := f.statuses[pid]
		c.ConfirmedOut = append(c.ConfirmedOut, rotation.Status{PlayerID: pid, Player: st.Name, Status: st.Status, At: st.At})
	}
	if !a.Valid {
		return c
	}
	ids := []int{}
	for pid := range f.scenarios[row.G.TeamID] {
		if pid != s.PlayerID {
			ids = append(ids, pid)
		}
	}
	sort.Ints(ids)
	for _, pid := range ids {
		alt := f.scenarios[row.G.TeamID][pid]
		if !alt.Valid {
			continue
		}
		target, ok := alt.Minutes[s.PlayerID]
		if !ok {
			continue
		}
		st := f.statuses[pid]
		p := base
		if f.strict {
			ownImpact := f.estimateImpact(row.G.TeamID, s.PlayerID, f.date, f.roster, alt.Out, false)
			if f.fallback {
				ownImpact = f.impacts.RoleImpact(f.roleModel, row.G.TeamID, s.PlayerID, f.date, f.roster, alt.Out, false, alt.Baseline, alt.Minutes)
			}
			impacts := []lineup.Impact{ownImpact}
			for _, impact := range c.Production {
				if impact.Kind == "opponent-allowance" || (f.contextual && impact.Kind == "opportunity-production") {
					impacts = append(impacts, impact)
				}
			}
			if f.learnedOwn {
				impacts = impacts[1:]
			}
			for k, scale := range f.rateScale(impacts) {
				p = p.Scaled(data.Stats[k], scale)
			}
		}
		_, applied := f.target(alt, s.PlayerID)
		if applied {
			p = p.WithTargetMinutes(target)
		}
		if f.contextual {
			if e := f.gameEstimate(row); e.Applied {
				p = p.WithTargetMinutes(math.Max(1, math.Min(48, p.Minutes+e.Shift)))
			}
		}
		scenario := rotation.Scenario{PlayerID: pid, Player: st.Name, Status: st.Status, Minutes: p.Minutes, MatchedGames: alt.MatchedGames, Supported: alt.Supported && (!f.strict || applied)}
		projection := 0.
		var raw float64
		var err error
		if s.Stat.IsCombo() {
			for _, part := range s.Stat.Parts() {
				projection += p.Mean(part)
			}
			pr, e := combo.Simulate(p, corr, 2000, int64(s.PlayerID)*1000003+int64(s.GameID)).ProbOver(s.Stat, s.Line)
			raw, err = pr.Over, e
		} else {
			projection = p.Mean(s.Stat)
			pr, e := p.ProbOver(s.Stat, s.Line)
			raw, err = pr.Over, e
		}
		scenario.Projection = &projection
		if err == nil {
			prob := blend(raw, market, weight)
			if s.Side == "UNDER" {
				prob = 1 - prob
			}
			scenario.Probability = &prob
		}
		c.Scenarios = append(c.Scenarios, scenario)
	}
	return c
}
func (f *rotationForecast) decorate(s scored, base predictor.Pred, row *feat.Row, market, weight float64, corr combo.Corr) scored {
	s.Availability = nil
	s.Rotation = f.context(s, base, row, market, weight, corr)
	s.Why = addWhy(s.Why, fmt.Sprintf("V2 rotation: %.1f baseline minutes → %.1f used; %d earlier complete team games, %d joint-absence matches; %s; bounded production adjustments shown separately", s.Rotation.BaselineMinutes, s.Rotation.UsedMinutes, s.Rotation.TeamGames, s.Rotation.MatchedGames, s.Rotation.Reason))
	for _, impact := range s.Rotation.Production {
		for _, e := range impact.Effects {
			relevant := e.Stat == s.Stat
			for _, part := range s.Stat.Parts() {
				relevant = relevant || part == e.Stat
			}
			if relevant && e.Applied {
				if e.Method == "pooled-opportunity" && e.Role != nil {
					s.Why = addWhy(s.Why, fmt.Sprintf("V2 opportunity component: %s rate %+.1f%% from earlier chances and conversion; experimental", e.Stat, (e.Scale-1)*100))
				} else if e.Method == "pooled-lineup" && e.Role != nil {
					s.Why = addWhy(s.Why, fmt.Sprintf("V2 %s: %s rate %+.1f%% (league-pooled projected rotation; %d training player-games / %d dates); experimental", impact.Kind, e.Stat, (e.Scale-1)*100, e.Role.Games, e.Role.Days))
				} else if e.Method == "individual-mixture" {
					s.Why = addWhy(s.Why, fmt.Sprintf("V2 %s: %s rate %+.1f%% (individual-absence mixture; %d supported components; no stacking); experimental", impact.Kind, e.Stat, (e.Scale-1)*100, len(e.Evidence)))
				} else {
					s.Why = addWhy(s.Why, fmt.Sprintf("V2 %s: %s rate %+.1f%% (%d joint-out / %d present games); experimental", impact.Kind, e.Stat, (e.Scale-1)*100, e.Without, e.With))
				}
			}
		}
	}
	return s
}
func quoteMarket(l props.Line) float64 { return bets.MarketOver(l.OverOdds, l.UnderOdds) }

// A separate scoring loop matters: a bench player can be ineligible for V1's
// old workload but become eligible under the new team rotation.
func scoreRotationSingles(f *rotationForecast, m *predictor.Model, b *feat.Builder, lines []props.Line, games map[int]props.GameInfo, teamOf func(int) (int, bool), rowFor func(int, props.GameInfo, int) *feat.Row, statuses map[int]string, date string, weight, maxGap float64, asOf time.Time) ([]scored, []replay.Record) {
	views := props.MarketViews(lines)
	out := []scored{}
	records := map[marketKey]replay.Record{}
	confirmed := map[int]string{}
	for pid, s := range f.statuses {
		if strings.EqualFold(strings.TrimSpace(s.Status), "out") {
			confirmed[pid] = s.Name
		}
	}
	at := asOf.UTC().Format(time.RFC3339Nano)
	for _, l := range lines {
		if l.Stat.IsCombo() {
			continue
		}
		g, ok := games[l.GameID]
		if !ok || l.OverOdds == 0 || l.UnderOdds == 0 {
			continue
		}
		if _, out := confirmed[l.PlayerID]; out {
			continue
		}
		if strings.Contains(strings.ToLower(statuses[l.PlayerID]), "out") {
			continue
		}
		team, ok := teamOf(l.PlayerID)
		if !ok || (team != g.Home && team != g.Visitor) {
			continue
		}
		row := rowFor(l.PlayerID, g, team)
		if row == nil || row.Games < 3 {
			continue
		}
		base := m.Forecast(row, predictor.Adjustments{})
		p := f.adjust(base, row)
		if !model.Eligible(row.AvgMin15, p.Minutes) {
			continue
		}
		raw, err := p.ProbOver(l.Stat, l.Line)
		if err != nil {
			continue
		}
		market := quoteMarket(l)
		pOver := blend(raw.Over, market, weight)
		side, prob, rp, odds := "OVER", pOver, raw.Over, l.OverOdds
		if pOver < .5 {
			side, prob, rp, odds = "UNDER", 1-pOver, 1-raw.Over, l.UnderOdds
		}
		tm := g.HomeAbbr
		opp := g.Visitor
		if team == g.Visitor {
			tm = g.VisitorAbbr
			opp = g.Home
		}
		v := views[props.MarketKey{Game: l.GameID, Player: l.PlayerID, Stat: string(l.Stat)}]
		s := scored{PlayerID: l.PlayerID, GameID: l.GameID, Player: b.Index().Name(l.PlayerID), Team: tm, Matchup: g.VisitorAbbr + " @ " + g.HomeAbbr, Stat: l.Stat, Side: side, Line: l.Line, Prob: prob, ProbRaw: rp, Odds: odds, Vendor: l.Vendor, ProjMean: p.Mean(l.Stat), ProjMin: p.Minutes, Edge: bets.Edge(prob, odds), Tier: "PAPER", Tip: g.Start, QuoteAt: l.CapturedAt, SourceUpdatedAt: l.UpdatedAt, AuditVersion: props.QuoteVersion, MarketMedian: v.Median, BookCount: v.Books, BookQuotes: v.Quotes, Books: v.Books, CaptureMaxAgeSeconds: 1800, PriceMaxAgeSeconds: 21600, Dist: p.Dist(l.Stat, distMax(l.Stat, l.Line, p.Mean(l.Stat))), Shift: p.Minutes - row.TypicalMinutes()}
		if len(v.Quotes) > 0 {
			lo, hi := v.Quotes[0].Line, v.Quotes[0].Line
			for _, q := range v.Quotes {
				lo = math.Min(lo, q.Line)
				hi = math.Max(hi, q.Line)
			}
			s.Spread = hi - lo
		}
		s.Mkt = market
		if side == "UNDER" {
			s.Mkt = 1 - market
		}
		if props.IsUncertain(statuses[l.PlayerID]) {
			s.Maybe = statuses[l.PlayerID]
		}
		if st, ok := f.statuses[l.PlayerID]; ok && props.IsUncertain(st.Status) {
			s.Maybe = st.Status
		}
		if maxGap > 0 && math.Abs(s.Line-s.ProjMean)/math.Max(s.ProjMean, 2) > maxGap {
			s.Far = true
		}
		s.Why = why(b, b.Index(), row, l.PlayerID, l.Stat, p, team, opp, confirmed, date)
		s.Why = addWhy(s.Why, fmt.Sprintf("V2 rotation experiment: current model per-minute rates; model/book blend %.0f/%.0f; no fitted probability calibration for this new policy", 100*weight, 100*(1-weight)))
		s = f.decorate(s, base, row, market, weight, combo.Corr{})
		out = append(out, s)
		mk := marketKey{l.PlayerID, data.StatIndex(l.Stat), l.GameID}
		old, exists := records[mk]
		if !exists || math.Abs(l.Line-v.Median) < math.Abs(old.Line-v.Median) {
			policy := f.adjustments(row)
			records[mk] = replay.Record{Rotation: s.Rotation, SelectionPolicy: f.policy(), ProbabilityMethod: "linear", BlendWeight: weight, Period: "game", BookCount: v.Books, Date: date, At: at, Tip: g.Start, Row: decisionIdentity(row, l.GameID, date, s.Player, tm), Policy: policy, Stat: l.Stat, Line: l.Line, Book: l.Vendor, QuoteAt: l.CapturedAt, UpdatedAt: l.UpdatedAt, OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, Market: market, P1: pOver}
		}
	}
	rs := []replay.Record{}
	for _, r := range records {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.Row.G.GameID != b.Row.G.GameID {
			return a.Row.G.GameID < b.Row.G.GameID
		}
		if a.Row.G.PlayerID != b.Row.G.PlayerID {
			return a.Row.G.PlayerID < b.Row.G.PlayerID
		}
		return a.Stat < b.Stat
	})
	return out, rs
}
func writeRotationEvidence(dir, date string, at time.Time, f *rotationForecast) error {
	root := filepath.Join(dir, rotation.EvidenceDirectory(f.policy()))
	if e := os.MkdirAll(root, 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(struct {
		Policy, Date, At string
		Teams            map[int]rotation.Allocation
		Scenarios        map[int]map[int]rotation.Allocation
	}{f.policy(), date, at.Format(time.RFC3339Nano), f.teams, f.scenarios}, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "rotation_"+date+".json"), b, 0644)
}

// Configure the candidate once per run. The historical fit ends strictly before
// the forecast date and is reused across all quotes and uncertain-out scenarios.
func (f *rotationForecast) configureRoles(rows []data.PlayerGame) {
	if !f.fallback {
		return
	}
	f.impacts = lineup.NewRoles(rows)
	f.roleModel = f.impacts.FitRoles(f.date)
	f.positions = map[int]string{}
	latest := map[int]string{}
	for _, r := range rows {
		if r.Date < f.date && r.Minutes > 0 && r.Date >= latest[r.PlayerID] && f.roster[r.PlayerID] == r.TeamID {
			f.positions[r.PlayerID] = lineup.Position(r.Pos)
			latest[r.PlayerID] = r.Date
		}
	}
	for team, a := range f.teams {
		f.teams[team] = rotation.ProjectRoles(a, f.positions)
	}
	for team, ss := range f.scenarios {
		for pid, a := range ss {
			f.scenarios[team][pid] = rotation.ProjectRoles(a, f.positions)
		}
	}
	f.impactCache = map[int][]lineup.Impact{}
}

func (f *rotationForecast) cloneForRoles() *rotationForecast {
	shadow := *f
	shadow.teams = map[int]rotation.Allocation{}
	for team, a := range f.teams {
		shadow.teams[team] = a
	}
	shadow.scenarios = map[int]map[int]rotation.Allocation{}
	for team, ss := range f.scenarios {
		shadow.scenarios[team] = map[int]rotation.Allocation{}
		for pid, a := range ss {
			shadow.scenarios[team][pid] = a
		}
	}
	shadow.impactCache = map[int][]lineup.Impact{}
	return &shadow
}

func (f *rotationForecast) replacements(team int) []rotation.Replacement {
	a := f.teams[team]
	if !a.Valid {
		return nil
	}
	var out []rotation.Replacement
	for pid, baseline := range a.Baseline {
		used := a.Minutes[pid]
		if math.Abs(used-baseline) < .25 {
			continue
		}
		name := ""
		for _, p := range f.impacts.Profiles(team, f.date, f.roster, []int{pid}) {
			name = p.Player
		}
		out = append(out, rotation.Replacement{PlayerID: pid, Player: name, Position: f.positions[pid], Baseline: baseline, Projected: used})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Projected-out[i].Baseline != out[j].Projected-out[j].Baseline {
			return out[i].Projected-out[i].Baseline > out[j].Projected-out[j].Baseline
		}
		return out[i].PlayerID < out[j].PlayerID
	})
	return out
}

func (f *rotationForecast) rateScale(impacts []lineup.Impact) [data.NStats]float64 {
	if f.contextual {
		return lineup.ScaleContext(impacts)
	}
	return lineup.Scale(impacts)
}
func (f *rotationForecast) configureContext(dir string, rows []data.PlayerGame, games map[int]props.GameInfo, asof time.Time) error {
	if !f.contextual {
		return nil
	}
	op, e := opportunity.Load(dir, rows)
	if e != nil {
		return e
	}
	op.AsOf = asof
	f.roleModel = f.impacts.FitContext(f.date, op)
	f.gameMinutes = gamecontext.Fit(rows, pipeline.LoadGameLines(dir, rows), f.date)
	f.pregame, e = opportunity.LoadPregame(filepath.Join(dir, "snapshots", "projected_lineups_"+f.date+".jsonl"), games, f.roster, asof)
	if e != nil {
		return e
	}
	for team, p := range f.pregame {
		conflict := false
		for _, s := range p.Lineup {
			if st, ok := f.statuses[s.Player.ID]; ok && strings.EqualFold(strings.TrimSpace(st.Status), "out") {
				conflict = true
			}
		}
		if conflict {
			delete(f.pregame, team)
		}
	}
	for _, p := range f.pregame {
		for _, s := range p.Lineup {
			f.positions[s.Player.ID] = lineup.Position(s.Position)
		}
	}
	for team, a := range f.teams {
		f.teams[team] = rotation.ProjectRoles(a, f.positions)
	}
	for team, ss := range f.scenarios {
		for pid, a := range ss {
			f.scenarios[team][pid] = rotation.ProjectRoles(a, f.positions)
		}
	}
	f.impactCache = map[int][]lineup.Impact{}
	f.impacts.SetProjectedRoles(f.positions)
	fmt.Printf("Context opportunities loaded: %v; projected/confirmed pregame lineups: %d teams.\n", op.Coverage, len(f.pregame))
	return nil
}

func (f *rotationForecast) gameEstimate(row *feat.Row) gamecontext.Estimate {
	if f.gameMinutes == nil {
		return gamecontext.Estimate{Reason: "historical pregame spread model unavailable"}
	}
	signed := row.AbsSpread()
	ci, _ := feat.FeatureIndex("fav_spread", data.Points)
	if ci >= 0 && ci < len(row.X) {
		signed = row.X[ci]
	}
	e := f.gameMinutes.Estimate(f.date, signed, row.TypicalMinutes())
	if f.primary != nil && f.primary.Minutes != nil {
		for i, name := range f.primary.Minutes.Names {
			if (name == "abs_spread" || name == "fav_spread" || name == "star_spread") && i < len(f.primary.Minutes.Coef) && math.Abs(f.primary.Minutes.Coef[i]) > 1e-8 {
				e.Applied = false
				e.Shift = 0
				e.Reason = "primary minutes model already includes a fitted spread effect; extra adjustment suppressed"
				break
			}
		}
	}
	return e
}

// These are sensitivity scenarios from the existing minutes distribution, not
// independent probabilities of a blowout or confidence intervals on hit rate.
func (f *rotationForecast) workloadScenarios(s scored, base predictor.Pred, row *feat.Row, market, weight float64) []rotation.Scenario {
	if s.Stat.IsCombo() {
		return nil
	}
	p := f.adjust(base, row)
	type node struct{ minutes, weight float64 }
	var nodes []node
	for i, m := range p.MinNodes {
		nodes = append(nodes, node{m, p.NodeWeight(i)})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].minutes < nodes[j].minutes })
	quant := func(q float64) float64 {
		sum := 0.
		for _, n := range nodes {
			sum += n.weight
			if sum >= q {
				return n.minutes
			}
		}
		return p.Minutes
	}
	var out []rotation.Scenario
	for i, minutes := range []float64{quant(.1), quant(.9)} {
		label := "Lower minutes scenario"
		if i == 1 {
			label = "Higher minutes scenario"
		}
		trial := p.WithMinutes(minutes)
		prob, e := trial.ProbOver(s.Stat, s.Line)
		if e != nil {
			continue
		}
		value := blend(prob.Over, market, weight)
		if s.Side == "UNDER" {
			value = 1 - value
		}
		projection := trial.Mean(s.Stat)
		out = append(out, rotation.Scenario{Player: label, Minutes: trial.Minutes, Projection: &projection, Probability: &value})
	}
	return out
}
