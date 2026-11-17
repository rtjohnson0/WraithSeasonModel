package availability

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Status struct {
	PlayerID int
	Name     string
	Status   string
	At       string
}

// FreshStatuses uses the newest timestamped observation, never file order or
// file modification time. An untimed, future or stale status cannot drive a
// numerical availability comparison. No absence probability is invented.
func FreshStatuses(path string, now time.Time, maxAge time.Duration) map[int]Status {
	out := map[int]Status{}
	latest := map[int]time.Time{}
	f, e := os.Open(path)
	if e != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 4<<20)
	for sc.Scan() {
		var r struct {
			At  string `json:"captured_at"`
			Row struct {
				Status string `json:"status"`
				Player struct {
					ID    int    `json:"id"`
					First string `json:"first_name"`
					Last  string `json:"last_name"`
				} `json:"player"`
			} `json:"row"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(sc.Text(), "\ufeff")), &r) != nil || r.Row.Player.ID <= 0 {
			continue
		}
		at, e := time.Parse(time.RFC3339, r.At)
		if e != nil || at.After(now) || now.Sub(at) > maxAge {
			continue
		}
		pid := r.Row.Player.ID
		if prev, ok := latest[pid]; ok && at.Before(prev) {
			continue
		}
		if prev, ok := latest[pid]; ok && at.Equal(prev) && out[pid].Status != r.Row.Status {
			out[pid] = Status{PlayerID: pid, Status: "conflicting reports", At: r.At}
			continue
		}
		latest[pid] = at
		out[pid] = Status{PlayerID: pid, Name: strings.TrimSpace(r.Row.Player.First + " " + r.Row.Player.Last), Status: r.Row.Status, At: r.At}
	}
	if sc.Err() != nil {
		return map[int]Status{}
	}
	return out
}
