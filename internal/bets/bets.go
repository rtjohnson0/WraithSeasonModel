// Package bets has the money math: implied probability, edge and stake sizing.
package bets

import "math"

// Implied is the break-even win probability for American odds (-110 if odds is 0).
func Implied(odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	if odds > 0 {
		return 100 / (float64(odds) + 100)
	}
	return -float64(odds) / (-float64(odds) + 100)
}

// Edge is the model probability minus the break-even probability.
func Edge(p float64, odds int) float64 { return p - Implied(odds) }

// Kelly is the full-Kelly fraction of bankroll to stake. Negative values mean no bet.
func Kelly(p float64, odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	var b float64 // profit per unit staked
	if odds > 0 {
		b = float64(odds) / 100
	} else {
		b = 100 / -float64(odds)
	}
	return (b*p - (1 - p)) / b
}

// Units is a conservative stake in units of 1% of bankroll: a quarter of full Kelly, rounded
// to the nearest 0.25 unit, never above maxUnits, and 0 when there is no edge. Full Kelly
// assumes the probability is exactly right, which it never is, so it is far too aggressive.
func Units(p float64, odds int, maxUnits float64) float64 {
	k := Kelly(p, odds) / 4 * 100
	if k <= 0 {
		return 0
	}
	u := math.Round(k*4) / 4
	if u < 0.25 {
		u = 0.25
	}
	return math.Min(u, maxUnits)
}

// MarketOver is the sportsbook's own probability of OVER implied by its two prices, with the
// vig removed. It is 0.5 when either price is missing. A line posted at over -150 / under +120
// is not a 50/50 proposition: the book is saying over is more likely, and a model has to beat
// THAT, not a coin flip.
func MarketOver(overOdds, underOdds int) float64 {
	if overOdds == 0 || underOdds == 0 {
		return 0.5
	}
	po, pu := Implied(overOdds), Implied(underOdds)
	if po+pu <= 0 {
		return 0.5
	}
	return po / (po + pu)
}
