// Command pull downloads NBA data from BALLDONTLIE into raw JSONL files under ./data.
//
//	go run ./cmd/pull stats    -seasons 2022,2023,2024,2025
//	go run ./cmd/pull advanced -seasons 2022,2023,2024,2025
//	go run ./cmd/pull props    -season 2025
//	go run ./cmd/pull injuries
//	go run ./cmd/pull players            (current team of every active player; run before predicting)
//	go run ./cmd/pull snapshot            (once)    or    go run ./cmd/pull snapshot -every 30m
//	go run ./cmd/pull compact             (drop duplicate rows from the stats and advanced files; pulls do this themselves)
//	go run ./cmd/pull raw -path /nba/v1/team_season_averages/general \
//	    -param season=2025 -param season_type=regular -param type=<value> -name team_general_2025
//
// Season numbers are the start year (2025 = the 2025-26 season). The API key is read from
// BALLDONTLIE_API_KEY (set it in .env next to where you run this).
//
// Stats and advanced stats are written one file per season (data/stats_2025.jsonl, ...), so a
// daily refresh of the current season is just: go run ./cmd/pull stats -seasons 2026
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed timezone data so America/New_York works on any machine

	"propsmodel/internal/bdl"
	"propsmodel/internal/data"
	"propsmodel/internal/envfile"
)

const dataDir = "data"

