package predictor

import (
	"fmt"
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"sort"
	"strings"
)

// ExplainContext reports fitted-model arithmetic, not a causal attribution.
// Neutralizing the defense ratios holds all other rate inputs fixed.
func (m *Model) ExplainContext(r *feat.Row, st data.Stat) string {
	if st.IsCombo() {
		return "combo uses component rate models with shared projected minutes"
	}
	g := m.Stat[data.StatIndex(st)]
	if g == nil {
		return "opponent feature effect unavailable"
	}
	x := r.Vec(st, g.Names)
	neutral := append([]float64(nil), x...)
	used := []string{}
	for i, n := range g.Names {
		if n == "opp_allow" || n == "pos5_allow" {
			neutral[i] = 0
			used = append(used, n)
		}
	}
	notes := []string{}
	disabled := disabledSet(m.Disabled)
	if disabled[feat.GroupTeammates] || disabled[feat.GroupOppOut] {
		groups := []string{}
		if disabled[feat.GroupTeammates] {
			groups = append(groups, "teammate absences")
		}
		if disabled[feat.GroupOppOut] {
			groups = append(groups, "opponent absences")
		}
		notes = append(notes, "injury feature policy: "+strings.Join(groups, " and ")+" are disabled in this fitted model; displayed injury lists are context, not a learned absence adjustment")
	}

	if len(used) > 0 {
		a, b := g.Rate(x), g.Rate(neutral)
		if b > 0 && !math.IsNaN(a) {
			notes = append(notes, fmt.Sprintf("fitted defense-rate sensitivity %+.1f%% versus league-neutral opponent inputs, other rate inputs held fixed (not the raw rank percentage)", 100*(a/b-1)))
		}
	} else {
		notes = append(notes, "opponent-defense rate features are disabled in this trained model; displayed ranking is context only")
	}
	if m.Minutes != nil {
		type contribution struct {
			name string
			v    float64
		}
		parts := []contribution{}
		v := r.Vec(data.Points, m.Minutes.Names)
		for i, n := range m.Minutes.Names {
			if i >= len(v) || math.IsNaN(v[i]) || m.Minutes.Std[i] == 0 {
				continue
			}
			c := m.Minutes.Coef[i] * (v[i] - m.Minutes.Mean[i]) / m.Minutes.Std[i]
			parts = append(parts, contribution{featureLabel(n), c})
		}
		sort.Slice(parts, func(i, j int) bool { return math.Abs(parts[i].v) > math.Abs(parts[j].v) })
		bits := []string{}
		for i := 0; i < len(parts) && i < 3; i++ {
			bits = append(bits, fmt.Sprintf("%s %+.1f min", parts[i].name, parts[i].v))
		}
		if len(bits) > 0 {
			notes = append(notes, "largest fitted minutes terms versus training-average inputs: "+strings.Join(bits, ", ")+" (before residual mixing and post-fit adjustments)")
		}
	}
	return strings.Join(notes, "; ")
}

func featureLabel(name string) string {
	labels := map[string]string{"m_ewma": "recent minutes trend", "m_last": "last-game minutes", "m_avg5": "last-five minutes", "m_avg30": "longer-run minutes", "m_start": "usual starter workload", "m_blowcut": "past blowout cuts", "m_med10": "typical recent workload", "m_low10": "low-minute outcomes", "rest": "days of rest", "b2b": "back-to-back schedule", "g7": "recent schedule density", "opp_b2b": "opponent rest", "missed": "games missed", "returning": "return from absence", "early": "early-season uncertainty", "early_m": "early-season workload", "home": "home court", "abs_spread": "expected blowout risk", "fav_spread": "favored-team game script", "game_total": "game scoring environment", "star_spread": "starter and blowout interaction", "net_diff": "team strength gap", "pace": "pace environment", "miss_min": "missing teammates' workload", "miss_starters": "missing starters", "miss_pos": "missing positional teammates", "wo_min": "role without absent teammates"}
	if label, ok := labels[name]; ok {
		return label
	}
	return strings.ReplaceAll(name, "_", " ")
}
