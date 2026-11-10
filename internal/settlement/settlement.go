// Package settlement verifies full-game actuals separately from model features.
package settlement

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/marketperiod"
	"propsmodel/internal/picklog"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type raw struct {
	FetchedAt           string          `json:"_stats_fetched_at"`
	Endpoint            string          `json:"_stats_endpoint"`
	Period              *int            `json:"period"`
	Min                 json.RawMessage `json:"min"`
	Pts, Reb, Ast, Fg3m *float64
	Oreb, Dreb          *float64
	Player              struct {
		ID    int    `json:"id"`
		First string `json:"first_name"`
		Last  string `json:"last_name"`
	} `json:"player"`
	Team struct {
		ID   int    `json:"id"`
		Abbr string `json:"abbreviation"`
	} `json:"team"`
	Game struct {
		ID     int    `json:"id"`
		Date   string `json:"date"`
		Status string `json:"status"`
		Home   int    `json:"home_team_id"`
		Away   int    `json:"visitor_team_id"`
	} `json:"game"`
}
type box struct {
	raw   raw
	hash  string
	valid bool
}
type key struct{ game, player int }
type Index struct{ rows map[key]box }

func Load(dir string) (*Index, error) {
	ix := &Index{rows: map[key]box{}}
	files, e := filepath.Glob(filepath.Join(dir, "stats_*.jsonl"))
	if e != nil {
		return nil, e
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no stats files: refresh final box scores first")
	}
	for _, name := range files {
		f, e := os.Open(name)
		if e != nil {
			return nil, e
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r raw
			b := bytes.TrimPrefix(sc.Bytes(), []byte{0xef, 0xbb, 0xbf})
			if e = json.Unmarshal(b, &r); e != nil {
				f.Close()
				return nil, fmt.Errorf("invalid stat JSON in %s", name)
			}
			if r.Player.ID == 0 || r.Game.ID == 0 {
				continue
			}
			if (r.Period != nil && *r.Period != 0) || !marketperiod.Metadata(b) {
				continue
			}
			h := sha256.Sum256(b)
			k := key{r.Game.ID, r.Player.ID}
			if prev, ok := ix.rows[k]; ok {
				before, eb := time.Parse(time.RFC3339Nano, prev.raw.FetchedAt)
				after, ea := time.Parse(time.RFC3339Nano, r.FetchedAt)
				if eb == nil && (ea != nil || after.Before(before)) {
					continue
				}
			}
			ix.rows[k] = box{r, hex.EncodeToString(h[:]), true}
		}
		e = sc.Err()
		f.Close()
		if e != nil {
			return nil, e
		}
	}
	return ix, nil
}
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
func value(r raw, stat string) (float64, bool) {
	fields := map[string]*float64{"points": r.Pts, "rebounds": r.Reb, "assists": r.Ast, "threes": r.Fg3m}
	parts := []data.Stat{data.Stat(stat)}
	if ps := data.Stat(stat).Parts(); ps != nil {
		parts = ps
	}
	sum := 0.0
	for _, s := range parts {
		p := fields[string(s)]
		if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) || *p < 0 || math.Trunc(*p) != *p {
			return 0, false
		}
		sum += *p
	}
	return sum, true
}
func (ix *Index) Grade(e picklog.Entry, close float64) picklog.Settled {
	review := func(reason string) picklog.Settled {
		source := ix.rows[key{e.GameID, e.PlayerID}]
		return picklog.Settled{Entry: e, Result: "REVIEW", SettlementReason: reason, SourceFetchedAt: source.raw.FetchedAt, SettledAt: time.Now().UTC().Format(time.RFC3339)}
	}
	if !e.FullGame() || !e.Pregame() || (e.Side != "OVER" && e.Side != "UNDER") {
		return review("pick period, timing or side is invalid")
	}
	if math.IsNaN(e.Line) || math.IsInf(e.Line, 0) || e.Line < 0 {
		return review("invalid line")
	}
	b, ok := ix.rows[key{e.GameID, e.PlayerID}]
	if !ok {
		return review("exact player/game final row missing; missing stats do not establish DNP")
	}
	r := b.raw
	if _, err := time.Parse(time.RFC3339Nano, r.FetchedAt); err != nil || r.Endpoint != "/nba/v1/stats" {
		return review("stats download provenance is missing; refresh this season with the updated pull command")
	}
	if !strings.EqualFold(strings.TrimSpace(r.Game.Status), "Final") {
		return review("box score is not explicitly final")
	}
	if data.NormalizeDate(r.Game.Date) != e.Date {
		return review("game date mismatch")
	}
	if e.Player == "" || !sameName(e.Player, r.Player.First+" "+r.Player.Last) {
		return review(fmt.Sprintf("player ID %d names %s %s in the final row, not logged name %s", e.PlayerID, r.Player.First, r.Player.Last, e.Player))
	}
	if r.Team.ID == 0 || r.Game.Home == 0 || r.Game.Away == 0 || (r.Team.ID != r.Game.Home && r.Team.ID != r.Game.Away) {
		return review("team is missing or is not in this game")
	}
	if e.Team != "" && (r.Team.Abbr == "" || !strings.EqualFold(e.Team, r.Team.Abbr)) {
		return review(fmt.Sprintf("logged team %s differs from final row %s (team ID %d), game %d", e.Team, r.Team.Abbr, r.Team.ID, e.GameID))
	}
	if data.ParseMinutes(r.Min) <= 0 {
		return review("zero or missing minutes; verify participation and sportsbook void rule")
	}
	if r.Reb != nil && r.Oreb != nil && r.Dreb != nil && *r.Reb != *r.Oreb+*r.Dreb {
		return review("total rebounds disagree with offensive plus defensive rebounds")
	}
	actual, ok := value(r, e.Stat)
	if !ok {
		return review("requested stat is missing, invalid or unknown")
	}
	s := picklog.Grade(e, actual, close, true)
	s.SettlementVersion = 1
	s.SettlementSource = "BALLDONTLIE final player/game box score"
	s.SourceHash = b.hash
	s.SourceFetchedAt = r.FetchedAt
	s.SourcePlayerID = r.Player.ID
	s.SourceGameID = r.Game.ID
	s.SourceTeamID = r.Team.ID
	s.SourcePlayer = r.Player.First + " " + r.Player.Last
	s.SourceDate = data.NormalizeDate(r.Game.Date)
	return s
}

