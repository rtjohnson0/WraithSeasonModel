package main

import (
	"sort"
	"time"

	"propsmodel/internal/lineslog"
	"propsmodel/internal/props"
)

// linesLogRows turns what the scoring loop collected into the rows internal/lineslog stores: the
// market rows get the day's opening median from the snapshot's line history, and the off-market
// rows keep the price, side and model probability the page showed.
func linesLogRows(date string, rows map[marketKey]lineslog.Market, series map[props.MoveKey][]props.SeriesPoint, off []offRow, at string, games map[int]props.GameInfo) ([]lineslog.Market, []lineslog.Off) {
	var mkt []lineslog.Market
	now, _ := time.Parse(time.RFC3339, at)
	started := func(gid int) bool {
		t, err := time.Parse(time.RFC3339, games[gid].Start)
		return err == nil && !now.IsZero() && !t.After(now)
	}
	for _, m := range rows {
		if started(m.GameID) {
			continue // once a game has tipped the feed can carry live lines, which are not what the model priced
		}
		for k, pts := range series {
			if k.Game == m.GameID && k.Player == m.PlayerID && string(k.Stat) == m.Stat && len(pts) > 1 {
				m.Open = pts[0].Line
				break
			}
		}
		mkt = append(mkt, m)
	}
	sort.Slice(mkt, func(i, j int) bool {
		if mkt[i].GameID != mkt[j].GameID {
			return mkt[i].GameID < mkt[j].GameID
		}
		if mkt[i].PlayerID != mkt[j].PlayerID {
			return mkt[i].PlayerID < mkt[j].PlayerID
		}
		return mkt[i].Stat < mkt[j].Stat
	})
	var out []lineslog.Off
	for _, r := range off {
		gid := 0
		for _, m := range mkt {
			if m.PlayerID == r.PlayerID && m.Stat == string(r.Stat) {
				gid = m.GameID
				break
			}
		}
		if gid == 0 || started(gid) {
			continue
		}
		out = append(out, lineslog.Off{
			Date: date, At: at, GameID: gid, PlayerID: r.PlayerID, Player: r.Player, Team: r.Team, Stat: string(r.Stat),
			Tip: games[gid].Start, Book: r.Vendor, Side: r.Side, Line: r.Line, Median: r.Median, Books: r.Books, Odds: r.Odds,
			PSide: r.Prob, Edge: r.Edge, Proj: r.Proj,
		})
	}
	return mkt, out
}
