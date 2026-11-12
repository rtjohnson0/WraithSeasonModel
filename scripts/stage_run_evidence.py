"""Retain current-season actuals alongside the existing workflow run evidence.
Inputs are copied, never moved. Old seasons, API keys and environment files are excluded.
"""
import argparse
import json
import shutil
from collections import Counter
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo


def stage(data, output, season=None):
    data, output = Path(data), Path(output)
    now = datetime.now(ZoneInfo('America/New_York'))
    season = season if season is not None else now.year if now.month >= 8 else now.year-1
    output.mkdir(parents=True, exist_ok=True)
    source = data/f'stats_{season}.jsonl'
    info = {'at': now.isoformat(), 'season': season, 'statsPresent': source.is_file(), 'finalThrough': None, 'rows': 0, 'statuses': {}}
    if source.is_file():
        statuses = Counter()
        with source.open(encoding='utf-8-sig') as stream:
            for line in stream:
                if not line.strip():
                    continue
                row=json.loads(line);game=row.get('game') or {};status=game.get('status','missing');statuses[status]+=1;info['rows']+=1
                date=str(game.get('date',''))[:10]
                if status.strip().casefold()=='final' and date:
                    info['finalThrough']=max(info['finalThrough'] or '',date)
        info['statuses']=dict(statuses)
        shutil.copyfile(source,output/source.name)
    board=data/'board.json'
    if board.is_file():
        snapshot=json.loads(board.read_text(encoding='utf-8-sig'))
        info['boardDate']=snapshot.get('date');info['boardScoredAt']=snapshot.get('at')
    (output/'coverage.json').write_text(json.dumps(info,indent=2),encoding='utf-8')
    return info


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--data',default='data');parser.add_argument('--out',default='run_evidence');parser.add_argument('--season',type=int)
    args=parser.parse_args();print(json.dumps(stage(args.data,args.out,args.season),indent=2))
