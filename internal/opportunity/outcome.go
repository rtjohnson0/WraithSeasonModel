package opportunity

import (
	"math"
	"propsmodel/internal/data"
	"time"
)

// OutcomeVolume is for postgame diagnostics only. Unlike Signals, it may read
// same-game tracking, but only for an identity-matched final and a known capture.
// It must never be called by the pregame prediction path.
func (ix *Index) OutcomeVolume(g data.PlayerGame, stat data.Stat) (float64, string, string, bool) {
	if !g.SourceFinal || !g.BoxScoreComplete || g.Minutes <= 0 {
		return 0, "", "", false
	}
	volume, source, captured := math.NaN(), "", ""
	switch stat {
	case data.Points:
		if g.ShootingComplete {
			volume, source = g.Fga+.44*g.Fta, "final box score: FGA + 0.44 FTA"
		}
	case data.Threes:
		if g.ShootingComplete {
			volume, source = g.Fg3a, "final box score: 3PA"
		}
	case data.Assists, data.Rebounds:
		if ix == nil {
			return 0, "", "", false
		}
		typ, keys := "passing", []string{"potential_ast"}
		if stat == data.Rebounds {
			typ, keys = "rebounding", []string{"reb_chances", "rebound_chances_total"}
		}
		r, ok := ix.Rows[Key{Player: g.PlayerID, Game: g.GameID, Type: typ}]
		stamp, err := time.Parse(time.RFC3339Nano, r.Captured)
		day, dayErr := time.Parse("2006-01-02", g.Date)
		if !ok || r.Player != g.PlayerID || r.Game != g.GameID || r.Team != g.TeamID || r.Date != g.Date || err != nil || dayErr != nil || stamp.Before(day) || (!ix.AsOf.IsZero() && stamp.After(ix.AsOf)) {
			return 0, "", "", false
		}
		volume, source, captured = r.Value(keys...), typ+": "+keys[0], r.Captured
	}
	if !finite(volume) || volume < 0 || (stat != data.Points && g.Value(stat) > volume) {
		return 0, "", "", false
	}
	return volume, source, captured, true
}
