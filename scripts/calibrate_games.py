#!/usr/bin/env python3
"""Prepare matching game-v2 calibration before any new game board is scored."""
import argparse
import json
import subprocess
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

POLICY = 'game-v2'

def current_season(now=None):
    now = now or datetime.now(ZoneInfo('America/New_York'))
    return now.year if now.month >= 8 else now.year - 1

def valid_calibration(path, today):
    try:
        c = json.loads(path.read_text(encoding='utf-8'))
        return c.get('policy') == POLICY and bool(c.get('trained_before')) and c['trained_before'] < today and all(c.get(k, {}).get('sigma', 0) > 0 for k in ('margin', 'total'))
    except (OSError, ValueError, TypeError):
        return False

def prepare(force=False, season=None):
    season = season or current_season() - 1
    today = datetime.now(ZoneInfo('America/New_York')).date().isoformat()
    cal = Path('data/gamecal_research.json')
    if not force and valid_calibration(cal, today):
        print('Game-v2 calibration is ready.', flush=True)
        return
    odds = Path(f'data/opening_games_{season}.jsonl')
    if not odds.exists() or odds.stat().st_size == 0:
        print(f'Downloading historical game odds for {season}.', flush=True)
        try:
            subprocess.run(['go','run','./cmd/pull','gameodds','-season',str(season)], check=True)
        except BaseException:
            odds.unlink(missing_ok=True)  # Do not mistake a partial first download for a complete input.
            raise
    print(f'Fitting game-v2 calibration on season {season}.', flush=True)
    previous = cal.read_bytes() if cal.exists() else None
    try:
        subprocess.run(['go','run','./cmd/games','-backtest','-season',str(season),'-research'], check=True)
        if not valid_calibration(cal, today):
            raise RuntimeError('Game-v2 calibration did not validate; no new game predictions will be published.')
    except BaseException:
        if previous is None: cal.unlink(missing_ok=True)
        else: cal.write_bytes(previous)
        raise

if __name__ == '__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--force', action='store_true')
    p.add_argument('--season', type=int)
    args=p.parse_args()
    prepare(args.force,args.season)
