#!/usr/bin/env python3
"""Refresh the prop model/calibration as a pair; restore both on any failure."""
import argparse
import json
import subprocess
from pathlib import Path
from calibrate_games import current_season

def cached_games(season):
    expected=set()
    with Path(f'data/stats_{season}.jsonl').open(encoding='utf-8-sig') as f:
        for line in f:
            if not line.strip():continue
            row=json.loads(line);g=row.get('game') or {}
            if g.get('id') and (g.get('season') or season)==season:expected.add(g['id'])
    done=set();path=Path(f'data/opening_props_{season}.jsonl')
    if path.exists():
        with path.open(encoding='utf-8-sig') as f:
            for line in f:
                if not line.strip():continue
                try:r=json.loads(line)
                except ValueError:continue  # An interrupted final line is not a cached game.
                if r.get('_game_id') and isinstance(r.get('response'),dict):done.add(r['_game_id'])
    return len(expected & done),len(expected)

def matching_pair(paths):
    try:
        m,c=(json.loads(p.read_text(encoding='utf-8')) for p in paths)
        return isinstance(m.get('version'),int) and m['version']==c.get('model_version')
    except (OSError,ValueError,TypeError):return False

def refresh(season=None, lineup=False, backfill_minutes=5):
    season = season or current_season()
    # The opening-props downloader resumes game by game; completed caches make
    # this a local scan, while a first run can take considerably longer.
    download_failed=False
    try:
        subprocess.run(['go','run','./cmd/pull','props','-season',str(season-1),'-maxminutes',str(backfill_minutes)],check=True)
    except subprocess.CalledProcessError:
        download_failed=True
    paths=[Path('data/model.json'),Path('data/calibration.json')]
    done,total=cached_games(season-1)
    status=dict(status='ready' if total and done==total and not download_failed else 'backfilling',downloadFailed=download_failed,cachedGames=done,totalGames=total,season=season-1)
    Path('data/model_refresh.json').write_text(json.dumps(status,indent=2)+'\n',encoding='utf-8')
    if download_failed or not total or done!=total:
        if not matching_pair(paths):raise RuntimeError('Historical quotes incomplete and no matching prop model/calibration pair is available.')
        print(f'Historical props {done}/{total} cached. Keeping the matching prop model/calibration pair; publishing continues. Next Daily resumes the download.',flush=True)
        return False
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
    status['status']='refreshed'
    Path('data/model_refresh.json').write_text(json.dumps(status,indent=2)+'\n',encoding='utf-8')
    print('Prop model and matching calibration refreshed.',flush=True)
    return True

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--season',type=int)
    p.add_argument('--lineup',action='store_true')
    p.add_argument('--backfill-minutes',type=float,default=5,help='Soft per-run historical download budget; current API request may finish afterwards')
    a=p.parse_args();refresh(a.season,a.lineup,a.backfill_minutes)
