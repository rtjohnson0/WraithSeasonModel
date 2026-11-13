package main

import (
	"propsmodel/internal/picklog"
	"testing"
)

func TestComboAuditRequiresIdentityAndAgreedGrade(t *testing.T) {
	r := picklog.Settled{
		Entry:  picklog.Entry{Date: "2026-11-01", PlayerID: 7, GameID: 9, Player: "A Player", Stat: "pra", Side: "UNDER", Line: 25.5, Odds: -110, ProjMean: 22, Prob: .6, ProbRaw: .8, Tier: "COMBO"},
		Result: "LOSS", Actual: 30, SettlementVersion: 1, SourceHash: "hash", SourceFetchedAt: "2026-11-02T12:00:00Z", SourcePlayerID: 7, SourceGameID: 9, SourceDate: "2026-11-01", SourcePlayer: "A Player", SourceTeamID: 1,
	}
	if len(auditRows([]picklog.Settled{r, r}, true)) != 1 {
		t.Fatal("duplicate results counted twice")
	}
	if len(auditRows([]picklog.Settled{r}, false)) != 0 {
		t.Fatal("legacy quote treated as verified")
	}
	for _, alter := range []func(*picklog.Settled){
		func(s *picklog.Settled) { s.Result = "WIN" },
		func(s *picklog.Settled) { s.SourcePlayerID = 99 },
		func(s *picklog.Settled) { s.SourceGameID = 99 },
		func(s *picklog.Settled) { s.SourceDate = "2026-10-30" },
		func(s *picklog.Settled) { s.Tier = "COMBOWATCH" },
		func(s *picklog.Settled) { s.Result = "REVIEW" },
		func(s *picklog.Settled) { s.Period = "1q" },
	} {
		bad := r
		alter(&bad)
		if len(auditRows([]picklog.Settled{bad}, true)) != 0 {
			t.Fatal("invalid historical result accepted", bad)
		}
	}
}
