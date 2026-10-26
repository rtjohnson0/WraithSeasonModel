// Command moves lists how today's prop lines have moved since the first snapshot of the day.
//
//	go run ./cmd/moves                 (biggest movers, today)
//	go run ./cmd/moves -min 1 -top 40
//
// It needs at least two snapshots (go run ./cmd/pull snapshot, or the scheduled runs). It shows the
// movement, not the reason: the feed has no bet counts. As a rule of thumb, a move across most books
// together (and the same direction in a teammate's line) is usually news; one book alone is usually
// a stale or shaded number. A line that moves toward your pick is the market agreeing with you.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"propsmodel/internal/pipeline"
	"propsmodel/internal/props"
)

func main() {
	dir := flag.String("data", "data", "data directory")
	date := flag.String("date", "", "game date YYYY-MM-DD (default: today, US Eastern)")
	top := flag.Int("top", 25, "how many movers to show")
	minMove := flag.Float64("min", 0.5, "smallest change in the market line to show")
	flag.Parse()
	if *date == "" {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			loc = time.Local
		}
		*date = time.Now().In(loc).Format("2006-01-02")
	}
	path := filepath.Join(*dir, "snapshots", fmt.Sprintf("props_%s.jsonl", *date))
	moves, err := props.LineMoves(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err, "(run: go run ./cmd/pull snapshot)")
		os.Exit(1)
	}
	if len(moves) == 0 {
		fmt.Println("No line has been captured twice yet. Run `go run ./cmd/pull snapshot` again later in the day and retry.")
		return
	}
	games, _ := props.LoadGames(path)
	d, err := pipeline.Load(*dir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	ix := d.Builder.Index()

	type row struct {
		k props.MoveKey
		m props.Move
	}
	var rows []row
	for k, m := range moves {
		if m.Books >= 3 && math.Abs(m.To-m.From) >= *minMove {
			rows = append(rows, row{k, m})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := math.Abs(rows[i].m.To-rows[i].m.From), math.Abs(rows[j].m.To-rows[j].m.From)
		if a != b {
			return a > b
		}
		return rows[i].m.Books > rows[j].m.Books
	})
	if len(rows) > *top {
		rows = rows[:*top]
	}
	if len(rows) == 0 {
		fmt.Printf("No market line has moved by %.1f or more since the first snapshot.\n", *minMove)
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "game\tplayer\tstat\tfrom\tto\tchange\tbooks up/down/total\tfirst snapshot\tlatest")
	for _, r := range rows {
		g := games[r.k.Game]
		match := fmt.Sprintf("game %d", r.k.Game)
		if g.HomeAbbr != "" && g.VisitorAbbr != "" {
			match = g.VisitorAbbr + " @ " + g.HomeAbbr
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%.1f\t%+.1f\t%d/%d/%d\t%s\t%s\n", match, ix.Name(r.k.Player), r.k.Stat, r.m.From, r.m.To, r.m.To-r.m.From,
			r.m.Up, r.m.Down, r.m.Books, clock(r.m.FirstAt), clock(r.m.LastAt))
	}
	tw.Flush()
	fmt.Println("\nA move across most books together is usually news (injury, lineup, role); one book alone is usually stale or shaded.")
	fmt.Println("The feed has no bet counts, so it cannot say whether public money or a sharp bettor moved a line.")
}

func clock(at string) string {
	t, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return at
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.Local
	}
	return t.In(loc).Format("3:04 pm")
}