type Change struct {
	ID        string   `json:"id"`
	Player    string   `json:"player"`
	Date      string   `json:"date"`
	GameID    int      `json:"gameId"`
	PlayerID  int      `json:"playerId"`
	Stat      string   `json:"stat"`
	Side      string   `json:"side"`
	Line      float64  `json:"line"`
	OldResult string   `json:"oldResult"`
	OldActual float64  `json:"oldActual"`
	NewResult string   `json:"newResult"`
	NewActual *float64 `json:"newActual,omitempty"`
	Reason    string   `json:"reason"`
}
type Report struct {
	Checks       []Change `json:"checks"`
	StaleSources int      `json:"staleSources"`
	At           string   `json:"at"`
	Log          string   `json:"log"`
	Checked      int      `json:"checked"`
	Confirmed    int      `json:"confirmed"`
	Review       int      `json:"review"`
	Changes      []Change `json:"changes"`
	History      []Change `json:"history"`
}

// Reconcile audits every saved result, not only new picks. Original JSON fields
// are preserved, and a backup is written before any result changes.
func Reconcile(dir, name string, entries []picklog.Entry, closeLine func(picklog.Entry) float64) (Report, error) {
	report := Report{At: time.Now().UTC().Format(time.RFC3339Nano), Log: name, Changes: []Change{}}
	ix, e := Load(dir)
	if e != nil {
		return report, e
	}
	path := filepath.Join(dir, name)
	original, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return report, e
	}
	rows := []map[string]json.RawMessage{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(string(original)))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	audit := func(row map[string]json.RawMessage, old picklog.Settled) error {
		if !old.Entry.FullGame() {
			return nil
		}
		report.Checked++
		close := old.CloseLine
		if closeLine != nil {
			close = closeLine(old.Entry)
		}
		fresh := ix.Grade(old.Entry, close)
		if old.SettlementVersion >= 1 && old.SourceHash != "" && old.SourceFetchedAt != "" {
			prior, pe := time.Parse(time.RFC3339Nano, old.SourceFetchedAt)
			incoming := ix.rows[key{old.GameID, old.PlayerID}]
			next, ne := time.Parse(time.RFC3339Nano, incoming.raw.FetchedAt)
			if pe == nil && (ne != nil || next.Before(prior) || (!next.After(prior) && incoming.hash != old.SourceHash)) {
				fresh = old
				report.StaleSources++
			}
		}
		if fresh.Result == "REVIEW" {
			report.Review++
		} else {
			report.Confirmed++
		}

		c := Change{ID: old.ID(), Player: old.Player, Date: old.Date, GameID: old.GameID, PlayerID: old.PlayerID, Stat: old.Stat, Side: old.Side, Line: old.Line, OldResult: old.Result, OldActual: old.Actual, NewResult: fresh.Result, Reason: fresh.SettlementReason}
		if fresh.SettlementVersion > 0 {
			v := fresh.Actual
			c.NewActual = &v
			tag := fresh.SourceHash
			if len(tag) > 12 {
				tag = tag[:12]
			}
			c.Reason = "identity-checked final player/game/stat; source " + tag + ", fetched " + fresh.SourceFetchedAt
		}
		report.Checks = append(report.Checks, c)
		if old.Result != fresh.Result || old.Actual != fresh.Actual || old.SettlementVersion != fresh.SettlementVersion {
			report.Changes = append(report.Changes, c)
		}

		// Stable timestamps make repeated identical audits a no-op.
		if old.Result == fresh.Result && old.Actual == fresh.Actual && old.SettlementVersion == fresh.SettlementVersion && old.SourceHash == fresh.SourceHash && old.SettlementReason == fresh.SettlementReason {
			fresh.SettledAt = old.SettledAt
		}
		b, _ := json.Marshal(fresh)
		var newRow map[string]json.RawMessage
		_ = json.Unmarshal(b, &newRow)
		for _, k := range []string{"settled_at", "result", "actual", "close_line", "clv", "profit", "flat", "settlement_version", "settlement_source", "settlement_reason", "source_hash", "source_fetched_at", "source_player_id", "source_game_id", "source_team_id", "source_player", "source_date"} {
			delete(row, k)
			if v, ok := newRow[k]; ok {
				row[k] = v
			}
		}
		return nil
	}
	for sc.Scan() {
		var row map[string]json.RawMessage
		var old picklog.Settled
		if e = json.Unmarshal(bytes.TrimPrefix(sc.Bytes(), []byte{0xef, 0xbb, 0xbf}), &row); e != nil {
			return report, e
		}
		if e = json.Unmarshal(bytes.TrimPrefix(sc.Bytes(), []byte{0xef, 0xbb, 0xbf}), &old); e != nil {
			return report, e
		}
		seen[old.ID()] = true
		if e = audit(row, old); e != nil {
			return report, e
		}
		rows = append(rows, row)
	}
	if e = sc.Err(); e != nil {
		return report, e
	}
	for _, entry := range entries {
		if seen[entry.ID()] {
			continue
		}
		seen[entry.ID()] = true
		b, _ := json.Marshal(entry)
		var row map[string]json.RawMessage
		_ = json.Unmarshal(b, &row)
		if e = audit(row, picklog.Settled{Entry: entry}); e != nil {
			return report, e
		}
		rows = append(rows, row)
	}
	var output strings.Builder
	enc := json.NewEncoder(&output)
	for _, r := range rows {
		if e = enc.Encode(r); e != nil {
			return report, e
		}
	}
	if !reflect.DeepEqual([]byte(output.String()), original) {
		if len(original) > 0 {
			backup := path + ".before-result-audit-" + time.Now().UTC().Format("20060102T150405.000000000Z")
			if e = os.WriteFile(backup, original, 0644); e != nil {
				return report, e
			}
		}
		if e = os.WriteFile(path+".tmp", []byte(output.String()), 0644); e != nil {
			return report, e
		}
		if e = os.Rename(path+".tmp", path); e != nil {
			return report, e
		}
	}
	auditPath := filepath.Join(dir, strings.TrimSuffix(name, ".jsonl")+"_audit.json")
	var previous Report
	if old, e := os.ReadFile(auditPath); e == nil {
		_ = json.Unmarshal(old, &previous)
	}
	report.History = append(previous.History, report.Changes...)
	if len(report.History) > 2000 {
		report.History = report.History[len(report.History)-2000:]
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	e = os.WriteFile(filepath.Join(dir, strings.TrimSuffix(name, ".jsonl")+"_audit.json"), b, 0644)
	return report, e
}

