package comboaudit

import (
	"math"
	"time"
)

type WorkloadComparison struct {
	Name        string  `json:"name"`
	PlayerGames int     `json:"playerGames"`
	PrimaryMAE  float64 `json:"primaryMAE"`
	V2MAE       float64 `json:"v2MAE"`
}

func compareWorkloads(rows []Row) []WorkloadComparison {
	unique := map[[2]int]Row{}
	for _, r := range rows {
		if r.BaselineMinutes == nil {
			continue
		}
		at, e := time.Parse(time.RFC3339Nano, r.LoggedAt)
		if e != nil {
			continue
		}
		key := [2]int{r.PlayerID, r.GameID}
		old, ok := unique[key]
		prior, _ := time.Parse(time.RFC3339Nano, old.LoggedAt)
		// One earliest retained decision per player-game, then lexical stat tie
		// break, so multi-prop players cannot inflate the minutes comparison.
		if !ok || at.Before(prior) || (at.Equal(prior) && r.Stat < old.Stat) {
			unique[key] = r
		}
	}
	var out []WorkloadComparison
	for _, name := range []string{"all posted player-games", "allocation cut 4+ min"} {
		s := WorkloadComparison{Name: name}
		for _, r := range unique {
			if name == "allocation cut 4+ min" && r.ProjectedMinutes-*r.BaselineMinutes > -4 {
				continue
			}
			s.PlayerGames++
			s.PrimaryMAE += math.Abs(r.ActualMinutes - *r.BaselineMinutes)
			s.V2MAE += math.Abs(r.ActualMinutes - r.ProjectedMinutes)
		}
		if s.PlayerGames > 0 {
			s.PrimaryMAE /= float64(s.PlayerGames)
			s.V2MAE /= float64(s.PlayerGames)
			out = append(out, s)
		}
	}
	return out
}
