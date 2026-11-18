package main

import (
	"fmt"
	"math"
	"propsmodel/internal/availability"
	"propsmodel/internal/combo"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/glm"
	"propsmodel/internal/model"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"testing"
	"time"
)

func rotationFixture() ([]data.PlayerGame, map[int]int) {
	var rows []data.PlayerGame
	roster := map[int]int{}
	for d := 1; d <= 8; d++ {
		for p := 1; p <= 10; p++ {
			roster[100+p] = 1
			rows = append(rows, data.PlayerGame{SourceFinal: true, Date: fmt.Sprintf("2026-11-%02d", d), TeamID: 1, GameID: d, PlayerID: 100 + p, Name: fmt.Sprintf("Player %d", 100+p), Minutes: 24})
		}
	}
	return rows, roster
}
func TestQuestionableTeammateIsScenarioNotAssumedOut(t *testing.T) {
	rows, roster := rotationFixture()
	fresh := map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out"}, 102: {PlayerID: 102, Name: "Player 102", Status: "Questionable"}}
	f := newRotationForecast(rows, roster, fresh, map[int]string{101: "Out", 102: "Questionable"}, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09")
	if !f.teams[1].Valid || f.teams[1].Minutes[101] != 0 || f.teams[1].Minutes[102] <= 0 {
		t.Fatal(f.teams[1])
	}
	alt := f.scenarios[1][102]
	if !alt.Valid || alt.Minutes[102] != 0 {
		t.Fatal(alt)
	}
	p := predictor.Pred{Minutes: 24}
	for n := range p.MinNodes {
		p.MinNodes[n] = 24
	}
	p.Rate[data.StatIndex(data.Points)] = .5
	row := &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}
	adjusted := f.adjust(p, row)
	if math.Abs(adjusted.Minutes-f.teams[1].Minutes[103]) > 1e-8 || adjusted.Rate != p.Rate {
		t.Fatal("allocation or rate changed incorrectly")
	}
	s := scored{PlayerID: 103, GameID: 9, Stat: data.Points, Side: "OVER", Line: 12.5, ProjMin: adjusted.Minutes}
	c := f.context(s, p, row, .5, .5, combo.Corr{})
	if len(c.Scenarios) != 1 || c.Scenarios[0].Probability == nil || c.Scenarios[0].Minutes <= c.UsedMinutes {
		t.Fatal(c)
	}
	if c.Supported {
		t.Fatal("unobserved absence combination should stay review-only")
	}
}
func TestStaleOutReportDoesNotNumericallyAlterRotation(t *testing.T) {
	rows, roster := rotationFixture()
	f := newRotationForecast(rows, roster, nil, map[int]string{101: "Out"}, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09")
	if f.teams[1].Valid {
		t.Fatal("stale known absence accepted")
	}
	p := predictor.Pred{Minutes: 24}
	if f.adjust(p, &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}).Minutes != 24 {
		t.Fatal("baseline changed")
	}
}

