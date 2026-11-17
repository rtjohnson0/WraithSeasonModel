// Package minutesaudit diagnoses frozen predictions; it never changes a live parameter.
package minutesaudit

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type Context struct {
	OwnStatus    string   `json:"ownStatus,omitempty"`
	ConfirmedOut int      `json:"confirmedOut"`
	AssumedOut   int      `json:"assumedOut"`
	Questionable int      `json:"questionable"`
	Doubtful     int      `json:"doubtful"`
	Returning    bool     `json:"returning"`
	B2B          bool     `json:"b2b"`
	Spread       *float64 `json:"spread,omitempty"`
}
type Observation struct {
	Cohort            string   `json:"cohort"`
	Date              string   `json:"date"`
	At                string   `json:"at"`
	GameID            int      `json:"gameId"`
	PlayerID          int      `json:"playerId"`
	Player            string   `json:"player"`
	Stat              string   `json:"stat"`
	Line              float64  `json:"line"`
	Minutes           float64  `json:"projectedMinutes"`
	ActualMinutes     float64  `json:"actualMinutes"`
	Typical           float64  `json:"typicalMinutes"`
	Projection        float64  `json:"projection"`
	Actual            float64  `json:"actual"`
	BeforeScale       *float64 `json:"beforeScale,omitempty"`
	RawOver           *float64 `json:"rawOver,omitempty"`
	FinalOver         *float64 `json:"finalOver,omitempty"`
	ZeroInterceptOver *float64 `json:"zeroInterceptOver,omitempty"`
	TypicalOver       *float64 `json:"typicalOver,omitempty"`
	Context           *Context `json:"context,omitempty"`
}
type MinutesSlice struct {
	Cohort          string  `json:"cohort"`
	Group           string  `json:"group"`
	N               int     `json:"n"`
	Days            int     `json:"days"`
	Predicted       float64 `json:"predicted"`
	Actual          float64 `json:"actual"`
	Bias            float64 `json:"predictedMinusActual"`
	MAE             float64 `json:"mae"`
	MedianAE        float64 `json:"medianAE"`
	P90AE           float64 `json:"p90AE"`
	UnderFive       int     `json:"underBy5Plus"`
	OverFive        int     `json:"overBy5Plus"`
	TypicalN        int     `json:"typicalN"`
	TypicalMAE      float64 `json:"typicalMAE"`
	TypicalModelMAE float64 `json:"modelMAEOnTypicalSubset"`
}
type StatSlice struct {
	RawUnderAboveMean   int     `json:"rawUnderDespiteMeanAtOrAboveLine"`
	Cohort              string  `json:"cohort"`
	Stat                string  `json:"stat"`
	N                   int     `json:"n"`
	Days                int     `json:"days"`
	MeanProjection      float64 `json:"meanProjection"`
	MeanActual          float64 `json:"meanActual"`
	Bias                float64 `json:"actualMinusProjection"`
	MAE                 float64 `json:"mae"`
	MinutesContribution float64 `json:"minutesContribution"`
	RateContribution    float64 `json:"rateContribution"`
	ScaleN              int     `json:"scaleN"`
	MeanScaleChange     float64 `json:"meanScaleChange"`
	ProjectionBelowLine int     `json:"projectionBelowLine"`
	ProbabilityN        int     `json:"probabilityN"`
	RawUnder            int     `json:"rawUnder"`
	FinalUnder          int     `json:"finalUnder"`
	CalibrationToUnder  int     `json:"calibrationToUnder"`
	CalibrationToOver   int     `json:"calibrationToOver"`
	InterceptToUnder    int     `json:"interceptToUnder"`
	TypicalMinutesFlip  int     `json:"typicalMinutesFlip"`
}

func valid(o Observation) bool {
	if o.Date == "" || o.GameID <= 0 || o.PlayerID <= 0 || o.Minutes <= 0 || o.Minutes > 70 || o.ActualMinutes < 0 || o.ActualMinutes > 70 || o.Projection < 0 || o.Actual < 0 {
		return false
	}
	for _, v := range []float64{o.Minutes, o.ActualMinutes, o.Projection, o.Actual, o.Line, o.Typical} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return o.Stat == "points" || o.Stat == "rebounds" || o.Stat == "assists" || o.Stat == "threes"
}

// Decompose gives an exact arithmetic identity, not a causal explanation or forecast.
// actual-projection = extra minutes at the forecast rate + remaining production error.
func Decompose(o Observation) (minutes, rate float64) {
	r := o.Projection / o.Minutes
	return (o.ActualMinutes - o.Minutes) * r, o.Actual - o.ActualMinutes*r
}
func earlier(a, b string) bool {
	at, ea := time.Parse(time.RFC3339Nano, a)
	bt, eb := time.Parse(time.RFC3339Nano, b)
	if ea == nil && eb == nil {
		return at.Before(bt)
	}
	return a < b
}
func questionKey(o Observation) string {
	return fmt.Sprintf("%s/%s/%d/%d/%s", o.Cohort, o.Date, o.GameID, o.PlayerID, o.Stat)
}
func playerKey(o Observation) string {
	return fmt.Sprintf("%s/%s/%d/%d", o.Cohort, o.Date, o.GameID, o.PlayerID)
}

