// Package espn reads ESPN's public NBA injury list as a second opinion on the injury report.
//
// The endpoint and field names (injuries[].injuries[] with status, date, shortComment and
// athlete.displayName) were checked against a live response, but the daily workflow's network
// access to it has not been verified from here. Everything built on it is best effort: if the fetch
// fails or the format changes, the model carries on with the BALLDONTLIE report alone.
package espn

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// URL is the injury list.
const URL = "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/injuries"

// Injury is one player on the list.
type Injury struct {
	Name    string `json:"name"`
	Team    string `json:"team"`   // abbreviation, e.g. ATL
	Status  string `json:"status"` // "Out" or "Day-To-Day"
	Comment string `json:"comment"`
	Date    string `json:"date"`
}

// Parse reads the response body.
func Parse(body []byte) ([]Injury, error) {
	var resp struct {
		Injuries []struct {
			Injuries []struct {
				Status       string `json:"status"`
				Date         string `json:"date"`
				ShortComment string `json:"shortComment"`
				Athlete      struct {
					DisplayName string `json:"displayName"`
					Team        struct {
						Abbreviation string `json:"abbreviation"`
					} `json:"team"`
				} `json:"athlete"`
			} `json:"injuries"`
		} `json:"injuries"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("espn: unexpected response: %w", err)
	}
	var out []Injury
	for _, t := range resp.Injuries {
		for _, i := range t.Injuries {
			if i.Athlete.DisplayName == "" {
				continue
			}
			out = append(out, Injury{Name: i.Athlete.DisplayName, Team: i.Athlete.Team.Abbreviation, Status: i.Status, Comment: i.ShortComment, Date: i.Date})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("espn: the response listed no injuries (format change, or a day with none)")
	}
	return out, nil
}

// Fetch downloads the raw response.
func Fetch() ([]byte, error) {
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Get(URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("espn: HTTP %d", resp.StatusCode)
	}
	return body, nil
}

// IsOut reports a status of "Out" (including "Out For Season").
func (i Injury) IsOut() bool { return strings.HasPrefix(strings.ToLower(i.Status), "out") }

// IsGameTime reports "Day-To-Day": ESPN's word for questionable, doubtful or probable. The comment
// says which ("is questionable for Wednesday's game").
func (i Injury) IsGameTime() bool {
	s := strings.ToLower(i.Status)
	return strings.Contains(s, "day")
}