func TestRotationCanScoreNewBenchOpportunityAndReplayExactly(t *testing.T) {
	rows, roster := rotationFixture()
	b := feat.NewBuilder(rows, nil)
	b.SetRoster(roster)
	games := map[int]props.GameInfo{9: {ID: 9, Home: 1, Visitor: 2, HomeAbbr: "ATL", VisitorAbbr: "BOS", Start: "2026-11-09T23:00:00Z"}}
	fresh := map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out"}}
	f := newRotationForecast(rows, roster, fresh, map[int]string{101: "Out"}, games, "2026-11-09")
	m := &predictor.Model{Version: 4, Minutes: &glm.Linear{Intercept: 12}, Quant: make([][]float64, 5)}
	for k := range m.Quant {
		m.Quant[k] = make([]float64, predictor.Nodes)
	}
	for k := range m.Stat {
		m.Stat[k] = &glm.Model{Intercept: math.Log(.5), Alpha: .05}
	}
	r := &feat.Row{G: data.PlayerGame{PlayerID: 103, TeamID: 1, Date: "2026-11-09"}, AvgMin15: 8, Games: 8, X: make([]float64, len(feat.CommonSpecs))}
	for k := range r.S {
		r.S[k] = make([]float64, len(feat.StatSpecs[k]))
	}
	if model.Eligible(r.AvgMin15, m.Predict(r).Minutes) {
		t.Fatal("fixture must be ineligible for baseline")
	}
	baseline := m.Predict(r)
	asOf, _ := time.Parse(time.RFC3339, "2026-11-09T18:00:00Z")
	line := props.Line{GameID: 9, PlayerID: 103, Stat: data.Points, Line: 10.5, OverOdds: -110, UnderOdds: -110, Vendor: "book", CapturedAt: asOf.Format(time.RFC3339), UpdatedAt: asOf.Format(time.RFC3339)}
	all, records := scoreRotationSingles(f, m, b, []props.Line{line}, games, func(pid int) (int, bool) { tm, ok := roster[pid]; return tm, ok }, func(int, props.GameInfo, int) *feat.Row { return r }, nil, "2026-11-09", .5, 1, asOf)
	if len(all) != 1 || len(records) != 1 || all[0].ProjMin <= 18 {
		t.Fatal("new bench opportunity was filtered by baseline eligibility")
	}
	p, err := replay.Recompute(m, records[0])
	if err != nil || math.Abs(p-records[0].P1) > 1e-10 {
		t.Fatal("rotation forecast not reproducible", p, err)
	}
	if records[0].Row.G.Name == "" || records[0].Rotation == nil || records[0].Policy.TargetMinutes == nil {
		t.Fatal("incomplete capture")
	}
	if m.Predict(r) != baseline {
		t.Fatal("V1 baseline changed")
	}
}

func TestStrictFallbackLeavesBaselineAndHealthyMinutesAlone(t *testing.T) {
	rows, roster := rotationFixture()
	for _, fresh := range []map[int]availability.Status{
		nil,
		{101: {PlayerID: 101, Name: "Player 101", Status: "Out", At: "2026-11-09T17:00:00Z"}},
	} {
		f := newRotationForecast(rows, roster, fresh, nil, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09", true)
		p := predictor.Pred{Minutes: 30}
		p.Rate[0] = .7
		for i := range p.MinNodes {
			p.MinNodes[i] = 30
		}
		row := &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}
		if got := f.adjust(p, row); got != p {
			t.Fatal("fallback changed baseline", got, p)
		}
		s := scored{PlayerID: 103, Stat: data.Points, ProjMin: p.Minutes}
		context := f.context(s, p, row, .5, .5, combo.Corr{})
		if context.AllocationApplied || context.Supported || context.Policy != rotation.StrictPolicy {
			t.Fatal(context)
		}
	}
}