func main() {
	envfile.Load(".env")
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	key := os.Getenv("BALLDONTLIE_API_KEY")
	if key == "" {
		fatal("set BALLDONTLIE_API_KEY in .env or your environment")
	}
	c := bdl.New(key)

	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "stats":
		err = runStats(c, args)
	case "advanced":
		err = runAdvanced(c, args)
	case "props":
		err = runProps(c, args)
	case "gameodds":
		err = runGameOdds(c, args)
	case "injuries":
		err = runInjuries(c)
	case "players":
		err = runPlayers(c)
	case "snapshot":
		err = runSnapshot(c, args)
	case "raw":
		err = runRaw(c, args)
	case "compact":
		err = data.CompactDir(dataDir)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(err.Error())
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: pull <command> [flags]

commands:
  players                          current team of every active player (fixes trades and free agency)
  stats     -seasons 2022,2023     per-game player stats (minutes, pts, reb, ast); add -new for a fast daily refresh
  advanced  -seasons 2022,2023     per-game advanced stats (usage, pace, shares); add -new for a fast daily refresh
  props     -season 2025           opening player-prop lines for that season's games
  gameodds  -season 2025           opening spread, moneyline and total for that season's games
  injuries                         current injury report
  snapshot  [-date D] [-every 30m] save today's games, live props, game odds and injuries
                                   with timestamps under data/snapshots
  raw      -path P -param k=v -name N   any other endpoint`)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

// ---- output helper ----

type output struct {
	f *os.File
	w *bufio.Writer
}

func create(name string, appendMode bool) (*output, error) {
	full := filepath.Join(dataDir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(full, flags, 0o644)
	if err != nil {
		return nil, err
	}
	return &output{f: f, w: bufio.NewWriterSize(f, 1<<20)}, nil
}

func (o *output) Close() error {
	if err := o.w.Flush(); err != nil {
		o.f.Close()
		return err
	}
	return o.f.Close()
}

// writeRow writes one JSON value as a single compact line.
func (o *output) writeRow(raw []byte) error {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return err
	}
	b.WriteByte('\n')
	_, err := o.w.Write(b.Bytes())
	return err
}

func parseSeasons(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("-seasons is required, e.g. -seasons 2022,2023,2024,2025")
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("bad season %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// ---- commands ----

func runStats(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	seasonsFlag := fs.String("seasons", "", "comma-separated season start years")
	onlyNew := fs.Bool("new", false, "only fetch games since the last date already in the file (fast daily refresh)")
	fs.Parse(args)
	seasons, err := parseSeasons(*seasonsFlag)
	if err != nil {
		return err
	}

	// One file per season (stats_2025.jsonl, ...), so refreshing the current season
	// never rewrites older ones.
	for _, s := range seasons {
		name := fmt.Sprintf("stats_%d.jsonl", s)
		path := filepath.Join(dataDir, name)
		start := ""
		if *onlyNew {
			if d := maxGameDate(path); d != "" {
				start = backDays(d, 2) // overlap a little; duplicates are removed when the files load
			}
		}
		out, err := create(name, start != "")
		if err != nil {
			return err
		}
		n := 0
		types := []string{"regular", "playoffs", "playin", "ist"}
		if start != "" {
			types = []string{""} // all non-preseason games in one pass
		}
		for _, st := range types {
			q := url.Values{}
			q.Set("seasons[]", strconv.Itoa(s))
			if st != "" {
				q.Set("season_type", st)
			}
			if start != "" {
				q.Set("start_date", start)
			}
			err := c.Paginate("/nba/v1/stats", q, func(raw json.RawMessage) error {
				n++
				return out.writeRow(raw)
			})
			if err != nil {
				out.Close()
				return err
			}
			fmt.Printf("season %d %s: %d rows so far\n", s, strings.TrimSpace(st+" "), n)
		}
		if err := out.Close(); err != nil {
			return err
		}
		if start != "" {
			fmt.Printf("added %d rows since %s to %s\n", n, start, path)
		} else {
			fmt.Printf("wrote %d rows to %s\n", n, path)
		}
	}
	for _, s := range seasons {
		if b, a, err := data.CompactStats(filepath.Join(dataDir, fmt.Sprintf("stats_%d.jsonl", s))); err == nil && b != a {
			fmt.Printf("compacted stats_%d.jsonl: %d rows to %d\n", s, b, a)
		}
	}
	return nil
}

// maxGameDate is the latest game date in a JSONL stats file ("" if the file is missing or empty).
func maxGameDate(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	best := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var row struct {
			Game struct {
				Date string `json:"date"`
			} `json:"game"`
		}
		if json.Unmarshal(sc.Bytes(), &row) != nil || len(row.Game.Date) < 10 {
			continue
		}
		if d := row.Game.Date[:10]; d > best {
			best = d
		}
	}
	return best
}

// backDays moves a YYYY-MM-DD date back by n days.
func backDays(date string, n int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, -n).Format("2006-01-02")
}

func runAdvanced(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("advanced", flag.ExitOnError)
	seasonsFlag := fs.String("seasons", "", "comma-separated season start years")
	onlyNew := fs.Bool("new", false, "only fetch games since the last date in the stats file (fast daily refresh)")
	fs.Parse(args)
	seasons, err := parseSeasons(*seasonsFlag)
	if err != nil {
		return err
	}

	// Each page is appended as it arrives and the next cursor is saved beside the file, so a
	// failed pull resumes where it stopped instead of starting over. A finished season has no
	// .cursor file and is skipped on later runs (delete its advanced_*.jsonl to pull it again).
	for _, s := range seasons {
		name := fmt.Sprintf("advanced_%d.jsonl", s)
		dataPath := filepath.Join(dataDir, name)
		curPath := dataPath + ".cursor"
		if *onlyNew {
			if err := advancedSince(c, s, dataPath); err != nil {
				return err
			}
			continue
		}
		_, dataErr := os.Stat(dataPath)
		cursorBytes, curErr := os.ReadFile(curPath)
		if dataErr == nil && curErr != nil {
			// A file with no cursor marker is either finished or left by an older version that
			// could not resume. Treat it as unfinished only if it is clearly short.
			if n := countLines(dataPath); n >= 20000 {
				fmt.Printf("%s already pulled (%d rows), skipping\n", name, n)
				continue
			}
		}
		cursor := strings.TrimSpace(string(cursorBytes))
		appendMode := cursor != ""
		out, err := create(name, appendMode)
		if err != nil {
			return err
		}
		n := 0
		for {
			q := url.Values{}
			q.Set("seasons[]", strconv.Itoa(s))
			q.Set("per_page", "100")
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			body, err := c.Get("/nba/v2/stats/advanced", q)
			if err != nil {
				out.Close()
				return fmt.Errorf("%w (progress is saved; rerun the same command to resume)", err)
			}
			var page struct {
				Data []json.RawMessage `json:"data"`
				Meta struct {
					NextCursor json.RawMessage `json:"next_cursor"`
				} `json:"meta"`
			}
			if err := json.Unmarshal(body, &page); err != nil {
				out.Close()
				return err
			}
			for _, row := range page.Data {
				if err := out.writeRow(row); err != nil {
					out.Close()
					return err
				}
				n++
			}
			cursor = strings.Trim(string(page.Meta.NextCursor), `"`)
			if cursor == "null" {
				cursor = ""
			}
			if err := out.w.Flush(); err != nil {
				out.Close()
				return err
			}
			if cursor == "" {
				os.Remove(curPath)
				break
			}
			os.WriteFile(curPath, []byte(cursor), 0o644)
			if n%5000 < 100 {
				fmt.Printf("season %d: %d rows so far\n", s, n)
			}
		}
		if err := out.Close(); err != nil {
			return err
		}
		fmt.Printf("wrote %d rows to %s\n", n, dataPath)
	}
	// Drop the rows the refresh downloaded twice so the files stay small.
	for _, s := range seasons {
		if b, a, err := data.CompactAdvanced(filepath.Join(dataDir, fmt.Sprintf("advanced_%d.jsonl", s))); err == nil && b != a {
			fmt.Printf("compacted advanced_%d.jsonl: %d rows to %d\n", s, b, a)
		}
	}
	return nil
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		n++
	}
	return n
}

