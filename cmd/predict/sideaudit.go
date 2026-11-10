package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/feat"
	"propsmodel/internal/replay"
	"time"
)

type SideSlice struct {
	Stat                string  `json:"stat"`
	Markets             int     `json:"markets"`
	RawOver             int     `json:"rawOver"`
	RawUnder            int     `json:"rawUnder"`
	FinalOver           int     `json:"finalOver"`
	FinalUnder          int     `json:"finalUnder"`
	OverProbBar         int     `json:"overProbBar"`
	UnderProbBar        int     `json:"underProbBar"`
	OverEdgeBar         int     `json:"overEdgeBar"`
	UnderEdgeBar        int     `json:"underEdgeBar"`
	PostedOver          int     `json:"postedOver"`
	PostedUnder         int     `json:"postedUnder"`
	AvgProjectionGap    float64 `json:"avgProjectionGap"`
	AvgMinutesShift     float64 `json:"avgMinutesShift"`
	MinutesDrivenUnders int     `json:"minutesDrivenUnders"`
}
type SideAudit struct {
	Findings []string    `json:"findings"`
	Date     string      `json:"date"`
	At       string      `json:"at"`
	Hi       float64     `json:"hi"`
	MinEdge  float64     `json:"minEdge"`
	Rows     []SideSlice `json:"rows"`
	Note     string      `json:"note"`
}

func saveSideAudit(dir, date string, all, picks []scored, hi, minEdge float64) error {
	a := SideAudit{Date: date, At: time.Now().UTC().Format(time.RFC3339Nano), Hi: hi, MinEdge: minEdge, Rows: []SideSlice{}, Note: "One quote nearest the market median per player/game/stat. Side counts diagnose model and selection behavior; they do not establish profitable bets. Refit only after actual-stat corrections and chronological validation; no forced Over quota."}
	type key struct {
		game, player int
		stat         string
	}
	near := map[key]scored{}
	for _, s := range all {
		k := key{s.GameID, s.PlayerID, string(s.Stat)}
		prev, ok := near[k]
		if !ok || math.Abs(s.Line-s.MarketMedian) < math.Abs(prev.Line-prev.MarketMedian) {
			near[k] = s
		}
	}
	rows := map[string]*SideSlice{}
	for _, st := range []string{"points", "rebounds", "assists", "threes"} {
		rows[st] = &SideSlice{Stat: st}
	}
	for _, s := range near {
		r := rows[string(s.Stat)]
		if r == nil {
			continue
		}
		r.Markets++
		rawOver := s.ProbRaw
		if s.Side == "UNDER" {
			rawOver = 1 - rawOver
		}
		if rawOver >= .5 {
			r.RawOver++
		} else {
			r.RawUnder++
		}
		if s.Side == "OVER" {
			r.FinalOver++
			if s.Prob >= hi {
				r.OverProbBar++
				if s.Edge >= minEdge {
					r.OverEdgeBar++
				}
			}
		} else {
			r.FinalUnder++
			if s.Prob >= hi {
				r.UnderProbBar++
				if s.Edge >= minEdge {
					r.UnderEdgeBar++
				}
			}
			if s.Shift <= -4 {
				r.MinutesDrivenUnders++
			}
		}
		r.AvgProjectionGap += s.ProjMean - s.Line
		r.AvgMinutesShift += s.Shift
	}
	for _, s := range picks {
		r := rows[string(s.Stat)]
		if r == nil {
			continue
		}
		if s.Side == "OVER" {
			r.PostedOver++
		} else {
			r.PostedUnder++
		}
	}
	for _, st := range []string{"points", "rebounds", "assists", "threes"} {
		r := rows[st]
		if r.Markets > 0 {
			r.AvgProjectionGap /= float64(r.Markets)
			r.AvgMinutesShift /= float64(r.Markets)
		}
		if r.Markets >= 5 {
			if float64(r.RawUnder)/float64(r.Markets) > .7 {
				a.Findings = append(a.Findings, fmt.Sprintf("%s: already Under-heavy before calibration (%d/%d markets); investigate rate and minutes levels first.", st, r.RawUnder, r.Markets))
			}
			if r.RawOver > r.RawUnder && r.FinalUnder > r.FinalOver {
				a.Findings = append(a.Findings, fmt.Sprintf("%s: calibration reverses the majority from Over to Under; validate saved calibration on corrected chronological results.", st))
			}
			if r.FinalOver > 0 && r.OverProbBar == 0 {
				a.Findings = append(a.Findings, fmt.Sprintf("%s: none of the %d calibrated Over leans reach the probability threshold.", st, r.FinalOver))
			} else if r.OverProbBar > 0 && r.OverEdgeBar == 0 {
				a.Findings = append(a.Findings, fmt.Sprintf("%s: %d Overs reach the probability bar, but none beat the exact price by the required margin.", st, r.OverProbBar))
			}
			if r.AvgMinutesShift <= -2 {
				a.Findings = append(a.Findings, fmt.Sprintf("%s: minutes average %.1f below normal; %d Under markets have cuts of at least four minutes. Check actual minutes errors before trusting this pattern.", st, -r.AvgMinutesShift, r.MinutesDrivenUnders))
			}
		}
		a.Rows = append(a.Rows, *r)
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "side_audit.json"), b, 0644)
}

// A forecast row is a hypothetical player/date, so it may lack a game ID.
// Bind the quote's real game before recording an auditable decision.
func decisionIdentity(r *feat.Row, gameID int, date string) replay.Row {
	captured := replay.Capture(r)
	captured.G.GameID = gameID
	captured.G.Date = date
	return captured
}

// The scored pool can be balanced while price selection leaves a one-sided
// board. Apply the existing skew limit to unique selected markets as well.
func guardSelectedSides(picks []scored, limit float64) ([]scored, string) {
	if limit <= 0 {
		return picks, ""
	}
	type market struct {
		game, player int
		stat         string
	}
	choices := map[market]scored{}
	for _, s := range picks {
		if s.Tier == "LEAN" {
			continue
		}
		k := market{s.GameID, s.PlayerID, string(s.Stat)}
		if old, ok := choices[k]; !ok || s.Edge > old.Edge {
			choices[k] = s
		}
	}
	if len(choices) < 10 {
		return picks, ""
	}
	under := 0
	for _, s := range choices {
		if s.Side == "UNDER" {
			under++
		}
	}
	side, share := "UNDER", float64(under)/float64(len(choices))
	if 1-share > share {
		side, share = "OVER", 1-share
	}
	if share <= limit {
		return picks, ""
	}
	reason := fmt.Sprintf("%.0f%% of %d selected markets lean %s: concentrated side, not independent opportunities; verify model/calibration before staking", share*100, len(choices), side)
	for i := range picks {
		if picks[i].Side == side {
			picks[i].Why = addWhy(picks[i].Why, reason)
			if picks[i].Tier == "PICK" {
				picks[i].Tier = "CHECK"
				picks[i].Units = 0
			}
		}
	}
	return picks, reason
}
