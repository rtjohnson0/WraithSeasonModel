package lineup

import (
	"math"
	"propsmodel/internal/data"
	"propsmodel/internal/linalg"
	"sort"
	"time"
)

const roleFeatures = 14

// RoleEvidence freezes the learned league-wide association and its input. The
// historical rotations are observed box-score exposures, not known starters or
// on/off stints; live exposures are projected team rotations.
type RoleEvidence struct {
	Before       string    `json:"before"`
	Games        int       `json:"games"`
	Days         int       `json:"days"`
	Features     []float64 `json:"features"`
	Coefficients []float64 `json:"coefficients"`
	Baseline     []float64 `json:"baseline"`
	Projected    []float64 `json:"projected"`
}
type RoleModel struct {
	Before string
	N      [data.NStats]int
	Days   [data.NStats]int
	Coef   [data.NStats][]float64
}
type roleProfile struct {
	minutes float64
	rate    [5]float64
	pos     string
	games   int
}

func (ix *Index) roleProfile(pid, team int, date string) roleProfile {
	p := roleProfile{}
	hs := ix.history[pid]
	end := sort.Search(len(hs), func(i int) bool { return hs[i].Date >= date })
	for i := end - 1; i >= 0 && p.games < 10; i-- {
		r := hs[i]
		if data.DayNumber(r.Date) < data.DayNumber(date)-730 {
			break
		}
		if p.games == 0 {
			p.pos = Position(r.Pos)
		}
		p.games++
		p.minutes += r.Minutes
		for k, st := range data.Stats {
			p.rate[k] += r.Value(st)
		}
		p.rate[4] += r.Blk
	}
	if p.games < 3 || p.minutes <= 0 {
		return roleProfile{}
	}
	for k := range p.rate {
		p.rate[k] /= p.minutes
	}
	p.minutes /= float64(p.games)
	return p
}

// Exposures describe scoring, rebounding, creation, threes, blocks, center/big
// minutes and guard minutes. Blocks are a box-score proxy, not a rim-defense rating.
func roleExposure(profiles map[int]roleProfile, minutes map[int]float64, exclude int) ([7]float64, bool) {
	var v [7]float64
	total, known := 0., 0.
	for _, pid := range sortedIDs(minutes) {
		m := minutes[pid]
		total += m
	}
	if total <= 0 {
		return v, false
	}
	for _, pid := range sortedIDs(minutes) {
		m := minutes[pid]
		p := profiles[pid]
		if p.games < 3 || p.pos == "" {
			continue
		}
		known += m
		if pid == exclude {
			continue
		}
		m *= 240 / total
		for k := 0; k < 5; k++ {
			v[k] += m * p.rate[k]
		}
		if p.pos == "C" || p.pos == "B" {
			v[5] += m
		}
		if p.pos == "G" {
			v[6] += m
		}
	}
	return v, known/total >= .9
}
func roleDelta(base, used [7]float64) []float64 {
	floors := [7]float64{40, 20, 10, 5, 3, 48, 48}
	x := make([]float64, 7)
	for k := range x {
		x[k] = math.Max(-1, math.Min(1, (used[k]-base[k])/math.Max(floors[k], base[k])))
	}
	return x
}
func (ix *Index) FitRoles(before string) *RoleModel {
	model := &RoleModel{Before: before}
	var a [data.NStats][][]float64
	var rhs [data.NStats][]float64
	dates := [data.NStats]map[string]bool{}
	for k := range data.Stats {
		a[k] = make([][]float64, roleFeatures)
		rhs[k] = make([]float64, roleFeatures)
		dates[k] = map[string]bool{}
		for j := range a[k] {
			a[k][j] = make([]float64, roleFeatures)
			a[k][j][j] = 100
		}
	}
	ix.walkRoleSamples(roleHistoryStart(before), before, func(k int, date string, x []float64, raw, exposure float64) {
		y := math.Max(-1, math.Min(2, raw))
		model.N[k]++
		dates[k][date] = true
		for i := range x {
			rhs[k][i] += x[i] * y
			for j := range x {
				a[k][i][j] += x[i] * x[j]
			}
		}
	})
	for k := range data.Stats {
		model.Days[k] = len(dates[k])
		if model.N[k] >= 500 && model.Days[k] >= 30 {
			model.Coef[k], _ = linalg.SolveSPD(a[k], rhs[k])
		}
	}
	return model
}

