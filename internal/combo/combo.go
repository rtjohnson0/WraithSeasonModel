// Package combo prices the combined player markets books list next to the single stats: points +
// rebounds + assists (PRA), points + rebounds (PR), points + assists (PA) and rebounds + assists
// (RA).
//
// The single-stat model gives, for one player-game, a distribution of minutes and a negative
// binomial for each stat given the minutes (internal/predictor). A combined stat is the sum of
// those, so it cannot be priced by adding probabilities: the parts move together. Two things tie
// them.
//
//  1. Minutes. One minutes draw is shared by every stat in the sum. A blowout that cuts his night
//     short cuts points, rebounds and assists at once. This is most of the link and is already in
//     the model, so the simulation takes each stat's draw at the same minutes node.
//  2. What is left once minutes are fixed: a hot shooting night with the ball in his hands also
//     brings assists, a rebound-heavy night comes with misses. That is a small positive
//     correlation, tied together here with a Gaussian copula. `go run ./cmd/combos fit` measures
//     it from the season's games; until then Default (small, positive) is used.
//
// The result is a histogram of the sum for each combined stat, from which any line's P(over) is read
// off. It is a fixed-seed simulation, so the same inputs always give the same answer.
package combo

import (
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"os"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/model"
	"propsmodel/internal/predictor"
)

// Corr is the correlation between the three stats' outcomes once minutes are fixed.
type Corr struct {
	PR      float64 `json:"pr"` // points and rebounds
	PA      float64 `json:"pa"` // points and assists
	RA      float64 `json:"ra"` // rebounds and assists
	Samples int     `json:"samples,omitempty"`
	Season  int     `json:"season,omitempty"`
	Updated string  `json:"updated,omitempty"`
}

// Default is used until `go run ./cmd/combos fit` has measured the real thing. The values are
// deliberately small: stats that share the same minutes are already tied through the minutes draw.
var Default = Corr{PR: 0.08, PA: 0.12, RA: 0.05}

// Fitted reports whether the numbers were measured from games (as opposed to Default).
func (c Corr) Fitted() bool { return c.Samples > 0 }

