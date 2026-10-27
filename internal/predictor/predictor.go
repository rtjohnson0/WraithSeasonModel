// Package predictor is the full model. For one player in one game it predicts a distribution
// over minutes, then a negative binomial distribution of the stat given those minutes, and
// mixes the two into a probability of going over a line:
//
//	minutes  ~ ridge regression on context + empirical residual quantiles (skewed, fat-tailed)
//	stat     ~ NB( minutes * exp(x . beta), alpha )   (per-minute rate depends on context)
//	P(over)  = average over the minutes distribution of P( NB(...) > line )
//
// Modelling minutes separately is the biggest accuracy lever for props: most of the surprise
// in a player's points, rebounds or assists is surprise in how long they play.
package predictor

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"sort"

	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/glm"
	"propsmodel/internal/model"
)

// Nodes is how many points represent the minutes distribution.
const Nodes = 15

const modelVersion = 3

// maxProjMinutes caps the central minutes projection. No one is projected past a heavy-starter
// load, even when many teammates are out and the regression extrapolates.
const maxProjMinutes = 40

var bucketEdges = []float64{16, 22, 28, 33}

// UseVolatility switches the per-player minutes spread on. The backtest's -novol flag turns it
// off so the two can be compared on the same lines.
var UseVolatility = true

const (
	volShrink = 20 // pseudo-games of "typical" volatility every player starts with
	volMin    = 0.75
	volMax    = 1.35
)

// Model is a trained predictor. It serialises to JSON.
type Model struct {
	Version       int         `json:"version"`
	Disabled      []string    `json:"disabled"`
	TrainedBefore string      `json:"trained_before"`
	Rows          int         `json:"rows"`
	Minutes       *glm.Linear `json:"minutes"`
	Edges         []float64   `json:"edges"`
	Quant         [][]float64 `json:"quant"` // residual quantiles of minutes, per predicted-minutes bucket
	BucketSD      []float64   `json:"bucket_sd"`
	// Vol scales the minutes spread for one player: above 1 for players whose minutes swing more
	// than others at the same projected level, below 1 for steady ones. It is shrunk hard toward
	// 1 (see volShrink), so a handful of games cannot make anyone look unusually safe or wild.
	Vol  map[int]float64 `json:"vol"`
	Stat [data.NStats]*glm.Model
}