// gameIDsForSeason reads data/stats_<season>.jsonl and returns the unique game ids in it.
func gameIDsForSeason(season int) ([]int, error) {
	f, err := os.Open(filepath.Join(dataDir, fmt.Sprintf("stats_%d.jsonl", season)))
	if err != nil {
		return nil, fmt.Errorf("run the stats pull for this season first (game ids come from it): %w", err)
	}
	defer f.Close()

	seen := map[int]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var row struct {
			Game struct {
				ID     int `json:"id"`
				Season int `json:"season"`
			} `json:"game"`
		}
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		if row.Game.ID != 0 && (row.Game.Season == 0 || row.Game.Season == season) {
			seen[row.Game.ID] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids, nil
}

func alreadyPulled(path string) map[int]bool {
	done := map[int]bool{}
	f, err := os.Open(path)
	if err != nil {
		return done
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var row struct {
			GameID int `json:"_game_id"`
		}
		if json.Unmarshal(sc.Bytes(), &row) == nil && row.GameID != 0 {
			done[row.GameID] = true
		}
	}
	return done
}

func runProps(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("props", flag.ExitOnError)
	season := fs.Int("season", 0, "season start year, e.g. 2025")
	fs.Parse(args)
	if *season == 0 {
		return fmt.Errorf("-season is required")
	}

	ids, err := gameIDsForSeason(*season)
	if err != nil {
		return err
	}
	fmt.Printf("%d games found for season %d\n", len(ids), *season)

	name := fmt.Sprintf("opening_props_%d.jsonl", *season)
	done := alreadyPulled(filepath.Join(dataDir, name)) // resume support
	out, err := create(name, true)
	if err != nil {
		return err
	}
	defer out.Close()

	for i, id := range ids {
		if done[id] {
			continue
		}
		q := url.Values{}
		q.Set("game_id", strconv.Itoa(id))
		body, err := c.Get("/nba/v2/odds/player_props/opening", q)
		if err != nil {
			return err
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, body); err != nil {
			return err
		}
		line := fmt.Sprintf(`{"_game_id":%d,"response":%s}`, id, compact.String())
		if err := out.writeRow([]byte(line)); err != nil {
			return err
		}
		if (i+1)%100 == 0 {
			fmt.Printf("%d/%d games\n", i+1, len(ids))
		}
	}
	fmt.Printf("wrote %s\n", filepath.Join(dataDir, name))
	return nil
}

// runGameOdds saves the opening spread, moneyline and total for every game of a season, which
// is what the game-market backtest (go run ./cmd/games -backtest) is graded against.
func runGameOdds(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("gameodds", flag.ExitOnError)
	season := fs.Int("season", 0, "season start year, e.g. 2025")
	fs.Parse(args)
	if *season == 0 {
		return fmt.Errorf("-season is required")
	}
	ids, err := gameIDsForSeason(*season)
	if err != nil {
		return err
	}
	fmt.Printf("%d games found for season %d\n", len(ids), *season)
	name := fmt.Sprintf("opening_games_%d.jsonl", *season)
	out, err := create(name, false)
	if err != nil {
		return err
	}
	defer out.Close()
	rows := 0
	for i := 0; i < len(ids); i += 20 {
		end := i + 20
		if end > len(ids) {
			end = len(ids)
		}
		q := url.Values{}
		for _, id := range ids[i:end] {
			q.Add("game_ids[]", strconv.Itoa(id))
		}
		err := c.Paginate("/nba/v2/odds/opening", q, func(raw json.RawMessage) error {
			rows++
			return out.writeRow(raw)
		})
		if err != nil {
			return err
		}
		if (i/20+1)%10 == 0 {
			fmt.Printf("%d/%d games, %d odds rows\n", end, len(ids), rows)
		}
	}
	fmt.Printf("wrote %d rows to %s\n", rows, filepath.Join(dataDir, name))
	return nil
}

// runPlayers saves each active player with his CURRENT team. Box scores only know the team a
// player last played for, which is wrong for anyone traded or signed since.
func runPlayers(c *bdl.Client) error {
	out, err := create("players_active.jsonl", false)
	if err != nil {
		return err
	}
	n := 0
	err = c.Paginate("/nba/v1/players/active", url.Values{}, func(raw json.RawMessage) error {
		n++
		return out.writeRow(raw)
	})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	fmt.Printf("wrote %d active players to %s\n", n, filepath.Join(dataDir, "players_active.jsonl"))
	return nil
}

func runInjuries(c *bdl.Client) error {
	out, err := create("injuries.jsonl", false)
	if err != nil {
		return err
	}
	defer out.Close()

	n := 0
	err = c.Paginate("/nba/v1/player_injuries", url.Values{}, func(raw json.RawMessage) error {
		n++
		return out.writeRow(raw)
	})
	if err != nil {
		return err
	}
	fmt.Printf("wrote %d injury rows to %s\n", n, filepath.Join(dataDir, "injuries.jsonl"))
	return nil
}

// ---- snapshot: build your own line history ----
//
// BALLDONTLIE does not keep live props after games finish, so this saves them as they are.
// Every line written carries a captured_at timestamp, so running it repeatedly records line
// movement and gives you a closing-line record to measure closing line value against.

func runSnapshot(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	dateFlag := fs.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	every := fs.Duration("every", 0, "repeat on this interval, e.g. 30m (default: run once)")
	fs.Parse(args)

	for {
		date := *dateFlag
		if date == "" {
			date = todayEastern()
		}
		if err := snapshotOnce(c, date); err != nil {
			if *every == 0 {
				return err
			}
			fmt.Fprintln(os.Stderr, "snapshot error:", err)
		}
		if *every == 0 {
			return nil
		}
		time.Sleep(*every)
	}
}

func todayEastern() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.Local
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func compactJSON(raw []byte) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return "null"
	}
	return b.String()
}

