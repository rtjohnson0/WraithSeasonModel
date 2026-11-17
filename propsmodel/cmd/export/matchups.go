package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"propsmodel/internal/picklog"
	"propsmodel/internal/props"
)

// placeholder reports whether a stored matchup is missing or one of predict's id fallbacks
// ("game 123"), which the website should never show.
func placeholder(m string) bool {
	m = strings.ToLower(strings.TrimSpace(m))
	return m == "" || strings.HasPrefix(m, "game ")
}

// matchupResolver turns a game id into "VIS @ HOME" using the day's snapshot (which team plays
// where) and the stats history (which abbreviation belongs to which team id). It exists because
// picks logged before the matchup was saved, or on a day the snapshot had no abbreviations,
// would otherwise show as a bare game number.
type matchupResolver struct {
	dir   string
	abbr  map[int]string
	games map[string]map[int]props.GameInfo
}

func newMatchupResolver(dir string, abbr map[int]string) *matchupResolver {
	return &matchupResolver{dir: dir, abbr: abbr, games: map[string]map[int]props.GameInfo{}}
}

// tip returns a game's scheduled tip-off (RFC3339 UTC) from the day's snapshot, or "".
func (r *matchupResolver) tip(date string, gameID int) string {
	if _, _, _, _ = r.lookup(date, gameID); r.games[date] == nil {
		return ""
	}
	return r.games[date][gameID].Start
}

// lookup returns the matchup and each side's abbreviation, or ok=false if it cannot tell.
func (r *matchupResolver) lookup(date string, gameID int) (matchup, home, visitor string, ok bool) {
	gs, seen := r.games[date]
	if !seen {
		gs, _ = props.LoadGames(filepath.Join(r.dir, "snapshots", fmt.Sprintf("props_%s.jsonl", date)))
		r.games[date] = gs
	}
	g, found := gs[gameID]
	if !found {
		return "", "", "", false
	}
	home, visitor = g.HomeAbbr, g.VisitorAbbr
	if home == "" {
		home = r.abbr[g.Home]
	}
	if visitor == "" {
		visitor = r.abbr[g.Visitor]
	}
	if home == "" || visitor == "" {
		return "", "", "", false
	}
	return visitor + " @ " + home, home, visitor, true
}

// fill repairs the matchup (and a "team 123" placeholder team) on entries in place.
func (r *matchupResolver) fill(entries []picklog.Entry) int {
	if r == nil {
		return 0
	}
	n := 0
	for i := range entries {
		e := &entries[i]
		if e.Tip == "" {
			e.Tip = r.tip(e.Date, e.GameID)
		}
		if !placeholder(e.Matchup) && !strings.HasPrefix(e.Team, "team ") && e.Team != "" {
			continue
		}
		m, home, visitor, ok := r.lookup(e.Date, e.GameID)
		if !ok {
			continue
		}
		if placeholder(e.Matchup) {
			e.Matchup = m
			n++
		}
		if e.Team == "" || strings.HasPrefix(e.Team, "team ") {
			// A pick's team is whichever side its player is on; the snapshot cannot say, so only
			// keep a real abbreviation that is one of the two sides.
			if e.Team != "" && e.Team != home && e.Team != visitor {
				e.Team = ""
			}
		}
	}
	return n
}
