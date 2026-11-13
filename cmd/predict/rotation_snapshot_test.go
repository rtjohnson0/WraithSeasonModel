package main

import (
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/pipeline"
	"propsmodel/internal/predictor"
	"propsmodel/internal/props"
	"propsmodel/internal/replay"
	"testing"
	"time"
)

// Opt-in integration against an archived input snapshot. It never writes into
// that data or treats retrospective execution as a live forward decision.
func TestArchivedRotationSnapshot(t *testing.T) {
	dir, rosterPath := os.Getenv("ROTATION_FIXTURE_DATA"), os.Getenv("ROTATION_FIXTURE_ROSTER")
	if dir == "" || rosterPath == "" {
		t.Skip("optional archived input integration")
	}
	asOf, e := time.Parse(time.RFC3339, "2026-11-13T16:18:58Z")
	if e != nil {
		t.Fatal(e)
	}
	date := "2026-11-13"
	d, e := pipeline.Load(dir, false)
	if e != nil {
		t.Fatal(e)
	}
	active, e := props.LoadActiveTeams(rosterPath)
	if e != nil {
		t.Fatal(e)
	}
	d.Builder.SetRoster(active)
	m, e := predictor.Load(filepath.Join(dir, "model.json"))
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "snapshots", "props_"+date+".jsonl")
	ls, e := props.LoadPregameSnapshot(path, false, asOf, 6*time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	games, _, _, e := props.LoadSlateGames(path, date, asOf)
	if e != nil {
		t.Fatal(e)
	}
	freshLines := []props.Line{}
	for _, l := range ls {
		if props.Executable(l, games[l.GameID], asOf, 6*time.Hour, 30*time.Minute) {
			freshLines = append(freshLines, l)
		}
	}
	freshLines, _ = props.ReconcileBooks(freshLines)
	freshLines, _ = props.FilterConsensus(freshLines, 3, .5)
	injury := filepath.Join(dir, "snapshots", "injuries_"+date+".jsonl")
	statuses, _ := props.LoadStatuses(injury)
	fresh := freshRotationStatuses(injury, filepath.Join(dir, "snapshots"), date, asOf, d.Builder, active, games)
	f := newRotationForecast(d.Games, active, fresh, statuses, games, date)
	valid := 0
	for _, a := range f.teams {
		sum := 0.
		for _, m := range a.Minutes {
			sum += m
		}
		if a.Valid {
			valid++
			if math.Abs(sum-240) > 1e-7 {
				t.Fatal("team minutes not conserved", a.Team, sum)
			}
		}
		t.Logf("team %d valid=%t matches=%d reason=%s", a.Team, a.Valid, a.MatchedGames, a.Reason)
	}
	if valid == 0 {
		t.Fatal("no team allocations from archived data")
	}
	out := map[int]bool{}
	for pid, s := range fresh {
		if s.Status == "Out" {
			out[pid] = true
		}
	}
	rows := map[int]*feat.Row{}
	rowFor := func(pid int, g props.GameInfo, team int) *feat.Row {
		if r := rows[pid]; r != nil {
			return r
		}
		opp := g.Visitor
		if team == g.Visitor {
			opp = g.Home
		}
		r := d.Builder.Live(pid, team, opp, team == g.Home, date, out)
		rows[pid] = r
		return r
	}
	all, records := scoreRotationSingles(f, m, d.Builder, freshLines, games, func(pid int) (int, bool) { tm, ok := active[pid]; return tm, ok }, rowFor, statuses, date, .5, 1, asOf)
	if len(records) == 0 {
		t.Fatal("no archived questions scored")
	}
	for _, r := range records {
		p, e := replay.Recompute(m, r)
		if e != nil || math.Abs(p-r.P1) > 1e-9 {
			t.Fatal("saved rotation forecast not exactly reproducible", e)
		}
	}
	for _, s := range all {
		if s.PlayerID == 313 && s.Stat == data.Rebounds {
			t.Logf("Archived Melton example: baseline %.2f rotation %.2f joint matches %d", s.Rotation.BaselineMinutes, s.ProjMin, s.Rotation.MatchedGames)
			break
		}
	}
	t.Logf("archived integration: %d valid teams, %d scored quotes, %d reproducible questions", valid, len(all), len(records))
}