// writeWrapped writes {"captured_at": ..., "row": <raw>} as one line.
func writeWrapped(o *output, captured string, raw []byte) error {
	line := fmt.Sprintf(`{"captured_at":%q,"row":%s}`, captured, compactJSON(raw))
	return o.writeRow([]byte(line))
}

func snapshotOnce(c *bdl.Client, date string) error {
	captured := time.Now().UTC().Format(time.RFC3339)

	// 1. Today's games.
	var games []json.RawMessage
	var ids []int
	q := url.Values{}
	q.Add("dates[]", date)
	err := c.Paginate("/nba/v1/games", q, func(raw json.RawMessage) error {
		var g struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			return err
		}
		games = append(games, raw)
		ids = append(ids, g.ID)
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d games on %s\n", captured, len(ids), date)
	if len(ids) == 0 {
		return nil
	}

	// 2. Live player props, one call per game.
	props, err := create(fmt.Sprintf("snapshots/props_%s.jsonl", date), true)
	if err != nil {
		return err
	}
	saved := 0
	for i, id := range ids {
		body, err := c.Get("/nba/v2/odds/player_props", url.Values{"game_id": {strconv.Itoa(id)}})
		if err != nil {
			fmt.Fprintf(os.Stderr, "game %d props: %v\n", id, err)
			continue
		}
		line := fmt.Sprintf(`{"captured_at":%q,"game_id":%d,"game":%s,"response":%s}`,
			captured, id, compactJSON(games[i]), compactJSON(body))
		if err := props.writeRow([]byte(line)); err != nil {
			props.Close()
			return err
		}
		saved++
	}
	if err := props.Close(); err != nil {
		return err
	}
	fmt.Printf("  props saved for %d/%d games\n", saved, len(ids))

	// 3. Game odds (spread and total), for game-script features.
	oddsOut, err := create(fmt.Sprintf("snapshots/odds_%s.jsonl", date), true)
	if err != nil {
		return err
	}
	oq := url.Values{}
	oq.Add("dates[]", date)
	err = c.Paginate("/nba/v2/odds", oq, func(raw json.RawMessage) error {
		return writeWrapped(oddsOut, captured, raw)
	})
	oddsOut.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "game odds:", err)
	}

	// 4. Injury report. The API has no injury history, so this is how you build one.
	injOut, err := create(fmt.Sprintf("snapshots/injuries_%s.jsonl", date), true)
	if err != nil {
		return err
	}
	err = c.Paginate("/nba/v1/player_injuries", url.Values{}, func(raw json.RawMessage) error {
		return writeWrapped(injOut, captured, raw)
	})
	injOut.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "injuries:", err)
	}
	return nil
}

