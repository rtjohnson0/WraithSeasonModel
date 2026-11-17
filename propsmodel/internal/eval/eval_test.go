package eval

import (
	"math"
	"math/rand"
	"propsmodel/internal/data"
	"testing"

	"propsmodel/internal/backtest"
	"propsmodel/internal/feat"
	"propsmodel/internal/synthworld"
)

func setup(t *testing.T) (*feat.Builder, []feat.Row, *synthworld.World) {
	t.Helper()
	w := synthworld.Generate(synthworld.Config{Seed: 21, Seasons: []int{2023, 2024}, Teams: 20, Days: 150})
	b := feat.NewBuilder(w.Games, w.Adv)
	return b, b.Rows(12), w
}

func TestFullModelBeatsBaselineOutOfSample(t *testing.T) {
	b, rows, w := setup(t)
	v2, err := WalkForward(rows, w.Lines, 2024, nil)
	if err != nil {
		t.Fatal(err)
	}
	v1 := backtest.Run(b.Index(), w.Lines)
	v2, v1 = Intersect(v2, v1)
	if len(v2) < 3000 {
		t.Fatalf("only %d comparable results", len(v2))
	}
	_, calV2, _ := Holdout(v2)
	_, calV1, _ := Holdout(v1)
	m2, m1 := Score(calV2), Score(calV1)
	t.Logf("calibrated hold-out log loss: v2 %.5f  baseline %.5f  (n=%d)", m2.LogLoss, m1.LogLoss, m2.N)
	if m2.LogLoss >= m1.LogLoss {
		t.Errorf("full model (%.5f) should beat the baseline (%.5f)", m2.LogLoss, m1.LogLoss)
	}
}

func TestAblationFindsTeammateEffect(t *testing.T) {
	_, rows, w := setup(t)
	full, err := WalkForward(rows, w.Lines, 2024, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, calFull, _ := Holdout(full)
	fm := Score(calFull)
	abl, err := Ablate(rows, w.Lines, 2024, []string{feat.GroupTeammates, feat.GroupSlump}, fm, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range abl {
		t.Logf("without %-10s log loss %.5f  delta %+.5f", a.Group, a.LogLoss, a.Delta)
	}
	for _, a := range abl {
		if a.Group == feat.GroupTeammates && a.Delta <= 0 {
			t.Errorf("removing the teammates-out features should hurt in a world where they matter, delta %.5f", a.Delta)
		}
	}
}

func TestCalibrationSaveLoad(t *testing.T) {
	_, rows, w := setup(t)
	rs, _ := WalkForward(rows, w.Lines, 2024, nil)
	c := FitCalibration(rs)
	path := t.TempDir() + "/cal.json"
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	c2, ok := LoadCalibration(path)
	if !ok || c2.Global != c.Global {
		t.Errorf("round trip failed: %+v vs %+v", c2, c)
	}
}

// When the book's prices are skewed and the outcomes follow them, the fit must lean on the
// sportsbook probability and give the (noise) model no weight. This is the failure the first
// real backtest exposed: a model compared against 50% instead of the real price.
func TestCalibrationLearnsFromMarketPrice(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var rs []backtest.Result
	for i := 0; i < 8000; i++ {
		market := 0.35 + 0.30*r.Float64()
		over := r.Float64() < market
		actual := 10.0
		if over {
			actual = 12
		} else {
			actual = 8
		}
		noise := 0.5 + 0.2*r.NormFloat64() // a model with no information
		rs = append(rs, backtest.Result{
			Stat: data.Points, Line: 10.5, POver: math.Min(math.Max(noise, 0.05), 0.95),
			Actual: actual, Market: market,
		})
	}
	c := fitCal(rs)
	if c.Market < 0.8 || c.Market > 1.2 {
		t.Errorf("market weight %.2f, want about 1", c.Market)
	}
	if c.Slope > 0.15 {
		t.Errorf("an uninformative model should get about no weight, got %.2f", c.Slope)
	}
}