// Actual supplies complete, final full-game rows to calibration diagnostics.
// Report consumers must additionally compare logged names/dates/teams.
func (ix *Index) Actual(gameID, playerID int) (data.PlayerGame, bool) {
	b, ok := ix.rows[key{gameID, playerID}]
	if !ok {
		return data.PlayerGame{}, false
	}
	r := b.raw
	e := picklog.Entry{GameID: gameID, PlayerID: playerID, Player: strings.TrimSpace(r.Player.First + " " + r.Player.Last), Date: data.NormalizeDate(r.Game.Date), Team: r.Team.Abbr, Period: "game", Side: "OVER", Stat: "points"}
	if ix.Grade(e, 0).Result == "REVIEW" {
		return data.PlayerGame{}, false
	}
	for _, st := range data.Stats {
		if _, ok := value(r, string(st)); !ok {
			return data.PlayerGame{}, false
		}
	}
	return data.PlayerGame{PlayerID: playerID, Name: e.Player, TeamID: r.Team.ID, TeamAbbr: r.Team.Abbr, GameID: gameID, Date: e.Date, Minutes: data.ParseMinutes(r.Min), Pts: *r.Pts, Reb: *r.Reb, Ast: *r.Ast, Fg3m: *r.Fg3m, HomeTeamID: r.Game.Home, VisitorTeamID: r.Game.Away}, true
}

// Allow only a reversible UTF-8-as-Latin-1 encoding error, confirmed against
// the exact player/game row. This is not fuzzy player-name matching.
func sameName(logged, source string) bool {
	if norm(logged) == norm(source) {
		return true
	}
	bytes := []byte{}
	for _, r := range logged {
		if r > 255 {
			return false
		}
		bytes = append(bytes, byte(r))
	}
	return utf8.Valid(bytes) && norm(string(bytes)) == norm(source)
}