// RoleImpact learns across teams; exact player-out sets and repeat head-to-head
// matchups are unnecessary. It evaluates projected minus baseline composition.
func (ix *Index) RoleImpact(model *RoleModel, team, player int, date string, roster map[int]int, outs []int, opponent bool, base, used map[int]float64) Impact {
	out := Impact{Kind: "teammate-production", Team: team, Missing: ix.profiles(team, date, roster, outs), Reason: "pooled lineup-composition association; projected replacements, not confirmed starters or causal injury effects"}
	if opponent {
		out.Kind = "opponent-allowance"
	}
	if len(out.Missing) == 0 {
		return out
	}
	profiles := map[int]roleProfile{}
	for pid, tm := range roster {
		if tm == team {
			profiles[pid] = ix.roleProfile(pid, team, date)
		}
	}
	exclude := player
	if opponent {
		exclude = 0
	}
	changedKnown := true
	for pid, minutes := range base {
		if math.Abs(used[pid]-minutes) > .25 {
			p := profiles[pid]
			if p.games < 3 || p.pos == "" {
				changedKnown = false
			}
		}
	}
	for pid, minutes := range used {
		if math.Abs(base[pid]-minutes) > .25 {
			p := profiles[pid]
			if p.games < 3 || p.pos == "" {
				changedKnown = false
			}
		}
	}
	b, ok1 := roleExposure(profiles, base, exclude)
	ok1 = ok1 && changedKnown
	u, ok2 := roleExposure(profiles, used, exclude)
	for k, st := range data.Stats {
		e := Effect{Stat: st, Scale: 1, Method: "pooled-lineup", Reason: "pooled model or projected-lineup coverage unavailable"}
		if model != nil && model.Before == date && len(model.Coef[k]) == roleFeatures && ok1 && ok2 {
			x := make([]float64, roleFeatures)
			offset := 0
			if opponent {
				offset = 7
			}
			copy(x[offset:], roleDelta(b, u))
			delta := 0.
			for j, v := range x {
				delta += v * model.Coef[k][j]
			}
			e.Scale = math.Max(.95, math.Min(1.05, 1+delta))
			e.Applied = true
			e.Role = &RoleEvidence{Before: model.Before, Games: model.N[k], Days: model.Days[k], Features: x, Coefficients: append([]float64(nil), model.Coef[k]...), Baseline: append([]float64(nil), b[:]...), Projected: append([]float64(nil), u[:]...)}
			e.Reason = "league-pooled composition model; projected minus baseline exposure; capped at ±5%; unvalidated research"
		}
		out.Effects = append(out.Effects, e)
	}
	return out
}

func sortedIDs[T any](m map[int]T) []int {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func (ix *Index) walkRoleSamples(after, before string, visit func(int, string, []float64, float64, float64)) {
	for _, g := range ix.games {
		if g.date >= before || g.date < after {
			continue
		}
		profiles := map[int]roleProfile{}
		base := map[int]map[int]float64{}
		used := map[int]map[int]float64{}
		for _, team := range sortedIDs(g.teams) {
			rs := g.teams[team]
			base[team] = map[int]float64{}
			used[team] = map[int]float64{}
			for _, pid := range sortedIDs(rs) {
				r := rs[pid]
				p := ix.roleProfile(pid, team, g.date)
				profiles[pid] = p
				used[team][pid] = r.Minutes
				if p.games >= 3 {
					base[team][pid] = p.minutes
				}
			}
		}
		for _, team := range sortedIDs(g.teams) {
			rs := g.teams[team]
			opp := 0
			for tm := range g.teams {
				if tm != team {
					opp = tm
				}
			}
			ob, ok1 := roleExposure(profiles, base[opp], 0)
			ou, ok2 := roleExposure(profiles, used[opp], 0)
			if !ok1 || !ok2 {
				continue
			}
			for _, pid := range sortedIDs(rs) {
				r := rs[pid]
				prior := profiles[pid]
				if r.Minutes < 12 || prior.games < 3 {
					continue
				}
				tb, ok1 := roleExposure(profiles, base[team], pid)
				tu, ok2 := roleExposure(profiles, used[team], pid)
				if !ok1 || !ok2 {
					continue
				}
				x := append(roleDelta(tb, tu), roleDelta(ob, ou)...)
				for k, st := range data.Stats {
					if prior.rate[k] <= 0 {
						continue
					}
					visit(k, g.date, x, r.Value(st)/r.Minutes/prior.rate[k]-1, prior.rate[k]*r.Minutes)
				}
			}
		}
	}
}
func roleHistoryStart(date string) string {
	t, _ := time.Parse("2006-01-02", date)
	return t.AddDate(0, 0, -730).Format("2006-01-02")
}

type RoleStudy struct {
	Stat           data.Stat `json:"stat"`
	N              int       `json:"playerGames"`
	Dates          int       `json:"dates"`
	BaselineMAE    float64   `json:"baselineMae"`
	CompositionMAE float64   `json:"compositionMae"`
}

// StudyRoles is a held-out conditional exposure study. It uses observed held-out
// rotation minutes, so this is not a pregame backtest and cannot certify live picks.
func (ix *Index) StudyRoles(m *RoleModel, through string) []RoleStudy {
	out := make([]RoleStudy, data.NStats)
	dates := [data.NStats]map[string]bool{}
	for k, st := range data.Stats {
		out[k].Stat = st
		dates[k] = map[string]bool{}
	}
	ix.walkRoleSamples(m.Before, through, func(k int, date string, x []float64, y, exposure float64) {
		if len(m.Coef[k]) != roleFeatures {
			return
		}
		delta := 0.
		for j, v := range x {
			delta += v * m.Coef[k][j]
		}
		delta = math.Max(-.08, math.Min(.08, delta))
		out[k].N++
		dates[k][date] = true
		out[k].BaselineMAE += math.Abs(y) * exposure
		out[k].CompositionMAE += math.Abs(y-delta) * exposure
	})
	for k := range out {
		out[k].Dates = len(dates[k])
		if out[k].N > 0 {
			out[k].BaselineMAE /= float64(out[k].N)
			out[k].CompositionMAE /= float64(out[k].N)
		}
	}
	return out
}

// NewRoles accepts complete archived finals for model fitting even when older
// downloads lack capture timestamps. It does not make those rows eligible for
// settlement or verified forward evaluation, and never fills null stat fields.
func NewRoles(rows []data.PlayerGame) *Index {
	copied := append([]data.PlayerGame(nil), rows...)
	for i := range copied {
		copied[i].AnalyticsReady = copied[i].BoxScoreComplete
	}
	return New(copied)
}
