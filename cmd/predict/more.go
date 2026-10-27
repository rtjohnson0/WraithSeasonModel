package main

import (
	"encoding/json"
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
func writeMore(dir, date string, extra []scored, ctx []GameCtx, moves map[string][]MovePoint, news []GameNews) error {
	f := MoreFile{Date: date, At: time.Now().UTC().Format(time.RFC3339), Games: ctx, Moves: moves, News: news}
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
