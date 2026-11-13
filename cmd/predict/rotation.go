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
	"propsmodel/internal/model"
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
	teams     map[int]rotation.Allocation
	scenarios map[int]map[int]rotation.Allocation
	statuses  map[int]availability.Status
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

func newRotationForecast(rows []data.PlayerGame, roster map[int]int, fresh map[int]availability.Status, statuses map[int]string, games map[int]props.GameInfo, date string) *rotationForecast {
	f := &rotationForecast{teams: map[int]rotation.Allocation{}, scenarios: map[int]map[int]rotation.Allocation{}, statuses: fresh}
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
func (f *rotationForecast) adjust(p predictor.Pred, row *feat.Row) predictor.Pred {
	a := f.teams[row.G.TeamID]
	if target, ok := a.Minutes[row.G.PlayerID]; a.Valid && ok {
		return p.WithTargetMinutes(target)
	}
	return p
}
func (f *rotationForecast) context(s scored, base predictor.Pred, row *feat.Row, market, weight float64, corr combo.Corr) *rotation.Context {
	a := f.teams[row.G.TeamID]
	c := &rotation.Context{OwnStatus: s.Maybe, Policy: rotation.Policy, BaselineMinutes: base.Minutes, AllocationMinutes: base.Minutes, UsedMinutes: s.ProjMin, TeamGames: a.TeamGames, MatchedGames: a.MatchedGames, Supported: a.Valid && a.Supported, Reason: a.Reason}
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
		p := base.WithTargetMinutes(target)
		scenario := rotation.Scenario{PlayerID: pid, Player: st.Name, Status: st.Status, Minutes: p.Minutes, MatchedGames: alt.MatchedGames, Supported: alt.Supported}
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
	s.Why = addWhy(s.Why, fmt.Sprintf("V2 rotation: %.1f baseline minutes → %.1f used; %d earlier complete team games, %d joint-absence matches; %s; per-minute rates unchanged", s.Rotation.BaselineMinutes, s.Rotation.UsedMinutes, s.Rotation.TeamGames, s.Rotation.MatchedGames, s.Rotation.Reason))
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
			policy := predictor.Adjustments{}
			if a := f.teams[team]; a.Valid {
				if target, ok := a.Minutes[l.PlayerID]; ok {
					policy.TargetMinutes = &target
				}
			}
			records[mk] = replay.Record{Rotation: s.Rotation, SelectionPolicy: rotation.Policy, ProbabilityMethod: "linear", BlendWeight: weight, Period: "game", BookCount: v.Books, Date: date, At: at, Tip: g.Start, Row: decisionIdentity(row, l.GameID, date, s.Player, tm), Policy: policy, Stat: l.Stat, Line: l.Line, Book: l.Vendor, QuoteAt: l.CapturedAt, UpdatedAt: l.UpdatedAt, OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, Market: market, P1: pOver}
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
	root := filepath.Join(dir, "v2_rotation")
	if e := os.MkdirAll(root, 0755); e != nil {
		return e
	}
	b, e := json.MarshalIndent(struct {
		Policy, Date, At string
		Teams            map[int]rotation.Allocation
		Scenarios        map[int]map[int]rotation.Allocation
	}{rotation.Policy, date, at.Format(time.RFC3339Nano), f.teams, f.scenarios}, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "rotation_"+date+".json"), b, 0644)
}
