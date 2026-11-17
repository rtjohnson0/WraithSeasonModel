package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/picklog"
	"propsmodel/internal/props"
)

// MoreFile is data/more.json: each game's next-best lines that did not make the board, for the
// site's "More plays" button. It is rewritten on every run and is not part of the pick record.
type MoreFile struct {
	Date  string          `json:"date"`
	At    string          `json:"at"`
	Plays []picklog.Entry `json:"plays"`
	// Games is each game's betting context (consensus spread and total), for the board header.
	Games []GameCtx `json:"games,omitempty"`
	// Moves is how the market's median line moved through the day for each player stat on the board,
	// keyed "playerID|stat".
	Moves map[string][]MovePoint `json:"moves,omitempty"`
	// News is each team's rotation players who are out or uncertain, by game.
	News []GameNews `json:"news,omitempty"`
	// Overs is the day's Overs closest to qualifying, and Overs at books priced off the market,
	// for the site's "Overs watch" section. Not picks.
	Overs []OverWatch `json:"overs,omitempty"`
	// Combos is the day's combined-market plays (points+rebounds+assists and so on), tier COMBO:
	// tracked, never staked, and kept out of the pick record.
	Combos []picklog.Entry `json:"combos,omitempty"`
	// ComboInfo says how the day's combo scoring went, so the site can explain an empty list.
	ComboInfo *ComboInfo `json:"combo_info,omitempty"`
}

// ComboInfo is a summary of one day's combo pricing.
type ComboInfo struct {
	Blocked map[string]int `json:"blocked,omitempty"`
	Loaded  int            `json:"loaded"`
	Fresh   int            `json:"fresh"`
	Usable  int            `json:"usable"`
	Priced  int            `json:"priced"`  // combo lines priced
	Over    int            `json:"over"`    // lean Over after calibration
	Under   int            `json:"under"`   // lean Under after calibration
	Cleared int            `json:"cleared"` // reach the 55%/45% bar with the edge rule
	Tracked int            `json:"tracked"` // plays tracked at the combo bar
	Bar     float64        `json:"bar"`     // the tracking bar (0.53 = 53% Over or 47% Under)
	Basis   string         `json:"basis"`   // how the probabilities were calibrated
	At      string         `json:"at"`
}

// OverWatch is one Over the site lists even though it is not on the board.
type OverWatch struct {
	Experimental bool           `json:"experimental,omitempty"`
	Forecast     *picklog.Entry `json:"forecast,omitempty"`
	GameID       int            `json:"game_id"`
	PlayerID     int            `json:"player_id"`
	Player       string         `json:"player"`
	Team         string         `json:"team"`
	Matchup      string         `json:"matchup"`
	Stat         string         `json:"stat"`
	Line         float64        `json:"line"`
	Odds         int            `json:"odds"`
	Book         string         `json:"book"`
	Prob         float64        `json:"prob"`
	Edge         float64        `json:"edge"`
	Proj         float64        `json:"proj"`
	ProjMin      float64        `json:"proj_min"`
	Tip          string         `json:"tip,omitempty"`
	LowLine      bool           `json:"low_line,omitempty"` // this book's line is a point or more below the market median
	Median       float64        `json:"median,omitempty"`
	Books        int            `json:"books,omitempty"`
	Status       string         `json:"status"` // why it is not a pick yet
}

// MovePoint is the median line across books at one capture.
type MovePoint struct {
	At   string  `json:"at"`
	Line float64 `json:"line"`
}

// GameNews lists one team's injury news for a game.
type GameNews struct {
	GameID int      `json:"game_id"`
	Team   string   `json:"team"`
	Out    []string `json:"out,omitempty"`
	Maybe  []string `json:"maybe,omitempty"` // "Name (questionable)"
}

