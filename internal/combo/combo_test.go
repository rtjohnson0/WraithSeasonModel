package combo

import (
	"math"
	"testing"

	"propsmodel/internal/data"
	"propsmodel/internal/predictor"
)

func pred(mins float64) predictor.Pred {
	var p predictor.Pred
	for i := range p.MinNodes {
		p.MinNodes[i] = mins
	}
	p.Minutes = mins
	p.Rate[data.StatIndex(data.Points)] = 0.6
	p.Rate[data.StatIndex(data.Rebounds)] = 0.25
	p.Rate[data.StatIndex(data.Assists)] = 0.2
	p.Alpha[data.StatIndex(data.Points)] = 0.1
	p.Alpha[data.StatIndex(data.Rebounds)] = 0.15
	p.Alpha[data.StatIndex(data.Assists)] = 0.2
	return p
}

func TestMeansAddUp(t *testing.T) {
	d := Simulate(pred(30), Corr{}, 4000, 1)
	// 30 min: points 18, rebounds 7.5, assists 6
	want := map[data.Stat]float64{data.PRA: 31.5, data.PR: 25.5, data.PA: 24, data.RA: 13.5}
	for s, w := range want {
		if got := d.Mean(s); math.Abs(got-w) > 0.15 {
			t.Errorf("%s mean = %.2f, want about %.1f", s, got, w)
		}
	}
}

func TestIndependentVariance(t *testing.T) {
	d := Simulate(pred(30), Corr{}, 6000, 2)
	var want float64
	for _, c := range []struct{ mean, alpha float64 }{{18, 0.1}, {7.5, 0.15}, {6, 0.2}} {
		want += c.mean * (1 + c.alpha*c.mean)
	}
	got := d.SD(data.PRA)
	if math.Abs(got*got-want)/want > 0.06 {
		t.Errorf("PRA variance = %.1f, want about %.1f for independent stats", got*got, want)
	}
}

func TestCorrelationWidensTheSum(t *testing.T) {
	ind := Simulate(pred(30), Corr{}, 5000, 3).SD(data.PRA)
	pos := Simulate(pred(30), Corr{PR: 0.3, PA: 0.3, RA: 0.3}, 5000, 3).SD(data.PRA)
	if pos <= ind*1.05 {
		t.Errorf("positive correlation should widen the sum: independent sd %.2f, correlated %.2f", ind, pos)
	}
}

func TestMinutesTieStatsTogether(t *testing.T) {
	steady := Simulate(pred(30), Corr{}, 3000, 4).SD(data.PRA)
	p := pred(30)
	for i := range p.MinNodes {
		p.MinNodes[i] = 20 + float64(i)*20.0/float64(len(p.MinNodes)-1) // 20..40, mean 30
	}
	wide := Simulate(p, Corr{}, 3000, 4).SD(data.PRA)
	if wide <= steady*1.1 {
		t.Errorf("a shared minutes spread should widen the combo: steady %.2f, spread %.2f", steady, wide)
	}
}

func TestProbOver(t *testing.T) {
	d := Simulate(pred(30), Default, 3000, 5)
	lo, _ := d.ProbOver(data.PRA, 25.5)
	hi, _ := d.ProbOver(data.PRA, 38.5)
	if !(lo.Over > 0.65 && hi.Over < 0.3 && lo.Over > hi.Over) {
		t.Errorf("P(over 25.5) = %.2f, P(over 38.5) = %.2f", lo.Over, hi.Over)
	}
	whole, _ := d.ProbOver(data.PRA, 31)
	if whole.Push < 0.02 {
		t.Errorf("a whole-number line near the mean should push sometimes, got %.3f", whole.Push)
	}
	if math.Abs(whole.Over+whole.Under-1) > 1e-9 {
		t.Error("over and under must add to one once pushes are removed")
	}
	if _, err := d.ProbOver(data.Points, 20); err == nil {
		t.Error("a single stat is not a combo")
	}
}

func TestRepeatable(t *testing.T) {
	a, _ := Simulate(pred(28), Default, 1000, 9).ProbOver(data.PA, 20.5)
	b, _ := Simulate(pred(28), Default, 1000, 9).ProbOver(data.PA, 20.5)
	if a != b {
		t.Error("same seed must give the same answer")
	}
}

func TestCholNotPositiveDefinite(t *testing.T) {
	l := chol(Corr{PR: 0.8, PA: 0.8, RA: -0.8}) // impossible together
	for i := 0; i < 3; i++ {
		if math.IsNaN(l[i][i]) || l[i][i] <= 0 {
			t.Fatalf("bad factor %v", l)
		}
	}
}
