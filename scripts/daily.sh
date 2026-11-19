#!/usr/bin/env bash
# Daily pipeline for macOS/Linux. Run from the project folder:  ./scripts/daily.sh [-l] [-s] [-f]
#   -l  include the teammates-out (lineup) features
#   -s  skip retraining
#   -f  re-download the whole current season instead of only new games
# Run it in the morning, then run ./scripts/pretip.sh an hour before tip-off.
set -euo pipefail
export PYTHONUTF8=1
LINEUP=""; SKIP=0; NEW="-new"
for a in "$@"; do
  case "$a" in -l) LINEUP="-lineup";; -s) SKIP=1;; -f) NEW="";; esac
done
MONTH=$(date +%m); YEAR=$(date +%Y)
if [ "$((10#$MONTH))" -ge 8 ]; then SEASON=$YEAR; else SEASON=$((YEAR-1)); fi

echo "== refresh box scores ($SEASON) =="
go run ./cmd/pull stats -seasons "$SEASON" $NEW
go run ./cmd/pull advanced -seasons "$SEASON" $NEW || echo "advanced refresh skipped"
echo "== tracking opportunities (bounded, resumable) =="
go run ./cmd/pull context -seasons "$SEASON,$((SEASON-1))" -feeds passing,rebounding -maxminutes 2 || echo "Tracking refresh incomplete; cached data retained"
go run ./cmd/pull context -seasons "$SEASON,$((SEASON-1))" -feeds possessions,drives,catchshoot,pullupshot,defense_rim,defense_threes,shooting_distance,lineup_stats,matchups -maxminutes 1 || echo "Additional context refresh incomplete"
echo "== current rosters =="
go run ./cmd/pull players
echo "== audit quote provenance =="
python3 scripts/audit_quotes.py --data data --apply
echo "== grade past picks =="
go run ./cmd/settle || echo "settle skipped"
go run ./cmd/combos settle || echo "combo settle skipped"
go run ./cmd/games -settle || echo "game settlement unavailable"
go run ./cmd/lines || echo "lines report skipped"
go run ./cmd/v2 settle || echo "v2 settle skipped"
go run ./cmd/comboaudit || echo "Combo diagnostic unavailable; forecasts unchanged"
echo "== review yesterday (csv) =="
go run ./cmd/review || echo "review skipped"
if [ "$SKIP" = 0 ]; then
  echo "== refresh prop model (bounded historical backfill) =="
  python3 scripts/refresh_models.py --season "$SEASON" ${LINEUP:+--lineup}
fi
echo "== calibrate game-v2 =="
python3 scripts/calibrate_games.py --force --season "$((SEASON-1))"
echo "== snapshot lines and injuries =="
go run ./cmd/pull snapshot
go run ./cmd/pull context -live || echo "Projected lineups unavailable; no post-start lineups substituted"
go run ./cmd/sgo snapshot || echo "sgo snapshot skipped"
echo "== player props =="
go run ./cmd/accuracy
python3 scripts/accuracy_report.py
go run ./cmd/combos calibrate || echo "combo calibration unavailable; comparison mode retained"
go run ./cmd/predict -rotationpolicy rotation-v4
go run ./cmd/minutesaudit -legacy || echo "Minutes audit unavailable; live model unchanged"
go run ./cmd/priceaudit || echo "Price audit unavailable; live model unchanged"
go run ./cmd/rotationaudit -policy rotation-v2 -out run_evidence/rotation-v2 || echo "Prior V2 evaluation unavailable"
go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback || echo "Absence fallback evaluation unavailable"
echo "== game lines =="
go run ./cmd/rotationaudit -policy rotation-v4 -out run_evidence/context-audit || echo "Context evaluation unavailable"
go run ./cmd/games || echo "game lines skipped"

go run ./cmd/quarter || echo "quarter comparisons unavailable; full-game data was not substituted"

python3 scripts/run_tail_shadow.py || echo "Experimental shadow unavailable; primary board retained"