// GameCtx is one game's market from the consensus of the books: the home spread (negative when the
// home team is favoured) and the total.
type GameCtx struct {
	GameID     int     `json:"game_id"`
	Home       string  `json:"home"`
	Visitor    string  `json:"visitor"`
	SpreadHome float64 `json:"spread_home"`
	Total      float64 `json:"total"`
}

// moreCandidates returns up to perGame extra plays per game: lines with a positive edge that are not
// already on the board (same player, stat and side), at most perPlayer per player, one line per
// player, stat and side (the best price), best edge first. Lines flagged as far from the projection
// are left out, since those are more often a bad line than an edge.
func moreCandidates(picks, all []scored, perGame, perPlayer int) []scored {
	posted := map[[3]string]bool{}
	for _, s := range picks {
		posted[[3]string{strconv.Itoa(s.GameID), strconv.Itoa(s.PlayerID), string(s.Stat)}] = true
	}
	cands := bestLinePerPick(all)
	sort.SliceStable(cands, func(i, j int) bool { return valueBefore(cands[i], cands[j]) })
	perG, perP := map[int]int{}, map[int]int{}
	var out []scored
	for _, c := range cands {
		if c.Edge <= 0 || c.Far || c.Maybe != "" || posted[[3]string{strconv.Itoa(c.GameID), strconv.Itoa(c.PlayerID), string(c.Stat)}] {
			continue
		}
		if perG[c.GameID] >= perGame || perP[c.PlayerID] >= perPlayer {
			continue
		}
		perG[c.GameID]++
		perP[c.PlayerID]++
		c.Tier, c.Units = "LEAN", 0
		out = append(out, c)
	}
	return out
}

// writeMore saves the extra plays to data/more.json.
func writeMore(dir, date string, extra []scored, ctx []GameCtx, moves map[string][]MovePoint, news []GameNews, overs []OverWatch, combos []picklog.Entry, info *ComboInfo, runAt string) error {
	f := MoreFile{Date: date, At: runAt, Games: ctx, Moves: moves, News: news, Overs: overs, Combos: combos, ComboInfo: info}
	for _, s := range extra {
		f.Plays = append(f.Plays, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor,
			Prob: s.Prob, ProbRaw: s.ProbRaw, Availability: s.Availability, ProjMean: s.ProjMean, ProjMin: s.ProjMin, Tier: "LEAN", Why: s.Why, Tip: s.Tip, Dist: s.Dist, QuoteAt: s.QuoteAt, SourceUpdatedAt: s.SourceUpdatedAt, AuditVersion: s.AuditVersion, MarketMedian: s.MarketMedian, BookCount: s.BookCount, BookQuotes: s.BookQuotes, CaptureMaxAgeSeconds: s.CaptureMaxAgeSeconds, PriceMaxAgeSeconds: s.PriceMaxAgeSeconds,
		})
	}
	b, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "more.json"), b, 0o644)
}

// buildMoves collects the line history for the plays on the board and the extras.
func buildMoves(series map[props.MoveKey][]props.SeriesPoint, plays ...[]scored) map[string][]MovePoint {
	out := map[string][]MovePoint{}
	for _, list := range plays {
		for _, s := range list {
			k := strconv.Itoa(s.PlayerID) + "|" + string(s.Stat)
			if _, done := out[k]; done {
				continue
			}
			pts := series[props.MoveKey{Game: s.GameID, Player: s.PlayerID, Stat: s.Stat}]
			if len(pts) < 2 {
				continue
			}
			for _, p := range pts {
				out[k] = append(out[k], MovePoint{At: p.At, Line: p.Line})
			}
		}
	}
	return out
}

