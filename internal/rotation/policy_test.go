package rotation

import (
	"math"
	"testing"
)

func TestSupportedTargetRejectsFallbackAndIrrelevantAllocations(t *testing.T) {
	a := Allocation{Valid: true, Supported: true, MatchedGames: 3, RelevantOut: []int{1}, Out: []int{1}, Minutes: map[int]float64{2: 32}}
	if target, ok := SupportedTarget(a, 2); !ok || target != 32 {
		t.Fatal(target, ok)
	}
	for _, mutate := range []func(*Allocation){
		func(a *Allocation) { a.Valid = false },
		func(a *Allocation) { a.Supported = false },
		func(a *Allocation) { a.MatchedGames = 2 },
		func(a *Allocation) { a.RelevantOut = nil },
		func(a *Allocation) { a.Out = append(a.Out, 2) },
	} {
		b := a
		mutate(&b)
		if _, ok := SupportedTarget(b, 2); ok {
			t.Fatal("unsupported allocation accepted", b)
		}
	}
	for _, target := range []float64{0, 49, math.NaN(), math.Inf(1)} {
		a.Minutes[2] = target
		if _, ok := SupportedTarget(a, 2); ok {
			t.Fatal("invalid target accepted", target)
		}
	}
}

func TestPoliciesHaveIndependentReplayAndResultPaths(t *testing.T) {
	a, b := Files(Policy)
	c, d := Files(StrictPolicy)
	if a == c || b == d || EvidenceDirectory(Policy) == EvidenceDirectory(StrictPolicy) {
		t.Fatal("research histories overlap")
	}
	if KnownPolicy("unknown") {
		t.Fatal("unknown policy accepted")
	}
}

func TestFallbackPolicyHasIsolatedEvidenceAndRecords(t *testing.T) {
	if !KnownPolicy(FallbackPolicy) || !ProductionPolicy(FallbackPolicy) {
		t.Fatal("fallback unsupported")
	}
	a, b := Files(FallbackPolicy)
	for _, p := range []string{Policy, StrictPolicy} {
		c, d := Files(p)
		if a == c || b == d || EvidenceDirectory(p) == EvidenceDirectory(FallbackPolicy) {
			t.Fatal("policy histories mixed")
		}
	}
}

func TestContextPolicyKeepsItsOwnRecord(t *testing.T) {
	if !KnownPolicy(ContextPolicy) || !ProductionPolicy(ContextPolicy) {
		t.Fatal("context policy unavailable")
	}
	a, b := Files(ContextPolicy)
	for _, p := range []string{Policy, StrictPolicy, FallbackPolicy} {
		c, d := Files(p)
		if a == c || b == d || EvidenceDirectory(p) == EvidenceDirectory(ContextPolicy) {
			t.Fatal("records mixed")
		}
	}
}


func TestGuardedV5HasIndependentReplayAndResultFiles(t *testing.T) {
    if !KnownPolicy(GuardedPolicy) || !ProductionPolicy(GuardedPolicy) {t.Fatal("V5 not a known tracked policy")}
    log, settle := Files(GuardedPolicy)
    oldLog, oldSettle := Files(ContextPolicy)
    if log==oldLog || settle==oldSettle || EvidenceDirectory(GuardedPolicy)==EvidenceDirectory(ContextPolicy) {
        t.Fatal("V5 histories must not mix with V4, old checkpoints or results")
    }
}
