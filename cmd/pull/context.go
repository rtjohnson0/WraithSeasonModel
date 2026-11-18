package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"propsmodel/internal/bdl"
	"propsmodel/internal/data"
	"strconv"
	"strings"
	"time"
)

type contextFeed struct{ Name, Path, Type string }

var contextFeeds = []contextFeed{
	{"passing", "/nba/v1/stats/tracking", "passing"}, {"rebounding", "/nba/v1/stats/tracking", "rebounding"},
	{"possessions", "/nba/v1/stats/tracking", "possessions"}, {"drives", "/nba/v1/stats/tracking", "drives"},
	{"catchshoot", "/nba/v1/stats/tracking", "catchshoot"}, {"pullupshot", "/nba/v1/stats/tracking", "pullupshot"},
	{"defense_rim", "/nba/v1/stats/defense", "less_than_6ft"}, {"defense_threes", "/nba/v1/stats/defense", "3_pointers"},
	{"shooting_distance", "/nba/v1/stats/shooting", "defender_distance"}, {"lineup_stats", "/nba/v1/lineup_stats", ""},
	{"matchups", "/nba/v1/matchups", ""},
}

type contextCursor struct {
	Cursor, Start, End string
	Complete           bool
	Rows               int
	Refreshed          string
}

func runContext(c *bdl.Client, args []string) error {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	seasonFlag := fs.String("seasons", "", "season start years; current season first is recommended")
	budget := fs.Float64("maxminutes", 3, "soft budget between requests; progress resumes on the next run; 0 unlimited")
	live := fs.Bool("live", false, "only capture pregame projected lineups")
	date := fs.String("date", todayEastern(), "projected-lineup date")
	only := fs.String("feeds", "", "optional comma-separated feed names")
	if e := fs.Parse(args); e != nil {
		return e
	}
	if *budget < 0 || *budget > 1440 || math.IsNaN(*budget) || math.IsInf(*budget, 0) {
		return fmt.Errorf("maxminutes must be finite, between 0 and 1440 (0 means unlimited)")
	}
	if *live {
		if _, e := time.Parse("2006-01-02", *date); e != nil {
			return e
		}
		q := url.Values{"dates[]": {*date}}
		o, e := create("snapshots/projected_lineups_"+*date+".jsonl", true)
		if e != nil {
			return e
		}
		defer o.Close()
		return c.Paginate("/nba/v1/projected_lineups", q, func(row json.RawMessage) error {
			return writeWrapped(o, time.Now().UTC().Format(time.RFC3339Nano), row)
		})
	}
	seasons, e := parseSeasons(*seasonFlag)
	if e != nil {
		return e
	}
	feeds := contextFeeds
	if *only != "" {
		feeds = nil
		for _, name := range strings.Split(*only, ",") {
			found := false
			for _, f := range contextFeeds {
				if f.Name == name {
					feeds = append(feeds, f)
					found = true
				}
			}
			if !found {
				return fmt.Errorf("unknown context feed %q", name)
			}
		}
	}
	return pullContextPages(c, seasons, feeds, time.Duration(*budget*float64(time.Minute)), time.Now())
}
func pullContextPages(c *bdl.Client, seasons []int, feeds []contextFeed, budget time.Duration, now time.Time) error {
	root := filepath.Join(dataDir, "context")
	if e := os.MkdirAll(root, 0755); e != nil {
		return e
	}
	path := filepath.Join(root, "progress.json")
	state := map[string]*contextCursor{}
	if raw, e := os.ReadFile(path); e == nil {
		if e = json.Unmarshal(raw, &state); e != nil {
			return fmt.Errorf("context progress: %w", e)
		}
	}
	save := func() error {
		raw, e := json.MarshalIndent(state, "", "  ")
		if e != nil {
			return e
		}
		if e = os.WriteFile(path+".tmp", raw, 0644); e != nil {
			return e
		}
		return os.Rename(path+".tmp", path)
	}
	// Recent windows run first so historical backfill cannot starve live inputs.
	type task struct {
		feed   contextFeed
		season int
		key    string
		cursor *contextCursor
	}
	var tasks []task
	end := now.In(eastern()).AddDate(0, 0, -1).Format("2006-01-02")
	recent := now.In(eastern()).AddDate(0, 0, -3).Format("2006-01-02")
	for _, mode := range []string{"recent", "history"} {
		for _, season := range seasons {
			if mode == "recent" && season != data.SeasonOf(end) {
				continue
			}
			for _, feed := range feeds {
				key := fmt.Sprintf("%s/%d/%s", feed.Name, season, mode)
				if mode == "recent" {
					legacy := state[key]
					key += "/" + end
					if state[key] == nil && legacy != nil && legacy.End == end {
						copy := *legacy
						state[key] = &copy
					}
				}
				p := state[key]
				if p == nil {
					p = &contextCursor{Start: fmt.Sprintf("%d-08-01", season), End: end}
					if mode == "recent" {
						p.Start = recent
					}
					state[key] = p
				}
				if mode == "recent" && p.Complete {
					last, e := time.Parse(time.RFC3339Nano, p.Refreshed)
					if e == nil && now.Sub(last) > 3*time.Hour {
						*p = contextCursor{Start: recent, End: end}
					}
				}
				if mode == "recent" && p.Complete && p.End != end {
					*p = contextCursor{Start: recent, End: end}
				}
				if mode == "history" && p.Complete {
					target := end
					seasonEnd := fmt.Sprintf("%d-07-31", season+1)
					if target > seasonEnd {
						target = seasonEnd
					}
					if p.End < target {
						last, e := time.Parse("2006-01-02", p.End)
						if e != nil {
							return e
						}
						*p = contextCursor{Start: last.AddDate(0, 0, 1).Format("2006-01-02"), End: target}
					}
				}
				if p.Complete {
					continue
				}
				tasks = append(tasks, task{feed, season, key, p})
			}
		}
	}
	if len(tasks) == 0 {
		return nil
	}
	names := []string{}
	for _, f := range feeds {
		names = append(names, f.Name)
	}
	queueKey := fmt.Sprintf("@queue/%x", sha256.Sum256([]byte(fmt.Sprint(seasons)+strings.Join(names, ","))))
	if state[queueKey] == nil {
		state[queueKey] = &contextCursor{}
	}
	queue := state[queueKey]
	offset := queue.Rows % len(tasks)
	if offset < 0 {
		offset = 0
	}
	started := time.Now()
	for {
		pending := false
		for step := 0; step < len(tasks); step++ {
			index := (offset + step) % len(tasks)
			t := tasks[index]
			p := t.cursor
			if p.Complete {
				continue
			}
			pending = true
			if budget > 0 && time.Since(started) >= budget {
				fmt.Println("Context download paused at budget; rows and cursors saved. Next run resumes.")
				return save()
			}
			q := url.Values{"seasons[]": {strconv.Itoa(t.season)}, "start_date": {p.Start}, "end_date": {p.End}, "per_page": {"100"}}
			if t.feed.Type != "" {
				q.Set("type", t.feed.Type)
			}
			if p.Cursor != "" {
				q.Set("cursor", p.Cursor)
			}
			body, e := c.Get(t.feed.Path, q)
			if e != nil {
				_ = save()
				return fmt.Errorf("context %s: %w; cached data retained", t.key, e)
			}
			var page struct {
				Data []json.RawMessage `json:"data"`
				Meta struct {
					Next json.RawMessage `json:"next_cursor"`
				} `json:"meta"`
			}
			if e = json.Unmarshal(body, &page); e != nil {
				return e
			}
			next := strings.Trim(string(page.Meta.Next), `"`)
			if next == "null" {
				next = ""
			}
			if next != "" && next == p.Cursor {
				return fmt.Errorf("context %s repeated cursor", t.key)
			}
			o, e := create(fmt.Sprintf("context/%s_%d.jsonl", t.feed.Name, t.season), true)
			if e != nil {
				return e
			}
			stamp := time.Now().UTC().Format(time.RFC3339Nano)
			for _, row := range page.Data {
				if e = writeWrapped(o, stamp, row); e != nil {
					_ = o.Close()
					return e
				}
			}
			if e = o.Close(); e != nil {
				return e
			}
			queue.Rows = (index + 1) % len(tasks)
			p.Cursor = next
			p.Rows += len(page.Data)
			p.Complete = next == ""
			p.Refreshed = stamp
			if e = save(); e != nil {
				return e
			}
			fmt.Printf("Context %s: %d rows saved; window complete=%t\n", t.key, p.Rows, p.Complete)
		}
		offset = queue.Rows % len(tasks)
		if !pending {
			return nil
		}
	}
}
func eastern() *time.Location {
	loc, e := time.LoadLocation("America/New_York")
	if e != nil {
		return time.UTC
	}
	return loc
}