// buildNews lists, for each team in today's games, the rotation players who are out or uncertain.
func buildNews(b *feat.Builder, name func(int) string, games map[int]props.GameInfo, out map[int]string, statuses map[int]string, date string) []GameNews {
	day := data.DayNumber(date)
	var ids []int
	for id := range games {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var news []GameNews
	for _, id := range ids {
		g := games[id]
		for _, t := range []struct {
			id   int
			abbr string
		}{{g.Visitor, g.VisitorAbbr}, {g.Home, g.HomeAbbr}} {
			n := GameNews{GameID: id, Team: t.abbr}
			for _, pid := range b.RotationOf(t.id, day) {
				if _, isOut := out[pid]; isOut {
					n.Out = append(n.Out, name(pid))
				} else if st, ok := statuses[pid]; ok && props.IsUncertain(st) {
					n.Maybe = append(n.Maybe, name(pid)+" ("+strings.ToLower(st)+")")
				}
			}
			if len(n.Out)+len(n.Maybe) > 0 && t.abbr != "" {
				news = append(news, n)
			}
		}
	}
	return news
}

// overWatchRows picks the Overs to list on the site: the strongest Overs by edge that are not on the
// board, plus Overs at books whose line sits a point or more below the market median. At most one
// row per player and stat; best edge first.
// Comparison candidates must have positive modeled value and retain the exact
// source quote. A lower market line alone is not an Over recommendation.
func overWatchRows(all, picks []scored, off []offRow, hi, minEdge float64, limit int) []OverWatch {
	blocked := map[string]bool{}
	key := func(s scored) string { return fmt.Sprintf("%d|%d|%s", s.GameID, s.PlayerID, s.Stat) }
	for _, s := range picks {
		blocked[key(s)] = true
	}
	var candidates []OverWatch
	add := func(s scored, low *offRow) {
		s.Edge = bets.Edge(s.Prob, s.Odds)
		if s.Side != "OVER" || s.Far || s.Maybe != "" || s.Edge <= 0 || s.Prob < .5 || blocked[key(s)] {
			return
		}
		// Missing source identity/times must never become an apparent live card.
		if s.GameID == 0 || s.PlayerID == 0 || s.Tip == "" || s.QuoteAt == "" || s.ProjMin <= 0 {
			return
		}
		e := boardEntry("", s, time.Now().UTC())
		e.Period, e.Tier, e.Units = "game", "OVERWATCH", 0
		status := "Positive-edge comparison; not a selected play"
		if s.Prob < hi {
			status = fmt.Sprintf("Model gives it %.0f%%; selection needs %.0f%%", 100*s.Prob, 100*hi)
		} else if s.Edge < minEdge {
			status = "Positive edge is below the selection requirement"
		}
		w := OverWatch{Forecast: &e, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Team: s.Team, Matchup: s.Matchup, Stat: string(s.Stat), Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, Edge: s.Edge, Proj: s.ProjMean, ProjMin: s.ProjMin, Tip: s.Tip, Status: status}
		if low != nil {
			w.LowLine, w.Median, w.Books = true, low.Median, low.Books
		}
		candidates = append(candidates, w)
	}
	for _, s := range all {
		add(s, nil)
	}
	for _, low := range off {
		if low.Side != "OVER" || low.Edge <= 0 || low.Prob < .5 {
			continue
		}
		// Resolve by exact quote, including line and book. Never borrow another
		// line's projection, distribution, timing, or injury status.
		for _, source := range all {
			if source.PlayerID != low.PlayerID || source.Stat != low.Stat || source.Line != low.Line || source.Vendor != low.Vendor || source.Matchup != low.Matchup {
				continue
			}
			if source.Side != "OVER" {
				source.Prob, source.ProbRaw = 1-source.Prob, 1-source.ProbRaw
			}
			source.Side, source.Odds, source.Edge = "OVER", low.Odds, low.Edge
			add(source, &low)
			break
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return bets.ExpectedReturn(candidates[i].Prob, candidates[i].Odds) > bets.ExpectedReturn(candidates[j].Prob, candidates[j].Odds)
	})
	seen := map[string]bool{}
	out := []OverWatch{}
	for _, w := range candidates {
		k := fmt.Sprintf("%d|%d|%s", w.GameID, w.PlayerID, w.Stat)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, w)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
