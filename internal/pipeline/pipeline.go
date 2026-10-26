// Package pipeline loads the data directory into a ready-to-use feature builder. The commands
// (backtest, train, predict) share it so they all see the data the same way.
package pipeline

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"strings"

	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/team"
)

// notBooks are prediction markets, not sportsbooks; their lines are not used as the game's market.
var notBooks = map[string]bool{"kalshi": true, "polymarket": true}

// IsSportsbook is false for prediction-market vendors.
func IsSportsbook(vendor string) bool { return !notBooks[strings.ToLower(vendor)] }

// LoadGameLines reads every known betting spread and total: last season's opening lines
// (opening_games_*.jsonl, from `pull gameodds`) and the daily odds snapshots. It returns them
// keyed by (day number, team) for feat.Builder.SetGameLines. The spread is signed from that
// team's side, negative for the favourite. Files that are missing are simply skipped.
func LoadGameLines(dir string, games []data.PlayerGame) map[[2]int]feat.GameLine {
	type meta struct{ home, away, day int }
	byGame := map[int]meta{}
	for _, g := range games {
		if g.HomeTeamID != 0 && g.VisitorTeamID != 0 {
			byGame[g.GameID] = meta{g.HomeTeamID, g.VisitorTeamID, g.Day}
		}
	}
	var files []string
	for _, pat := range []string{"opening_games_*.jsonl", "snapshots/odds_*.jsonl"} {
		m, _ := filepath.Glob(filepath.Join(dir, pat))
		files = append(files, m...)
	}
	out := map[[2]int]feat.GameLine{}
	for _, f := range files {
		odds, err := team.LoadOdds(f)
		if err != nil {
			continue
		}
		var keep []team.Odds
		for _, o := range odds {
			if IsSportsbook(o.Vendor) {
				keep = append(keep, o)
			}
		}
		for id, c := range team.ConsensusOf(keep) {
			m, ok := byGame[id]
			if !ok || !c.HasSpread || !c.HasTotal {
				continue
			}
			out[[2]int{m.day, m.home}] = feat.GameLine{Spread: c.SpreadHome, Total: c.Total}
			out[[2]int{m.day, m.away}] = feat.GameLine{Spread: -c.SpreadHome, Total: c.Total}
		}
	}
	return out
}

// MinAvgMinutes is the smallest 15-game minutes average a player needs to appear in training.
// The posting rule only covers players averaging 20+ (or projected 18+), so training focuses on
// players near and above that level.
const MinAvgMinutes = 12

// Data is everything loaded from the data directory.
type Data struct {
	Games   []data.PlayerGame
	Adv     map[data.AdvKey]data.AdvRow
	Builder *feat.Builder
	Rows    []feat.Row // only when requested
}

// Load reads stats (required) and advanced stats (optional) from dir and builds the feature
// builder. With withRows it also builds a feature row for every historical game, which the
// backtest and train commands need and predict does not.
func Load(dir string, withRows bool) (*Data, error) {
	games, err := data.LoadStatsDir(dir)
	if err != nil {
		return nil, err
	}
	adv, err := data.LoadAdvancedDir(dir)
	if err != nil {
		return nil, fmt.Errorf("advanced stats: %w", err)
	}
	d := &Data{Games: games, Adv: adv, Builder: feat.NewBuilder(games, adv)}
	d.Builder.SetGameLines(LoadGameLines(dir, games))
	if withRows {
		d.Rows = d.Builder.Rows(MinAvgMinutes)
	}
	return d, nil
}

// DisabledFor returns the feature groups to leave out. Without -lineup the teammates-out and
// opponent-out groups are off: it is the one group a backtest against opening lines cannot honestly validate.
func DisabledFor(lineup bool) []string {
	if lineup {
		return nil
	}
	return []string{feat.GroupTeammates, feat.GroupOppOut}
}

// fingerprint is a short hash of the feature list, so a calibration fitted for an older feature
// set is detected instead of being silently applied to a newer model.
func fingerprint() string {
	h := fnv.New32a()
	for _, s := range feat.CommonSpecs {
		h.Write([]byte(s.Name + "/" + s.Group + ";"))
	}
	for _, specs := range feat.StatSpecs {
		for _, s := range specs {
			h.Write([]byte(s.Name + "/" + s.Group + ";"))
		}
		h.Write([]byte("|"))
	}
	return fmt.Sprintf("%08x", h.Sum32())[:6]
}

// ConfigName names the feature configuration for calibration bookkeeping. It includes a
// fingerprint of the feature list: if the features change, the calibration must be refitted.
func ConfigName(disabled []string) string {
	for _, g := range disabled {
		if g == feat.GroupTeammates {
			return "pregame-" + fingerprint()
		}
	}
	return "lineup-" + fingerprint()
}
