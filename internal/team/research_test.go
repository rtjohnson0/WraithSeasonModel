package team

import (
	"math"
	"propsmodel/internal/data"
	"testing"
)

func TestResearchExpiresOffseasonBeforeReadinessAndOnlyOnce(t *testing.T) {
	st := NewState()
	st.SeasonSafe = true
	for d := 1; d <= 12; d++ {
		st.Update(Game{Home: 1, Away: 2, Day: d, HPts: 120, APts: 100})
	}
	if !st.Ready(1, 2) {
		t.Fatal("fixture not ready")
	}
	if st.ReadyAt(1, 2, 100) {
		t.Fatal("offseason retained full-season readiness")
	}
	first := st.Vec(1, 2, 100)
	second := st.Vec(1, 2, 100)
	for i := range first {
		if first[i] != second[i] {
			t.Fatal("ratings shrank repeatedly")
		}
	}
	old := st.get(1).off10
	st.Update(Game{Home: 1, Away: 2, Day: 100, HPts: 120, APts: 100})
	if math.Abs(st.get(1).off10-(old+a10*(120-old))) > 1e-10 || st.Games(1) != 5 {
		t.Fatal("update reset a second time")
	}
}
func TestVerifiedHistoryUsesOfficialFinalsAndRejectsConflicts(t *testing.T) {
	r := data.PlayerGame{SourceFinal: true, PlayerID: 1, GameID: 7, TeamID: 1, HomeTeamID: 1, VisitorTeamID: 2, Date: "2026-11-01", Season: 2026, Pts: 12, FinalHomeScore: 110, FinalAwayScore: 100}
	gs := BuildVerified([]data.PlayerGame{r})
	if len(gs) != 1 || gs[0].Total() != 210 {
		t.Fatal(gs)
	}
	conflicting := r
	conflicting.FinalHomeScore = 111
	if len(BuildVerified([]data.PlayerGame{r, conflicting})) != 0 {
		t.Fatal("conflicting finals accepted")
	}
	r.SourceFinal = false
	if len(BuildVerified([]data.PlayerGame{r})) != 0 {
		t.Fatal("unconfirmed game accepted")
	}
}
func TestAnchoredMixCannotReverseOrAmplifyRatingsDisagreement(t *testing.T) {
	for _, sign := range []float64{-2, 3} {
		var rs []Rec
		for i := 0; i < 200; i++ {
			market := 220 + float64(i%11)
			delta := float64(i%7) - 3
			rs = append(rs, Rec{Market: market, Proj: market + delta, Actual: market + sign*delta})
		}
		m, err := FitAnchoredMix(rs)
		if err != nil {
			t.Fatal(err)
		}
		book, ratings := m.Weights()
		if ratings < 0 || ratings > 1 || math.Abs(book+ratings-1) > 1e-10 {
			t.Fatal(book, ratings)
		}
	}
}
func TestScheduleUpdateReadsBothOpponentsBeforeChangingEither(t *testing.T) {
	a, b := NewState(), NewState()
	a.ScheduleAdjusted = true
	b.ScheduleAdjusted = true
	for i := 1; i < 20; i++ {
		g := Game{Home: 1, Away: 2, Day: i, HPts: 120, APts: 105}
		a.Update(g)
		b.Update(Game{Home: 2, Away: 1, Day: i, HPts: 105, APts: 120})
	}
	if math.Abs(a.get(1).off30-b.get(1).off30) > 1e-12 || math.Abs(a.get(2).def30-b.get(2).def30) > 1e-12 {
		t.Fatal("home/away update order changed ratings")
	}
}

func TestResearchFeaturesDoNotReadFutureGameResults(t *testing.T) {
	games := Build(league(140))
	full, _ := ResearchSamples(games)
	prefix, _ := ResearchSamples(games[:70])
	for i := range prefix {
		if full[i].Ready != prefix[i].Ready {
			t.Fatal("future readiness leaked")
		}
		for j, x := range prefix[i].X {
			if math.Abs(x-full[i].X[j]) > 1e-12 {
				t.Fatal("future score leaked into research rating")
			}
		}
	}
}
