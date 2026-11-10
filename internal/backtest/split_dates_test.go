package backtest

import "testing"

func TestSplitNeverSeparatesSameGameDay(t *testing.T) {
	rs := []Result{{Date: "b"}, {Date: "a"}, {Date: "b"}, {Date: "b"}, {Date: "c"}}
	first, second := Split(rs)
	seen := map[string]bool{}
	for _, r := range first {
		seen[r.Date] = true
	}
	for _, r := range second {
		if seen[r.Date] {
			t.Fatalf("date %s in calibration and test", r.Date)
		}
	}
}
