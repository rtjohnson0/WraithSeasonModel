#!/usr/bin/env bash
# Quick refresh close to tip-off (no retrain): newest lines and injuries, then picks.
set -euo pipefail
go run ./cmd/pull snapshot
go run ./cmd/sgo snapshot || echo "sgo snapshot skipped"
go run ./cmd/accuracy
python3 scripts/accuracy_report.py
go run ./cmd/combos calibrate || echo "combo calibration unavailable; comparison mode retained"
go run ./cmd/predict
go run ./cmd/minutesaudit -legacy || echo "Minutes audit unavailable; live model unchanged"
go run ./cmd/priceaudit || echo "Price audit unavailable; live model unchanged"
go run ./cmd/rotationaudit || echo "Rotation evaluation unavailable; V1 unchanged"
go run ./cmd/games || echo "game lines skipped"

go run ./cmd/quarter || echo "quarter comparisons unavailable; full-game data was not substituted"

python3 scripts/run_tail_shadow.py || echo "Experimental shadow unavailable; primary board retained"
