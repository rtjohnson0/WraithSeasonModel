package eval

import "math"

// MinutesStats summarises one slice of minutes predictions.
type MinutesStats struct {
	N                    int
	ModelMAE, NaiveMAE   float64 // mean absolute error in minutes
	ModelBias, NaiveBias float64 // actual minus projected, so positive means projected too low
}

// SummarizeMinutes scores the samples accepted by keep.
func SummarizeMinutes(samples []MinSample, keep func(MinSample) bool) MinutesStats {
	var s MinutesStats
	for _, x := range samples {
		if !keep(x) {
			continue
		}
		s.N++
		s.ModelMAE += math.Abs(x.Actual - x.Pred)
		s.NaiveMAE += math.Abs(x.Actual - x.Naive)
		s.ModelBias += x.Actual - x.Pred
		s.NaiveBias += x.Actual - x.Naive
	}
	if s.N > 0 {
		n := float64(s.N)
		s.ModelMAE /= n
		s.NaiveMAE /= n
		s.ModelBias /= n
		s.NaiveBias /= n
	}
	return s
}
