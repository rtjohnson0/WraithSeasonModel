package main

// V2.1 is a read-only, predeclared minutes sensitivity trial. It evaluates
// exactly the same independently verified rotation-v4 questions, odds and
// per-minute production rates. No challenger is used to publish bets.

import (
    "math"
    "strings"
    "time"

    "propsmodel/internal/predictor"
    "propsmodel/internal/replay"
)

const labPolicy = "v21-minute-guard-and-cap-20261121"
const labCapMinutes = 2.5

type LabCandidate struct {
    ID string `json:"id"`
    Label string `json:"label"`
    PlayerGames int `json:"playerGames"`
    Questions int `json:"questions"`
    MinutesMAE float64 `json:"minutesMAE"`
    MinutesBias float64 `json:"minutesBias"`
    Brier float64 `json:"brier"`
    LogLoss float64 `json:"logLoss"`
    StatMAE float64 `json:"statMAE"`
}

type LabSummary struct {
    Schema int `json:"schema"`
    Policy string `json:"policy"`
    GeneratedAt string `json:"generatedAt"`
    Through string `json:"through"`
    Model string `json:"model"`
    Status string `json:"status"`
    Dates int `json:"dates"`
    PlayerGames int `json:"playerGames"`
    Questions int `json:"questions"`
    Candidates []LabCandidate `json:"candidates"`
    Note string `json:"note"`
}

type labAggregate struct {
    playerGames, questions int
    minuteAbs, minuteBias, brier, logLoss, statAbs float64
}
type minutesLab struct {
    dates map[string]bool
    counts [4]labAggregate
}
func newMinutesLab() *minutesLab { return &minutesLab{dates: map[string]bool{}} }

func labCandidates() [4]LabCandidate {
    return [4]LabCandidate{
        {ID: "base-minutes", Label: "Baseline minutes"},
        {ID: "rotation-v4", Label: "Current V2 rotation-v4"},
        {ID: "supported-only", Label: "Supported absences only"},
        {ID: "capped-shift", Label: "Supported + capped ±2.5m"},
    }
}

func isUncertainStatus(status string) bool {
    switch strings.ToLower(strings.TrimSpace(status)) {
    case "questionable", "doubtful", "probable", "day-to-day":
        return true
    }
    return false
}

// This gate is frozen before reviewing the next games. Unsupported absences,
// uncertain own status and missing matched rotations all fall back to the
// baseline minutes; no later participation information can enter the decision.
func supportsRotation(r replay.Record) bool {
    rot := r.Rotation
    return rot != nil && len(rot.RelevantOut) > 0 && len(rot.ConfirmedOut) > 0 &&
        rot.AllocationApplied && rot.Supported && rot.MatchedGames >= 3 &&
        !isUncertainStatus(rot.OwnStatus)
}

func minuteTargets(r replay.Record, baseline, rotation float64) [4]float64 {
    guarded := baseline
    if supportsRotation(r) { guarded = rotation }
    delta := math.Max(-labCapMinutes, math.Min(labCapMinutes, guarded-baseline))
    return [4]float64{baseline, rotation, guarded, baseline+delta}
}

// Each candidate uses V4's *same* saved pregame production-rate assumptions,
// same minutes distribution shape, same quote, and same model/market blend.
// Only projected minutes differ. A published V4 score is always r.P1.
func labPredictions(r replay.Record, v4 predictor.Pred, baseline float64) [4]predictor.Pred {
    targets := minuteTargets(r, baseline, v4.Minutes)
    return [4]predictor.Pred{
        v4.WithMinutes(targets[0]),
        v4,
        v4.WithMinutes(targets[2]),
        v4.WithMinutes(targets[3]),
    }
}

func (lab *minutesLab) observePlayer(date string, preds [4]predictor.Pred, minutes float64) {
    if minutes <= 0 || math.IsNaN(minutes) || math.IsInf(minutes, 0) { return }
    lab.dates[date] = true
    for i, p := range preds {
        lab.counts[i].playerGames++
        lab.counts[i].minuteAbs += math.Abs(p.Minutes-minutes)
        lab.counts[i].minuteBias += minutes-p.Minutes
    }
}

func (lab *minutesLab) observeQuestion(r replay.Record, preds [4]predictor.Pred, actual float64) {
    if math.IsNaN(actual) || math.IsInf(actual, 0) || actual == r.Line { return }
    var probabilities [4]float64
    for i, p := range preds {
        raw, err := p.ProbOver(r.Stat, r.Line)
        if err != nil || math.IsNaN(raw.Over) || math.IsInf(raw.Over, 0) { return }
        probabilities[i] = math.Max(.01, math.Min(.99, r.BlendWeight*raw.Over+(1-r.BlendWeight)*r.Market))
    }
    // The exact captured production probability is the trusted reference.
    probabilities[1] = r.P1
    y := 0.0
    if actual > r.Line { y = 1 }
    for i, p := range preds {
        score := probabilities[i]
        lab.counts[i].questions++
        lab.counts[i].brier += math.Pow(score-y, 2)
        lab.counts[i].logLoss += -y*math.Log(score) -(1-y)*math.Log(1-score)
        lab.counts[i].statAbs += math.Abs(p.Mean(r.Stat)-actual)
    }
}

// Snapshot exposes *only aggregate evidence*. No player names, game IDs,
// book odds, lines, picks or individual probabilities can be serialized.
func (lab *minutesLab) snapshot(through string) *LabSummary {
    labels := labCandidates()
    result := &LabSummary{
        Schema: 1, Policy: labPolicy, Model: "rotation-v4", Through: through,
        GeneratedAt: time.Now().UTC().Format(time.RFC3339),
        Status: "research_only_not_validated", Dates: len(lab.dates),
        PlayerGames: lab.counts[0].playerGames, Questions: lab.counts[0].questions,
        Note: "Four shadow-only minutes variants on identical verified pregame rotation-v4 questions. All share rotation-v4's production-rate model and recorded quotes. Differences here are not independent forward validation or evidence of profitable picks. Do not automatically promote any variant.",
    }
    if result.Dates < 20 || result.PlayerGames < 200 {
        result.Status = "insufficient_evaluation_dates"
    }
    for i, label := range labels {
        x := lab.counts[i]
        label.PlayerGames, label.Questions = x.playerGames, x.questions
        if x.playerGames > 0 {
            label.MinutesMAE = x.minuteAbs/float64(x.playerGames)
            label.MinutesBias = x.minuteBias/float64(x.playerGames)
        }
        if x.questions > 0 {
            label.Brier = x.brier/float64(x.questions)
            label.LogLoss = x.logLoss/float64(x.questions)
            label.StatMAE = x.statAbs/float64(x.questions)
        }
        // Suppress scores for empty/small cohorts, rather than show bogus 0.
        if x.playerGames < 30 || x.questions < 30 {
            label.MinutesMAE, label.MinutesBias = 0, 0
            label.Brier, label.LogLoss, label.StatMAE = 0, 0, 0
        }
        result.Candidates = append(result.Candidates, label)
    }
    return result
}

