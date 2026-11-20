package main

import (
    "encoding/json"
    "strings"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/lineup"
	"propsmodel/internal/predictor"
	"propsmodel/internal/replay"
	"propsmodel/internal/rotation"
	"testing"
)

func TestTodaysForecastIsPendingAndInputsRemainUntouched(t *testing.T) {
	dir := t.TempDir()
	stats := `{"_stats_fetched_at":"2026-11-11T04:00:00Z","_stats_endpoint":"/nba/v1/stats","min":"30","pts":20,"reb":5,"ast":4,"fg3m":2,"player":{"id":1,"first_name":"Test","last_name":"Player"},"team":{"id":1,"abbreviation":"BOS"},"game":{"id":7,"date":"2026-11-10","status":"Final","home_team_id":1,"visitor_team_id":2}}` + "\n"
	if e := os.WriteFile(filepath.Join(dir, "stats_2026.jsonl"), []byte(stats), 0644); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "v2_rotation", "replay")
	os.MkdirAll(path, 0755)
	capture := `{"date":"2026-11-13","selection_policy":"rotation-v1"}` + "\n"
	file := filepath.Join(path, "decisions_2026-11-13.jsonl")
	os.WriteFile(file, []byte(capture), 0644)
	r, e := collect(dir, "2026-11-13")
	if e != nil {
		t.Fatal(e)
	}
	if r.PlayerGames != 0 || r.Questions != 0 || r.Excluded["date not completed"] != 1 {
		t.Fatal(r)
	}
	b, _ := os.ReadFile(file)
	if string(b) != capture {
		t.Fatal("audit changed original forecasts")
	}
}

func TestAbsenceCohortsRequireKnownFreshConfirmedStatuses(t *testing.T) {
	r := replay.Record{At: "2026-11-16T18:00:00Z", Rotation: &rotation.Context{Supported: true, MatchedGames: 3, ConfirmedOut: []rotation.Status{{PlayerID: 7, Status: "Out", At: "2026-11-16T17:00:00Z"}}}}
	group, ok := absenceCohort(r)
	if !ok || group != "supported-joint-absence" {
		t.Fatal(group, ok)
	}
	for _, at := range []string{"", "2026-11-16T19:00:00Z", "2026-11-14T17:00:00Z"} {
		r.Rotation.ConfirmedOut[0].At = at
		if _, ok := absenceCohort(r); ok {
			t.Fatal("unavailable injury evidence accepted", at)
		}
	}
	r.Rotation.ConfirmedOut[0].At = "2026-11-16T17:00:00Z"
	r.Rotation.ConfirmedOut[0].Status = "Questionable"
	if _, ok := absenceCohort(r); ok {
		t.Fatal("questionable treated as confirmed out")
	}
}
func TestIrrelevantOutsAreNotEvidenceForJointAbsenceAdjustment(t *testing.T) {
	r := replay.Record{At: "2026-11-16T18:00:00Z", Rotation: &rotation.Context{Supported: true, Reason: "recent complete-team workload; confirmed outs have no recent rotation minutes to redistribute", ConfirmedOut: []rotation.Status{{PlayerID: 7, Status: "Out", At: "2026-11-16T17:00:00Z"}}}}
	if group, ok := absenceCohort(r); !ok || group != "irrelevant-outs" {
		t.Fatal(group, ok)
	}
	r.Rotation.Reason = "limited matching rotations; recent-workload fallback, review only"
	if group, ok := absenceCohort(r); !ok || group != "unsupported-fallback" {
		t.Fatal(group, ok)
	}
	r.Rotation.ConfirmedOut = nil
	if group, ok := absenceCohort(r); !ok || group != "no-confirmed-out" {
		t.Fatal(group, ok)
	}
}

func TestProductionCaptureRequiresFreshKnownStatusesAndExactBoundedScales(t *testing.T) {
	impact := lineup.Impact{Kind: "opponent-allowance", Team: 2, Missing: []lineup.Profile{{PlayerID: 7, Status: "Out", At: "2026-11-16T17:50:00Z"}}, Effects: []lineup.Effect{{Stat: data.Points, Scale: 1.03, Applied: true, Without: 5, With: 5}}}
	r := replay.Record{At: "2026-11-16T18:00:00Z", Rotation: &rotation.Context{Production: []lineup.Impact{impact}}, Policy: predictor.Adjustments{Scale: lineup.Scale([]lineup.Impact{impact})}}
	if !validProductionCapture(r) {
		t.Fatal("valid capture rejected")
	}
	for _, stamp := range []string{"", "2026-11-16T19:00:00Z", "2026-11-14T17:50:00Z"} {
		r.Rotation.Production[0].Missing[0].At = stamp
		if validProductionCapture(r) {
			t.Fatal("unavailable status accepted", stamp)
		}
	}
	r.Rotation.Production[0].Missing[0].At = "2026-11-16T17:50:00Z"
	r.Policy.Scale[0] = 1.08
	if validProductionCapture(r) {
		t.Fatal("scale provenance mismatch accepted")
	}
}

func TestPhase3QuestionLevelDetailsCannotSerializeInNormalReport(t *testing.T) {
    report := Report{Phase3: []Phase3Question{{
        Date: "2026-11-18", Stat: "points", ModelID: "private-checkpoint",
        Line: 987.5, OverOdds: -130, UnderOdds: 110,
    }}}
    b, err := json.Marshal(report)
    if err != nil { t.Fatal(err) }
    for _, sensitive := range []string{"987.5", "private-checkpoint", "over_odds", "phase3"} {
        if strings.Contains(strings.ToLower(string(b)), sensitive) {
            t.Fatalf("question-level private data leaked to default report: %s", sensitive)
        }
    }
}
