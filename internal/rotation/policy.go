package rotation

import "math"

// StrictPolicy is a distinct research policy. Unsupported workload allocations
// must not replace the existing forecast merely because they sum to 240 minutes.
const StrictPolicy = "rotation-v2"

const FallbackPolicy = "rotation-v3"
const ContextPolicy = "rotation-v4"

func ProductionPolicy(policy string) bool {
	return policy == StrictPolicy || policy == FallbackPolicy || policy == ContextPolicy
}
func KnownPolicy(policy string) bool { return policy == Policy || ProductionPolicy(policy) }

func Files(policy string) (log, settled string) {
	if policy == ContextPolicy {
		return "v2_contextlog.jsonl", "v2_contextsettled.jsonl"
	}
	if policy == FallbackPolicy {
		return "v2_rotation_fallbacklog.jsonl", "v2_rotation_fallbacksettled.jsonl"
	}
	if policy == StrictPolicy {
		return "v2_rotation_strictlog.jsonl", "v2_rotation_strictsettled.jsonl"
	}
	return LogFile, SettledFile
}

func EvidenceDirectory(policy string) string {
	if policy == ContextPolicy {
		return "v2_rotation/context"
	}
	if policy == FallbackPolicy {
		return "v2_rotation/fallback"
	}
	if policy == StrictPolicy {
		return "v2_rotation/strict"
	}
	return "v2_rotation"
}

// SupportedTarget requires a relevant confirmed absence and at least three
// matching prior joint rotations. Current injury freshness/roster checks remain
// the caller's responsibility. This is an evidence floor, not an accuracy claim.
func SupportedTarget(a Allocation, player int) (float64, bool) {
	if !a.Valid || !a.Supported || len(a.RelevantOut) == 0 || a.MatchedGames < 3 {
		return 0, false
	}
	for _, id := range a.Out {
		if id == player {
			return 0, false
		}
	}
	target, ok := a.Minutes[player]
	if !ok || math.IsNaN(target) || math.IsInf(target, 0) || target <= 0 || target > 48 {
		return 0, false
	}
	return target, true
}
