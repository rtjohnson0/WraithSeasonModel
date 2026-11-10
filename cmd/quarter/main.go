// Command quarter compares explicitly tagged 1Q point lines with verified 1Q scoring.
// Full-game models and performance records are never used for these comparisons.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"propsmodel/internal/bdl"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/envfile"
	"propsmodel/internal/props"
	"propsmodel/internal/quarter"
	"sort"
	"strconv"
	"time"
	_ "time/tzdata"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "board date (Eastern today)")
	offline := flag.Bool("offline", false, "use previously validated Q1 cache only")
	flag.Parse()
	envfile.Load(".env")
	now := time.Now().UTC()
	if *date == "" {
		loc, _ := time.LoadLocation("America/New_York")
		*date = now.In(loc).Format("2006-01-02")
	}
	if e := run(*dir, *date, now, *offline); e != nil {
		fmt.Fprintln(os.Stderr, "quarter:", e)
		os.Exit(1)
	}
}
func run(dir, date string, now time.Time, offline bool) error {
	board := quarter.Board{Date: date, At: now.Format(time.RFC3339Nano), Cards: []quarter.Card{}, Recent: []quarter.Entry{}}
	// Clear the prior snapshot immediately, so a failed refresh cannot retain it.
	if e := quarter.Save(filepath.Join(dir, "quarterboard.json"), board); e != nil {
		return e
	}
	path := filepath.Join(dir, "snapshots", "props_"+date+".jsonl")
	gm, e := props.LoadGames(path)
	if e != nil {
		return e
	}
	raw, e := props.LoadQuarterSnapshot(path, now)
	if e != nil {
		return e
	}
	var lines []props.Line
	for _, l := range raw {
		if props.QuarterExecutable(l, gm[l.GameID], now) {
			lines = append(lines, l)
		}
	}
	lines, _ = props.ReconcileBooks(lines)
	lines, _ = props.FilterConsensus(lines, 3, .5)
	counts := quarter.BookCount(lines)
	stats, e := data.LoadStatsDir(dir)
	if e != nil {
		return e
	}
	byGame := map[int][]data.PlayerGame{}
	byPlayer := map[int][]data.PlayerGame{}
	names := map[int]string{}
	for _, r := range stats {
		byGame[r.GameID] = append(byGame[r.GameID], r)
		byPlayer[r.PlayerID] = append(byPlayer[r.PlayerID], r)
		names[r.PlayerID] = r.Name
	}
	logpath := filepath.Join(dir, "quarterlog.jsonl")
	logs, e := quarter.LoadLog(logpath)
	if e != nil {
		return e
	}
	expected := map[int][]int{}
	needed := map[int]bool{}
	for _, l := range lines {
		if _, ok := expected[l.PlayerID]; ok {
			continue
		}
		rs := byPlayer[l.PlayerID]
		for i := len(rs) - 1; i >= 0 && len(expected[l.PlayerID]) < 10; i-- {
			r := rs[i]
			if r.Date < date && r.Season >= 2025 && r.Minutes > 0 {
				expected[l.PlayerID] = append(expected[l.PlayerID], r.GameID)
				needed[r.GameID] = true
			}
		}
	}
	for _, r := range logs {
		if r.Date < date {
			needed[r.GameID] = true
		}
	}
	ids := make([]int, 0, len(needed))
	for id := range needed {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	key := os.Getenv("BALLDONTLIE_API_KEY")
	client := bdl.New(key)
	games := map[int]quarter.GameData{}
	var observations []quarter.Observation
	failures := 0
	for _, id := range ids {
		cache := filepath.Join(dir, "quarter_games", strconv.Itoa(id)+".json")
		var cached quarter.GameData
		b, err := os.ReadFile(cache)
		valid := err == nil && json.Unmarshal(b, &cached) == nil && cached.Version == 1 && cached.GameID == id
		recent := len(byGame[id]) > 0 && byGame[id][0].Date >= now.AddDate(0, 0, -3).Format("2006-01-02")
		if (!valid || recent) && !offline {
			if key == "" {
				board.Errors = append(board.Errors, "Set BALLDONTLIE_API_KEY to download verified first-quarter scoring.")
				break
			}
			fresh, err := fetch(client, id, byGame[id])
			if err != nil {
				failures++
				board.Errors = append(board.Errors, fmt.Sprintf("Game %d: Q1 data unavailable (%s)", id, err))
				if failures >= 3 {
					board.Errors = append(board.Errors, "Stopped quarter downloads after three failures; remaining games need another refresh.")
					break
				}
				continue
			}
			fresh.FetchedAt = now.Format(time.RFC3339Nano)
			if err = quarter.Save(cache, fresh); err != nil {
				return err
			}
			cached = fresh
			valid = true
		}
		if valid {
			games[id] = cached
			observations = append(observations, cached.Rows...)
		}
	}
	// Regrade the separate paper log from quarter actuals only. REVIEW is never a loss.
	seen := map[string]bool{}
	for i := range logs {
		r := &logs[i]
		seen[r.ID()] = true
		if g, ok := games[r.GameID]; ok {
			r.Result = "REVIEW"
			r.Actual = nil
			for _, a := range g.Rows {
				if a.PlayerID == r.PlayerID {
					r.Result = quarter.Result(a, r.Side, r.Line)
					if a.Participated {
						n := a.Points
						r.Actual = &n
					}
					break
				}
			}
		}
	}
	for _, l := range lines {
		g := gm[l.GameID]
		c := quarter.Card{GameID: l.GameID, PlayerID: l.PlayerID, Player: names[l.PlayerID], Matchup: g.VisitorAbbr + " @ " + g.HomeAbbr, Period: "1q", Stat: "points", Book: l.Vendor, Line: l.Line, OverOdds: l.OverOdds, UnderOdds: l.UnderOdds, QuoteAt: l.CapturedAt, SourceUpdatedAt: l.UpdatedAt, Tip: g.Start, BookCount: counts[props.MarketKey{Game: l.GameID, Player: l.PlayerID, Stat: "points"}], Comparison: quarter.Compare(observations, l.PlayerID, date, l.Line, expected[l.PlayerID])}
		if c.Player == "" {
			c.Player = fmt.Sprintf("Player %d", l.PlayerID)
		}
		cmp := c.Comparison
		if cmp.N >= 8 && cmp.Missing == 0 {
			over := float64(cmp.Over+1) / float64(cmp.N-cmp.Push+2)
			under := 1 - over
			side, odds, p := "OVER", l.OverOdds, over
			if under-bets.Implied(l.UnderOdds) > over-bets.Implied(l.OverOdds) {
				side, odds, p = "UNDER", l.UnderOdds, under
			}
			c.Side = side
			c.Estimate = &p
			c.BreakEven = bets.Implied(odds)
			entry := quarter.Entry{Date: date, GameID: l.GameID, PlayerID: l.PlayerID, Player: c.Player, Period: "1q", Stat: "points", Side: side, Book: l.Vendor, Line: l.Line, Odds: odds, LoggedAt: now.Format(time.RFC3339Nano), Tip: g.Start, QuoteAt: l.CapturedAt}
			if !seen[entry.ID()] {
				logs = append(logs, entry)
				seen[entry.ID()] = true
			}
		}
		board.Cards = append(board.Cards, c)
	}
	for _, r := range logs {
		switch r.Result {
		case "WIN":
			board.Record.Wins++
		case "LOSS":
			board.Record.Losses++
		case "PUSH":
			board.Record.Pushes++
		case "REVIEW":
			board.Record.Review++
		default:
			board.Record.Pending++
		}
	}
	for i := len(logs) - 1; i >= 0 && len(board.Recent) < 30; i-- {
		if logs[i].Result != "" {
			board.Recent = append(board.Recent, logs[i])
		}
	}
	if e = quarter.SaveLog(logpath, logs); e != nil {
		return e
	}
	if e = quarter.Save(filepath.Join(dir, "quarterboard.json"), board); e != nil {
		return e
	}
	fmt.Printf("1Q points: %d exact book quotes; %d validated games; %d unavailable games. Full-game data was not substituted.\n", len(board.Cards), len(games), len(board.Errors))
	return nil
}
func fetch(c *bdl.Client, id int, roster []data.PlayerGame) (quarter.GameData, error) {
	var game struct {
		Data quarter.Game `json:"data"`
	}
	b, e := c.Get("/v1/games/"+strconv.Itoa(id), nil)
	if e != nil {
		return quarter.GameData{}, e
	}
	if e = json.Unmarshal(b, &game); e != nil {
		return quarter.GameData{}, e
	}
	if game.Data.ID != id {
		return quarter.GameData{}, fmt.Errorf("game ID mismatch")
	}
	b, e = c.Get("/v1/plays", url.Values{"game_id": {strconv.Itoa(id)}})
	if e != nil {
		return quarter.GameData{}, e
	}
	var response struct {
		Data []quarter.Play `json:"data"`
	}
	if e = json.Unmarshal(b, &response); e != nil {
		return quarter.GameData{}, e
	}
	starters := map[int]bool{}
	e = c.Paginate("/nba/v1/lineups", url.Values{"game_ids": {strconv.Itoa(id)}}, func(raw json.RawMessage) error {
		var row struct {
			GameID  int  `json:"game_id"`
			Starter bool `json:"starter"`
			Player  struct {
				ID int `json:"id"`
			} `json:"player"`
		}
		if e := json.Unmarshal(raw, &row); e != nil {
			return e
		}
		if row.GameID != id {
			return fmt.Errorf("lineup game mismatch")
		}
		if row.Starter {
			starters[row.Player.ID] = true
		}
		return nil
	})
	if e != nil {
		return quarter.GameData{}, e
	}
	if len(starters) != 10 {
		return quarter.GameData{}, fmt.Errorf("incomplete confirmed starters")
	}
	return quarter.Extract(game.Data, response.Data, roster, starters)
}
