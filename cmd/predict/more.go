package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"propsmodel/internal/picklog"
)

// MoreFile is data/more.json: each game's next-best lines that did not make the board, for the
// site's "More plays" button. It is rewritten on every run and is not part of the pick record.
type MoreFile struct {
	Date  string          `json:"date"`
	At    string          `json:"at"`
	Plays []picklog.Entry `json:"plays"`
	// Games is each game's betting context (consensus spread and total), for the board header.
	Games []GameCtx `json:"games,omitempty"`
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
func writeMore(dir, date string, extra []scored, ctx []GameCtx) error {
	f := MoreFile{Date: date, At: time.Now().UTC().Format(time.RFC3339), Games: ctx}
	for _, s := range extra {
		f.Plays = append(f.Plays, picklog.Entry{
			Date: date, GameID: s.GameID, PlayerID: s.PlayerID, Player: s.Player, Stat: string(s.Stat),
			Team: s.Team, Matchup: s.Matchup, Edge: s.Edge, Side: s.Side, Line: s.Line, Odds: s.Odds, Book: s.Vendor,
			Prob: s.Prob, ProbRaw: s.ProbRaw, ProjMean: s.ProjMean, ProjMin: s.ProjMin, Tier: "LEAN", Why: s.Why, Tip: s.Tip,
		})
	}
	b, err := json.MarshalIndent(f, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "more.json"), b, 0o644)
}