// Save writes the correlations as JSON.
func (c Corr) Save(path string) error {
	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads data/combocorr.json, or returns Default if there is none.
func Load(path string) Corr {
	b, err := os.ReadFile(path)
	if err != nil {
		return Default
	}
	var c Corr
	if json.Unmarshal(b, &c) != nil {
		return Default
	}
	return c
}

// chol returns the lower Cholesky factor of the 3x3 correlation matrix for (points, rebounds,
// assists). If the matrix is not positive definite the off-diagonals are halved until it is.
func chol(c Corr) [3][3]float64 {
	clamp := func(x float64) float64 { return math.Max(-0.8, math.Min(0.8, x)) }
	r01, r02, r12 := clamp(c.PR), clamp(c.PA), clamp(c.RA)
	for try := 0; try < 20; try++ {
		var l [3][3]float64
		l[0][0] = 1
		l[1][0] = r01
		d1 := 1 - l[1][0]*l[1][0]
		l[2][0] = r02
		if d1 > 1e-9 {
			l[1][1] = math.Sqrt(d1)
			l[2][1] = (r12 - l[2][0]*l[1][0]) / l[1][1]
			d2 := 1 - l[2][0]*l[2][0] - l[2][1]*l[2][1]
			if d2 > 1e-9 {
				l[2][2] = math.Sqrt(d2)
				return l
			}
		}
		r01, r02, r12 = r01/2, r02/2, r12/2
	}
	return [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
}

// Dist is the simulated distribution of each combined stat.
type Dist struct {
	pmf map[data.Stat][]float64 // pmf[s][k] = P(sum == k)
}

const maxCount = 160 // no single stat is simulated past this

// nbCDF returns the cumulative distribution of a negative binomial with the given mean and
// variance, long enough to cover essentially all of its mass.
func nbCDF(mean, variance float64) []float64 {
	if mean <= 1e-9 {
		return []float64{1}
	}
	maxK := int(math.Min(maxCount, math.Ceil(mean+9*math.Sqrt(math.Max(variance, mean))+10)))
	pm := model.PMF(mean, variance, maxK)
	cdf := make([]float64, len(pm))
	var run float64
	for i, v := range pm {
		run += v
		cdf[i] = run
	}
	// Close the tail so a draw can never fall off the end.
	cdf[len(cdf)-1] = 1
	return cdf
}

func inv(cdf []float64, u float64) int {
	i := sort.SearchFloat64s(cdf, u)
	if i >= len(cdf) {
		i = len(cdf) - 1
	}
	return i
}

func phi(z float64) float64 { return 0.5 * (1 + math.Erf(z/math.Sqrt2)) }

// Simulate draws perNode outcomes at each of the prediction's minutes nodes (every node is equally
// likely) and builds the distribution of every combined stat. seed makes it repeatable.
func Simulate(p predictor.Pred, c Corr, perNode int, seed int64) *Dist {
	if perNode < 100 {
		perNode = 100
	}
	l := chol(c)
	rng := rand.New(rand.NewSource(seed))
	pi, ri, ai := data.StatIndex(data.Points), data.StatIndex(data.Rebounds), data.StatIndex(data.Assists)
	size := 3*maxCount + 2
	counts := map[data.Stat][]float64{}
	for _, s := range data.Combos {
		counts[s] = make([]float64, size)
	}
	total := 0
	for _, mins := range p.MinNodes {
		var cdf [3][]float64
		for j, k := range [3]int{pi, ri, ai} {
			mean := mins * p.Rate[k]
			cdf[j] = nbCDF(mean, mean*(1+p.Alpha[k]*mean))
		}
		for i := 0; i < perNode; i++ {
			n0, n1, n2 := rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()
			z0 := l[0][0] * n0
			z1 := l[1][0]*n0 + l[1][1]*n1
			z2 := l[2][0]*n0 + l[2][1]*n1 + l[2][2]*n2
			pts, reb, ast := inv(cdf[0], phi(z0)), inv(cdf[1], phi(z1)), inv(cdf[2], phi(z2))
			counts[data.PRA][pts+reb+ast]++
			counts[data.PR][pts+reb]++
			counts[data.PA][pts+ast]++
			counts[data.RA][reb+ast]++
			total++
		}
	}
	d := &Dist{pmf: map[data.Stat][]float64{}}
	for s, cn := range counts {
		end := len(cn)
		for end > 1 && cn[end-1] == 0 {
			end--
		}
		pm := make([]float64, end)
		for i := range pm {
			pm[i] = cn[i] / float64(total)
		}
		d.pmf[s] = pm
	}
	return d
}

// Mean is the expected value of a combined stat.
func (d *Dist) Mean(s data.Stat) float64 {
	var m float64
	for k, v := range d.pmf[s] {
		m += float64(k) * v
	}
	return m
}

// SD is the standard deviation of a combined stat.
func (d *Dist) SD(s data.Stat) float64 {
	m := d.Mean(s)
	var v float64
	for k, p := range d.pmf[s] {
		v += (float64(k) - m) * (float64(k) - m) * p
	}
	return math.Sqrt(v)
}

// CDF is P(sum <= x) for whole x.
func (d *Dist) CDF(s data.Stat, x int) float64 {
	var c float64
	for k, v := range d.pmf[s] {
		if k > x {
			break
		}
		c += v
	}
	return math.Min(c, 1)
}

// ProbOver is P(sum > line) with pushes removed, like predictor.Pred.ProbOver.
func (d *Dist) ProbOver(s data.Stat, line float64) (model.Probs, error) {
	pm, ok := d.pmf[s]
	if !ok {
		return model.Probs{}, errors.New("combo: not a combined stat")
	}
	var over, under, push float64
	for k, v := range pm {
		switch kf := float64(k); {
		case kf > line:
			over += v
		case kf < line:
			under += v
		default:
			push += v
		}
	}
	if over+under <= 0 {
		return model.Probs{}, errors.New("combo: degenerate distribution")
	}
	return model.Probs{Over: over / (over + under), Under: under / (over + under), Push: push}, nil
}

// Support is the number of integer outcomes the distribution covers (for printing a curve).
func (d *Dist) Support(s data.Stat) []float64 { return append([]float64(nil), d.pmf[s]...) }
