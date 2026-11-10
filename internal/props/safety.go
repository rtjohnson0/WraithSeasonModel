package props

import (
	"math"
	"propsmodel/internal/marketperiod"
	"strings"
	"time"
)

// QuoteVersion marks quotes checked for period, matching line/price, and pre-tip capture.
const QuoteVersion = 2

func ValidOdds(odds int) bool { return odds >= 100 || odds <= -100 }

// BookID counts one physical sportsbook once even when both feeds cover it.
func BookID(vendor string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(vendor)), "sgo/")
}

// BeforeTip is deliberately strict: a missing or invalid start time cannot establish pregame status.
func BeforeTip(start string, asOf time.Time) bool {
	t, err := time.Parse(time.RFC3339, start)
	return err == nil && !asOf.IsZero() && asOf.Before(t)
}

// Executable checks quote and capture ages separately. A new response does not make an old price fresh.
func Executable(l Line, g GameInfo, asOf time.Time, quoteAge, captureAge time.Duration) bool {
	if !marketperiod.FullGame(l.Period) || !BeforeTip(g.Start, asOf) || !l.QuoteVerified || !ValidOdds(l.OverOdds) || !ValidOdds(l.UnderOdds) || l.Line < 0 || math.IsNaN(l.Line) || math.IsInf(l.Line, 0) {
		return false
	}
	cap, ec := time.Parse(time.RFC3339, l.CapturedAt)
	updated, eu := time.Parse(time.RFC3339, l.UpdatedAt)
	if ec != nil || eu != nil || cap.After(asOf) || updated.After(asOf) {
		return false
	}
	if captureAge > 0 && asOf.Sub(cap) > captureAge {
		return false
	}
	if quoteAge > 0 && asOf.Sub(updated) > quoteAge {
		return false
	}
	return true
}

// ExecutableLines removes unsafe quotes before consensus, anchoring, prediction or logging.
func ExecutableLines(lines []Line, games map[int]GameInfo, asOf time.Time, quoteAge, captureAge time.Duration) []Line {
	var out []Line
	for _, l := range lines {
		if Executable(l, games[l.GameID], asOf, quoteAge, captureAge) {
			out = append(out, l)
		}
	}
	return UniqueBooks(out)
}

// UniqueBooks chooses the freshest capture for each real book/player/game/stat.
// Feed duplicates must never inflate the consensus book count or the model's sample size.
func UniqueBooks(lines []Line) []Line {
	latest := map[lineKey]Line{}
	var order []lineKey
	for _, l := range lines {
		k := lineKey{l.GameID, l.PlayerID, BookID(l.Vendor), l.Stat}
		prev, ok := latest[k]
		if !ok {
			order = append(order, k)
		}
		if !ok || quoteLater(l, prev) {
			latest[k] = l
		}
	}
	out := make([]Line, 0, len(order))
	for _, k := range order {
		out = append(out, latest[k])
	}
	return out
}

func quoteLater(a, b Line) bool {
	for _, pair := range [][2]string{{a.UpdatedAt, b.UpdatedAt}, {a.CapturedAt, b.CapturedAt}} {
		at, ae := time.Parse(time.RFC3339, pair[0])
		bt, be := time.Parse(time.RFC3339, pair[1])
		if ae == nil && be != nil {
			return true
		}
		if ae != nil && be == nil {
			return false
		}
		if ae == nil && be == nil && !at.Equal(bt) {
			return at.After(bt)
		}
	}
	// Prefer the direct source on a timestamp tie.
	return strings.HasPrefix(b.Vendor, "sgo/") && !strings.HasPrefix(a.Vendor, "sgo/")
}

// LoadClosingSnapshot requires a capture within the requested pre-tip window.
func LoadClosingSnapshot(path string, window time.Duration) ([]Line, error) {
	lines, err := LoadPregameSnapshot(path, false, time.Time{}, 6*time.Hour)
	if err != nil {
		return nil, err
	}
	games, err := LoadGames(path)
	if err != nil {
		return nil, err
	}
	var out []Line
	for _, l := range lines {
		at, ea := time.Parse(time.RFC3339, l.CapturedAt)
		tip, et := time.Parse(time.RFC3339, games[l.GameID].Start)
		if ea == nil && et == nil && at.Before(tip) && tip.Sub(at) <= window {
			out = append(out, l)
		}
	}
	return UniqueBooks(out), nil
}
