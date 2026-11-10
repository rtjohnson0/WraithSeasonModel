#!/usr/bin/env python3
"""Bind missing replay game IDs only when saved market and snapshot tip agree."""
import argparse
import json
import shutil
from datetime import datetime,timezone
from pathlib import Path

def read(path):
    with path.open(encoding='utf-8-sig') as handle:
        for line in handle:
            if line.strip():yield json.loads(line.lstrip('\ufeff'))

def stamp(value):
    try:return datetime.fromisoformat(value.replace('Z','+00:00')).timestamp()
    except (ValueError,TypeError,AttributeError):return None

def repair(directory,apply=False):
    root=Path(directory);report=[]
    for file in sorted((root/'replay').glob('decisions_*.jsonl')):
        day=file.stem.removeprefix('decisions_');marketfile=root/'lines'/f'market_{day}.jsonl';snapshot=root/'snapshots'/f'props_{day}.jsonl'
        if not marketfile.exists() or not snapshot.exists():continue
        markets={}
        for m in read(marketfile):markets.setdefault((m.get('pid'),m.get('stat')),{}).setdefault(m.get('game'),m)
        tips={}
        for capture in read(snapshot):
            g=capture.get('game') or {};gid=capture.get('game_id') or g.get('id');tip=g.get('datetime') or g.get('status')
            if gid and stamp(tip) is not None:tips[gid]=stamp(tip)
        rows=list(read(file));changed=0;unresolved=0
        for r in rows:
            identity=r.get('row',{}).get('identity',{})
            if identity.get('GameID'):continue
            candidates=markets.get((identity.get('PlayerID'),r.get('stat')),{})
            eligible=[(gid,m) for gid,m in candidates.items() if tips.get(gid) is not None and tips[gid]==stamp(r.get('tip'))]
            if len(eligible)!=1:unresolved+=1;continue
            gid,m=eligible[0]
            if stamp(r.get('at')) is None or stamp(r['at'])>=tips[gid]:unresolved+=1;continue
            identity.update(GameID=gid,Date=r.get('date'),Name=m.get('player') or identity.get('Name',''),TeamAbbr=m.get('team') or identity.get('TeamAbbr',''))
            changed+=1
        report.append(dict(file=file.name,repaired=changed,unresolved=unresolved))
        if apply and changed:
            tag=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
            shutil.copy2(file,file.with_name(file.name+'.before-identity-repair-'+tag))
            file.write_text(''.join(json.dumps(r,ensure_ascii=False,separators=(',',':'))+'\n' for r in rows),encoding='utf-8')
    (root/'replay_identity_audit.json').write_text(json.dumps(report,indent=2),encoding='utf-8')
    print(json.dumps(report,indent=2));return report
if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--data',default='data');p.add_argument('--apply',action='store_true');a=p.parse_args();repair(a.data,a.apply)
