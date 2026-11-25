package props

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// LoadCapturedInjuries reads ONLY the last completed, time-stamped,
 // league-wide pregame capture. The file is append-only across the day:
 // selecting each player's most recent appearance can falsely preserve an
 // earlier "Out" after the player disappears from the latest report.
 // Empty latest reports are accepted if the separate completion receipt
 // proves the endpoint completed successfully.
func LoadCapturedInjuries(snapDir, date string, now time.Time) (map[int]string, string, error) {
    if now.IsZero() { return nil, "", fmt.Errorf("missing injury evaluation time") }
    receiptPath := filepath.Join(snapDir, "injury_health_"+date+".json")
    b, err := os.ReadFile(receiptPath)
    if err != nil { return nil, "", fmt.Errorf("current injury completion receipt missing: %w", err) }
    var health struct {
        Date string `json:"date"`
        CapturedAt string `json:"captured_at"`
        Games int `json:"games"`
        Complete bool `json:"injuries_complete"`
    }
    if err := json.Unmarshal(b, &health); err != nil { return nil, "", fmt.Errorf("invalid injury completion receipt: %w", err) }
    captured, err := time.Parse(time.RFC3339, health.CapturedAt)
    if err != nil || health.Date != date || !health.Complete || health.Games < 1 ||
       captured.After(now.Add(2*time.Minute)) || now.Sub(captured) > 30*time.Minute {
        return nil, "", fmt.Errorf("injury report for %s is incomplete or older than 30 minutes", date)
    }
    statuses := map[int]string{}
    path := filepath.Join(snapDir, "injuries_"+date+".jsonl")
    err = readLines(path, func(b []byte) {
        var entry struct {
            At string `json:"captured_at"`
            Row struct {
                Status string `json:"status"`
                Player struct { ID int `json:"id"` } `json:"player"`
            } `json:"row"`
        }
        if json.Unmarshal(b,&entry) != nil || entry.At != health.CapturedAt ||
           entry.Row.Player.ID <= 0 || strings.TrimSpace(entry.Row.Status) == "" {
            return
        }
        statuses[entry.Row.Player.ID] = entry.Row.Status
    })
    // A successful zero-row injury response can legitimately yield a missing
    // JSONL when no players are injured; only allow that after the receipt.
    if os.IsNotExist(err) { err = nil }
    if err != nil { return nil, "", fmt.Errorf("read captured injuries: %w", err) }
    return statuses, health.CapturedAt, nil
}
