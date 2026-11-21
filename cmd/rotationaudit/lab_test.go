package main

import (
    "encoding/json"
    "math"
    "strings"
    "testing"

    "propsmodel/internal/data"
    "propsmodel/internal/predictor"
    "propsmodel/internal/replay"
    "propsmodel/internal/rotation"
)

func TestV21MinutesTargetsFallBackWhenRotationsUnsupported(t *testing.T) {
    r := replay.Record{Rotation:&rotation.Context{
        ConfirmedOut: []rotation.Status{{PlayerID:23}},
        RelevantOut: []int{23}, MatchedGames:2, Supported:false, AllocationApplied:true,
    }}
    got := minuteTargets(r, 29, 37)
    if got != [4]float64{29, 37, 29, 29} {t.Fatalf("unsupported rotation must fall back to baseline: %v", got)}
    r.Rotation.MatchedGames = 4
    r.Rotation.Supported = true
    got = minuteTargets(r, 29, 37)
    if got != [4]float64{29, 37, 37, 31.5} {t.Fatalf("expected support and +2.5 cap: %v", got)}
    r.Rotation.OwnStatus = "Questionable"
    got = minuteTargets(r, 29, 37)
    if got != [4]float64{29, 37, 29, 29} {t.Fatal("questionable player cannot qualify for new shadow allocation", got)}
}

func TestV21CapsNegativeShiftsAndNoOutStatus(t *testing.T) {
    r := replay.Record{Rotation:&rotation.Context{
        ConfirmedOut: []rotation.Status{{PlayerID:23}},
        RelevantOut: []int{23}, MatchedGames:5, Supported:true, AllocationApplied:true,
    }}
    if got := minuteTargets(r, 33, 25); got != [4]float64{33, 25, 25, 30.5} {
        t.Fatal("negative cap incorrect", got)
    }
    r.Rotation.RelevantOut = nil
    if got := minuteTargets(r, 33, 25); got != [4]float64{33, 25, 33, 33} {
        t.Fatal("no relevant confirmed-out cannot trigger guarded correction", got)
    }
}

func TestV21LabOutputsAggregatesOnlyAndSuppressesSmallSamples(t *testing.T) {
    lab := newMinutesLab()
    r := replay.Record{
        Date: "2026-11-20", Stat: data.Points, Line:18.5,
        P1:.55, BlendWeight:.6, Market:.5,
        Rotation:&rotation.Context{Supported:false},
    }
    v4 := predictor.Pred{Minutes:30}
    for i := range v4.MinNodes { v4.MinNodes[i] = 30 }
    v4.Rate[data.StatIndex(data.Points)] = .64
    v4.Alpha[data.StatIndex(data.Points)] = .2
    candidates := labPredictions(r, v4, 29)
    lab.observePlayer("2026-11-20", candidates, 32)
    lab.observeQuestion(r, candidates, 22)
    report := lab.snapshot("2026-11-20")
    if report.PlayerGames != 1 || report.Questions != 1 || report.Dates != 1 ||
        report.Status != "insufficient_evaluation_dates" {
        t.Fatal("incorrect aggregate counters", report)
    }
    for _, c := range report.Candidates {
        if c.Questions != 1 || c.MinutesMAE != 0 || c.Brier != 0 {t.Fatal("small sample must be suppressed", c)}
    }
    b, err := json.Marshal(report)
    if err != nil {t.Fatal(err)}
    text := string(b)
    for _, forbidden := range []string{"2026-11-20T17:00", "PlayerID", "GameID", "player_name", "over_odds", "under_odds", "18.5", "individualPick"} {
        if strings.Contains(text, forbidden) {t.Fatalf("individual outcome/quote leaked: %s", forbidden)}
    }
    if math.IsNaN(candidates[2].Minutes) {t.Fatal("invalid minutes projection")}
}

func TestV21LabDoesNotSerializeIntoOriginalRotationAudit(t *testing.T) {
    report := Report{Lab: (&minutesLab{dates:map[string]bool{}}).snapshot("")}
    b, err := json.Marshal(report)
    if err != nil {t.Fatal(err)}
    if strings.Contains(string(b), labPolicy) || strings.Contains(string(b), "candidates") {
        t.Fatal("V2.1 summary should be in dedicated output, not legacy audit report")
    }
}
