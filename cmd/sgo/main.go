// Command sgo pulls NBA player-prop lines from SportsGameOdds, which has more sportsbooks than
// BALLDONTLIE, so predict can shop for the best number.
//
//	go run ./cmd/sgo sample                        look at the raw data first (do this once)
//	go run ./cmd/sgo snapshot                      today's lines -> data/snapshots/sgo_DATE.jsonl
//	go run ./cmd/sgo snapshot -date 2026-10-21 -every 30m
//
// The API key is read from SPORTSGAMEODDS_API_KEY in .env. I could not reach this API while
// writing the code, so field names follow the provider's docs but are unproven: run `sample`
// and check that the parsed lines look right (player names, lines, odds) before relying on it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"propsmodel/internal/envfile"
	"propsmodel/internal/sgo"
)

const dataDir = "data"

func main() {
	envfile.Load(".env")
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ./cmd/sgo sample | snapshot [-date D] [-every 30m]")
		os.Exit(2)
	}
	key := os.Getenv("SPORTSGAMEODDS_API_KEY")
	if key == "" {
		fatal(fmt.Errorf("set SPORTSGAMEODDS_API_KEY in .env or your environment"))
	}
	c := sgo.New(key)
	var err error
	switch os.Args[1] {
	case "sample":
		err = runSample(c)
	case "snapshot":
		err = runSnapshot(c, os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fatal(err)
	}
}

// oddIDs asks only for full-game over/under lines for any player: points, rebounds, assists and
// threes, plus the four combined markets (points+rebounds+assists, points+rebounds, points+assists,
// rebounds+assists).
func oddIDs() string {
	var ids []string
	for _, s := range []string{"points", "rebounds", "assists", "threePointersMade",
		"points+rebounds+assists", "points+rebounds", "points+assists", "rebounds+assists"} {
		ids = append(ids, s+"-PLAYER_ID-game-ou-over", s+"-PLAYER_ID-game-ou-under")
	}
	return strings.Join(ids, ",")
}

func baseQuery() url.Values {
	q := url.Values{}
	q.Set("leagueID", "NBA")
	q.Set("oddsAvailable", "true")
	q.Set("oddIDs", oddIDs())
	return q
}

func runSample(c *sgo.Client) error {
	q := baseQuery()
	q.Set("limit", "1")
	body, err := c.Get("/events", q)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dataDir, "sgo_sample.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("saved the raw response (%d bytes) to %s\n", len(body), path)
	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || len(resp.Data) == 0 {
		fmt.Println("no events came back. There may be no NBA games with open odds right now,")
		fmt.Println("or the response shape differs from the docs: open the saved file and send me the first 40 lines.")
		return nil
	}
	lines := sgo.ParseEvent(resp.Data[0])
	fmt.Printf("parsed %d player lines from the first event. First few:\n", len(lines))
	for i, l := range lines {
		if i == 12 {
			break
		}
		fmt.Printf("  %-22s %-9s %-14s line %.1f  over %d  under %d\n", l.Name, l.Stat, l.Book, l.Line, l.OverOdds, l.UnderOdds)
	}
	if len(lines) == 0 {
		fmt.Println("nothing parsed: send me the first 40 lines of the saved file and I will fix the parser.")
	}
	return nil
}

func runSnapshot(c *sgo.Client, args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	date := fs.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	every := fs.Duration("every", 0, "repeat at this interval, e.g. 30m")
	fs.Parse(args)
	for {
		d := *date
		if d == "" {
			d = todayEastern()
		}
		if err := snapshotOnce(c, d); err != nil {
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

func snapshotOnce(c *sgo.Client, date string) error {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.Local
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return fmt.Errorf("bad -date: %w", err)
	}
	q := baseQuery()
	q.Set("startsAfter", day.UTC().Format(time.RFC3339))
	q.Set("startsBefore", day.Add(30*time.Hour).UTC().Format(time.RFC3339))
	q.Set("limit", "50")

	captured := time.Now().UTC().Format(time.RFC3339)
	var rows []sgo.SnapshotRow
	events := 0
	err = c.Events(q, func(raw json.RawMessage) error {
		events++
		for _, l := range sgo.ParseEventAll(raw) {
			rows = append(rows, sgo.SnapshotRow{CapturedAt: captured, PlayerLine: l})
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d events, %d player lines for %s\n", captured, events, len(rows), date)
	if len(rows) == 0 {
		return nil
	}
	dir := filepath.Join(dataDir, "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("sgo_%s.jsonl", date)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
