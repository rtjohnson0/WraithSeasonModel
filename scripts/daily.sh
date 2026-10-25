#!/usr/bin/env bash
# Daily pipeline for macOS/Linux. Run from the project folder:  ./scripts/daily.sh [-l] [-s] [-f]
#   -l  include the teammates-out (lineup) features
#   -s  skip retraining
#   -f  re-download the whole current season instead of only new games
# Run it in the morning, then run ./scripts/pretip.sh an hour before tip-off.
set -euo pipefail
LINEUP=""; SKIP=0; NEW="-new"
for a in "$@"; do
  case "$a" in -l) LINEUP="-lineup";; -s) SKIP=1;; -f) NEW="";; esac
done
MONTH=$(date +%m); YEAR=$(date +%Y)
if [ "$((10#$MONTH))" -ge 8 ]; then SEASON=$YEAR; else SEASON=$((YEAR-1)); fi

echo "== refresh box scores ($SEASON) =="
go run ./cmd/pull stats -seasons "$SEASON" $NEW
go run ./cmd/pull advanced -seasons "$SEASON" $NEW || echo "advanced refresh skipped"
echo "== current rosters =="
go run ./cmd/pull players
echo "== grade past picks =="
go run ./cmd/settle || echo "settle skipped"
echo "== review yesterday (csv) =="
go run ./cmd/review || echo "review skipped"
if [ "$SKIP" = 0 ]; then
  echo "== retrain =="
  go run ./cmd/train $LINEUP
fi
echo "== snapshot lines and injuries =="
go run ./cmd/pull snapshot
go run ./cmd/sgo snapshot || echo "sgo snapshot skipped"
echo "== player props =="
go run ./cmd/predict
echo "== game lines =="
go run ./cmd/games || echo "game lines skipped"
