package opportunity

import (
	"math"
	"propsmodel/internal/data"
	"sort"
	"time"
)

type Metric struct {
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Games    int     `json:"games"`
	Attempts float64 `json:"attempts,omitempty"`
}

// Styles are descriptive earlier-game measurements. No defense multiplier is
// inferred from these small samples; the numeric rate model is fitted separately.
func (ix *Index) Styles(player int, date string) []Metric {
	specs := []struct{ typ, rng, num, den, label, unit string }{
		{"possessions", "", "touches", "min", "Touches per minute", "/min"},
		{"drives", "", "drives", "", "Drives per covered game", "/game"},
		{"catchshoot", "", "catch_shoot_fg3_a", "", "Catch-and-shoot 3PA", "/game"},
		{"pullupshot", "", "pull_up_fg3_a", "", "Pull-up 3PA", "/game"},
		{"shooting_distance", "6_plus_ft", "fg3_a", "", "Wide-open 3PA (6+ ft)", "/game"},
		{"defense_rim", "", "fgm_lt_06", "fga_lt_06", "FG allowed inside 6 ft", "%"},
		{"defense_threes", "", "fg3_m", "fg3_a", "3PT FG allowed", "%"},
	}
	var out []Metric
	for _, s := range specs {
		var rows []Observation
		for k, r := range ix.Rows {
			stamp, err := time.Parse(time.RFC3339Nano, r.Captured)
			if k.Player == player && k.Type == s.typ && k.Range == s.rng && r.Date < date && data.DayNumber(r.Date) >= data.DayNumber(date)-90 && err == nil && (ix.AsOf.IsZero() || !stamp.After(ix.AsOf)) {
				rows = append(rows, r)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
		if len(rows) > 10 {
			rows = rows[:10]
		}
		num, den, n := 0., 0., 0
		for _, r := range rows {
			v := r.Value(s.num)
			d := 1.
			if s.den != "" {
				d = r.Value(s.den)
			}
			if math.IsNaN(v) || math.IsNaN(d) || d < 0 {
				continue
			}
			num += v
			den += d
			n++
		}
		if n >= 3 && den > 0 {
			value := num / den
			if s.unit == "%" {
				value *= 100
			}
			out = append(out, Metric{s.label, value, s.unit, n, den})
		}
	}
	return out
}
