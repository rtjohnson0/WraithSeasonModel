package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"propsmodel/internal/runlog"
)

func toRunPlay(s scored) runlog.Play {
	return runlog.Play{
		PlayerID: s.PlayerID, Player: s.Player, Team: s.Team, GameID: s.GameID, Matchup: s.Matchup,
		Stat: string(s.Stat), Side: s.Side, Line: s.Line, Tier: s.Tier, Prob: s.Prob, Edge: s.Edge,
		Units: s.Units, ProjMean: s.ProjMean, ProjMin: s.ProjMin,
	}
}

// explainDrop says why a play from the previous run is no longer on the board. It uses this run's
// scoring of every line, so the reason is what the model now sees, not a guess.
// currentBest is the best-edge scored line for a play's player and stat: what the model would post now.
func currentBest(p runlog.Play, all []scored) *scored {
	var cur *scored
	for i := range all {
		s := &all[i]
		if s.PlayerID != p.PlayerID || string(s.Stat) != p.Stat {
			continue
		}
		if cur == nil || s.Edge > cur.Edge {
			cur = s
		}
	}
	return cur
}

func explainDrop(p runlog.Play, all, board []scored, out map[int]string, hi, minEdge float64) string {
	if st, isOut := out[p.PlayerID]; isOut {
		if st == "" {
			st = "out"
		}
		return "he is now ruled out (" + strings.ToLower(st) + ")"
	}
	cur := currentBest(p, all)
	if cur == nil {
		return "the line is no longer offered (or he no longer qualifies on minutes)"
	}
	var parts []string
	if cur.Side != p.Side {
		parts = append(parts, fmt.Sprintf("the model now leans %s (%.0f%%)", cur.Side, 100*cur.Prob))
	}
	if math.Abs(cur.Line-p.Line) >= 0.05 {
		parts = append(parts, fmt.Sprintf("line moved %.1f to %.1f", p.Line, cur.Line))
	}
	if p.ProjMean > 0 && math.Abs(cur.ProjMean-p.ProjMean) >= 0.4 {
		parts = append(parts, fmt.Sprintf("projection %.1f to %.1f", p.ProjMean, cur.ProjMean))
	}
	if p.ProjMin > 0 && math.Abs(cur.ProjMin-p.ProjMin) >= 2 {
		parts = append(parts, fmt.Sprintf("minutes %.0f to %.0f", p.ProjMin, cur.ProjMin))
	}
	if cur.Side == p.Side {
		switch {
		case cur.Prob < hi:
			parts = append(parts, fmt.Sprintf("probability fell to %.0f%% (bar is %.0f%%)", 100*cur.Prob, 100*hi))
		case cur.Edge < minEdge:
			parts = append(parts, fmt.Sprintf("edge fell to %+.1f%% (needs %+.1f%%)", 100*cur.Edge, 100*minEdge))
		default:
			parts = append(parts, "it still clears the bar but lost its place to stronger plays in the same game or on the same player")
		}
	}
	if cur.Maybe != "" {
		parts = append(parts, "his status is now "+strings.ToLower(cur.Maybe))
	}
	for _, b := range board {
		if b.PlayerID == p.PlayerID && string(b.Stat) == p.Stat && b.Side == p.Side && b.Tier == "LEAN" {
			parts = append(parts, "now only a fill-in lean")
			break
		}
	}
	if len(parts) == 0 {
		return "no longer clears the posting rules"
	}
	return strings.Join(parts, "; ")
}

// recordRun saves this run's board, compares it with the run before it today, prints what changed
// and appends the run to data/runs.jsonl.
func recordRun(dir, date string, picks, all []scored, out map[int]string, hi, minEdge float64) {
	now := time.Now()
	label, clock := runlog.LabelFor(now)
	run := runlog.Run{Date: date, At: now.UTC().Format(time.RFC3339), Label: label, Clock: clock}
	for _, s := range picks {
		if s.Tier != "LEAN" { // fill-ins churn every run and are not plays
			run.Plays = append(run.Plays, toRunPlay(s))
		}
	}
	path := filepath.Join(dir, "runs.jsonl")
	runs, err := runlog.Load(path)
	if err != nil {
		fmt.Println("note: could not read data/runs.jsonl:", err)
	}
	if day := runlog.ForDate(runs, date); len(day) > 0 {
		prev := day[len(day)-1]
		run.Prev = prev.Label
		kept, added, dropped := runlog.Diff(prev.Plays, run.Plays)
		for _, p := range dropped {
			d := runlog.Dropped{Play: p, Reason: explainDrop(p, all, picks, out, hi, minEdge)}
			if _, isOut := out[p.PlayerID]; !isOut {
				if cur := currentBest(p, all); cur != nil {
					d.Now = &runlog.Now{Side: cur.Side, Line: cur.Line, Prob: cur.Prob, Edge: cur.Edge, Odds: cur.Odds, Book: cur.Vendor}
				}
			}
			run.Dropped = append(run.Dropped, d)
		}
		was := map[string]runlog.Play{}
		for _, p := range prev.Plays {
			was[p.Key()] = p
		}
		for _, p := range kept {
			if old := was[p.Key()]; old.Tier != p.Tier {
				run.Changed = append(run.Changed, runlog.TierChange{Play: p, From: old.Tier})
			}
		}
		fmt.Printf("\nCHANGES SINCE THE %s RUN: %d dropped, %d new, %d tier changes, %d unchanged\n",
			prev.Label, len(dropped), len(added), len(run.Changed), len(kept)-len(run.Changed))
		for _, d := range run.Dropped {
			fmt.Printf("  DROPPED  %s %s %s %.1f (%s): %s\n", d.Player, d.Stat, d.Side, d.Line, d.Tier, d.Reason)
		}
		for _, p := range added {
			fmt.Printf("  NEW      %s %s %s %.1f (%s, %.0f%%)\n", p.Player, p.Stat, p.Side, p.Line, p.Tier, 100*p.Prob)
		}
		for _, c := range run.Changed {
			fmt.Printf("  TIER     %s %s %s: %s to %s\n", c.Player, c.Stat, c.Side, c.From, c.Tier)
		}
		fmt.Println()
	}
	if err := runlog.Append(path, run); err != nil {
		fmt.Println("note: could not save this run to data/runs.jsonl:", err)
	}
}
