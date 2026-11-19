package comboaudit

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/picklog"
	"testing"
)

func TestFrozenErrorDecompositionDedupAndIdentity(t *testing.T) {
	s := picklog.Settled{Entry: picklog.Entry{Date: "2026-11-18", PlayerID: 1, GameID: 9, Player: "Test", Stat: "pr", Side: "OVER", Line: 20.5, Tier: "COMBORESEARCH", ProjMin: 30, ProjMean: 30, Prob: .65, AuditVersion: 2, LoggedAt: "2026-11-18T18:00:00Z", QuoteAt: "2026-11-18T17:59:00Z", Tip: "2026-11-18T23:00:00Z"}, Result: "LOSS", Actual: 15, SettlementVersion: 1, SourceHash: "hash", SourceFetchedAt: "2026-11-19T08:00:00Z", SourcePlayerID: 1, SourcePlayer: "Test", SourceGameID: 9, SourceTeamID: 2, SourceDate: "2026-11-18"}
	final := data.PlayerGame{PlayerID: 1, GameID: 9, TeamID: 2, Date: s.Date, Minutes: 20, Pts: 10, Reb: 5, SourceFinal: true, BoxScoreComplete: true}
	later := s
	later.LoggedAt = "2026-11-18T19:00:00Z"
	later.ProjMean = 999
	report := Build([]picklog.Settled{s, later}, []data.PlayerGame{final}, "")
	if len(report.Rows) != 1 || report.Duplicates != 1 {
		t.Fatal(report)
	}
	r := report.Rows[0]
	if r.MinutesComponent != -10 || r.RateComponent != -5 || math.Abs(r.Actual-r.Projection-r.MinutesComponent-r.RateComponent) > 1e-8 {
		t.Fatal(r)
	}
	if len(Build([]picklog.Settled{s}, []data.PlayerGame{final}, "2026-11-17").Rows) != 0 {
		t.Fatal("wrong date")
	}
	final.TeamID = 3
	if len(Build([]picklog.Settled{s}, []data.PlayerGame{final}, "").Rows) != 0 {
		t.Fatal("wrong final identity")
	}
	final.TeamID = 2
	final.Pts = 12
	if len(Build([]picklog.Settled{s}, []data.PlayerGame{final}, "").Rows) != 0 {
		t.Fatal("revised actual silently mixed with frozen settlement")
	}
}
