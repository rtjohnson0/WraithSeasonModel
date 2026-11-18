package lineup

import (
	"math"
	"propsmodel/internal/data"
)

// Component describes an individual-absence association. Sample counts are
// games, never summed across players: the same game can inform several components.
type Component struct {
	PlayerID int     `json:"playerId"`
	Without  int     `json:"without"`
	With     int     `json:"with"`
	Weight   float64 `json:"weight"`
	Scale    float64 `json:"scale"`
}

// EstimateFallback preserves supported joint estimates. For unsupported stats,
// it averages supported individual-absence estimates rather than multiplying or
// adding them. Unknown components retain their share at neutral, so one observed
// absence cannot stand in for an entire missing lineup. This is an observational
// association with historical replacement lineups, not a causal player rating.
func (ix *Index) EstimateFallback(team, player int, date string, roster map[int]int, outs []int, opponent bool) Impact {
	out := ix.Estimate(team, player, date, roster, outs, opponent)
	if len(out.Missing) < 2 || len(out.Effects) == 0 {
		return out
	}
	singles := make([]Impact, len(out.Missing))
	for i, p := range out.Missing {
		singles[i] = ix.Estimate(team, player, date, roster, []int{p.PlayerID}, opponent)
	}
	for k := range out.Effects {
		e := &out.Effects[k]
		if e.Applied {
			continue
		}
		// Minutes measure the absent rotation exposure. For opponent production,
		// weight by the missing player's observed production in the relevant stat.
		// This supplies relevance, not a presumed direction or a transferred stat total.
		weights := make([]float64, len(out.Missing))
		total := 0.
		for i, p := range out.Missing {
			w := p.Minutes
			if opponent {
				switch e.Stat {
				case data.Points:
					w = p.Minutes // no defensive scoring rating is inferred
				case data.Rebounds:
					w = p.Rebounds
				case data.Assists:
					w = p.Minutes
				case data.Threes:
					w = p.Minutes
				}
			}
			weights[i] = math.Max(0, w)
			total += weights[i]
		}
		if total <= 0 {
			continue
		}
		delta := 0.
		for i, one := range singles {
			if len(one.Effects) <= k || !one.Effects[k].Applied {
				continue
			}
			component := one.Effects[k]
			w := weights[i] / total
			// Extra shrinkage and a tighter cap because co-absences are uncontrolled.
			scale := 1 + .5*(component.Scale-1)
			delta += w * (scale - 1)
			e.Evidence = append(e.Evidence, Component{PlayerID: out.Missing[i].PlayerID, Without: component.Without, With: component.With, Weight: w, Scale: scale})
		}
		if len(e.Evidence) == 0 {
			continue
		}
		e.Scale = math.Max(.975, math.Min(1.025, 1+delta))
		e.Applied = true
		e.Method = "individual-mixture"
		e.Reason = "weighted individual-absence associations; unobserved shares neutral; no stacking; capped at ±2.5%; co-absence confounding remains"
	}
	out.Reason = "joint evidence preferred; individual-mixture fallback where supported; observational research, not causal or validated"
	return out
}
