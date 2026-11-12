package predictor

import (
	"encoding/json"
	"errors"
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"sort"
)

// TailVersion changes the uncertainty model, not the feature schema. It must use
// separately fitted probability calibration; a v4 calibration cannot price it.
const TailVersion = 5

var alphaMeanEdges = []float64{2, 5, 10, 20, 35}

func meanBucket(mu float64) int {
	for i, e := range alphaMeanEdges {
		if mu < e {
			return i
		}
	}
	return len(alphaMeanEdges)
}
func residualQuantiles(xs []float64, n int) []float64 {
	sort.Float64s(xs)
	out := make([]float64, n)
	for i := range out {
		k := int((float64(i) + .5) / float64(n) * float64(len(xs)))
		if k >= len(xs) {
			k = len(xs) - 1
		}
		out[i] = xs[k]
	}
	return out
}

// TrainTails learns role-specific dispersion and a distinct early-exit minutes
// component from later dates a prefix model never trained on. All fit dates
// precede a future caller's evaluation month. This is an opt-in challenger.
func TrainTails(rows []feat.Row, disabled []string) (*Model, error) {
	rows = append([]feat.Row(nil), rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].G.Date < rows[j].G.Date })
	base, e := Train(rows, disabled)
	if e != nil {
		return nil, e
	}
	return FitTailUncertainty(base, rows)
}

// FitTailUncertainty clones an already fitted reference and never mutates it.
func FitTailUncertainty(base *Model, rows []feat.Row) (*Model, error) {
	disabled := base.Disabled
	var played []feat.Row
	for _, r := range rows {
		if r.G.Minutes >= 1 {
			played = append(played, r)
		}
	}
	sort.SliceStable(played, func(i, j int) bool { return played[i].G.Date < played[j].G.Date })
	cut := len(played) * 4 / 5
	for cut > 0 && cut < len(played) && played[cut-1].G.Date == played[cut].G.Date {
		cut--
	}
	if cut < 500 || len(played)-cut < 300 {
		return nil, errors.New("tail model needs 500 earlier fit rows and 300 later residual rows on separate dates")
	}
	prefix, e := Train(played[:cut], disabled)
	if e != nil {
		return nil, e
	}
	if base.Version != 4 || base.Rows != len(played) || base.TrainedBefore != played[len(played)-1].G.Date {
		return nil, errors.New("tail fitter needs a matching v4 training reference")
	}
	encoded, e := json.Marshal(base)
	if e != nil {
		return nil, e
	}
	var cloned Model
	if e = json.Unmarshal(encoded, &cloned); e != nil {
		return nil, e
	}
	final := &cloned
	final.Version = TailVersion
	final.ResidualMethod = "chronological-prefix-early-exit-and-mean-bucket-dispersion"
	final.TailFitAfter = played[cut].G.Date
	nb := len(bucketEdges) + 1
	normal := make([][]float64, nb)
	exits := make([][]float64, nb)
	counts := make([]int, nb)
	type moments struct{ num, den, n float64 }
	var ms [data.NStats][]moments
	for k := range ms {
		ms[k] = make([]moments, len(alphaMeanEdges)+1)
	}
	var normalAll, exitAll []float64
	highN := 0
	for i := cut; i < len(played); i++ {
		r := &played[i]
		p := prefix.Predict(r)
		center := math.Min(math.Max(prefix.Minutes.Predict(r.Vec(data.Points, prefix.Minutes.Names)), 0), maxProjMinutes)
		b := bucketOf(center)
		counts[b]++
		early := center >= 18 && r.G.Minutes < 8
		if center >= 18 {
			highN++
		}
		if early {
			exits[b] = append(exits[b], r.G.Minutes)
			exitAll = append(exitAll, r.G.Minutes)
		} else {
			normal[b] = append(normal[b], r.G.Minutes-center)
			normalAll = append(normalAll, r.G.Minutes-center)
		}
		for _, st := range data.Stats {
			k := data.StatIndex(st)
			mu := r.G.Minutes * p.Rate[k]
			err := r.G.Value(st) - mu
			b := meanBucket(p.Mean(st))
			a := &ms[k][b]
			a.num += err*err - mu
			a.den += mu * mu
			a.n++
		}
	}
	final.MinuteWeights = make([][]float64, nb)
	final.ExitMinutes = make([]float64, nb)
	exitRate := 0.
	exitMean := 4.
	if highN > 0 {
		exitRate = float64(len(exitAll)) / float64(highN)
	}
	if len(exitAll) > 0 {
		exitMean = 0
		for _, m := range exitAll {
			exitMean += m
		}
		exitMean /= float64(len(exitAll))
	}
	for b := range normal {
		// Very low-minute roles keep their existing uncertainty; they do not inherit
		// a starter's early-exit mechanism.
		if b == 0 || counts[b] < 250 {
			continue
		}
		res := normal[b]
		if len(res) < 200 {
			res = normalAll
		}
		if len(res) < 200 {
			continue
		}
		risk := (float64(len(exits[b])) + 50*exitRate) / (float64(counts[b]) + 50)
		risk = math.Min(math.Max(risk, 0), .15)
		meanExit := exitMean
		if len(exits[b]) > 0 {
			total := 20 * exitMean
			for _, v := range exits[b] {
				total += v
			}
			meanExit = total / (20 + float64(len(exits[b])))
		}
		q := make([]float64, Nodes)
		copy(q[1:], residualQuantiles(append([]float64(nil), res...), Nodes-1))
		final.Quant[b] = q
		w := make([]float64, Nodes)
		w[0] = risk
		for j := 1; j < Nodes; j++ {
			w[j] = (1 - risk) / float64(Nodes-1)
		}
		final.MinuteWeights[b] = w
		final.ExitMinutes[b] = meanExit
	}
	for k := range ms {
		final.AlphaBuckets[k] = make([]float64, len(ms[k]))
		prior := final.Stat[k].Alpha
		for b, a := range ms[k] {
			alpha := prior
			if a.den > 0 {
				raw := math.Min(math.Max(a.num/a.den, 1e-4), 3)
				alpha = (a.n*raw + 500*prior) / (a.n + 500)
			}
			final.AlphaBuckets[k][b] = alpha
		}
	}
	return final, nil
}

// NodeWeight is uniform for v3/v4 or hand-built predictions. V5 may assign a
// small explicit early-exit mass. It is used identically by singles and combos.
func (p Pred) NodeWeight(k int) float64 {
	sum := 0.
	for _, w := range p.MinWeights {
		sum += w
	}
	if sum <= 0 {
		return 1. / Nodes
	}
	return p.MinWeights[k] / sum
}
