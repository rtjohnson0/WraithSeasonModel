package gameboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/marketperiod"
	"sort"
	"strings"
	"time"
)

// Decision freezes the first visible pregame direction and exact quote per game/market.
// Leans stay separate from selected plays; neither can be relabelled after the result.
type Decision struct {
	Date      string  `json:"date"`
	At        string  `json:"at"`
	GameID    int     `json:"game_id"`
	HomeID    int     `json:"home_id"`
	AwayID    int     `json:"away_id"`
	Home      string  `json:"home"`
	Away      string  `json:"away"`
	Tip       string  `json:"tip"`
	Market    string  `json:"market"`
	Side      string  `json:"side"`
	Line      float64 `json:"line"`
	Odds      int     `json:"odds"`
	Book      string  `json:"book"`
	QuoteAt   string  `json:"quote_at"`
	UpdatedAt string  `json:"updated_at"`
	Prob      float64 `json:"prob"`
	Tier      string  `json:"tier"`
}
type Result struct {
	Decision
	Result    string  `json:"result"`
	HomeScore int     `json:"homeScore"`
	AwayScore int     `json:"awayScore"`
	Units     float64 `json:"units"`
	Reason    string  `json:"reason,omitempty"`
	SourceAt  string  `json:"sourceAt,omitempty"`
}

func readJSONL[T any](path string) ([]T, error) {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var out []T
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1<<24)
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var r T
		if e = json.Unmarshal([]byte(strings.TrimPrefix(s.Text(), "\ufeff")), &r); e != nil {
			return nil, fmt.Errorf("%s: %w", path, e)
		}
		out = append(out, r)
	}
	return out, s.Err()
}
func writeJSONL[T any](path string, rows []T) error {
	f, e := os.Create(path + ".tmp")
	if e != nil {
		return e
	}
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if e = enc.Encode(r); e != nil {
			f.Close()
			return e
		}
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(path+".tmp", path)
}
func decisionKey(d Decision) string { return fmt.Sprintf("%s/%d/%s", d.Date, d.GameID, d.Market) }
func validDecision(d Decision) bool {
	at, a := time.Parse(time.RFC3339Nano, d.At)
	tip, b := time.Parse(time.RFC3339Nano, d.Tip)
	q, c := time.Parse(time.RFC3339Nano, d.QuoteAt)
	u, e := time.Parse(time.RFC3339Nano, d.UpdatedAt)
	sideOK := (d.Market == "total" && (d.Side == "Over" || d.Side == "Under")) || ((d.Market == "moneyline" || d.Market == "spread") && (d.Side == d.Home || d.Side == d.Away))
	return a == nil && b == nil && c == nil && e == nil && at.Before(tip) && !q.After(at) && !u.After(at) && at.Sub(q) <= 30*time.Minute && at.Sub(u) <= 6*time.Hour && d.GameID > 0 && d.HomeID > 0 && d.AwayID > 0 && d.HomeID != d.AwayID && d.Home != "" && d.Away != "" && d.Home != d.Away && data.DayNumber(d.Date) > 0 && sideOK && (d.Odds <= -100 || d.Odds >= 100) && d.Prob > 0 && d.Prob < 1 && !math.IsNaN(d.Line) && !math.IsInf(d.Line, 0) && (d.Tier == "PICK" || d.Tier == "LEAN" || d.Tier == "PAPER" || d.Tier == "CHECK")
}
func Log(dir string, s Snapshot, hi, edge float64) error {
	path := filepath.Join(dir, "gamepicklog.jsonl")
	rows, e := readJSONL[Decision](path)
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[decisionKey(r)] = true
	}
	for _, g := range s.Games {
		for _, market := range []string{"moneyline", "total", "spread"} {
			var best *Quote
			var chosen Side
			for _, q := range g.Quotes {
				if q.Market != market || q.BookCount < 3 || len(q.Sides) != 2 {
					continue
				}
				primary := q.Sides[0]
				if q.Sides[1].Conditional > primary.Conditional {
					primary = q.Sides[1]
				}
				// Choose the best price for the higher-probability direction, same as the website.
				if best == nil || primary.EV > chosen.EV || (primary.EV == chosen.EV && q.Book < best.Book) {
					v := q
					best = &v
					chosen = primary
				}
			}
			if best == nil {
				continue
			}
			tier := "LEAN"
			if chosen.Conditional >= hi && chosen.Edge >= edge {
				tier = "PICK"
				if z, ok := s.Z[market]; !ok || z < 2 {
					tier = "PAPER"
				}
			}
			if (market == "total" && g.MarketTotal != nil && math.Abs(g.Total-*g.MarketTotal) > 7) || (market != "total" && g.MarketMargin != nil && math.Abs(g.Margin-*g.MarketMargin) > 4) {
				tier = "CHECK"
			}
			d := Decision{Date: s.Date, At: s.At, GameID: g.ID, HomeID: g.HomeID, AwayID: g.AwayID, Home: g.Home, Away: g.Away, Tip: g.Tip, Market: market, Side: chosen.Label, Line: best.Line, Odds: chosen.Odds, Book: best.Book, QuoteAt: best.QuoteAt, UpdatedAt: best.SourceUpdatedAt, Prob: chosen.Conditional, Tier: tier}
			if validDecision(d) && !seen[decisionKey(d)] {
				rows = append(rows, d)
				seen[decisionKey(d)] = true
			}
		}
	}
	return writeJSONL(path, rows)
}

