#!/usr/bin/env python3
"""Refresh the prop model/calibration as a pair; restore both on any failure."""
import argparse
import json
import subprocess
from pathlib import Path
from calibrate_games import current_season

def refresh(season=None, lineup=False):
    season = season or current_season()
    # The opening-props downloader resumes game by game; completed caches make
    # this a local scan, while a first run can take considerably longer.
    subprocess.run(['go','run','./cmd/pull','props','-season',str(season-1)],check=True)
    paths=[Path('data/model.json'),Path('data/calibration.json')]
    backup={p:p.read_bytes() if p.exists() else None for p in paths}
    flags=['-lineup'] if lineup else []
    try:
        subprocess.run(['go','run','./cmd/train',*flags],check=True)
        subprocess.run(['go','run','./cmd/backtest','-season',str(season-1),*flags],check=True)
        model,cal=(json.loads(p.read_text(encoding='utf-8')) for p in paths)
        if model.get('version') != cal.get('model_version'):
            raise RuntimeError('New prop model and calibration versions do not match.')
    except BaseException:
        for p,b in backup.items():
            if b is None:p.unlink(missing_ok=True)
            else:p.write_bytes(b)
        raise
    print('Prop model and matching calibration refreshed.',flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--season',type=int)
    p.add_argument('--lineup',action='store_true')
    a=p.parse_args();refresh(a.season,a.lineup)