// Dedupe chooses an entire earliest record; it never combines a line/probability from another run.
func Dedupe(obs []Observation) []Observation {
	best := map[string]Observation{}
	for _, o := range obs {
		if !valid(o) {
			continue
		}
		k := questionKey(o)
		p, ok := best[k]
		if !ok || earlier(o.At, p.At) {
			best[k] = o
		}
	}
	out := []Observation{}
	for _, o := range best {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.At != b.At {
			return a.At < b.At
		}
		return questionKey(a) < questionKey(b)
	})
	return out
}
func Summary(obs []Observation) ([]MinutesSlice, []StatSlice) {
	obs = Dedupe(obs)
	players := map[string]Observation{}
	cohorts := map[string]bool{}
	for _, o := range obs {
		cohorts[o.Cohort] = true
		k := playerKey(o)
		if p, ok := players[k]; !ok || earlier(o.At, p.At) {
			players[k] = o
		}
	}
	cohortNames := []string{}
	for c := range cohorts {
		cohortNames = append(cohortNames, c)
	}
	sort.Strings(cohortNames)
	type group struct {
		name string
		keep func(Observation) bool
	}
	groups := []group{{"all player-games", func(o Observation) bool { return true }}, {"projected 28+ minutes", func(o Observation) bool { return o.Minutes >= 28 }}, {"projected below 28 minutes", func(o Observation) bool { return o.Minutes < 28 }}, {"projected 5+ below typical", func(o Observation) bool { return o.Typical > 0 && o.Minutes <= o.Typical-5 }}, {"availability context not captured", func(o Observation) bool { return o.Context == nil }}, {"confirmed teammate out", func(o Observation) bool { return o.Context != nil && o.Context.ConfirmedOut > 0 }}, {"questionable/doubtful teammate", func(o Observation) bool { return o.Context != nil && o.Context.Questionable+o.Context.Doubtful > 0 }}, {"own status uncertain", func(o Observation) bool { return o.Context != nil && o.Context.OwnStatus != "" }}, {"returning from absence", func(o Observation) bool { return o.Context != nil && o.Context.Returning }}, {"back-to-back", func(o Observation) bool { return o.Context != nil && o.Context.B2B }}, {"pregame spread 10+", func(o Observation) bool {
		return o.Context != nil && o.Context.Spread != nil && *o.Context.Spread >= 10
	}}, {"observed DNP (postgame diagnostic)", func(o Observation) bool { return o.ActualMinutes == 0 }}}
	mins := []MinutesSlice{}
	stats := []StatSlice{}
	for _, c := range cohortNames {
		for _, g := range groups {
			s := MinutesSlice{Cohort: c, Group: g.name}
			days := map[string]bool{}
			errors := []float64{}
			keys := []string{}
			for k := range players {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				o := players[k]
				if o.Cohort != c || !g.keep(o) {
					continue
				}
				s.N++
				days[o.Date] = true
				s.Predicted += o.Minutes
				s.Actual += o.ActualMinutes
				err := o.Minutes - o.ActualMinutes
				s.Bias += err
				s.MAE += math.Abs(err)
				errors = append(errors, math.Abs(err))
				if err <= -5 {
					s.UnderFive++
				}
				if err >= 5 {
					s.OverFive++
				}
				if o.Typical > 0 {
					s.TypicalN++
					s.TypicalMAE += math.Abs(o.Typical - o.ActualMinutes)
					s.TypicalModelMAE += math.Abs(o.Minutes - o.ActualMinutes)
				}
			}
			if s.N == 0 {
				continue
			}
			s.Days = len(days)
			n := float64(s.N)
			s.Predicted /= n
			s.Actual /= n
			s.Bias /= n
			s.MAE /= n
			sort.Float64s(errors)
			s.MedianAE = errors[(len(errors)-1)/2]
			s.P90AE = errors[int(math.Ceil(.9*n))-1]
			if s.TypicalN > 0 {
				s.TypicalMAE /= float64(s.TypicalN)
				s.TypicalModelMAE /= float64(s.TypicalN)
			}
			mins = append(mins, s)
		}
		for _, st := range []string{"points", "rebounds", "assists", "threes"} {
			s := StatSlice{Cohort: c, Stat: st}
			days := map[string]bool{}
			for _, o := range obs {
				if o.Cohort != c || o.Stat != st || o.ActualMinutes == 0 {
					continue
				}
				s.N++
				days[o.Date] = true
				s.MeanProjection += o.Projection
				s.MeanActual += o.Actual
				s.Bias += o.Actual - o.Projection
				s.MAE += math.Abs(o.Actual - o.Projection)
				a, b := Decompose(o)
				s.MinutesContribution += a
				s.RateContribution += b
				if o.BeforeScale != nil {
					s.ScaleN++
					s.MeanScaleChange += o.Projection - *o.BeforeScale
				}
				if o.Projection < o.Line {
					s.ProjectionBelowLine++
				}
				if o.RawOver != nil && o.FinalOver != nil {
					s.ProbabilityN++
					raw, final := *o.RawOver, *o.FinalOver
					if raw < .5 {
						s.RawUnder++
						if o.Projection >= o.Line {
							s.RawUnderAboveMean++
						}
					}
					if final < .5 {
						s.FinalUnder++
					}
					if raw >= .5 && final < .5 {
						s.CalibrationToUnder++
					}
					if raw < .5 && final >= .5 {
						s.CalibrationToOver++
					}
					if o.ZeroInterceptOver != nil && *o.ZeroInterceptOver >= .5 && final < .5 {
						s.InterceptToUnder++
					}
					if o.TypicalOver != nil && (*o.TypicalOver < .5) != (final < .5) {
						s.TypicalMinutesFlip++
					}
				}
			}
			if s.N == 0 {
				continue
			}
			s.Days = len(days)
			n := float64(s.N)
			s.MeanProjection /= n
			s.MeanActual /= n
			s.Bias /= n
			s.MAE /= n
			s.MinutesContribution /= n
			s.RateContribution /= n
			if s.ScaleN > 0 {
				s.MeanScaleChange /= float64(s.ScaleN)
			}
			stats = append(stats, s)
		}
	}
	return mins, stats
}
