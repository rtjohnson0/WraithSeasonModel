// priceaudit compares both sides of identical frozen markets and audits posted
// history by stat and offered price. It never changes predictions or picks.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/bets"
	"propsmodel/internal/data"
	"propsmodel/internal/lineslog"
	"propsmodel/internal/picklog"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"propsmodel/internal/settlement"
	"sort"
	"strings"
	"time"
)

type Question struct {
	Cohort, Date, Player, Stat, Side, Book, Reason string
	GameID, PlayerID, Odds                         int
	Line, Raw, Final, Market, BreakEven, Edge      float64
	Result                                         string `json:",omitempty"`
}
type Group struct {
	Cohort, Stat, Side, Price                                                              string
	N, RawN, MarketN, Lean, ProbabilityBar, EdgeBar, BothBars, Wins, Losses, Pushes, Voids int
	Raw, Final, Market, BreakEven, Edge, Flat                                              float64
}
type Report struct {
	Note, Before string
	Excluded     map[string]int
	Questions    []Question
	Groups       []Group
}

func read[T any](path string) ([]T, error) {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 65536), 1<<24)
	var out []T
	for s.Scan() {
		if strings.TrimSpace(s.Text()) == "" {
			continue
		}
		var row T
		if e = json.Unmarshal([]byte(strings.TrimPrefix(s.Text(), "\ufeff")), &row); e != nil {
			return nil, fmt.Errorf("invalid JSON in %s", filepath.Base(path))
		}
		out = append(out, row)
	}
	return out, s.Err()
}
func band(odds int) string {
	switch {
	case odds <= -200:
		return "01: -200 or shorter"
	case odds <= -150:
		return "02: -199 to -150"
	case odds <= -120:
		return "03: -149 to -120"
	case odds <= -100:
		return "04: -119 to -100"
	case odds >= 100 && odds < 120:
		return "05: +100 to +119"
	case odds >= 120 && odds < 150:
		return "06: +120 to +149"
	case odds >= 150 && odds < 200:
		return "07: +150 to +199"
	case odds >= 200:
		return "08: +200 or longer"
	}
	return "invalid"
}
func reason(p float64, odds int) string {
	if p < .5 {
		return "opposite side more likely"
	}
	if p < .55 {
		return "below 55% probability bar"
	}
	if bets.Edge(p, odds) < .01 {
		return "below 1pp price edge"
	}
	return "clears numerical bars; other board checks still apply"
}
func key(r replay.Record) string {
	return fmt.Sprintf("%s/%d/%d/%s", r.Date, r.Row.G.GameID, r.Row.G.PlayerID, r.Stat)
}
func choose(old, r replay.Record, before string) bool {
	a, ea := time.Parse(time.RFC3339Nano, r.At)
	b, eb := time.Parse(time.RFC3339Nano, old.At)
	if ea != nil {
		return false
	}
	if eb != nil {
		return true
	}
	if a.Equal(b) {
		return r.Book < old.Book
	}
	if r.Date < before {
		return a.Before(b)
	}
	return a.After(b)
}
func summarize(qs []Question) []Group {
	gs := map[string]*Group{}
	for _, q := range qs {
		b := band(q.Odds)
		k := q.Cohort + "/" + q.Stat + "/" + q.Side + "/" + b
		g := gs[k]
		if g == nil {
			g = &Group{Cohort: q.Cohort, Stat: q.Stat, Side: q.Side, Price: b}
			gs[k] = g
		}
		g.N++
		if q.Raw > 0 && q.Raw < 1 {
			g.Raw += q.Raw
			g.RawN++
		}
		g.Final += q.Final
		if q.Market > 0 && q.Market < 1 {
			g.Market += q.Market
			g.MarketN++
		}
		g.BreakEven += q.BreakEven
		g.Edge += q.Edge
		if q.Final >= .5 {
			g.Lean++
		}
		if q.Final >= .55 {
			g.ProbabilityBar++
		}
		if q.Edge >= .01 {
			g.EdgeBar++
		}
		if q.Final >= .55 && q.Edge >= .01 {
			g.BothBars++
		}
		switch q.Result {
		case "WIN":
			g.Wins++
			g.Flat += picklog.Payout(q.Odds)
		case "LOSS":
			g.Losses++
			g.Flat--
		case "PUSH":
			g.Pushes++
		case "VOID":
			g.Voids++
		}
	}
	keys := []string{}
	for k := range gs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []Group{}
	for _, k := range keys {
		g := gs[k]
		n := float64(g.N)
		if g.RawN > 0 {
			g.Raw /= float64(g.RawN)
		}
		g.Final /= n
		if g.MarketN > 0 {
			g.Market /= float64(g.MarketN)
		}
		g.BreakEven /= n
		g.Edge /= n
		out = append(out, *g)
	}
	return out
}
func collect(dir, before string, recoverNames bool) (Report, error) {
	rep := Report{Before: before, Excluded: map[string]int{}, Note: "Read-only exploratory price audit. Both sides use the same line, book, feature row and checkpoint. Completed dates use the earliest eligible capture; current dates use the latest. Bars are fixed at 55% and 1pp price edge, before status/history/concentration checks. Recovered identities are explicitly separate and never written into source records. Posted history is selection-biased, not an all-market backtest or certified forward record. No parameters, thresholds or public records are modified."}
	ix, e := settlement.Load(dir)
	if e != nil {
		return rep, e
	}
	identities := map[string][]lineslog.Market{}
	if recoverNames {
		fs, _ := filepath.Glob(filepath.Join(dir, "lines", "market_*.jsonl"))
		for _, f := range fs {
			rs, e := read[lineslog.Market](f)
			if e != nil {
				return rep, e
			}
			for _, r := range rs {
				at, ea := time.Parse(time.RFC3339Nano, r.At)
				tip, et := time.Parse(time.RFC3339Nano, r.Tip)
				if ea == nil && et == nil && at.Before(tip) && r.Player != "" && r.Team != "" {
					k := fmt.Sprintf("%s/%d/%d", r.Date, r.GameID, r.PlayerID)
					identities[k] = append(identities[k], r)
				}
			}
		}
	}
	models := map[string]*predictor.Model{}
	chosen := map[string]replay.Record{}
	fs, e := filepath.Glob(filepath.Join(dir, "replay", "decisions_*.jsonl"))
	if e != nil {
		return rep, e
	}
	sort.Strings(fs)
	for _, f := range fs {
		rs, e := read[replay.Record](f)
		if e != nil {
			return rep, e
		}
		for _, r := range rs {
			skip := func(s string) { rep.Excluded[s]++ }
			if r.Date > before {
				skip("future date")
				continue
			}
			if r.Row.G.GameID <= 0 || r.Row.G.PlayerID <= 0 || r.Row.G.Date != r.Date || r.Period != "game" || r.BookCount < 3 || r.CalibrationBefore == "" || r.CalibrationBefore >= r.Date {
				skip("identity/period/consensus/calibration metadata")
				continue
			}
			supported := false
			for _, st := range data.Stats {
				if r.Stat == st {
					supported = true
				}
			}
			if !supported {
				skip("unsupported stat")
				continue
			}
			at, ea := time.Parse(time.RFC3339Nano, r.At)
			tip, et := time.Parse(time.RFC3339Nano, r.Tip)
			qt, eq := time.Parse(time.RFC3339Nano, r.QuoteAt)
			ut, eu := time.Parse(time.RFC3339Nano, r.UpdatedAt)
			if ea != nil || et != nil || eq != nil || eu != nil || !at.Before(tip) || qt.After(at) || ut.After(at) || at.Sub(qt) > 30*time.Minute || at.Sub(ut) > 6*time.Hour {
				skip("quote/timing")
				continue
			}
			if band(r.OverOdds) == "invalid" || band(r.UnderOdds) == "invalid" || r.Market <= 0 || r.Market >= 1 || math.Abs(r.Market-bets.MarketOver(r.OverOdds, r.UnderOdds)) > 1e-9 {
				skip("price metadata")
				continue
			}
			m := models[r.ModelID]
			if m == nil {
				id, err := hex.DecodeString(r.ModelID)
				if err != nil || len(id) != 32 {
					skip("invalid model hash")
					continue
				}
				path := filepath.Join(dir, "replay", "model_"+r.ModelID+".json")
				b, err := os.ReadFile(path)
				if err != nil {
					skip("missing checkpoint")
					continue
				}
				h := sha256.Sum256(b)
				if hex.EncodeToString(h[:]) != r.ModelID {
					return rep, fmt.Errorf("checkpoint hash mismatch")
				}
				m, err = predictor.Load(path)
				if err != nil {
					return rep, err
				}
				models[r.ModelID] = m
			}
			if m.TrainedBefore == "" || m.TrainedBefore >= r.Date {
				skip("model cutoff")
				continue
			}
			p, err := replay.Recompute(m, r)
			if err != nil || math.IsNaN(p) || math.Abs(p-r.P1) > 1e-9 {
				skip("not exactly reproducible")
				continue
			}
			k := key(r)
			old, ok := chosen[k]
			if !ok || choose(old, r, before) {
				chosen[k] = r
			}
		}
	}
	keys := []string{}
	for k := range chosen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := chosen[k]
		p := models[r.ModelID].Forecast(r.Row.Features(), r.Policy)
		raw, e := p.ProbOver(r.Stat, r.Line)
		if e != nil {
			return rep, e
		}
		cohort := "current frozen markets"
		name, tm := r.Row.G.Name, r.Row.G.TeamAbbr
		recovered := false
		if name == "" && recoverNames {
			names := map[string]lineslog.Market{}
			ik := fmt.Sprintf("%s/%d/%d", r.Date, r.Row.G.GameID, r.Row.G.PlayerID)
			for _, m := range identities[ik] {
				if m.Tip == r.Tip {
					names[m.Player+"/"+m.Team] = m
				}
			}
			if len(names) == 1 {
				for _, m := range names {
					name, tm = m.Player, m.Team
					recovered = true
				}
			}
		}
		if r.Date < before {
			cohort = "completed frozen markets"
			if recovered {
				cohort = "completed markets: recovered identity"
			}
		}
		for _, side := range []string{"OVER", "UNDER"} {
			rp, fp, mp, odds := raw.Over, r.P1, r.Market, r.OverOdds
			if side == "UNDER" {
				rp, fp, mp, odds = 1-rp, 1-fp, 1-mp, r.UnderOdds
			}
			q := Question{Cohort: cohort, Date: r.Date, Player: name, GameID: r.Row.G.GameID, PlayerID: r.Row.G.PlayerID, Stat: string(r.Stat), Side: side, Book: r.Book, Line: r.Line, Odds: odds, Raw: rp, Final: fp, Market: mp, BreakEven: bets.Implied(odds), Edge: bets.Edge(fp, odds), Reason: reason(fp, odds)}
			if r.Date < before {
				entry := picklog.Entry{Player: name, Team: tm, GameID: q.GameID, PlayerID: q.PlayerID, Date: q.Date, LoggedAt: r.At, Tip: r.Tip, Period: "game", Stat: q.Stat, Side: side, Line: q.Line, Odds: odds}
				g := ix.Grade(entry, 0)
				actual, ok := ix.Actual(q.GameID, q.PlayerID)
				if g.Result != "REVIEW" && ok && actual.TeamID == r.Row.G.TeamID {
					q.Result = g.Result
				} else {
					rep.Excluded["completed replay outcome identity/source unavailable"]++
				}
			}
			rep.Questions = append(rep.Questions, q)
		}
	}
	// Historical selection rows are regraded from the exact final source, not
	// trusted simply because an earlier export marked them as wins.
	history, e := picklog.LoadSettled(filepath.Join(dir, "settled.jsonl"))
	if e != nil {
		return rep, e
	}
	sort.SliceStable(history, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, history[i].LoggedAt)
		b, _ := time.Parse(time.RFC3339Nano, history[j].LoggedAt)
		return a.Before(b)
	})
	seen := map[string]bool{}
	for _, s := range history {
		if s.Date >= before || s.Prob <= 0 || s.Prob >= 1 || band(s.Odds) == "invalid" {
			continue
		}
		k := fmt.Sprintf("%s/%d/%d/%s/%s", s.Date, s.GameID, s.PlayerID, s.Stat, s.Side)
		if seen[k] {
			continue
		}
		seen[k] = true
		g := ix.Grade(s.Entry, 0)
		if g.Result == "REVIEW" {
			rep.Excluded["posted history identity/source unavailable"]++
			continue
		}
		tier := s.Tier
		if tier == "" {
			tier = "PICK"
		}
		historyCohort := "legacy quote provenance: " + tier
		at, ea := time.Parse(time.RFC3339Nano, s.LoggedAt)
		qt, eq := time.Parse(time.RFC3339Nano, s.QuoteAt)
		ut, eu := time.Parse(time.RFC3339Nano, s.SourceUpdatedAt)
		if s.AuditVersion == props.QuoteVersion && s.Period == "game" && s.BookCount >= 3 && ea == nil && eq == nil && eu == nil && !qt.After(at) && !ut.After(at) && at.Sub(qt) <= 30*time.Minute && at.Sub(ut) <= 6*time.Hour {
			historyCohort = "quote-checked posted history: " + tier
		}
		q := Question{Cohort: historyCohort, Date: s.Date, Player: s.Player, Stat: s.Stat, Side: s.Side, Book: s.Book, GameID: s.GameID, PlayerID: s.PlayerID, Odds: s.Odds, Line: s.Line, Raw: s.ProbRaw, Final: s.Prob, BreakEven: bets.Implied(s.Odds), Edge: bets.Edge(s.Prob, s.Odds), Reason: reason(s.Prob, s.Odds), Result: g.Result}
		rep.Questions = append(rep.Questions, q)
	}
	rep.Groups = summarize(rep.Questions)
	return rep, nil
}
func main() {
	dir := flag.String("data", "data", "input directory")
	out := flag.String("out", filepath.Join("run_evidence", "price-audit"), "private output directory")
	before := flag.String("before", "", "today Eastern; earlier dates may have outcomes")
	recoverNames := flag.Bool("recoveridentities", false, "separate supplemental identity recovery from exact pre-tip logs; never modify replay files")
	flag.Parse()
	if *before == "" {
		loc, e := time.LoadLocation("America/New_York")
		if e != nil {
			fatal(e)
		}
		*before = time.Now().In(loc).Format("2006-01-02")
	}
	if _, e := time.Parse("2006-01-02", *before); e != nil {
		fatal(e)
	}
	r, e := collect(*dir, *before, *recoverNames)
	if e != nil {
		fatal(e)
	}
	if e = os.MkdirAll(*out, 0755); e != nil {
		fatal(e)
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		fatal(e)
	}
	if e = os.WriteFile(filepath.Join(*out, "report.json"), b, 0644); e != nil {
		fatal(e)
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Over/Under price audit\n\n%s\n\nPrice groups compare offered odds, not identical players or independent samples. Both sides of a frozen question share one outcome. Flat results are hypothetical one-unit diagnostics, including paper rows; not money actually won.\n\n", r.Note)
	fmt.Fprintln(&md, "| Cohort | Stat | Side | Price | N | Mean raw | Mean final | Mean break-even | >=55% | Edge >=1pp | Both | W–L / push / void | Flat units |\n|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---|---:|")
	for _, g := range r.Groups {
		rawCell := "unavailable"
		if g.RawN > 0 {
			rawCell = fmt.Sprintf("%.1f%% (%d)", 100*g.Raw, g.RawN)
		}
		resultCell, flatCell := "pending / unavailable", "—"
		if g.Wins+g.Losses+g.Pushes+g.Voids > 0 {
			resultCell = fmt.Sprintf("%d–%d / %d / %d", g.Wins, g.Losses, g.Pushes, g.Voids)
			flatCell = fmt.Sprintf("%+.2f", g.Flat)
		}
		fmt.Fprintf(&md, "| %s | %s | %s | %s | %d | %s | %.1f%% | %.1f%% | %d | %d | %d | %s | %s |\n", g.Cohort, g.Stat, g.Side, g.Price, g.N, rawCell, 100*g.Final, 100*g.BreakEven, g.ProbabilityBar, g.EdgeBar, g.BothBars, resultCell, flatCell)
	}
	if e = os.WriteFile(filepath.Join(*out, "report.md"), []byte(md.String()), 0644); e != nil {
		fatal(e)
	}
	fmt.Printf("Price audit: %d side rows, %d groups. %s\nNo predictions or records changed.\n", len(r.Questions), len(r.Groups), filepath.Join(*out, "report.md"))
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
