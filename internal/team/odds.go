package team

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"propsmodel/internal/bets"
)

// Odds is one sportsbook's spread, moneyline and total for one game. Spread values are from
// the home team's side: -5.5 means the home team is favoured by 5.5.
type Odds struct {
	GameID         int
	Vendor         string
	SpreadHome     float64
	SpreadAway     float64
	SpreadHomeOdds int
	SpreadAwayOdds int
	HasSpread      bool
	MLHome, MLAway int
	Total          float64
	Over, Under    int
	HasTotal       bool
	Stamp          string
}

func num(raw json.RawMessage) (float64, bool) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	s = strings.TrimPrefix(s, "+")
	if s == "" || s == "null" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func whole(raw json.RawMessage) int {
	v, ok := num(raw)
	if !ok {
		return 0
	}
	return int(math.Round(v))
}

type apiOdds struct {
	GameID    int             `json:"game_id"`
	Vendor    string          `json:"vendor"`
	SHV       json.RawMessage `json:"spread_home_value"`
	SHO       json.RawMessage `json:"spread_home_odds"`
	SAV       json.RawMessage `json:"spread_away_value"`
	SAO       json.RawMessage `json:"spread_away_odds"`
	MLH       json.RawMessage `json:"moneyline_home_odds"`
	MLA       json.RawMessage `json:"moneyline_away_odds"`
	TV        json.RawMessage `json:"total_value"`
	TO        json.RawMessage `json:"total_over_odds"`
	TU        json.RawMessage `json:"total_under_odds"`
	UpdatedAt string          `json:"updated_at"`
	OpenedAt  string          `json:"opened_at"`
}

func (a apiOdds) toOdds() Odds {
	o := Odds{GameID: a.GameID, Vendor: a.Vendor, Stamp: a.UpdatedAt}
	if o.Stamp == "" {
		o.Stamp = a.OpenedAt
	}
	if v, ok := num(a.SHV); ok {
		o.SpreadHome, o.HasSpread = v, true
		o.SpreadAway = -v
		if w, ok := num(a.SAV); ok {
			o.SpreadAway = w
		}
		o.SpreadHomeOdds, o.SpreadAwayOdds = whole(a.SHO), whole(a.SAO)
		if o.SpreadHomeOdds == 0 || o.SpreadAwayOdds == 0 {
			o.HasSpread = false
		}
	}
	o.MLHome, o.MLAway = whole(a.MLH), whole(a.MLA)
	if v, ok := num(a.TV); ok {
		o.Total, o.HasTotal = v, true
		o.Over, o.Under = whole(a.TO), whole(a.TU)
		if o.Over == 0 || o.Under == 0 {
			o.HasTotal = false
		}
	}
	return o
}

// LoadOdds reads a JSONL file of game odds: either bare rows (the opening-odds pull) or
// {"captured_at":..., "row":{...}} (the daily snapshot). When a game and book appear more
// than once the latest row wins.
func LoadOdds(path string) ([]Odds, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	type key struct {
		g int
		v string
	}
	latest := map[key]Odds{}
	capt := map[key]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var w struct {
			CapturedAt string          `json:"captured_at"`
			Row        json.RawMessage `json:"row"`
		}
		b := sc.Bytes()
		var a apiOdds
		if json.Unmarshal(b, &w) == nil && len(w.Row) > 0 {
			if json.Unmarshal(w.Row, &a) != nil {
				continue
			}
		} else if json.Unmarshal(b, &a) != nil {
			continue
		}
		if a.GameID == 0 {
			continue
		}
		o := a.toOdds()
		k := key{o.GameID, o.Vendor}
		c := w.CapturedAt
		if prev, ok := capt[k]; !ok || c >= prev {
			capt[k] = c
			latest[k] = o
		}
	}
	out := make([]Odds, 0, len(latest))
	for _, o := range latest {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		return out[i].Vendor < out[j].Vendor
	})
	return out, sc.Err()
}

func median(v []float64) float64 {
	sort.Float64s(v)
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

func medianInt(v []int) int {
	f := make([]float64, len(v))
	for i, x := range v {
		f[i] = float64(x)
	}
	return int(math.Round(median(f)))
}

// Consensus is the middle of the market for one game: median line, with the median prices
// among books that post that line.
type Consensus struct {
	GameID                         int
	HasSpread, HasTotal            bool
	SpreadHome                     float64
	SpreadHomeOdds, SpreadAwayOdds int
	Total                          float64
	Over, Under                    int
	MLHome, MLAway                 int
}

// ConsensusOf collapses every book's odds into one Consensus per game.
func ConsensusOf(all []Odds) map[int]Consensus {
	by := map[int][]Odds{}
	for _, o := range all {
		by[o.GameID] = append(by[o.GameID], o)
	}
	out := map[int]Consensus{}
	for id, os := range by {
		c := Consensus{GameID: id}
		var sp, tt []float64
		for _, o := range os {
			if o.HasSpread {
				sp = append(sp, o.SpreadHome)
			}
			if o.HasTotal {
				tt = append(tt, o.Total)
			}
		}
		if len(sp) > 0 {
			c.HasSpread, c.SpreadHome = true, median(sp)
			var ho, ao []int
			for _, o := range os {
				if o.HasSpread && o.SpreadHome == c.SpreadHome {
					ho, ao = append(ho, o.SpreadHomeOdds), append(ao, o.SpreadAwayOdds)
				}
			}
			c.SpreadHomeOdds, c.SpreadAwayOdds = medianInt(ho), medianInt(ao)
		}
		if len(tt) > 0 {
			c.HasTotal, c.Total = true, median(tt)
			var ov, un []int
			for _, o := range os {
				if o.HasTotal && o.Total == c.Total {
					ov, un = append(ov, o.Over), append(un, o.Under)
				}
			}
			c.Over, c.Under = medianInt(ov), medianInt(un)
		}
		var mh, ma []int
		for _, o := range os {
			if o.MLHome != 0 && o.MLAway != 0 {
				mh, ma = append(mh, o.MLHome), append(ma, o.MLAway)
			}
		}
		if len(mh) > 0 {
			c.MLHome, c.MLAway = medianInt(mh), medianInt(ma)
		}
		out[id] = c
	}
	return out
}

// DevigHome is the book's own probability for the first of two prices, vig removed.
func DevigHome(first, second int) float64 { return bets.MarketOver(first, second) }
