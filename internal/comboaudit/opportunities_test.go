package comboaudit

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/opportunity"
	"propsmodel/internal/picklog"
	"propsmodel/internal/rotation"
	"strings"
	"testing"
)

func opportunityFixture() (picklog.Settled, data.PlayerGame) {
	s := picklog.Settled{Entry: picklog.Entry{Date: "2026-11-19", PlayerID: 1, GameID: 9, Player: "Test", Stat: "points", Side: "OVER", Line: 20.5, Tier: "CHECK", ProjMin: 30, ProjMean: 22, Prob: .65, AuditVersion: 2, LoggedAt: "2026-11-19T18:00:00Z", QuoteAt: "2026-11-19T17:59:00Z", Tip: "2026-11-19T23:00:00Z", Rotation: &rotation.Context{Opportunities: []opportunity.Signal{{Stat: data.Points, Through: "2026-11-17", Available: true, Games: 10, Chances: 200, Minutes: 300, Conversion: 1.2}}}}, Result: "LOSS", Actual: 12, SettlementVersion: 1, SourceHash: "hash", SourceFetchedAt: "2026-11-20T08:00:00Z", SourcePlayerID: 1, SourcePlayer: "Test", SourceGameID: 9, SourceTeamID: 2, SourceDate: "2026-11-19"}
	g := data.PlayerGame{PlayerID: 1, GameID: 9, TeamID: 2, Date: s.Date, Minutes: 20, Pts: 12, Fga: 10, SourceFinal: true, BoxScoreComplete: true, ShootingComplete: true}
	return s, g
}

func TestOpportunityDecompositionReconcilesWithContextAndWinControls(t *testing.T) {
	s, g := opportunityFixture()
	r := BuildSinglesWithOpportunities([]picklog.Settled{s}, []data.PlayerGame{g}, "", nil)
	o := r.Rows[0].Opportunity
	if o == nil || !o.Available || o.ExpectedVolume != 20 || *o.ActualVolume != 10 || math.Abs(o.ContextComponent-2) > 1e-8 || math.Abs(o.MinutesComponent+8) > 1e-8 || math.Abs(o.VolumeComponent+4) > 1e-8 || o.ConversionComponent != 0 {
		t.Fatal(o)
	}
	if math.Abs(g.Pts-s.ProjMean-o.MinutesComponent-o.VolumeComponent-o.ConversionComponent-o.ContextComponent) > 1e-8 {
		t.Fatal("decomposition fails reconciliation")
	}
	if !strings.Contains(r.Markdown(), "points LOSS") || !strings.Contains(r.Markdown(), "20.0 → 10.0") {
		t.Fatal(r.Markdown())
	}
	s.Side, s.Result = "UNDER", "WIN"
	r = BuildSinglesWithOpportunities([]picklog.Settled{s}, []data.PlayerGame{g}, "", nil)
	found := false
	for _, slice := range r.Slices {
		if slice.Name == "points WIN" {
			found = slice.OpportunityN == 1 && slice.PlayerGames == 1
		}
	}
	if !found {
		t.Fatal("wins absent from comparison", r)
	}
}

func TestMissingTrackingAndFutureFrozenSignalsNeverFilled(t *testing.T) {
	s, g := opportunityFixture()
	s.Stat, s.Actual, g.Ast = "assists", 2, 2
	s.Rotation.Opportunities[0] = opportunity.Signal{Stat: data.Assists, Through: "2026-11-17", LatestCapture: "2026-11-19T17:00:00Z", Available: true, Games: 10, Chances: 200, Minutes: 300, Conversion: .5}
	if o := reviewOpportunity(s, g, nil); o.Available || o.ActualVolume != nil {
		t.Fatal("missing chances filled", o)
	}
	s, g = opportunityFixture()
	s.Rotation.Opportunities[0].Through = s.Date
	if reviewOpportunity(s, g, nil).Available {
		t.Fatal("same-day frozen history accepted")
	}
	s.Rotation.Opportunities[0].Through = "2026-11-17"
	s.Rotation.Opportunities[0].LatestCapture = "2026-11-20T08:00:00Z"
	if reviewOpportunity(s, g, nil).Available {
		t.Fatal("post-pick signal accepted")
	}
}

func TestWorkloadComparisonCountsPlayerOnceAndUsesEarliestDecision(t *testing.T) {
	baseline := 30.
	row := Row{PlayerID: 1, GameID: 9, Stat: "points", LoggedAt: "2026-11-19T18:00:00Z", BaselineMinutes: &baseline, ProjectedMinutes: 20, ActualMinutes: 32}
	later := row
	later.Stat = "assists"
	later.LoggedAt = "2026-11-19T19:00:00Z"
	later.ProjectedMinutes = 32
	for _, rows := range [][]Row{{row, later}, {later, row}} {
		s := compareWorkloads(rows)
		if len(s) != 2 || s[0].PlayerGames != 1 || s[0].PrimaryMAE != 2 || s[0].V2MAE != 12 {
			t.Fatal("duplicate props or later decision changed minutes comparison", s)
		}
	}
}
