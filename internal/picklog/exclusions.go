package picklog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ID is the immutable quoted-pick identity used by the audit exclusion registry.
func (e Entry) ID() string { return e.key() }

// Pregame rejects a known after-tip pick. Missing times remain legacy/unverified.
func (e Entry) Pregame() bool {
	tip, et := time.Parse(time.RFC3339, e.Tip)
	at, ea := time.Parse(time.RFC3339, e.LoggedAt)
	if et == nil && ea == nil && !at.Before(tip) {
		return false
	}
	cap, ec := time.Parse(time.RFC3339, e.QuoteAt)
	return !(et == nil && ec == nil && !cap.Before(tip))
}

// Verified is provenance, not a claim of predictive skill.
func (e Entry) Verified() bool { return e.AuditVersion >= 2 && e.FullGame() && e.Pregame() }

func exclusionIDs(logPath string) (map[string]bool, error) {
	ids := map[string]bool{}
	f, err := os.Open(filepath.Join(filepath.Dir(logPath), "exclusions.jsonl"))
	if os.IsNotExist(err) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 65536), 1<<24)
	for sc.Scan() {
		var r struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		ids[r.ID] = true
	}
	return ids, sc.Err()
}

func VerifiedEntries(rows []Entry) []Entry {
	var out []Entry
	for _, r := range rows {
		if r.Verified() {
			out = append(out, r)
		}
	}
	return out
}
func VerifiedSettled(rows []Settled) []Settled {
	var out []Settled
	for _, r := range rows {
		if r.Verified() {
			out = append(out, r)
		}
	}
	return out
}
