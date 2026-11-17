// Package bdl is a small client for the BALLDONTLIE API (GOAT tier).
package bdl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.balldontlie.io"

// Client talks to BALLDONTLIE with retry on rate limits and a small delay between calls
// (the GOAT tier allows 600 requests/minute).
type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	Sleep   time.Duration
	// Logf optionally reports download activity; credentials and query values are never logged.
	Logf func(string, ...any)
}

func New(key string) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		Key:     key,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Sleep:   120 * time.Millisecond,
	}
}

func (c *Client) progress(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// Get performs one GET and returns the raw response body.
func (c *Client) Get(path string, q url.Values) ([]byte, error) {
	var last string // what went wrong on the most recent attempt, shown if we give up
	for attempt := 0; attempt < 6; attempt++ {
		c.progress("Requesting %s (attempt %d/6)...", path, attempt+1)
		backoff := time.Duration(1<<attempt) * time.Second

		req, err := http.NewRequest(http.MethodGet, c.BaseURL+path+"?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.Key)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			last = "network error: " + err.Error()
			c.progress("%s: network/read error; retrying in %s", path, backoff)
			time.Sleep(backoff)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			last = "reading response: " + readErr.Error()
			c.progress("%s: network/read error; retrying in %s", path, backoff)
			time.Sleep(backoff)
			continue
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			last = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
			wait := backoff
			if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && ra > 0 && ra <= 120 {
				wait = time.Duration(ra) * time.Second
			}
			c.progress("%s: HTTP %d; retrying in %s", path, resp.StatusCode, wait)
			time.Sleep(wait)
			continue
		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("%d from %s?%s: %s", resp.StatusCode, path, q.Encode(), truncate(string(body), 300))
		}

		time.Sleep(c.Sleep)
		return body, nil
	}
	return nil, fmt.Errorf("giving up on %s after repeated errors; last problem: %s", path, last)
}

// Paginate walks every page of a cursor-paginated endpoint and calls fn for each row.
// Rows are passed as raw JSON so no field names are assumed.
func (c *Client) Paginate(path string, q url.Values, fn func(json.RawMessage) error) error {
	params := url.Values{}
	for k, v := range q {
		params[k] = append([]string(nil), v...)
	}
	params.Set("per_page", "100")
	pages, rows := 0, 0

	for {
		body, err := c.Get(path, params)
		if err != nil {
			return err
		}
		var page struct {
			Data []json.RawMessage `json:"data"`
			Meta struct {
				NextCursor json.RawMessage `json:"next_cursor"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("decoding %s: %w", path, err)
		}
		for _, row := range page.Data {
			if err := fn(row); err != nil {
				return err
			}
		}
		pages++
		rows += len(page.Data)
		c.progress("%s: page %d received, %d rows on page, %d rows total", path, pages, len(page.Data), rows)
		cursor := strings.Trim(string(page.Meta.NextCursor), `"`)
		if cursor == "" || cursor == "null" {
			return nil
		}
		params.Set("cursor", cursor)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
