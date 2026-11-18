// Package gamecontext learns small workload changes associated with pregame
// spreads. Final margins are outcomes for a separate risk diagnostic, never inputs.
package gamecontext

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/linalg"
	"sort"
)

type Model struct {
	Before             string
	N, Days            int
	Coef               []float64
	ReferenceSpread    float64
	ReferenceSigned    float64
	RiskN              [4]int
	RiskEvents         [4]int
	RiskTotal, RiskAll int
}
type Estimate struct {
	Applied         bool      `json:"applied"`
	Reason          string    `json:"reason"`
	Before          string    `json:"before"`
	N               int       `json:"playerGames"`
	Days            int       `json:"days"`
	Spread          float64   `json:"spread"`
	Reference       float64   `json:"referenceSpread"`
	ReferenceSigned float64   `json:"referenceSigned"`
	Typical         float64   `json:"typicalMinutes"`
	Shift           float64   `json:"minutesShift"`
	Coefficients    []float64 `json:"coefficients,omitempty"`
	Risk            *float64  `json:"largeMarginRisk,omitempty"`
	RiskGames       int       `json:"riskGames"`
}

func bin(s float64) int {
	if s < 4 {
		return 0
	}
	if s < 8 {
		return 1
	}
	if s < 12 {
		return 2
	}
	return 3
}
func features(spread, typical float64) []float64 {
	return []float64{math.Abs(spread) / 10, math.Abs(spread) / 10 * typical / 36, spread / 10}
}
func Fit(rows []data.PlayerGame, lines map[[2]int]feat.GameLine, before string) *Model {
	m := &Model{Before: before}
	a := make([][]float64, 3)
	for i := range a {
		a[i] = make([]float64, 3)
		a[i][i] = 100
	}
	rhs := make([]float64, 3)
	sorted := append([]data.PlayerGame(nil), rows...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Date != sorted[j].Date {
			return sorted[i].Date < sorted[j].Date
		}
		return sorted[i].PlayerID < sorted[j].PlayerID
	})
	history := map[int][]data.PlayerGame{}
	dates := map[string]bool{}
	seenGames := map[int]bool{}
	seen := map[[2]int]bool{}
	for _, r := range sorted {
		if r.Date >= before || !r.SourceFinal || !r.BoxScoreComplete {
			continue
		}
		key := [2]int{r.GameID, r.PlayerID}
		if seen[key] {
			continue
		}
		seen[key] = true
		gl, has := lines[[2]int{data.DayNumber(r.Date), r.TeamID}]
		if has && !math.IsNaN(gl.Spread) && !math.IsInf(gl.Spread, 0) && math.Abs(gl.Spread) <= 30 {
			if !seenGames[r.GameID] {
				seenGames[r.GameID] = true
				b := bin(math.Abs(gl.Spread))
				m.RiskN[b]++
				m.RiskTotal++
				if math.Abs(r.FinalHomeScore-r.FinalAwayScore) >= 15 {
					m.RiskEvents[b]++
					m.RiskAll++
				}
			}
			hs := history[r.PlayerID]
			sum, n := 0., 0
			for i := len(hs) - 1; i >= 0 && n < 10; i-- {
				if hs[i].Date < r.Date && data.DayNumber(hs[i].Date) >= data.DayNumber(r.Date)-180 {
					sum += hs[i].Minutes
					n++
				}
			}
			if n >= 5 && r.Minutes > 0 {
				typical := sum / float64(n)
				x := features(gl.Spread, typical)
				y := math.Max(-20, math.Min(20, r.Minutes-typical))
				for i := range x {
					rhs[i] += x[i] * y
					for j := range x {
						a[i][j] += x[i] * x[j]
					}
				}
				m.N++
				dates[r.Date] = true
				m.ReferenceSpread += math.Abs(gl.Spread)
				m.ReferenceSigned += gl.Spread
			}
		}
		if r.Minutes > 0 {
			history[r.PlayerID] = append(history[r.PlayerID], r)
		}
	}
	m.Days = len(dates)
	if m.N > 0 {
		m.ReferenceSpread /= float64(m.N)
		m.ReferenceSigned /= float64(m.N)
	}
	if m.N >= 1000 && m.Days >= 30 {
		m.Coef, _ = linalg.SolveSPD(a, rhs)
	}
	return m
}
func (m *Model) Estimate(date string, spread, typical float64) Estimate {
	e := Estimate{Before: m.Before, N: m.N, Days: m.Days, Reason: "historical pregame spreads insufficient"}
	if m.Before != date || math.IsNaN(spread) || math.IsInf(spread, 0) || math.Abs(spread) > 30 || typical <= 0 {
		return e
	}
	e.Spread = spread
	e.Reference = m.ReferenceSpread
	e.ReferenceSigned = m.ReferenceSigned
	e.Typical = typical
	b := bin(math.Abs(spread))
	if m.RiskTotal >= 100 && m.RiskN[b] >= 20 {
		p := (float64(m.RiskEvents[b]) + 50*float64(m.RiskAll)/float64(m.RiskTotal)) / float64(m.RiskN[b]+50)
		e.Risk = &p
		e.RiskGames = m.RiskN[b]
	}
	if len(m.Coef) != 3 {
		return e
	}
	x := features(spread, typical)
	ref := []float64{m.ReferenceSpread / 10, m.ReferenceSpread / 10 * typical / 36, m.ReferenceSigned / 10}
	shift := 0.
	for i := range x {
		shift += (x[i] - ref[i]) * m.Coef[i]
	}
	e.Shift = math.Max(-3, math.Min(3, shift))
	e.Applied = true
	e.Coefficients = append([]float64(nil), m.Coef...)
	e.Reason = "pooled spread/workload association relative to training reference; bounded ±3 minutes; experimental"
	return e
}
