// Package gameboard publishes a replaceable, pregame-only game-market snapshot.
package gameboard

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/bets"
	"propsmodel/internal/team"
)

type Side struct {
	Label       string  `json:"label"`
	Odds        int     `json:"odds"`
	Win         float64 `json:"win"`
	Push        float64 `json:"push"`
	Conditional float64 `json:"conditional"`
	BreakEven   float64 `json:"breakEven"`
	Market      float64 `json:"market"`
	Edge        float64 `json:"edge"`
	EV          float64 `json:"ev"`
}
type Quote struct {
	Market          string  `json:"market"`
	Book            string  `json:"book"`
	Line            float64 `json:"line"`
	QuoteAt         string  `json:"quoteAt"`
	SourceUpdatedAt string  `json:"sourceUpdatedAt"`
	Tip             string  `json:"tip"`
	BookCount       int     `json:"bookCount"`
	Sides           []Side  `json:"sides"`
}
type Game struct {
	ID        int     `json:"id"`
	Home      string  `json:"home"`
	Away      string  `json:"away"`
	Tip       string  `json:"tip"`
	HomeScore float64 `json:"homeScore"`
	AwayScore float64 `json:"awayScore"`
	Margin    float64 `json:"margin"`
	Total     float64 `json:"total"`
	TotalLow  float64 `json:"totalLow"`
	TotalHigh float64 `json:"totalHigh"`
	Quotes    []Quote `json:"quotes"`
}
type Snapshot struct {
	Date  string             `json:"date"`
	At    string             `json:"at"`
	Games []Game             `json:"games"`
	Z     map[string]float64 `json:"z"`
}

func (s Snapshot) Save(dir string) error {
	if s.Games == nil {
		s.Games = []Game{}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "gameboard.json")
	if err = os.WriteFile(path+".tmp", b, 0644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func Load(dir, date string) (*Snapshot, error) {
	b, e := os.ReadFile(filepath.Join(dir, "gameboard.json"))
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var s Snapshot
	if e = json.Unmarshal(b, &s); e != nil {
		return nil, e
	}
	if s.Date != date {
		return nil, nil
	}
	return &s, nil
}

// TotalProb uses a continuity correction for whole-number totals, which can push.
// A half-point threshold has no push. Conditional probability excludes pushes.
func TotalProb(mu, sigma, line float64) (over, under, push float64) {
	if sigma <= 0 {
		return 0, 0, 0
	}
	lo, hi := line, line
	if math.Abs(line-math.Round(line)) < 1e-9 {
		lo, hi = line-.5, line+.5
	}
	under = team.NormCDF((lo - mu) / sigma)
	over = 1 - team.NormCDF((hi-mu)/sigma)
	return over, under, math.Max(0, 1-over-under)
}
func Pair(labels [2]string, prices [2]int, wins [2]float64, push float64) []Side {
	sum := bets.Implied(prices[0]) + bets.Implied(prices[1])
	out := make([]Side, 2)
	for i := range out {
		be := bets.Implied(prices[i])
		p := wins[i] / (1 - push)
		profit := float64(prices[i]) / 100
		if prices[i] < 0 {
			profit = 100 / float64(-prices[i])
		}
		out[i] = Side{Label: labels[i], Odds: prices[i], Win: wins[i], Push: push, Conditional: p, BreakEven: be, Market: be / sum, Edge: p - be, EV: wins[i]*profit - (1 - wins[i] - push)}
	}
	return out
}
