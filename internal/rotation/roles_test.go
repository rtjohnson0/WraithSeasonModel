package rotation

import (
	"math"
	"testing"
)

func TestRoleReplacementConservesMinutesWithoutInjuryMatches(t *testing.T) {
	a := Allocation{Valid: true, Out: []int{1}, RelevantOut: []int{1}, Baseline: map[int]float64{}, Minutes: map[int]float64{}}
	pos := map[int]string{}
	for i := 1; i <= 10; i++ {
		a.Baseline[i] = 24
		pos[i] = "G"
	}
	pos[1] = "C"
	pos[2] = "C"
	got := ProjectRoles(a, pos)
	sum := 0.
	for pid, m := range got.Minutes {
		if pid == 1 || m > 40 || m < 24 {
			t.Fatal(got)
		}
		sum += m
	}
	if math.Abs(sum-240) > 1e-8 || got.Minutes[2] <= got.Minutes[3] {
		t.Fatal("missing center workload did not favor observed center replacement", got)
	}
	if len(a.Minutes) != 0 {
		t.Fatal("input mutated")
	}
}
