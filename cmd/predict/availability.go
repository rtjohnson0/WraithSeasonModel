package main

import (
	"fmt"
	"math"
	"propsmodel/internal/availability"
	"propsmodel/internal/feat"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"sort"
	"strings"
)

// availabilityView is a minutes-only sensitivity comparison, not a newly fitted
// conditional forecast. It never modifies selection, stake, or headline odds.
func availabilityView(ix *availability.Index, fresh map[int]availability.Status, date, at string, teamOf func(int) (int, bool), out map[int]string) func(scored, predictor.Pred, *feat.Row, func(predictor.Pred) (float64, error)) *availability.Context {
	cache := map[string]availability.Evidence{}
	return func(s scored, p predictor.Pred, row *feat.Row, price func(predictor.Pred) (float64, error)) *availability.Context {
		c := &availability.Context{Version: 1, AsOf: at, BaselineMinutes: p.Minutes, BaselineProbability: s.Prob}
		if st, ok := fresh[s.PlayerID]; ok && props.IsUncertain(st.Status) {
			c.OwnStatus = st.Status
		}
		ids := make([]int, 0, len(fresh))
		for id := range fresh {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			status := fresh[id]
			lower := strings.ToLower(status.Status)
			if status.Name == "" || id == s.PlayerID || (!props.IsUncertain(status.Status) && !strings.Contains(lower, "out")) {
				continue
			}
			team, ok := teamOf(id)
			if !ok || team != row.G.TeamID {
				continue
			}
			k := fmt.Sprintf("%d|%d|%d", team, s.PlayerID, id)
			e, ok := cache[k]
			if !ok {
				e = ix.Compare(team, s.PlayerID, id, date)
				cache[k] = e
			}
			e.ReportedAt = status.At
			e.Player = status.Name
			e.Status = status.Status
			e.ConfirmedOut = strings.Contains(lower, "out")
			_, e.AssumedOut = out[id]
			if e.Supported {
				with, without := p.WithMinutes(e.WithMinutes), p.WithMinutes(e.WithoutMinutes)
				e.WithMinutes, e.WithoutMinutes = with.Minutes, without.Minutes
				projection := func(p predictor.Pred) float64 {
					if !s.Stat.IsCombo() {
						return p.Mean(s.Stat)
					}
					sum := 0.
					for _, part := range s.Stat.Parts() {
						sum += p.Mean(part)
					}
					return sum
				}
				e.WithProjection, e.WithoutProjection = projection(with), projection(without)
				if price != nil {
					a, ea := price(with)
					b, eb := price(without)
					if ea == nil && eb == nil && !math.IsNaN(a) && !math.IsNaN(b) && a >= 0 && a <= 1 && b >= 0 && b <= 1 {
						if s.Side == "UNDER" {
							a, b = 1-a, 1-b
						}
						e.WithProbability, e.WithoutProbability = &a, &b
						e.Reverses = (a >= .5) != (b >= .5)
					}
				}
			}
			c.Teammates = append(c.Teammates, e)
		}
		if c.OwnStatus == "" && len(c.Teammates) == 0 {
			return nil
		}
		return c
	}
}
