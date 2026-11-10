package settlement

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/picklog"
	"testing"
)

func fixture(t *testing.T, dir string, reb any, status string) {
	t.Helper()
	row := map[string]any{"_stats_fetched_at": "2026-11-10T18:00:00Z", "_stats_endpoint": "/nba/v1/stats", "min": "31:22", "pts": 20, "reb": reb, "ast": 2, "fg3m": 1, "oreb": 3, "dreb": 10, "player": map[string]any{"id": 7, "first_name": "Naz", "last_name": "Reid"}, "team": map[string]any{"id": 16, "abbreviation": "MIN"}, "game": map[string]any{"id": 9, "date": "2026-11-09", "status": status, "home_team_id": 16, "visitor_team_id": 2}}
	b, _ := json.Marshal(row)
	if e := os.WriteFile(filepath.Join(dir, "stats_2026.jsonl"), append(b, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
func entry() picklog.Entry {
	return picklog.Entry{Date: "2026-11-09", GameID: 9, PlayerID: 7, Player: "Naz Reid", Team: "MIN", Period: "game", Stat: "rebounds", Side: "UNDER", Line: 9.5, Odds: -110, Units: 1, AuditVersion: 2}
}
func TestNazThirteenReboundsIsLoss(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, 13, "Final")
	ix, e := Load(dir)
	if e != nil {
		t.Fatal(e)
	}
	s := ix.Grade(entry(), 0)
	if s.Result != "LOSS" || s.Actual != 13 || s.Profit != -1 || !s.Verified() {
		t.Fatalf("wrong result %+v", s)
	}
	for _, change := range []func(*picklog.Entry){func(e *picklog.Entry) { e.Player = "Another Player" }, func(e *picklog.Entry) { e.Team = "CHA" }, func(e *picklog.Entry) { e.Date = "2026-11-08" }, func(e *picklog.Entry) { e.GameID = 8 }, func(e *picklog.Entry) { e.Stat = "unknown" }, func(e *picklog.Entry) { e.Period = "1q" }} {
		r := entry()
		change(&r)
		s = ix.Grade(r, 0)
		if s.Result != "REVIEW" || s.Verified() {
			t.Fatalf("mismatch counted %+v", s)
		}
	}
}
func TestMissingAndLiveValuesNeverBecomeUnderWins(t *testing.T) {
	for _, tc := range []struct {
		reb    any
		status string
	}{{nil, "Final"}, {1, "In Progress"}, {1, ""}, {1, "Final"}} {
		dir := t.TempDir()
		fixture(t, dir, tc.reb, tc.status)
		ix, _ := Load(dir)
		s := ix.Grade(entry(), 0)
		if s.Result != "REVIEW" {
			t.Fatalf("bad box score graded %+v", s)
		}
	}
}
func TestReconcileRepairsExistingWinAndKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, 13, "Final")
	old := picklog.Grade(entry(), 1, 0, true)
	b, _ := json.Marshal(old)
	path := filepath.Join(dir, "settled.jsonl")
	os.WriteFile(path, append(b, '\n'), 0600)
	report, e := Reconcile(dir, "settled.jsonl", nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if report.Checked != 1 || report.Confirmed != 1 || len(report.Changes) != 1 || report.Changes[0].OldResult != "WIN" || report.Changes[0].NewResult != "LOSS" {
		t.Fatalf("audit %+v", report)
	}
	results, e := picklog.LoadSettled(path)
	if e != nil || len(results) != 1 || results[0].Actual != 13 || results[0].Result != "LOSS" {
		t.Fatal("saved wrong actual")
	}
	backups, _ := filepath.Glob(path + ".before-result-audit-*")
	if len(backups) != 1 {
		t.Fatal("missing original backup")
	}
	before, _ := os.ReadFile(path)
	report, e = Reconcile(dir, "settled.jsonl", nil, nil)
	after, _ := os.ReadFile(path)
	if e != nil || len(report.Changes) != 0 || string(before) != string(after) {
		t.Fatal("audit is not stable on repeated run")
	}
}

func TestLegacyWinsAreNotVerifiedWithoutActualEvidence(t *testing.T) {
	old := picklog.Grade(entry(), 1, 0, true)
	if old.Verified() {
		t.Fatal("quote provenance alone verified a result")
	}
	review := picklog.Settled{Entry: entry(), Result: "REVIEW", Profit: 100}
	if s := picklog.Summarize([]picklog.Settled{review}); s.Picks != 0 || s.Profit != 0 {
		t.Fatal("review counted in performance")
	}
}

func TestOlderCacheCannotReverseNewerCorrectedLoss(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, 13, "Final")
	if _, e := Reconcile(dir, "settled.jsonl", []picklog.Entry{entry()}, nil); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "stats_2026.jsonl")
	b, _ := os.ReadFile(path)
	var row map[string]any
	json.Unmarshal(b, &row)
	row["_stats_fetched_at"] = "2026-11-10T17:00:00Z"
	row["reb"] = 1
	row["oreb"] = 0
	row["dreb"] = 1
	b, _ = json.Marshal(row)
	os.WriteFile(path, append(b, '\n'), 0600)
	report, e := Reconcile(dir, "settled.jsonl", nil, nil)
	if e != nil || report.StaleSources != 1 {
		t.Fatal("older cache not detected")
	}
	rs, _ := picklog.LoadSettled(filepath.Join(dir, "settled.jsonl"))
	if len(rs) != 1 || rs[0].Result != "LOSS" || rs[0].Actual != 13 {
		t.Fatal("older cache reversed a correction")
	}
}

func TestUTF8BOMDoesNotPreventResultAudit(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, 13, "Final")
	p := filepath.Join(dir, "stats_2026.jsonl")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append([]byte{0xef, 0xbb, 0xbf}, b...), 0600)
	old := picklog.Grade(entry(), 1, 0, true)
	b, _ = json.Marshal(old)
	os.WriteFile(filepath.Join(dir, "settled.jsonl"), append([]byte{0xef, 0xbb, 0xbf}, append(b, '\n')...), 0600)
	r, e := Reconcile(dir, "settled.jsonl", nil, nil)
	if e != nil || r.Confirmed != 1 || len(r.Changes) != 1 || r.Changes[0].NewResult != "LOSS" {
		t.Fatalf("BOM audit failed %+v %v", r, e)
	}
}

func TestOnlyExactReversibleNameEncodingRepairIsAllowed(t *testing.T) {
	if !sameName("Kasparas JakuÄ\u008dionis", "Kasparas Jakučionis") {
		t.Fatal("known reversible UTF-8 corruption was not recognized")
	}
	if sameName("Naz Reid", "Nathan Reid") {
		t.Fatal("fuzzy identity matching admitted another player")
	}
}