func TestStrictSupportedJointAbsenceUsesObservedTarget(t *testing.T) {
	rows, roster := rotationFixture()
	for i := range rows {
		r := &rows[i]
		if r.Date >= "2026-11-06" {
			switch r.PlayerID {
			case 101:
				r.Minutes = 0
			case 102, 103:
				r.Minutes = 36
			}
		}
	}
	f := newRotationForecast(rows, roster, map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out", At: "2026-11-09T17:00:00Z"}}, nil, map[int]props.GameInfo{9: {Home: 1, Visitor: 2}}, "2026-11-09", true)
	a := f.teams[1]
	if !a.Supported || a.MatchedGames != 3 {
		t.Fatal(a)
	}
	p := predictor.Pred{Minutes: 24}
	p.Rate[0] = .7
	for i := range p.MinNodes {
		p.MinNodes[i] = 24
	}
	row := &feat.Row{G: data.PlayerGame{TeamID: 1, PlayerID: 103}}
	got := f.adjust(p, row)
	if math.Abs(got.Minutes-a.Minutes[103]) > 1e-9 || got.Rate != p.Rate {
		t.Fatal(got, a)
	}
	context := f.context(scored{PlayerID: 103, Stat: data.Points, ProjMin: got.Minutes}, p, row, .5, .5, combo.Corr{})
	if !context.AllocationApplied || len(context.RelevantOut) != 1 || !context.Supported {
		t.Fatal(context)
	}
}

func TestStrictFallbackCaptureReplaysBaselineWithoutTarget(t *testing.T) {
	rows, roster := rotationFixture()
	b := feat.NewBuilder(rows, nil)
	b.SetRoster(roster)
	games := map[int]props.GameInfo{9: {ID: 9, Home: 1, Visitor: 2, HomeAbbr: "ATL", VisitorAbbr: "BOS", Start: "2026-11-09T23:00:00Z"}}
	fresh := map[int]availability.Status{101: {PlayerID: 101, Name: "Player 101", Status: "Out"}}
	f := newRotationForecast(rows, roster, fresh, map[int]string{101: "Out"}, games, "2026-11-09", true)
	m := &predictor.Model{Version: 4, Minutes: &glm.Linear{Intercept: 30}, Quant: make([][]float64, 5)}
	for k := range m.Quant {
		m.Quant[k] = make([]float64, predictor.Nodes)
	}
	for k := range m.Stat {
		m.Stat[k] = &glm.Model{Intercept: math.Log(.5), Alpha: .05}
	}
	r := &feat.Row{G: data.PlayerGame{PlayerID: 103, TeamID: 1, Date: "2026-11-09"}, AvgMin15: 24, Games: 8, X: make([]float64, len(feat.CommonSpecs))}
	for k := range r.S {
		r.S[k] = make([]float64, len(feat.StatSpecs[k]))
	}
	if !model.Eligible(r.AvgMin15, m.Predict(r).Minutes) {
		t.Fatal("fixture must be eligible for baseline")
	}
	baseline := m.Predict(r)
	asOf, _ := time.Parse(time.RFC3339, "2026-11-09T18:00:00Z")
	line := props.Line{GameID: 9, PlayerID: 103, Stat: data.Points, Line: 10.5, OverOdds: -110, UnderOdds: -110, Vendor: "book", CapturedAt: asOf.Format(time.RFC3339), UpdatedAt: asOf.Format(time.RFC3339)}
	all, records := scoreRotationSingles(f, m, b, []props.Line{line}, games, func(pid int) (int, bool) { tm, ok := roster[pid]; return tm, ok }, func(int, props.GameInfo, int) *feat.Row { return r }, nil, "2026-11-09", .5, 1, asOf)
	if len(all) != 1 || len(records) != 1 || all[0].ProjMin != baseline.Minutes {
		t.Fatal("strict fallback must score baseline minutes")
	}
	p, err := replay.Recompute(m, records[0])
	if err != nil || math.Abs(p-records[0].P1) > 1e-10 {
		t.Fatal("rotation forecast not reproducible", p, err)
	}
	if records[0].Row.G.Name == "" || records[0].Rotation == nil || records[0].Policy.TargetMinutes != nil || records[0].Rotation.AllocationApplied || records[0].SelectionPolicy != rotation.StrictPolicy {
		t.Fatal("incomplete capture")
	}
	if m.Predict(r) != baseline {
		t.Fatal("V1 baseline changed")
	}
}

func TestProductionAdjustmentsReplayExactlyForTeammateAndOpponentAbsences(t *testing.T) {
	var rows []data.PlayerGame
	roster := map[int]int{}
	for day := 1; day <= 30; day++ {
		for team := 1; team <= 2; team++ {
			for p := 1; p <= 10; p++ {
				id := team*100 + p
				roster[id] = team
				r := data.PlayerGame{AnalyticsReady: true, SourceFinal: true, PlayerID: id, TeamID: team, GameID: day, Date: fmt.Sprintf("2026-09-%02d", day), Name: fmt.Sprintf("Player %d", id), Pos: "C", Minutes: 24, Pts: 12, Reb: 4, Ast: 3, Fg3m: 1, Blk: 1}
				if day >= 26 {
					if team == 2 {
						if p == 1 {
							r.Minutes = 0
						}
						if p == 2 || p == 3 {
							r.Minutes = 36
						}
					}
					if team == 1 || p == 2 {
						r.Pts = 18
						r.Reb = 6
						r.Ast = 4
						r.Fg3m = .5
					}
				}
				if r.Minutes == 36 {
					r.Pts *= 1.5
					r.Reb *= 1.5
					r.Ast *= 1.5
					r.Fg3m *= 1.5
				}
				if r.Minutes == 0 {
					r.Pts = 0
					r.Reb = 0
					r.Ast = 0
					r.Fg3m = 0
					r.Blk = 0
				}
				rows = append(rows, r)
			}
		}
	}
	games := map[int]props.GameInfo{31: {ID: 31, Home: 1, Visitor: 2, HomeAbbr: "ATL", VisitorAbbr: "BOS", Start: "2026-10-01T23:00:00Z"}}
	f := newRotationForecast(rows, roster, map[int]availability.Status{201: {PlayerID: 201, Name: "Player 201", Status: "Out", At: "2026-10-01T17:50:00Z"}}, nil, games, "2026-10-01", true)
	m := &predictor.Model{Version: 6, Disabled: []string{feat.GroupTeammates, feat.GroupOppOut}, Minutes: &glm.Linear{Intercept: 30}, Quant: make([][]float64, 5)}
	for k := range m.Quant {
		m.Quant[k] = make([]float64, predictor.Nodes)
	}
	for k := range m.Stat {
		m.Stat[k] = &glm.Model{Intercept: math.Log(.5), Alpha: .05}
	}
	f.configureProduction(m)
	b := feat.NewBuilder(rows, nil)
	b.SetRoster(roster)
	asOf, _ := time.Parse(time.RFC3339, "2026-10-01T18:00:00Z")
	for _, pid := range []int{101, 202} {
		team := roster[pid]
		row := &feat.Row{G: data.PlayerGame{PlayerID: pid, TeamID: team, Date: "2026-10-01"}, AvgMin15: 24, Games: 30, X: make([]float64, len(feat.CommonSpecs))}
		for k := range row.S {
			row.S[k] = make([]float64, len(feat.StatSpecs[k]))
		}
		base := m.Predict(row)
		adjusted := f.adjust(base, row)
		if adjusted.Rate[0] <= base.Rate[0] || adjusted.Rate[3] >= base.Rate[3] {
			t.Fatal("rates ignored or all directions forced", adjusted, base)
		}
		if pid == 101 && adjusted.Minutes != base.Minutes {
			t.Fatal("opponent absence altered own minutes")
		}
		line := props.Line{GameID: 31, PlayerID: pid, Stat: data.Points, Line: 15.5, OverOdds: -110, UnderOdds: -110, Vendor: "book", CapturedAt: asOf.Format(time.RFC3339), UpdatedAt: asOf.Format(time.RFC3339)}
		_, records := scoreRotationSingles(f, m, b, []props.Line{line}, games, func(pid int) (int, bool) { tm, ok := roster[pid]; return tm, ok }, func(int, props.GameInfo, int) *feat.Row { return row }, nil, "2026-10-01", .5, 1, asOf)
		if len(records) != 1 || records[0].Policy.Scale[0] <= 1 {
			t.Fatal(records)
		}
		prob, err := replay.Recompute(m, records[0])
		if err != nil || math.Abs(prob-records[0].P1) > 1e-10 {
			t.Fatal("rate replay mismatch", prob, err)
		}
		if m.Predict(row) != base {
			t.Fatal("V1 model mutated")
		}
		// A model already fitted with absence features must not get a second bonus.
		f.configureProduction(&predictor.Model{})
		if got := f.adjust(base, row); got.Rate != base.Rate {
			t.Fatal("learned and research effects stacked")
		}
		f.configureProduction(m)
	}
}

func TestRoleCandidateCloneCannotMutateActiveRotations(t *testing.T) {
	f := &rotationForecast{teams: map[int]rotation.Allocation{1: {Reason: "active"}}, scenarios: map[int]map[int]rotation.Allocation{1: {9: {Reason: "active scenario"}}}}
	shadow := f.cloneForRoles()
	shadow.teams[1] = rotation.Allocation{Reason: "shadow"}
	shadow.scenarios[1][9] = rotation.Allocation{Reason: "shadow scenario"}
	if f.teams[1].Reason != "active" || f.scenarios[1][9].Reason != "active scenario" {
		t.Fatal("shadow modified live forecast")
	}
}
