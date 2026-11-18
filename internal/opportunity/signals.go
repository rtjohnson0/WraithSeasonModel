package opportunity

import (
	"math"
	"propsmodel/internal/data"
	"sort"
	"time"
)

type Signal struct {
	Through       string    `json:"through,omitempty"`
	LatestCapture string    `json:"latestCapture,omitempty"`
	Stat          data.Stat `json:"stat"`
	Games         int       `json:"games"`
	Chances       float64   `json:"chances"`
	Minutes       float64   `json:"minutes"`
	Rate          float64   `json:"rate"`
	Conversion    float64   `json:"conversion"`
	Delta         float64   `json:"delta"`
	Available     bool      `json:"available"`
	Reason        string    `json:"reason"`
}

// Signals projects opportunity per minute separately from historical conversion.
// Conversion uses up to 40 earlier games; opportunity volume uses the latest ten.
// Same-day games, absent rows, null fields and incompatible counts stay excluded.
func (ix *Index) Signals(player int, date string, baseline [data.NStats]float64) []Signal {
	key := signalKey{player, date, baseline, ix.AsOf.UnixNano()}
	if cached, ok := ix.cache[key]; ok {
		return append([]Signal(nil), cached...)
	}
	hist := append([]data.PlayerGame(nil), ix.History[player]...)
	sort.Slice(hist, func(i, j int) bool {
		if hist[i].Date != hist[j].Date {
			return hist[i].Date < hist[j].Date
		}
		return hist[i].GameID < hist[j].GameID
	})
	var prior []data.PlayerGame
	for _, g := range hist {
		if g.Date < date && data.DayNumber(g.Date) >= data.DayNumber(date)-365 && g.Minutes > 0 {
			prior = append(prior, g)
		}
	}
	if len(prior) > 40 {
		prior = prior[len(prior)-40:]
	}
	out := make([]Signal, data.NStats)
	for k, st := range data.Stats {
		s := Signal{Stat: st, Reason: "opportunity coverage unavailable"}
		success, chances, recentCh, recentMin := 0., 0., 0., 0.
		recentN := 0
		latestRecent := ""
		for i, g := range prior {
			c, y := math.NaN(), g.Value(st)
			capture := ""
			switch st {
			case data.Assists:
				if r, ok := ix.row(player, g.GameID, "passing", date); ok {
					c = r.Value("potential_ast")
					capture = r.Captured
				}
			case data.Rebounds:
				if r, ok := ix.row(player, g.GameID, "rebounding", date); ok {
					c = r.Value("reb_chances", "rebound_chances_total")
					capture = r.Captured
				}
			case data.Threes:
				if g.ShootingComplete {
					c = g.Fg3a
				}
			case data.Points:
				if g.ShootingComplete {
					c = g.Fga + .44*g.Fta
				}
			}
			if math.IsNaN(c) || c < 0 || !finite(y) || y < 0 {
				continue
			}
			// Points per shooting opportunity can exceed one. Other chance conversions cannot.
			if st != data.Points && y > c {
				continue
			}
			if stamp, e := time.Parse(time.RFC3339Nano, capture); e == nil {
				old, _ := time.Parse(time.RFC3339Nano, s.LatestCapture)
				if stamp.After(old) {
					s.LatestCapture = capture
				}
			}
			chances += c
			success += y
			if i >= len(prior)-10 && data.DayNumber(g.Date) >= data.DayNumber(date)-60 {
				recentCh += c
				recentMin += g.Minutes
				recentN++
				if g.Date > latestRecent {
					latestRecent = g.Date
				}
			}
		}
		s.Through = latestRecent
		s.Games = recentN
		s.Chances = recentCh
		s.Minutes = recentMin
		needed := len(prior)
		if needed > 10 {
			needed = 10
		}
		latestRequired := ""
		if len(prior) > 1 {
			latestRequired = prior[len(prior)-2].Date
		}
		if recentN >= 3 && float64(recentN) >= .6*float64(needed) && latestRecent >= latestRequired && recentMin > 0 && chances > 0 && baseline[k] > 0 {
			s.Conversion = success / chances
			s.Rate = recentCh / recentMin * s.Conversion
			s.Delta = math.Max(-1, math.Min(1, s.Rate/baseline[k]-1))
			s.Available = true
			s.Reason = "earlier opportunity volume × historical conversion; experimental"
		}
		if !s.Available {
			switch {
			case recentN == 0:
				s.Reason = "No recent covered games"
			case recentN < 3:
				s.Reason = "Need three recent covered games"
			case float64(recentN) < .6*float64(needed):
				s.Reason = "Recent tracking coverage is incomplete"
			case latestRecent < latestRequired:
				s.Reason = "Newest player games lack tracking"
			case chances <= 0:
				s.Reason = "No opportunity volume for conversion"
			case baseline[k] <= 0:
				s.Reason = "No nonzero historical baseline"
			}
		}
		out[k] = s
	}
	if ix.cache == nil {
		ix.cache = map[signalKey][]Signal{}
	}
	ix.cache[key] = append([]Signal(nil), out...)
	return out
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
