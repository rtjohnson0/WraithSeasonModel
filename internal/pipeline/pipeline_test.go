package pipeline

import (
	"strings"
	"testing"

	"propsmodel/internal/feat"
)

func TestMissedGroupIsOptIn(t *testing.T) {
	defer func() { Missed = false }()
	Missed = false
	off := DisabledFor(false)
	base := ConfigName(off)
	if !contains(off, feat.GroupMisses) || strings.HasSuffix(base, "-misses") {
		t.Fatalf("misses must be off by default: %v %s", off, base)
	}
	Missed = true
	on := DisabledFor(false)
	if contains(on, feat.GroupMisses) {
		t.Fatalf("-missed should switch the group on: %v", on)
	}
	if got := ConfigName(on); got != base+"-misses" {
		t.Errorf("enabling the feature must change the config name so the calibration is refitted: %s vs %s", got, base)
	}
	// The teammate switch is independent of it.
	if ConfigName(DisabledFor(true)) == ConfigName(DisabledFor(false)) {
		t.Error("lineup and pregame configs must differ")
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
