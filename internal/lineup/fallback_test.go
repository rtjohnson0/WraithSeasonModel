package lineup

import (
	"math"
	"propsmodel/internal/data"
	"reflect"
	"testing"
)

func TestFallbackUsesIndividualEvidenceWithoutStackingOrReplacingJoint(t *testing.T) {
	rows, roster := fixture(2)
	ix := New(rows)
	old := ix.Estimate(2, 101, "2026-10-01", roster, []int{201, 202}, true)
	got := ix.EstimateFallback(2, 101, "2026-10-01", roster, []int{201, 202}, true)
	for k, e := range got.Effects {
		if old.Effects[k].Applied || !e.Applied || e.Method != "individual-mixture" || len(e.Evidence) != 1 || e.Evidence[0].PlayerID != 201 {
			t.Fatal(old, got)
		}
		single := ix.Estimate(2, 101, "2026-10-01", roster, []int{201}, true).Effects[k]
		if math.Abs(e.Scale-1) >= math.Abs(single.Scale-1)/2 {
			t.Fatal("unknown player's exposure was redistributed", e, single)
		}
		if e.Scale < .975 || e.Scale > 1.025 {
			t.Fatal(e)
		}
	}
	joint := ix.Estimate(2, 101, "2026-10-01", roster, []int{201}, true)
	if !reflect.DeepEqual(joint, ix.EstimateFallback(2, 101, "2026-10-01", roster, []int{201}, true)) {
		t.Fatal("supported joint changed")
	}
	// The own-production path also supports the fallback.
	own := ix.EstimateFallback(2, 203, "2026-10-01", roster, []int{201, 202}, false)
	if !own.Effects[0].Applied {
		t.Fatal(own)
	}
	// Duplicate input must not multiply a player's contribution.
	if !reflect.DeepEqual(got, ix.EstimateFallback(2, 101, "2026-10-01", roster, []int{202, 201, 201}, true)) {
		t.Fatal("duplicate/order changed fallback")
	}
	// A later game and same-day rows cannot affect the forecast.
	var future []data.PlayerGame
	future = append(future, rows...)
	for _, r := range rows {
		r.GameID += 100
		r.Date = "2026-10-01"
		r.Reb *= 100
		future = append(future, r)
	}
	if !reflect.DeepEqual(got, New(future).EstimateFallback(2, 101, "2026-10-01", roster, []int{201, 202}, true)) {
		t.Fatal("future leak")
	}
}
func TestFallbackDoesNotInferAbsencesFromMissingRows(t *testing.T) {
	rows, roster := fixture(2)
	var missing []data.PlayerGame
	for _, r := range rows {
		if r.PlayerID == 201 && r.Minutes == 0 {
			continue
		}
		missing = append(missing, r)
	}
	got := New(missing).EstimateFallback(2, 101, "2026-10-01", roster, []int{201, 202}, true)
	for _, e := range got.Effects {
		if e.Applied || e.Scale != 1 {
			t.Fatal(e)
		}
	}
}
