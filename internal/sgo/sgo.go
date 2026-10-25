// Package sgo reads SportsGameOdds (https://sportsgameodds.com), which adds more sportsbooks
// than BALLDONTLIE so the model can shop for the best line.
//
// The base URL, the x-api-key header, the events endpoint, the oddID format
// (statID-statEntityID-periodID-betTypeID-sideID) and the odd fields (byBookmaker.<book>.odds,
// .overUnder, .available) come from the provider's public docs. I have NOT been able to call
// the API from where this was written, so run `go run ./cmd/sgo sample` first and check that
// the lines it prints look right before relying on it.
package sgo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"propsmodel/internal/data"
)

// BaseURL is the API root.
const BaseURL = "https://api.sportsgameodds.com/v2"

// Client calls the SportsGameOdds API.
type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	Sleep   func(time.Duration)
}

// New returns a client for the given API key.
func New(key string) *Client {
	return &Client{BaseURL: BaseURL, Key: key, HTTP: &http.Client{Timeout: 60 * time.Second}, Sleep: time.Sleep}
}

// Get performs a GET with retries on rate limits and server errors.
func (c *Client) Get(path string, q url.Values) ([]byte, error) {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.Key)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			c.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch {
		case rerr != nil:
			lastErr = rerr
		case resp.StatusCode == 429 || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(body))
		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(body))
		default:
			return body, nil
		}
		c.Sleep(time.Duration(1<<attempt) * time.Second)
	}
	return nil, lastErr
}

func truncate(b []byte) string {
	if len(b) > 300 {
		b = b[:300]
	}
	return string(b)
}

// Events pages through /events, calling fn with each raw event. The API returns a nextCursor
// when more pages exist.
func (c *Client) Events(q url.Values, fn func(raw json.RawMessage) error) error {
	q = cloneValues(q)
	for page := 0; page < 50; page++ {
		body, err := c.Get("/events", q)
		if err != nil {
			return err
		}
		var resp struct {
			Data       []json.RawMessage `json:"data"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("unexpected response: %w", err)
		}
		for _, e := range resp.Data {
			if err := fn(e); err != nil {
				return err
			}
		}
		if resp.NextCursor == "" {
			return nil
		}
		q.Set("cursor", resp.NextCursor)
	}
	return nil
}

func cloneValues(q url.Values) url.Values {
	out := url.Values{}
	for k, v := range q {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// PlayerLine is one book's over/under line for one player and stat.
type PlayerLine struct {
	PlayerKey string // SportsGameOdds player id, e.g. JALEN_DUREN_1_NBA
	Name      string
	Stat      data.Stat
	Book      string
	Line      float64
	OverOdds  int
	UnderOdds int
	UpdatedAt string
}

// flex reads a JSON value that may be a number or a numeric string.
type flex struct {
	v  float64
	ok bool
}

func (f *flex) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	s = strings.TrimPrefix(s, "+")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	f.v, f.ok = v, true
	return nil
}

type bookEntry struct {
	Odds          flex   `json:"odds"`
	OverUnder     flex   `json:"overUnder"`
	Available     *bool  `json:"available"`
	LastUpdatedAt string `json:"lastUpdatedAt"`
}

type odd struct {
	ByBookmaker map[string]bookEntry `json:"byBookmaker"`
}

type event struct {
	Odds    map[string]odd `json:"odds"`
	Players map[string]struct {
		Name string `json:"name"`
	} `json:"players"`
}

func statFor(id string) (data.Stat, bool) {
	switch id {
	case "points":
		return data.Points, true
	case "rebounds":
		return data.Rebounds, true
	case "assists":
		return data.Assists, true
	case "threePointersMade":
		return data.Threes, true
	}
	return "", false
}

// ParseEvent extracts full-game points, rebounds and assists over/under lines from one event.
func ParseEvent(raw json.RawMessage) []PlayerLine {
	var ev event
	if json.Unmarshal(raw, &ev) != nil {
		return nil
	}
	var out []PlayerLine
	for id, o := range ev.Odds {
		parts := strings.Split(id, "-")
		if len(parts) != 5 || parts[2] != "game" || parts[3] != "ou" || parts[4] != "over" {
			continue
		}
		stat, ok := statFor(parts[0])
		if !ok {
			continue
		}
		under := ev.Odds[strings.Join(parts[:4], "-")+"-under"]
		name := ev.Players[parts[1]].Name
		if name == "" {
			name = NameFromKey(parts[1])
		}
		for book, over := range o.ByBookmaker {
			if !over.OverUnder.ok || (over.Available != nil && !*over.Available) {
				continue
			}
			pl := PlayerLine{
				PlayerKey: parts[1], Name: name, Stat: stat, Book: book, Line: over.OverUnder.v,
				OverOdds: int(over.Odds.v), UpdatedAt: over.LastUpdatedAt,
			}
			if u, ok := under.ByBookmaker[book]; ok && u.Odds.ok {
				pl.UnderOdds = int(u.Odds.v)
			}
			out = append(out, pl)
		}
	}
	return out
}

// NameFromKey turns JALEN_DUREN_1_NBA into "Jalen Duren".
func NameFromKey(key string) string {
	parts := strings.Split(key, "_")
	if len(parts) >= 2 {
		parts = parts[:len(parts)-2]
	}
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

// NormName lowercases a name and drops punctuation and generational suffixes so names from
// different sources match: "Jaren Jackson Jr." == "JAREN_JACKSON_JR_1_NBA".
func NormName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-' || r == '.' || r == '\'':
			b.WriteRune(' ')
		}
	}
	var keep []string
	for _, w := range strings.Fields(b.String()) {
		switch w {
		case "jr", "sr", "ii", "iii", "iv", "v":
			continue
		}
		keep = append(keep, w)
	}
	return strings.Join(keep, " ")
}
