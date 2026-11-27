#!/usr/bin/env bash
# Quick refresh close to tip-off (no retrain): newest lines and injuries, then picks.
set -euo pipefail
python3 scripts/calibrate_games.py
YEAR=$(date +%Y); MONTH=$(date +%m)
if [ "$((10#$MONTH))" -ge 8 ]; then SEASON=$YEAR; else SEASON=$((YEAR-1)); fi
go run ./cmd/pull context -seasons "$SEASON" -feeds passing,rebounding -recentonly -maxminutes 1 || echo "Tracking refresh incomplete"
go run ./cmd/pull snapshot
go run ./cmd/pull context -live || echo "Projected lineups unavailable; no post-start lineups substituted"
go run ./cmd/sgo snapshot || echo "sgo snapshot skipped"
go run ./cmd/accuracy
python3 scripts/accuracy_report.py
go run ./cmd/combos calibrate || echo "combo calibration unavailable; comparison mode retained"
# V2.2 rotation-v5 actually changes V2 projections; revert with WRAITH_V2_ROTATION_POLICY=rotation-v4.
go run ./cmd/predict -rotationpolicy "${WRAITH_V2_ROTATION_POLICY:-rotation-v5}"
# Read-only evidence: neither command changes picks, prices, or public site data.
python3 scripts/archive_forward_evidence.py || echo "Versioned evidence archive unavailable"
python3 scripts/closing_line_audit.py || echo "Closing-line coverage audit unavailable"
go run ./cmd/minutesaudit -legacy || echo "Minutes audit unavailable; live model unchanged"
python3 scripts/minutes_context_diagnostic.py || echo "Minutes context diagnostic unavailable; live model unchanged"
# Fixed, zero-stake V2 research challenger; never publishes per-pick predictions.
go run ./cmd/priceaudit || echo "Price audit unavailable; live model unchanged"
go run ./cmd/rotationaudit -policy rotation-v2 -out run_evidence/rotation-v2 || echo "Prior V2 evaluation unavailable"
go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback || echo "Absence fallback evaluation unavailable"
go run ./cmd/rotationaudit -policy rotation-v4 -phase3 -v21lab -out run_evidence/context-audit || echo "Context evaluation unavailable"
# Independent v5 cohort: don't mix it with the three-date v4 evaluation.
go run ./cmd/rotationaudit -policy rotation-v5 -out run_evidence/guarded-v5 || echo "Guarded V5 evaluation unavailable"
# The private question file is consumed and deleted by the following aggregate-only shadow report.
python3 scripts/phase3_v2_shadow.py || echo "Phase 3 shadow unavailable; production board unchanged"
go run ./cmd/pull oddsrefresh || echo "Fresh game odds unavailable; stale prices remain ineligible"
go run ./cmd/games || echo "game lines skipped"
go run ./cmd/gamepace || echo "Game challenger unavailable; original game board retained"

go run ./cmd/quarter || echo "quarter comparisons unavailable; full-game data was not substituted"

python3 scripts/run_tail_shadow.py || echo "Experimental shadow unavailable; primary board retained"
