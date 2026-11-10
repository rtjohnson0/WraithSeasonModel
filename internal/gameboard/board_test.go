package gameboard

import (
	"math"
	"testing"
)

func TestTotalThresholdAndPush(t *testing.T) {
	o, u, p := TotalProb(220, 15, 220)
	if math.Abs(o-u) > 1e-9 || p <= 0 || math.Abs(o+u+p-1) > 1e-9 {
		t.Fatalf("whole total %v %v %v", o, u, p)
	}
	a, b, c := TotalProb(220, 15, 220.5)
	if c != 0 || math.Abs(a+b-1) > 1e-9 {
		t.Fatal("half point must not push")
	}
	low, _, _ := TotalProb(220, 15, 218.5)
	if low <= a {
		t.Fatal("probability must follow quoted total")
	}
}
func TestPriceAndRefund(t *testing.T) {
	sides := Pair([2]string{"Over", "Under"}, [2]int{-110, -110}, [2]float64{.48, .48}, .04)
	s := sides[0]
	if math.Abs(s.Conditional-.5) > 1e-9 || math.Abs(s.Market-.5) > 1e-9 || s.EV >= 0 || s.Edge >= 0 {
		t.Fatalf("bad price: %+v", s)
	}
}
func TestEmptyReplacesOldBoard(t *testing.T) {
	dir := t.TempDir()
	if e := (Snapshot{Date: "2026-11-10", Games: []Game{{ID: 1}}}).Save(dir); e != nil {
		t.Fatal(e)
	}
	if e := (Snapshot{Date: "2026-11-10"}).Save(dir); e != nil {
		t.Fatal(e)
	}
	s, e := Load(dir, "2026-11-10")
	if e != nil || s == nil || len(s.Games) != 0 {
		t.Fatal("old game survived empty run")
	}
	s, e = Load(dir, "2026-11-11")
	if e != nil || s != nil {
		t.Fatal("wrong date must not publish")
	}
}
