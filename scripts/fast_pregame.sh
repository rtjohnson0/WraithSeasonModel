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

# Off-night: a legitimately empty game slate does not require a fake injury
# receipt. Do not generate/overwrite a new predictions board in that case.
if python3 - <<'PY'
import json
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo
today = datetime.now(ZoneInfo("America/New_York")).date().isoformat()
path = Path("data/snapshots") / f"schedule_{today}.json"
if not path.is_file():
    raise SystemExit(1)
try:
    schedule = json.loads(path.read_text())
except (ValueError, OSError):
    raise SystemExit(1)
raise SystemExit(0 if schedule.get("date") == today and schedule.get("games") == [] else 1)
PY
then
  echo "FAST: no scheduled NBA games tonight; original board and models preserved."
  exit 0
fi

# Complete current BDL injuries for ALL scheduled teams are mandatory.
# This check fails closed if the API was throttled, the report is older than
# 30 minutes, or the schedule lacks a home/away team. Never reuse yesterday's.
python3 scripts/verify_pregame_injuries.py

echo "== FAST: latest projected lineups (bounded; missing context remains unknown) =="
timeout 90s go run ./cmd/pull context -live || echo "Projected lineups unavailable; only verified earlier context is usable"

echo "== FAST: extra sportsbook quotes (bounded, optional) =="
timeout 90s go run ./cmd/sgo snapshot || echo "Supplemental quotes unavailable; primary freshly collected odds remain"

echo "== FAST: generate current player props, combos and V2 =="
# The predictor validates quote capture freshness; missing/stale quotes fail closed.
# Skip only research-only alternative scenario simulations. Same quotes, rotations, primary and V5 probabilities.
timeout 540s go run ./cmd/predict -rotationpolicy "$policy" -fastdetails

echo "== FAST: refresh live moneyline, spread and totals =="
# Player projections can take longer than the 30-minute quote window.
# Refresh ONLY whole-game odds immediately before game model evaluation.
timeout 120s go run ./cmd/pull oddsrefresh || echo "Fresh game odds unavailable; game markets stay empty rather than using stale prices"
go run ./cmd/games

echo "== FAST: current boards generated; cached training/history unchanged =="
