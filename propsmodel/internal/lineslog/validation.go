package lineslog

import (
	"math"
	"sort"
)

// priorRates uses only complete earlier dates. Forty pseudo observations at 50%
// prevent a tiny first slate from creating an extreme benchmark.
func priorRates(gm []GradedMarket) map[[2]string]float64 {
	byDate := map[string][]GradedMarket{}
	for _, g := range gm {
		if !g.Push {
			byDate[g.Date] = append(byDate[g.Date], g)
		}
	}
	var dates []string
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	out := map[[2]string]float64{}
	n, overs := map[string]float64{}, map[string]float64{}
	for _, d := range dates {
		for _, g := range byDate[d] {
			out[[2]string{d, g.Stat}] = (overs[g.Stat] + 20) / (n[g.Stat] + 40)
		}
		for _, g := range byDate[d] {
			n[g.Stat]++
			if g.Over {
				overs[g.Stat]++
			}
		}
	}
	return out
}

// Comparison scores both models on the identical stored question and timestamp.
type Comparison struct {
	N, Days                 int
	Brier1, Brier2, Gain, Z float64
}

func CompareModels(gm []GradedMarket) Comparison {
	var c Comparison
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, g := range gm {
		if g.Push || g.P2 <= 0 || g.POver <= 0 {
			continue
		}
		y := 0.
		if g.Over {
			y = 1
		}
		a, b := (g.POver-y)*(g.POver-y), (g.P2-y)*(g.P2-y)
		c.N++
		c.Brier1 += a
		c.Brier2 += b
		sums[g.Date] += a - b
		counts[g.Date]++
	}
	c.Days = len(sums)
	if c.N == 0 {
		return c
	}
	c.Brier1 /= float64(c.N)
	c.Brier2 /= float64(c.N)
	c.Gain = c.Brier1 - c.Brier2
	var ss float64
	for d, s := range sums {
		v := s - c.Gain*float64(counts[d])
		ss += v * v
	}
	if c.Days > 1 {
		g := float64(c.Days)
		se := math.Sqrt(g/(g-1)*ss) / float64(c.N)
		if se > 0 {
			c.Z = c.Gain / se
		}
	}
	return c
}
