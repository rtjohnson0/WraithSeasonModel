package picklog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// Settled is a logged pick after the game, with its result and closing line value.
type Settled struct {
	Entry
	SettledAt string  `json:"settled_at"`
	Result    string  `json:"result"` // WIN, LOSS, PUSH or VOID (player did not play)
	Actual    float64 `json:"actual"`
	CloseLine float64 `json:"close_line"` // median line across books at the last snapshot; 0 if unknown
	CLV       float64 `json:"clv"`        // closing line value in stat points; positive means we beat the close
	Profit    float64 `json:"profit"`     // in units, at the logged stake
	Flat      float64 `json:"flat"`       // in units, betting 1 unit
}

// Payout is the profit on a winning 1-unit bet at American odds (-110 if unknown).
func Payout(odds int) float64 {
	if odds == 0 {
		odds = -110
	}
	if odds > 0 {
		return float64(odds) / 100
	}
	return 100 / math.Abs(float64(odds))
}

// Grade settles an entry given the actual stat and the closing line (0 if unknown).
func Grade(e Entry, actual, closeLine float64, played bool) Settled {
	s := Settled{Entry: e, SettledAt: time.Now().UTC().Format(time.RFC3339), Actual: actual, CloseLine: closeLine}
	stake := e.Units
	if stake <= 0 {
		stake = 1
	}
	switch {
	case !played:
		s.Result = "VOID"
		return s
	case actual == e.Line:
		s.Result = "PUSH"
	case (actual > e.Line) == (e.Side == "OVER"):
		s.Result = "WIN"
		s.Profit, s.Flat = stake*Payout(e.Odds), Payout(e.Odds)
	default:
		s.Result = "LOSS"
		s.Profit, s.Flat = -stake, -1
	}
	if closeLine > 0 {
		if e.Side == "OVER" {
			s.CLV = closeLine - e.Line
		} else {
			s.CLV = e.Line - closeLine
		}
	}
	return s
}

// LoadSettled reads the settled log. A missing file is empty.
func LoadSettled(path string) ([]Settled, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Settled
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var s Settled
		if json.Unmarshal(sc.Bytes(), &s) == nil {
			out = append(out, s)
		}
	}
	return out, sc.Err()
}

// AppendSettled adds settled entries not already present.
func AppendSettled(path string, rows []Settled) (int, error) {
	existing, err := LoadSettled(path)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, s := range existing {
		seen[s.key()] = true
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n := 0
	for _, s := range rows {
		if seen[s.key()] {
			continue
		}
		seen[s.key()] = true
		b, err := json.Marshal(s)
		if err != nil {
			return n, err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Summary totals a set of settled picks.
type Summary struct {
	Picks, Wins, Losses, Pushes, Voids int
	Profit, Staked, Flat               float64
	AvgProb                            float64 // average predicted win probability on graded picks
	CLVN                               int
	AvgCLV                             float64 // average closing line value in stat points
	CLVPositive                        float64 // share of picks with positive closing line value
}

// HitRate is wins over wins plus losses.
func (s Summary) HitRate() float64 {
	if s.Wins+s.Losses == 0 {
		return 0
	}
	return float64(s.Wins) / float64(s.Wins+s.Losses)
}

// ROI is profit over amount staked.
func (s Summary) ROI() float64 {
	if s.Staked == 0 {
		return 0
	}
	return s.Profit / s.Staked
}

// Summarize totals the given picks.
func Summarize(rows []Settled) Summary {
	var s Summary
	var probSum float64
	var clvSum float64
	var clvPos int
	for _, r := range rows {
		s.Picks++
		switch r.Result {
		case "WIN":
			s.Wins++
		case "LOSS":
			s.Losses++
		case "PUSH":
			s.Pushes++
		case "VOID":
			s.Voids++
			continue
		}
		stake := r.Units
		if stake <= 0 {
			stake = 1
		}
		if r.Result != "PUSH" {
			s.Staked += stake
			probSum += r.Prob
		}
		s.Profit += r.Profit
		s.Flat += r.Flat
		if r.CloseLine > 0 {
			s.CLVN++
			clvSum += r.CLV
			if r.CLV > 0 {
				clvPos++
			}
		}
	}
	if graded := s.Wins + s.Losses; graded > 0 {
		s.AvgProb = probSum / float64(graded)
	}
	if s.CLVN > 0 {
		s.AvgCLV = clvSum / float64(s.CLVN)
		s.CLVPositive = float64(clvPos) / float64(s.CLVN)
	}
	return s
}

var tierRank = map[string]int{"": 0, "PICK": 0, "CHECK": 1, "PAPER": 2, "LEAN": 3}

// TierRank orders tiers from most to least trusted (PICK first). Unknown tiers sort last.
func TierRank(t string) int {
	if r, ok := tierRank[t]; ok {
		return r
	}
	return 4
}

// DedupeSettled collapses the same player, stat and side on the same day (logged again at another
// book or line, or by a re-run) into one play: the most trusted tier, then the earliest logged. It
// returns how many duplicate lines it dropped, so one repeated pick never counts as several results.
func DedupeSettled(in []Settled) ([]Settled, int) {
	best := map[string]int{}
	var out []Settled
	for _, r := range in {
		k := fmt.Sprintf("%s|%d|%s|%s", r.Date, r.PlayerID, r.Stat, r.Side)
		i, ok := best[k]
		if !ok {
			best[k] = len(out)
			out = append(out, r)
			continue
		}
		cur := out[i]
		if TierRank(r.Tier) < TierRank(cur.Tier) || (TierRank(r.Tier) == TierRank(cur.Tier) && r.LoggedAt < cur.LoggedAt) {
			out[i] = r
		}
	}
	return out, len(in) - len(out)
}
