package bets

import (
	"math"
	"testing"
)

func TestImplied(t *testing.T) {
	if math.Abs(Implied(-110)-0.5238095) > 1e-6 {
		t.Errorf("-110: %v", Implied(-110))
	}
	if math.Abs(Implied(150)-0.4) > 1e-9 || Implied(0) != Implied(-110) {
		t.Error("+150 should be 40% and unknown odds should default to -110")
	}
}

func TestKellyAndUnits(t *testing.T) {
	// At -110 a 55% bet: b = 0.909, kelly = (0.909*0.55 - 0.45)/0.909 = 0.055.
	if k := Kelly(0.55, -110); math.Abs(k-0.055) > 0.001 {
		t.Errorf("kelly %v", k)
	}
	if Units(0.52, -110, 2) != 0 {
		t.Error("52% at -110 has no edge, so no stake")
	}
	if u := Units(0.55, -110, 2); u != 1.5 {
		t.Errorf("55%% at -110 should stake 1.5u (quarter Kelly is 1.375, rounded), got %v", u)
	}
	if u := Units(0.75, -110, 2); u != 2 {
		t.Errorf("stake should be capped at 2u, got %v", u)
	}
}

func TestMarketOver(t *testing.T) {
	if MarketOver(-110, -110) != 0.5 || MarketOver(0, -110) != 0.5 {
		t.Error("even prices or missing prices mean 50%")
	}
	// Over -150 / under +120: the book thinks over is likelier than 50%.
	if p := MarketOver(-150, 120); p < 0.55 || p > 0.60 {
		t.Errorf("over -150 / under +120 should imply about 57%% over, got %v", p)
	}
}
