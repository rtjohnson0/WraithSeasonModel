// Package model turns a predicted stat distribution and a sportsbook line into
// P(over) / P(under), and applies the posting rule: OVER at >=55%, UNDER at <=45%.
package model

import (
	"errors"
	"fmt"
	"math"
)

const (
	PostOver        = 0.55 // post an OVER at or above this P(over)
	PostUnder       = 0.45 // post an UNDER at or below this P(over)
	MinMinutesFloor = 18.0 // eligible if projected for at least this many minutes
	MinMinutesAvg   = 20.0 // ...or averaging at least this many
)

// Eligible reports whether a player qualifies: 20+ average minutes, or projected for 18+.
func Eligible(avgMinutes, projectedMinutes float64) bool {
	return avgMinutes >= MinMinutesAvg || projectedMinutes >= MinMinutesFloor
}

// Probs holds the outcome probabilities for one line. Over and Under are renormalised
// with pushes removed (a push only happens on whole-number lines).
type Probs struct {
	Over  float64
	Under float64
	Push  float64
}

// nbParams converts a mean and variance into negative binomial (n, p). The variance must
// exceed the mean, so it is nudged up if it does not.
func nbParams(mean, variance float64) (n, p float64) {
	if floor := mean*1.0001 + 1e-6; variance < floor {
		variance = floor
	}
	p = mean / variance
	n = mean * p / (1 - p)
	return n, p
}

func nbPMF(k int, n, p float64) float64 {
	lgKN, _ := math.Lgamma(float64(k) + n)
	lgN, _ := math.Lgamma(n)
	lgK1, _ := math.Lgamma(float64(k) + 1)
	return math.Exp(lgKN - lgN - lgK1 + n*math.Log(p) + float64(k)*math.Log(1-p))
}

func nbCDF(k int, n, p float64) float64 {
	sum := 0.0
	for i := 0; i <= k; i++ {
		sum += nbPMF(i, n, p)
	}
	return math.Min(sum, 1)
}

// ProbOver returns P(over), P(under) and P(push) for a stat with the given predicted mean
// and variance against a line. Half-point lines (24.5) cannot push.
func ProbOver(mean, variance, line float64) (Probs, error) {
	if mean <= 0 || variance <= 0 || line < 0 {
		return Probs{}, errors.New("mean, variance must be positive and line non-negative")
	}
	n, p := nbParams(mean, variance)

	var push, overRaw float64
	if line == math.Floor(line) {
		k := int(line)
		push = nbPMF(k, n, p)
		overRaw = 1 - nbCDF(k, n, p)
	} else {
		overRaw = 1 - nbCDF(int(math.Floor(line)), n, p)
	}
	underRaw := 1 - overRaw - push
	denom := overRaw + underRaw
	return Probs{Over: overRaw / denom, Under: underRaw / denom, Push: push}, nil
}

// Pick is a posted recommendation.
type Pick struct {
	Player string
	Stat   string
	Line   float64
	Side   string  // "OVER" or "UNDER"
	Prob   float64 // model probability for the chosen side
}

// MakePick returns a Pick if the player is eligible and the model clears 55/45, else nil.
func MakePick(player, stat string, line, mean, variance, avgMinutes, projectedMinutes float64) (*Pick, error) {
	if !Eligible(avgMinutes, projectedMinutes) {
		return nil, nil
	}
	pr, err := ProbOver(mean, variance, line)
	if err != nil {
		return nil, err
	}
	switch {
	case pr.Over >= PostOver:
		return &Pick{player, stat, line, "OVER", pr.Over}, nil
	case pr.Over <= PostUnder:
		return &Pick{player, stat, line, "UNDER", pr.Under}, nil
	}
	return nil, nil
}

// CalRow is one row of a calibration table.
type CalRow struct {
	Bin       string
	N         int
	Predicted float64 // mean predicted P(over) in the bin
	Actual    float64 // share of bets in the bin that actually went over
}

// DefaultBins are the calibration bin edges.
var DefaultBins = []float64{0, 0.40, 0.45, 0.50, 0.55, 0.60, 0.70, 1.0}

// CalibrationTable checks whether stated probabilities match reality. wentOver[i] is true if
// bet i went over (drop pushes first). Within each bin the actual over-rate should sit near
// the mean predicted probability; if it does not, the model is miscalibrated.
func CalibrationTable(pOver []float64, wentOver []bool, bins []float64) []CalRow {
	var rows []CalRow
	for b := 0; b+1 < len(bins); b++ {
		lo, hi := bins[b], bins[b+1]
		last := b+2 == len(bins)
		var n, overs int
		var sumP float64
		for i, p := range pOver {
			in := p >= lo && p < hi
			if last {
				in = p >= lo && p <= hi
			}
			if !in {
				continue
			}
			n++
			sumP += p
			if wentOver[i] {
				overs++
			}
		}
		if n == 0 {
			continue
		}
		rows = append(rows, CalRow{
			Bin:       binLabel(lo, hi),
			N:         n,
			Predicted: sumP / float64(n),
			Actual:    float64(overs) / float64(n),
		})
	}
	return rows
}

func binLabel(lo, hi float64) string {
	return fmt.Sprintf("%.2f-%.2f", lo, hi)
}
