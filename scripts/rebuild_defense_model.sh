#!/usr/bin/env bash
# Refit the corrected feature policy and calibration together. Existing model
# files and all historical quotes/results remain unchanged if either fit fails.
set -euo pipefail
DATA_DIR="${1:-data}"
YEAR=$(date +%Y); MONTH=$((10#$(date +%m)))
if [ "$MONTH" -ge 8 ]; then CURRENT_SEASON=$YEAR; else CURRENT_SEASON=$((YEAR-1)); fi
CALIBRATION_SEASON="${2:-$((CURRENT_SEASON-1))}"
go version >/dev/null
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT
python3 - "$DATA_DIR" "$STAGE" <<'PY'
import pathlib,sys
source,stage=map(lambda p:pathlib.Path(p).resolve(),sys.argv[1:])
if not list(source.glob('stats_*.jsonl')):
    raise SystemExit('Missing downloaded box scores. Restore the input cache or run cmd/pull stats before rebuilding.')
if not list(source.glob('opening_props_*.jsonl')):
    raise SystemExit('Missing historical opening prop quotes. Restore the input cache before fitting calibration; current quotes must not substitute for historical quotes.')
for p in source.iterdir():
    if p.name not in ('model.json','calibration.json'):
        (stage/p.name).symlink_to(p,target_is_directory=p.is_dir())
PY
echo "Fitting corrected model in staging..."
go run ./cmd/train -data "$STAGE"
echo "Fitting matching calibration on historical quotes..."
go run ./cmd/backtest -data "$STAGE" -season "$CALIBRATION_SEASON"
python3 - "$DATA_DIR" "$STAGE" <<'PY'
import json,pathlib,sys,shutil,os
dest,stage=map(lambda p:pathlib.Path(p).resolve(),sys.argv[1:])
m=json.loads((stage/'model.json').read_text());c=json.loads((stage/'calibration.json').read_text())
if m['version']!=6 or c.get('model_version')!=6:
    raise SystemExit('Refusing to publish mismatched model/calibration versions.')
if not c.get('by_stat'):
    raise SystemExit('No stat calibration was fitted; original model retained.')
for name in ('model.json','calibration.json'):
    shutil.copy2(dest/name,dest/(name+'.before-defense-fix'))
    shutil.copy2(stage/name,dest/(name+'.defense-ready'))
for name in ('model.json','calibration.json'):
    os.replace(dest/(name+'.defense-ready'),dest/name)
print('Corrected model and calibration installed. Next: run the normal pre-tip refresh for new quotes. Historical results were not rewritten.')
PY