func disabledSet(names []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

func bucketOf(pred float64) int {
	for i, e := range bucketEdges {
		if pred < e {
			return i
		}
	}
	return len(bucketEdges)
}

func quantiles(res []float64) []float64 {
	sort.Float64s(res)
	q := make([]float64, Nodes)
	for k := range q {
		pos := (float64(k) + 0.5) / Nodes * float64(len(res))
		i := int(pos)
		if i >= len(res) {
			i = len(res) - 1
		}
		q[k] = res[i]
	}
	return q
}

// Train fits the model on historical rows. disabled lists feature groups to leave out.
func Train(rows []feat.Row, disabledGroups []string) (*Model, error) {
	disabled := disabledSet(disabledGroups)
	var played []*feat.Row
	for i := range rows {
		if rows[i].G.Minutes >= 1 {
			played = append(played, &rows[i])
		}
	}
	if len(played) < 500 {
		return nil, errors.New("predictor: need at least 500 training rows")
	}
	m := &Model{Version: modelVersion, Disabled: disabledGroups, Rows: len(played), Edges: bucketEdges}
	last := ""
	for _, r := range played {
		if r.G.Date > last {
			last = r.G.Date
		}
	}
	m.TrainedBefore = last

	// Minutes model.
	minNames := feat.MinutesNames(disabled)
	X := make([][]float64, len(played))
	y := make([]float64, len(played))
	for i, r := range played {
		X[i] = r.Vec(data.Points, minNames)
		y[i] = r.G.Minutes
	}
	lin, err := glm.FitLinear(X, y, minNames, 5)
	if err != nil {
		return nil, err
	}
	m.Minutes = lin
	buckets := make([][]float64, len(bucketEdges)+1)
	var all []float64
	for i := range played {
		pred := lin.Predict(X[i])
		res := y[i] - pred
		buckets[bucketOf(pred)] = append(buckets[bucketOf(pred)], res)
		all = append(all, res)
	}
	global := quantiles(append([]float64(nil), all...))
	sdOf := func(v []float64) float64 {
		var ss float64
		for _, x := range v {
			ss += x * x
		}
		if len(v) == 0 {
			return 1
		}
		return math.Max(math.Sqrt(ss/float64(len(v))), 0.5)
	}
	m.Quant = make([][]float64, len(buckets))
	m.BucketSD = make([]float64, len(buckets))
	for i, b := range buckets {
		if len(b) < 300 {
			m.Quant[i] = global
			m.BucketSD[i] = sdOf(all)
		} else {
			m.Quant[i] = quantiles(b)
			m.BucketSD[i] = sdOf(b)
		}
	}
	// Per-player minutes volatility: how large his misses are relative to the typical miss at
	// the same projected level, shrunk toward 1.
	zsq := map[int]float64{}
	zn := map[int]float64{}
	for i, r := range played {
		pred := lin.Predict(X[i])
		z := (y[i] - pred) / m.BucketSD[bucketOf(pred)]
		zsq[r.G.PlayerID] += z * z
		zn[r.G.PlayerID]++
	}
	m.Vol = map[int]float64{}
	for id, n := range zn {
		v := math.Sqrt((zsq[id] + volShrink) / (n + volShrink))
		m.Vol[id] = math.Min(math.Max(v, volMin), volMax)
	}

	// Stat models, conditional on the minutes actually played.
	logMin := make([]float64, len(played))
	for i, r := range played {
		logMin[i] = math.Log(r.G.Minutes)
	}
	for _, s := range data.Stats {
		names := feat.StatNames(s, disabled)
		XS := make([][]float64, len(played))
		ys := make([]float64, len(played))
		for i, r := range played {
			XS[i] = r.Vec(s, names)
			ys[i] = r.G.Value(s)
		}
		g, err := glm.Fit(XS, ys, logMin, names, 10)
		if err != nil {
			return nil, err
		}
		m.Stat[data.StatIndex(s)] = g
	}
	return m, nil
}

// Pred is the model's view of one player-game.
type Pred struct {
	Minutes  float64 // average of the minutes distribution
	MinNodes [Nodes]float64
	Rate     [data.NStats]float64 // per-minute mean for points, rebounds, assists, threes
	Alpha    [data.NStats]float64
}

// Predict applies the model to a feature row.
func (m *Model) Predict(r *feat.Row) Pred {
	disabled := disabledSet(m.Disabled)
	var p Pred
	pred := m.Minutes.Predict(r.Vec(data.Points, m.Minutes.Names))
	pred = math.Min(math.Max(pred, 0), maxProjMinutes) // a long-run starter load; blocks wild extrapolation
	q := m.Quant[bucketOf(pred)]
	vol := 1.0
	if v, ok := m.Vol[r.G.PlayerID]; ok && UseVolatility {
		vol = v
	}
	for k := 0; k < Nodes; k++ {
		p.MinNodes[k] = math.Min(math.Max(pred+vol*q[k], 1), 48)
		p.Minutes += p.MinNodes[k] / Nodes
	}
	for _, s := range data.Stats {
		k := data.StatIndex(s)
		g := m.Stat[k]
		p.Rate[k] = g.Rate(r.Vec(s, feat.StatNames(s, disabled)))
		p.Alpha[k] = g.Alpha
	}
	return p
}

// Mean is the expected value of a stat.
// Scaled returns a copy whose mean for stat s is multiplied by f (the bias correction).
func (p Pred) Scaled(s data.Stat, f float64) Pred {
	p.Rate[data.StatIndex(s)] *= f
	return p
}

// WithMinutes shifts the whole minutes distribution so its average is target, keeping the
// per-minute rates. Used to pull a projection toward the player's normal minutes.
func (p Pred) WithMinutes(target float64) Pred {
	d := target - p.Minutes
	p.Minutes = 0
	for k := range p.MinNodes {
		p.MinNodes[k] = math.Min(math.Max(p.MinNodes[k]+d, 1), 48)
		p.Minutes += p.MinNodes[k] / Nodes
	}
	return p
}

// BlendMinutes moves the projected minutes the share w of the way to typical (his normal minutes).
// w = 0 changes nothing. A typical of zero or NaN is ignored.
func (p Pred) BlendMinutes(typical, w float64) Pred {
	if w <= 0 || typical <= 0 || math.IsNaN(typical) {
		return p
	}
	return p.WithMinutes(p.Minutes + w*(typical-p.Minutes))
}

func (p Pred) Mean(s data.Stat) float64 { return p.Minutes * p.Rate[data.StatIndex(s)] }

// Dist is the probability of finishing with 0..maxK of a stat, mixing over the minutes distribution
// the same way ProbOver does. Values are rounded to three decimals and the empty tail is trimmed.
func (p Pred) Dist(s data.Stat, maxK int) []float64 {
	k := data.StatIndex(s)
	out := make([]float64, maxK+1)
	for _, mins := range p.MinNodes {
		mean := mins * p.Rate[k]
		pm := model.PMF(mean, mean*(1+p.Alpha[k]*mean), maxK)
		for i, v := range pm {
			out[i] += v / Nodes
		}
	}
	end := len(out)
	for end > 1 && out[end-1] < 0.0005 {
		end--
	}
	out = out[:end]
	for i := range out {
		out[i] = math.Round(out[i]*1000) / 1000
	}
	return out
}

// ProbOver is P(stat > line) with pushes removed, mixing over the minutes distribution.
func (p Pred) ProbOver(s data.Stat, line float64) (model.Probs, error) {
	k := data.StatIndex(s)
	var over, under, push float64
	for _, mins := range p.MinNodes {
		mean := mins * p.Rate[k]
		pr, err := model.ProbOver(mean, mean*(1+p.Alpha[k]*mean), line)
		if err != nil {
			return model.Probs{}, err
		}
		over += pr.Over * (1 - pr.Push)
		under += pr.Under * (1 - pr.Push)
		push += pr.Push
	}
	denom := over + under
	if denom <= 0 {
		return model.Probs{}, errors.New("predictor: degenerate distribution")
	}
	return model.Probs{Over: over / denom, Under: under / denom, Push: push / Nodes}, nil
}

// Save writes the model as JSON.
func (m *Model) Save(path string) error {
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads a model saved by Save.
func Load(path string) (*Model, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Model
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Minutes == nil || m.Stat[0] == nil || m.Stat[1] == nil || m.Stat[2] == nil || m.Version != modelVersion {
		return nil, errors.New("predictor: model file is incomplete or from another version; retrain with go run ./cmd/train")
	}
	return &m, nil
}

// Importance lists a stat model's coefficients (on standardised features), largest first.
func (m *Model) Importance(s data.Stat) []Coef {
	g := m.Stat[data.StatIndex(s)]
	out := make([]Coef, len(g.Names))
	for i, n := range g.Names {
		out[i] = Coef{n, g.Coef[i]}
	}
	sort.Slice(out, func(i, j int) bool { return math.Abs(out[i].Value) > math.Abs(out[j].Value) })
	return out
}

// Coef is one named coefficient.
type Coef struct {
	Name  string
	Value float64
}
