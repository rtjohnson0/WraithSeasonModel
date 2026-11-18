package rotation

import "math"

// ProjectRoles redistributes the minutes vacated by confirmed outs toward
// observed same-role replacements. This is a projected rotation, not confirmed
// starters. It needs no matching injury games and never invents roster players.
func ProjectRoles(a Allocation, positions map[int]string) Allocation {
	if !a.Valid || len(a.RelevantOut) == 0 || a.MatchedGames >= 3 {
		return a
	}
	missing := map[string]float64{}
	vacated := 0.
	for _, pid := range a.RelevantOut {
		p := positions[pid]
		if p == "" {
			return a
		}
		m := a.Baseline[pid]
		missing[p] += m
		vacated += m
	}
	if vacated <= 0 {
		return a
	}
	out := map[int]bool{}
	for _, pid := range a.Out {
		out[pid] = true
	}
	used := map[int]float64{}
	weights := map[int]float64{}
	caps := map[int]float64{}
	for pid, m := range a.Baseline {
		if out[pid] || m <= 0 {
			continue
		}
		p := positions[pid]
		if p == "" {
			return a
		}
		used[pid] = m
		caps[pid] = math.Min(48, math.Max(m, math.Min(40, m+12)))
		compatibility := 0.
		for role, minutes := range missing {
			similarity := .15
			if role == p {
				similarity = 1
			} else if (role == "C" || role == "B") && (p == "C" || p == "B") {
				similarity = .8
			} else if (role == "W" || role == "F") && (p == "W" || p == "F") {
				similarity = .8
			}
			compatibility += minutes / vacated * similarity
		}
		weights[pid] = math.Sqrt(m) * compatibility
	}
	// Bounded water fill of only the vacated workload. Existing workload stays
	// intact; overtime and a second 240-minute normalization cannot add exposure.
	remaining := vacated
	for iter := 0; iter < len(used)+1 && remaining > 1e-8; iter++ {
		total := 0.
		for pid, w := range weights {
			if used[pid] < caps[pid]-1e-8 {
				total += w
			}
		}
		if total <= 0 {
			return a
		}
		spent := 0.
		for pid, w := range weights {
			extra := math.Min(caps[pid]-used[pid], remaining*w/total)
			extra = math.Max(0, extra)
			used[pid] += extra
			spent += extra
		}
		remaining -= spent
	}
	if remaining > 1e-7 {
		return a
	}
	a.RoleProjected = true
	a.Minutes = used
	a.Reason = "projected role replacements redistribute confirmed-out minutes; no exact injury history required; experimental"
	return a
}
