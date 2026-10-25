package predictor

import (
	"math"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/synthworld"
)

func world(t *testing.T) (*synthworld.World, *feat.Builder, []feat.Row) {
	t.Helper()
	w := synthworld.Generate(synthworld.Config{Seed: 3, Seasons: []int{2023, 2024}, Teams: 20, Days: 150})
	b := feat.NewBuilder(w.Games, w.Adv)
	rows := b.Rows(12)
	if len(rows) < 5000 {
		t.Fatalf("only %d rows", len(rows))
	}
	return w, b, rows
}

func TestTrainAndPredictBeatsBaseline(t *testing.T) {
	_, _, rows := world(t)
	cut := len(rows) * 3 / 4
	train, test := rows[:cut], rows[cut:]
	m, err := Train(train, nil)
	if err != nil {
		t.Fatal(err)
	}
	var seV2, seBase, n float64
	for i := range test {
		r := &test[i]
		p := m.Predict(r)
		actual := r.G.Pts
		seV2 += (p.Mean(data.Points) - actual) * (p.Mean(data.Points) - actual)
		pj := math.Exp(r.S[0][0]) * r.X[0] // baseline per-minute rate x recent-average minutes
		seBase += (pj - actual) * (pj - actual)
		n++
	}
	rv2, rb := math.Sqrt(seV2/n), math.Sqrt(seBase/n)
	t.Logf("points RMSE: v2 %.3f  baseline %.3f", rv2, rb)
	if rv2 >= rb {
		t.Errorf("full model RMSE %.3f should beat the baseline %.3f in a world with real context effects", rv2, rb)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	_, _, rows := world(t)
	m, err := Train(rows[:6000], []string{feat.GroupSlump})
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/model.json"
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	m2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &rows[7000]
	a, b := m.Predict(r), m2.Predict(r)
	if a.Mean(data.Points) != b.Mean(data.Points) {
		t.Errorf("round trip changed predictions: %v vs %v", a.Mean(data.Points), b.Mean(data.Points))
	}
}

func TestProbOverIsSane(t *testing.T) {
	_, _, rows := world(t)
	m, _ := Train(rows[:8000], nil)
	p := m.Predict(&rows[9000])
	mean := p.Mean(data.Points)
	lo, _ := p.ProbOver(data.Points, math.Floor(mean)-8+0.5)
	hi, _ := p.ProbOver(data.Points, math.Floor(mean)+8+0.5)
	mid, _ := p.ProbOver(data.Points, math.Floor(mean)+0.5)
	if !(lo.Over > mid.Over && mid.Over > hi.Over) {
		t.Errorf("P(over) should fall as the line rises: %v %v %v", lo.Over, mid.Over, hi.Over)
	}
	if mid.Over < 0.3 || mid.Over > 0.7 {
		t.Errorf("a line at the mean should be near a coin flip, got %v", mid.Over)
	}
	whole, _ := p.ProbOver(data.Points, math.Floor(mean))
	if math.Abs(whole.Over+whole.Under-1) > 1e-9 || whole.Push <= 0 {
		t.Errorf("whole-number line: over+under should be 1 with a push chance, got %+v", whole)
	}
}

func TestVolatilityIsShrunkAndBounded(t *testing.T) {
	_, _, rows := world(t)
	m, err := Train(rows, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Vol) == 0 {
		t.Fatal("no volatility ratios")
	}
	var lo, hi = 10.0, 0.0
	for _, v := range m.Vol {
		if v < volMin-1e-9 || v > volMax+1e-9 {
			t.Fatalf("ratio %v outside [%v,%v]", v, volMin, volMax)
		}
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	t.Logf("volatility ratios range %.2f to %.2f over %d players", lo, hi, len(m.Vol))
}
