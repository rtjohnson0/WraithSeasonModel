package main

import (
	"fmt"
	"propsmodel/internal/picklog"
)

// oneDirection keeps one current direction per game/player/stat. Different
// alternate lines remain in market comparison data, never competing picks.
func oneDirection(rows []scored) []scored {
	key := func(s scored) string { return fmt.Sprintf("%d|%d|%s", s.GameID, s.PlayerID, s.Stat) }
	best := map[string]scored{}
	for _, s := range rows {
		cur, ok := best[key(s)]
		if !ok || picklog.TierRank(s.Tier) < picklog.TierRank(cur.Tier) || (picklog.TierRank(s.Tier) == picklog.TierRank(cur.Tier) && boardBefore(s, cur)) {
			best[key(s)] = s
		}
	}
	var out []scored
	seen := map[string]bool{}
	for _, s := range rows {
		k := key(s)
		if !seen[k] {
			out = append(out, best[k])
			seen[k] = true
		}
	}
	return out
}
