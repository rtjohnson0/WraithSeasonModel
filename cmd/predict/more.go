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
}

// OverWatch is one Over the site lists even though it is not on the board.
type OverWatch struct {
	GameID   int     `json:"game_id"`
	PlayerID int     `json:"player_id"`
	Player   string  `json:"player"`
	Team     string  `json:"team"`
	Matchup  string  `json:"matchup"`
	Stat     string  `json:"stat"`
	Line     float64 `json:"line"`
	Odds     int     `json:"odds"`
	Book     string  `json:"book"`
	Prob     float64 `json:"prob"`
	Edge     float64 `json:"edge"`
	Proj     float64 `json:"proj"`
	ProjMin  float64 `json:"proj_min"`
	Tip      string  `json:"tip,omitempty"`
	LowLine  bool    `json:"low_line,omitempty"` // this book's line is a point or more below the market median
	Median   float64 `json:"median,omitempty"`
	Books    int     `json:"books,omitempty"`
	Status   string  `json:"status"` // why it is not a pick yet
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
		posted[[3]string{strconv.Itoa(s.PlayerID), string(s.Stat), s.Side}] = true
	}
	cands := bestLinePerPick(all)
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Edge > cands[j].Edge })
	perG, perP := map[int]int{}, map[int]int{}
	var out []scored
	for _, c := range cands {
		if c.Edge <= 0 || c.Far || posted[[3]string{strconv.Itoa(c.PlayerID), string(c.Stat), c.Side}] {
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
func writeMore(dir, date string, extra []scored, ctx []GameCtx, moves map[string][]MovePoint, news []GameNews, overs []OverWatch) error {
	f := MoreFile{Date: date, At: time.Now().UTC().Format(time.RFC3339), Games: ctx, Moves: moves, News: news, Overs: overs}
	for _, s := range extra {
		f.Plays = append(f.Plays, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor,
			Prob: s.Prob, ProbRaw: s.ProbRaw, ProjMean: s.ProjMean, ProjMin: s.ProjMin, Tier: "LEAN", Why: s.Why, Tip: s.Tip, Dist: s.Dist,
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
func overWatchRows(all, picks []scored, off []offRow, hi, minEdge float64, limit int) []OverWatch {
	on := map[[2]string]bool{}
	for _, s := range picks {
		if s.Side == "OVER" {
			on[[2]string{strconv.Itoa(s.PlayerID), string(s.Stat)}] = true
		}
	}
	lowLine := map[[2]string]offRow{}
	for _, r := range off {
		if r.Side != "OVER" {
			continue
		}
		k := [2]string{strconv.Itoa(r.PlayerID), string(r.Stat)}
		if cur, ok := lowLine[k]; !ok || r.Edge > cur.Edge {
			lowLine[k] = r
		}
	}
	var cands []scored
	for _, s := range bestLinePerPick(all) {
		if s.Side == "OVER" && !s.Far {
			cands = append(cands, s)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Edge > cands[j].Edge })
	var out []OverWatch
	seen := map[[2]string]bool{}
	add := func(s scored, low *offRow) {
		k := [2]string{strconv.Itoa(s.PlayerID), string(s.Stat)}
		if seen[k] || on[k] {
			return
		}
		seen[k] = true
		w := OverWatch{GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Team: s.Team, Matchup: s.Matchup, Stat: string(s.Stat),
			Line: s.Line, Odds: s.Odds, Book: s.Vendor, Prob: s.Prob, Edge: s.Edge, Proj: s.ProjMean, ProjMin: s.ProjMin, Tip: s.Tip}
		if low != nil {
			w.Line, w.Odds, w.Book, w.Prob, w.Edge = low.Line, low.Odds, low.Vendor, low.Prob, low.Edge
			w.LowLine, w.Median, w.Books = true, low.Median, low.Books
		}
		switch {
		case w.Prob < hi && w.Edge >= minEdge:
			w.Status = fmt.Sprintf("price is fine, but the model gives it %.0f%% and a pick needs %.0f%%", 100*w.Prob, 100*hi)
		case w.Prob < hi:
			w.Status = fmt.Sprintf("model gives it %.0f%%; a pick needs %.0f%%", 100*w.Prob, 100*hi)
		case w.Edge < minEdge:
			w.Status = fmt.Sprintf("model likes it (%.0f%%) but the price of %s leaves no edge", 100*w.Prob, oddsStr(w.Odds))
		default:
			w.Status = "cleared the bar; lost its place to stronger plays"
		}
		if w.LowLine {
			w.Status = fmt.Sprintf("this book's line is %.1f below the %d-book median of %.1f. %s", w.Median-w.Line, w.Books, w.Median, capFirst(w.Status))
		}
		out = append(out, w)
	}
	// off-market Overs first (the mirror of the Under edge), then the strongest by edge
	var lows []offRow
	for _, r := range lowLine {
		lows = append(lows, r)
	}
	sort.Slice(lows, func(i, j int) bool { return lows[i].Edge > lows[j].Edge })
	byKey := map[[2]string]scored{}
	for _, c := range cands {
		byKey[[2]string{strconv.Itoa(c.PlayerID), string(c.Stat)}] = c
	}
	for i := range lows {
		r := lows[i]
		base, ok := byKey[[2]string{strconv.Itoa(r.PlayerID), string(r.Stat)}]
		if !ok {
			base = scored{PlayerID: r.PlayerID, Player: r.Player, Team: r.Team, Matchup: r.Matchup, Stat: r.Stat, ProjMean: r.Proj}
		}
		add(base, &r)
	}
	for _, c := range cands {
		if len(out) >= limit {
			break
		}
		add(c, nil)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LowLine != out[j].LowLine {
			return out[i].LowLine
		}
		return out[i].Edge > out[j].Edge
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