type finalRow struct {
	At       string `json:"_stats_fetched_at"`
	Endpoint string `json:"_stats_endpoint"`
	Period   *int   `json:"period"`
	Game     struct {
		ID     int    `json:"id"`
		Date   string `json:"date"`
		Status string `json:"status"`
		Home   int    `json:"home_team_id"`
		Away   int    `json:"visitor_team_id"`
		HP     *int   `json:"home_team_score"`
		AP     *int   `json:"visitor_team_score"`
	} `json:"game"`
}

// Settle uses the provider's explicit final game scores, never sums an incomplete player download.
func Settle(dir string) error {
	rows, e := readJSONL[Decision](filepath.Join(dir, "gamepicklog.jsonl"))
	if e != nil {
		return e
	}
	finals := map[int]finalRow{}
	conflict := map[int]bool{}
	files, e := filepath.Glob(filepath.Join(dir, "stats_*.jsonl"))
	if e != nil {
		return e
	}
	for _, file := range files {
		f, e := os.Open(file)
		if e != nil {
			return e
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 4096), 1<<24)
		for sc.Scan() {
			var r finalRow
			b := []byte(strings.TrimPrefix(sc.Text(), "\ufeff"))
			if json.Unmarshal(b, &r) != nil {
				f.Close()
				return fmt.Errorf("invalid stats JSON in %s", file)
			}
			if r.Endpoint != "/nba/v1/stats" || !strings.EqualFold(strings.TrimSpace(r.Game.Status), "Final") || r.Game.HP == nil || r.Game.AP == nil || (r.Period != nil && *r.Period != 0) || !marketperiod.Metadata(b) {
				continue
			}
			fetched, err := time.Parse(time.RFC3339Nano, r.At)
			if err != nil {
				continue
			}
			if old, ok := finals[r.Game.ID]; ok {
				prior, _ := time.Parse(time.RFC3339Nano, old.At)
				if fetched.Before(prior) {
					continue
				}
				if fetched.Equal(prior) && (*r.Game.HP != *old.Game.HP || *r.Game.AP != *old.Game.AP || r.Game.Home != old.Game.Home || r.Game.Away != old.Game.Away) {
					conflict[r.Game.ID] = true
					continue
				}
				if fetched.After(prior) {
					conflict[r.Game.ID] = false
				}
			}
			finals[r.Game.ID] = r
		}
		err := sc.Err()
		f.Close()
		if err != nil {
			return err
		}
	}
	out := []Result{}
	seen := map[string]bool{}
	for _, d := range rows {
		if seen[decisionKey(d)] {
			continue
		}
		seen[decisionKey(d)] = true
		r := Result{Decision: d, Result: "PENDING", Reason: "Waiting for matching final game scores"}
		f, ok := finals[d.GameID]
		if !validDecision(d) {
			r.Result = "REVIEW"
			r.Reason = "Invalid pregame decision provenance"
		} else if ok {
			tip, _ := time.Parse(time.RFC3339Nano, d.Tip)
			fetched, _ := time.Parse(time.RFC3339Nano, f.At)
			if conflict[d.GameID] || data.NormalizeDate(f.Game.Date) != d.Date || f.Game.Home != d.HomeID || f.Game.Away != d.AwayID || *f.Game.HP <= 0 || *f.Game.AP <= 0 || *f.Game.HP == *f.Game.AP || !fetched.After(tip) {
				r.Result = "REVIEW"
				r.Reason = "Final score identity, timing or source conflict"
			} else {
				r.HomeScore, r.AwayScore = *f.Game.HP, *f.Game.AP
				r.SourceAt = f.At
				r.Reason = ""
				value := float64(r.HomeScore - r.AwayScore)
				if d.Market == "total" {
					value = float64(r.HomeScore+r.AwayScore) - d.Line
					if d.Side == "Under" {
						value = -value
					}
				} else {
					if d.Side == d.Away {
						value = -value
					}
					if d.Market == "spread" {
						if d.Side == d.Home {
							value += d.Line
						} else {
							value -= d.Line
						}
					}
				}
				switch {
				case math.Abs(value) < 1e-9:
					r.Result = "PUSH"
				case value > 0:
					r.Result = "WIN"
					if d.Odds > 0 {
						r.Units = float64(d.Odds) / 100
					} else {
						r.Units = 100 / float64(-d.Odds)
					}
				default:
					r.Result = "LOSS"
					r.Units = -1
				}
			}
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return decisionKey(out[i].Decision) < decisionKey(out[j].Decision)
	})
	return writeJSONL(filepath.Join(dir, "gamesettled.jsonl"), out)
}
func LoadResults(dir string) ([]Result, error) {
	return readJSONL[Result](filepath.Join(dir, "gamesettled.jsonl"))
}
