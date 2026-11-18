package lineup

import (
	"propsmodel/internal/data"
	"reflect"
	"testing"
)

func TestPooledRoleFitExcludesSameDayAndFutureAndDoesNotNeedMatchingAbsences(t *testing.T) {
	rows, roster := fixture(2)
	var history []data.PlayerGame
	history = append(history, rows...)
	for _, r := range rows {
		r.GameID += 100
		r.Date = "2026-08-" + r.Date[8:]
		history = append(history, r)
	}
	ix := New(history)
	m := ix.FitRoles("2026-10-01")
	if m.N[0] < 500 || len(m.Coef[0]) != 14 {
		t.Fatal("league pool did not fit", m)
	}
	baseline, used := map[int]float64{}, map[int]float64{}
	for p, tm := range roster {
		if tm == 2 {
			baseline[p] = 24
			used[p] = 240. / 9
		}
	}
	delete(used, 201)
	effect := ix.RoleImpact(m, 2, 101, "2026-10-01", roster, []int{201}, true, baseline, used)
	if !effect.Effects[0].Applied || effect.Effects[0].Role == nil || effect.Effects[0].Without != 0 {
		t.Fatal("exact absence samples still required", effect)
	}
	healthy := ix.RoleImpact(m, 2, 101, "2026-10-01", roster, []int{201}, true, baseline, baseline)
	for _, e := range healthy.Effects {
		if e.Scale != 1 {
			t.Fatal("unchanged composition moved rates", e)
		}
	}
	for _, r := range rows {
		r.Date = "2026-10-01"
		r.GameID += 1000
		r.Reb *= 100
		history = append(history, r)
	}
	if !reflect.DeepEqual(m, New(history).FitRoles("2026-10-01")) {
		t.Fatal("future leaked into pooled model")
	}
	// Production cannot be invented for a rotation with unknown player profiles.
	used[999999] = 100
	got := ix.RoleImpact(m, 2, 101, "2026-10-01", roster, []int{201}, true, baseline, used)
	for _, e := range got.Effects {
		if e.Applied {
			t.Fatal("unknown replacement changed rates", got)
		}
	}
}
