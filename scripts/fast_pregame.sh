#!/usr/bin/env bash
# Fast pregame path. Reuse the most recent cached season/tracking/model state.
# Never downloads an entire season, backfills odds, retrains, or runs audits.
# Fresh injuries, prices and lines remain mandatory for a valid pick update.
set -euo pipefail
export PYTHONUTF8=1

year=$(date +%Y)
month=$((10#$(date +%m)))
if [ "$month" -ge 8 ]; then season=$year; else season=$((year-1)); fi
policy="${WRAITH_V2_ROTATION_POLICY:-rotation-v5}"
case "$policy" in
  rotation-v4|rotation-v5) ;;
  *) echo "Unknown V2 policy: $policy" >&2; exit 2 ;;
esac

echo "== FAST: verify cached season stats and trained model =="
test -s "data/stats_${season}.jsonl" || { echo "Cached season stats absent; run full Daily once to seed data before using Fast." >&2; exit 1; }
test -s data/model.json || { echo "Missing trained model; run the full Daily/bootstrap." >&2; exit 1; }
test -s data/calibration.json || { echo "Missing calibration; run the full Daily/bootstrap." >&2; exit 1; }

echo "== FAST: fetch NEW pregame sportsbook lines, game odds and injury statuses =="
# Do not fall back to yesterday's sportsbook odds if this fails.
timeout 360s go run ./cmd/pull snapshot

echo "== FAST: latest projected lineups (bounded; missing context remains unknown) =="
timeout 90s go run ./cmd/pull context -live || echo "Projected lineups unavailable; only verified earlier context is usable"

echo "== FAST: extra sportsbook quotes (bounded, optional) =="
timeout 90s go run ./cmd/sgo snapshot || echo "Supplemental quotes unavailable; primary freshly collected odds remain"

echo "== FAST: generate current player props, combos and V2 =="
# The predictor validates quote capture freshness; missing/stale quotes fail closed.
go run ./cmd/predict -rotationpolicy "$policy"

echo "== FAST: refresh live moneyline, spread and totals =="
go run ./cmd/games

echo "== FAST: current boards generated; cached training/history unchanged =="