// multiFlag collects a repeated flag (-param a=1 -param b=2).
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func runRaw(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("raw", flag.ExitOnError)
	path := fs.String("path", "", "endpoint path, e.g. /nba/v1/team_season_averages/general")
	name := fs.String("name", "", "output name; writes data/raw_<name>.jsonl")
	var params multiFlag
	fs.Var(&params, "param", "query parameter k=v (repeat for more; repeat a key for a list)")
	fs.Parse(args)
	if *path == "" || *name == "" {
		return fmt.Errorf("-path and -name are required")
	}

	q := url.Values{}
	for _, p := range params {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return fmt.Errorf("bad -param %q, want k=v", p)
		}
		q.Add(k, v)
	}

	out, err := create("raw_"+*name+".jsonl", false)
	if err != nil {
		return err
	}
	defer out.Close()

	n := 0
	err = c.Paginate(*path, q, func(raw json.RawMessage) error {
		n++
		return out.writeRow(raw)
	})
	if err != nil {
		return err
	}
	fmt.Printf("wrote %d rows to %s\n", n, filepath.Join(dataDir, "raw_"+*name+".jsonl"))
	return nil
}

// advancedSince appends advanced stats for games after the newest date in the matching stats
// file minus a few days. Rows for games already in the advanced file are harmless duplicates.
func advancedSince(c *bdl.Client, season int, dataPath string) error {
	last := maxGameDate(filepath.Join(dataDir, fmt.Sprintf("stats_%d.jsonl", season)))
	if last == "" {
		return fmt.Errorf("no stats file for season %d; pull stats first", season)
	}
	start := backDays(last, 7)
	out, err := create(filepath.Base(dataPath), true)
	if err != nil {
		return err
	}
	n := 0
	q := url.Values{}
	q.Set("seasons[]", strconv.Itoa(season))
	q.Set("start_date", start)
	err = c.Paginate("/nba/v2/stats/advanced", q, func(raw json.RawMessage) error {
		n++
		return out.writeRow(raw)
	})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	fmt.Printf("added %d advanced rows since %s to %s\n", n, start, dataPath)
	return nil
}
