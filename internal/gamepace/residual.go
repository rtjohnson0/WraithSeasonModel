package gamepace

import (
	"math"
	"propsmodel/internal/data"
	"sort"
)

// ResidualStudy tests a predeclared recency correction on out-of-fold point
// forecasts. It is not sportsbook calibration or evidence of betting edge.
type ResidualStudy struct {
	Policy             string  `json:"policy"`
	Games              int     `json:"games"`
	Dates              int     `json:"dates"`
	RawMarginMAE       float64 `json:"rawMarginMAE"`
	CorrectedMarginMAE float64 `json:"correctedMarginMAE"`
	RawTotalMAE        float64 `json:"rawTotalMAE"`
	CorrectedTotalMAE  float64 `json:"correctedTotalMAE"`
	Note               string  `json:"note"`
}

// ResidualCorrection pools only strictly earlier dates within 180 days, with
// a 45-day half-life and 100 neutral pseudo-games. Parameters are fixed before
// inspecting the new study; same-day outcomes cannot affect another game.
func ResidualCorrection(rows []Replay, date string) (margin, total float64, n int) {
	weight := 100.
	seen := map[int]bool{}
	for _, r := range rows {
		age := data.DayNumber(date) - data.DayNumber(r.Date)
		if r.Date >= date || age <= 0 || age > 180 || seen[r.GameID] || !residualFinite(r) {
			continue
		}
		seen[r.GameID] = true
		n++
		w := math.Exp(-math.Ln2 * float64(age) / 45)
		margin += w * (r.ActualMargin - r.Margin)
		total += w * (r.ActualTotal - r.Total)
		weight += w
	}
	return margin / weight, total / weight, n
}

func StudyResiduals(rows []Replay) ResidualStudy {
	out := ResidualStudy{Policy: "game-residual-study-v1", Note: "Historical point forecasts only, no injury reconstruction or sportsbook edge claim. Corrections use earlier out-of-fold forecasts; live picks unchanged."}
	ordered := append([]Replay(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Date != ordered[j].Date {
			return ordered[i].Date < ordered[j].Date
		}
		return ordered[i].GameID < ordered[j].GameID
	})
	dates := map[string]bool{}
	seen := map[int]bool{}
	for _, r := range ordered {
		if seen[r.GameID] || !residualFinite(r) {
			continue
		}
		seen[r.GameID] = true
		m, t, n := ResidualCorrection(ordered, r.Date)
		if n < 200 {
			continue
		}
		out.Games++
		dates[r.Date] = true
		out.RawMarginMAE += math.Abs(r.ActualMargin - r.Margin)
		out.CorrectedMarginMAE += math.Abs(r.ActualMargin - r.Margin - m)
		out.RawTotalMAE += math.Abs(r.ActualTotal - r.Total)
		out.CorrectedTotalMAE += math.Abs(r.ActualTotal - r.Total - t)
	}
	out.Dates = len(dates)
	if out.Games > 0 {
		n := float64(out.Games)
		out.RawMarginMAE /= n
		out.CorrectedMarginMAE /= n
		out.RawTotalMAE /= n
		out.CorrectedTotalMAE /= n
	}
	return out
}
func residualFinite(r Replay) bool {
	if data.DayNumber(r.Date) == 0 || r.GameID <= 0 {
		return false
	}
	for _, x := range []float64{r.Margin, r.Total, r.ActualMargin, r.ActualTotal} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}
