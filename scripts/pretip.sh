#!/usr/bin/env bash
# Quick refresh close to tip-off (no retrain): newest lines and injuries, then picks.
set -euo pipefail
go run ./cmd/pull snapshot
go run ./cmd/sgo snapshot || echo "sgo snapshot skipped"
go run ./cmd/predict
go run ./cmd/games || echo "game lines skipped"
