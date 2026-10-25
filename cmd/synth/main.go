// Command synth writes a SYNTHETIC data directory in the same file formats the pull command
// produces, so you can dry-run the whole pipeline without an API key:
//
//	go run ./cmd/synth -out demo_data
//	go run ./cmd/backtest -data demo_data -season 2025 -ablate
//	go run ./cmd/train    -data demo_data
//	go run ./cmd/predict  -data demo_data -date <date printed by synth>
//
// The league is simulated (see internal/synthworld): it has opponents, pace, injuries, blowouts
// and streaks, and a simulated sportsbook that sees most of them but not all. It checks that
// the code finds structure that is really there. It proves NOTHING about edge against real
// sportsbooks, which are much sharper than this one. Only a backtest on your own data can say.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"propsmodel/internal/data"
	"propsmodel/internal/props"
	"propsmodel/internal/synthworld"
)

func main() {
	out := flag.String("out", "demo_data", "output directory")
	seed := flag.Int64("seed", 42, "random seed")
	teams := flag.Int("teams", 30, "number of teams")
	flag.Parse()

	seasons := []int{2023, 2024, 2025}
	w := synthworld.Generate(synthworld.Config{Seed: *seed, Seasons: seasons, Teams: *teams})
	must(os.MkdirAll(filepath.Join(*out, "snapshots"), 0o755))

	bySeason := map[int][]data.PlayerGame{}
	for _, g := range w.Games {
		bySeason[g.Season] = append(bySeason[g.Season], g)
	}
	for _, s := range seasons {
		sf, sw := createW(filepath.Join(*out, fmt.Sprintf("stats_%d.jsonl", s)))
		af, aw := createW(filepath.Join(*out, fmt.Sprintf("advanced_%d.jsonl", s)))
		for _, g := range bySeason[s] {
			writeJSON(sw, map[string]any{
				"min": fmt.Sprintf("%d", int(g.Minutes)), "pts": g.Pts, "reb": g.Reb, "ast": g.Ast,
				"fga": g.Fga, "fgm": g.Fgm, "fg3a": g.Fg3a, "fg3m": g.Fg3m,
				"player": map[string]any{"id": g.PlayerID, "first_name": strings.Fields(g.Name)[0], "last_name": "Synthetic", "team_id": g.TeamID},
				"team":   map[string]any{"id": g.TeamID},
				"game": map[string]any{
					"id": g.GameID, "date": g.Date, "season": g.Season,
					"home_team_id": g.HomeTeamID, "visitor_team_id": g.VisitorTeamID,
				},
			})
			a := w.Adv[data.AdvKey{Player: g.PlayerID, Game: g.GameID}]
			row := map[string]any{"period": 0, "player": map[string]any{"id": g.PlayerID}, "game": map[string]any{"id": g.GameID}}
			put := func(k string, v float64) {
				if !math.IsNaN(v) {
					row[k] = v
				}
			}
			put("usage_percentage", a.Usage)
			put("pace", a.Pace)
			put("assist_percentage", a.AstPct)
			put("rebound_percentage", a.RebPct)
			put("touches", a.Touches)
			put("passes", a.Passes)
			put("rebound_chances_total", a.RebCh)
			put("rebound_chances_off", a.RebChOff)
			writeJSON(aw, row)
		}
		must(sw.Flush())
		must(aw.Flush())
		sf.Close()
		af.Close()
	}

	// Opening lines for the last season, grouped by game as `pull props` writes them.
	last := seasons[len(seasons)-1]
	gameSeason := map[int]int{}
	for _, g := range w.Games {
		gameSeason[g.GameID] = g.Season
	}
	byGame := map[int][]props.Line{}
	var order []int
	for _, l := range w.Lines {
		if gameSeason[l.GameID] != last {
			continue
		}
		if _, ok := byGame[l.GameID]; !ok {
			order = append(order, l.GameID)
		}
		byGame[l.GameID] = append(byGame[l.GameID], l)
	}
	sort.Ints(order)
	pf, pw := createW(filepath.Join(*out, fmt.Sprintf("opening_props_%d.jsonl", last)))
	for _, id := range order {
		writeJSON(pw, map[string]any{"_game_id": id, "response": map[string]any{"data": apiRows(byGame[id])}})
	}
	must(pw.Flush())
	pf.Close()

	// Snapshot for the day after the last game, as `pull snapshot` writes it.
	up := w.Upcoming
	captured := time.Now().UTC().Format(time.RFC3339)
	sf, sw := createW(filepath.Join(*out, "snapshots", fmt.Sprintf("props_%s.jsonl", up.Date)))
	linesByGame := map[int][]props.Line{}
	for _, l := range up.Lines {
		linesByGame[l.GameID] = append(linesByGame[l.GameID], l)
	}
	for _, g := range up.Games {
		writeJSON(sw, map[string]any{
			"captured_at": captured, "game_id": g.ID,
			"game": map[string]any{
				"id": g.ID, "date": up.Date,
				"home_team": map[string]any{"id": g.Home}, "visitor_team": map[string]any{"id": g.Visitor},
			},
			"response": map[string]any{"data": apiRows(linesByGame[g.ID])},
		})
	}
	must(sw.Flush())
	sf.Close()

	jf, jw := createW(filepath.Join(*out, "snapshots", fmt.Sprintf("injuries_%s.jsonl", up.Date)))
	var outIDs []int
	for id := range up.Out {
		outIDs = append(outIDs, id)
	}
	sort.Ints(outIDs)
	for _, id := range outIDs {
		writeJSON(jw, map[string]any{"captured_at": captured, "row": map[string]any{
			"status": "Out", "player": map[string]any{"id": id},
		}})
	}
	must(jw.Flush())
	jf.Close()

	fmt.Printf("wrote synthetic data to %s (%d player-games, %d teams)\n", *out, len(w.Games), *teams)
	fmt.Printf("try:\n  go run ./cmd/backtest -data %s -season %d -ablate\n  go run ./cmd/train    -data %s\n  go run ./cmd/predict  -data %s -date %s\n",
		*out, last, *out, *out, up.Date)
}

func apiRows(lines []props.Line) []map[string]any {
	var rows []map[string]any
	for _, l := range lines {
		rows = append(rows, map[string]any{
			"game_id": l.GameID, "player_id": l.PlayerID, "vendor": l.Vendor, "prop_type": string(l.Stat),
			"line_value": fmt.Sprintf("%.1f", l.Line),
			"market":     map[string]any{"type": "over_under", "over_odds": l.OverOdds, "under_odds": l.UnderOdds},
			"updated_at": l.UpdatedAt,
		})
	}
	return rows
}

func createW(path string) (*os.File, *bufio.Writer) {
	f, err := os.Create(path)
	must(err)
	return f, bufio.NewWriterSize(f, 1<<20)
}

func writeJSON(w *bufio.Writer, v any) {
	b, err := json.Marshal(v)
	must(err)
	w.Write(b)
	w.WriteByte('\n')
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
