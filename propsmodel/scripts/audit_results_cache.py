#!/usr/bin/env python3
"""Forensic comparison of saved grades against uploaded/cached final box scores.
Default is report-only. --apply corrects derived results with original backups.
Unstamped cached rows never acquire an invented API download timestamp.
"""
import argparse
import csv
import hashlib
import json
import math
import re
import shutil
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path

PARTS = {'points':('pts',),'rebounds':('reb',),'assists':('ast',),'threes':('fg3m',),
         'pra':('pts','reb','ast'),'pr':('pts','reb'),'pa':('pts','ast'),'ra':('reb','ast')}

def read(path):
    with path.open(encoding='utf-8-sig') as handle:
        for number, line in enumerate(handle, 1):
            if line.strip():
                yield number, json.loads(line.lstrip('\ufeff'))

def norm(name):
    return ''.join(c for c in str(name).casefold() if c.isalnum())

def minutes(value):
    try:
        fields = str(value).split(':')
        return float(fields[0]) + (float(fields[1])/60 if len(fields)>1 else 0)
    except (TypeError,ValueError):
        return 0

def result(e, actual):
    if actual == e['line']: return 'PUSH'
    return 'WIN' if (actual>e['line']) == (e['side']=='OVER') else 'LOSS'

def payout(odds):
    return odds/100 if odds>0 else 100/abs(odds or -110)

def compare(entry, boxes):
    row = boxes.get((entry.get('game_id'),entry.get('player_id')))
    if entry.get('period') not in (None,'','game','full_game','full game','full',0,'0'):
        return None,'partial-period pick requires its own period actuals',row
    if not isinstance(entry.get('line'),(int,float)) or not math.isfinite(entry['line']) or entry['line']<0:
        return None,'invalid quoted threshold',row
    if entry.get('stat') not in PARTS or entry.get('side') not in ('OVER','UNDER'):
        return None,'unsupported stat or side',row
    if not row: return None,'exact player/game row missing from supplied cache',None
    if str(row['status']).strip().casefold() != 'final': return None,'cached row is not explicitly final',row
    if entry.get('date') != row['date']: return None,'game date mismatch',row
    name = entry.get('player','')
    repaired = False
    if norm(name) != norm(row['player']):
        try:
            fixed_name = name.encode('latin1').decode('utf-8')
        except (UnicodeError,AttributeError):
            fixed_name = ''
        if norm(fixed_name) != norm(row['player']): return None,'player name / player ID mismatch',row
        repaired = True
    if not row['team_id'] or row['team_id'] not in (row['home'],row['away']): return None,'actual team is not in this game',row
    if entry.get('team') and entry['team'].casefold()!=row['team'].casefold(): return None,'logged team differs from final row team',row
    if row['minutes']<=0: return None,'participation not confirmed; missing/zero minutes is not an automatic void',row
    values = [row.get(k) for k in PARTS[entry['stat']]]
    if any(v is None or not isinstance(v,(int,float)) or not math.isfinite(v) or v<0 or int(v)!=v for v in values):
        return None,'requested stat/component missing or invalid',row
    if all(row.get(k) is not None for k in ('reb','oreb','dreb')) and row['reb']!=row['oreb']+row['dreb']:
        return None,'rebound total does not equal offensive plus defensive rebounds',row
    return sum(values),'matched cached final player/game/stat; '+('reversible UTF-8 name corruption confirmed against source; ' if repaired else '')+'API fetch time may be unavailable',row

def time_value(value):
    try:
        dt=datetime.fromisoformat(value.replace('Z','+00:00'))
        return dt.timestamp() if dt.tzinfo else None
    except (ValueError,TypeError,AttributeError):return None

def run(directory, apply=False):
    root=Path(directory)
    active=list(root.glob('stats_*.jsonl'))
    backups=list(root.glob('stats_*.jsonl.before-result-refresh-*'))
    fallback=[]
    for file in backups:
        target=root/file.name.split('.before-result-refresh-',1)[0]
        if target.exists(): continue
        if not any(p.name.startswith(target.name+'.before-result-refresh-') and p.name>file.name for p in backups):
            fallback.append(file)
    sources=sorted(active+fallback)
    boxes={}; source_summary=[]
    for file in sources:
        count=0
        for line,r in read(file):
            count+=1
            if r.get('period') not in (None,0,'0'): continue
            player=r.get('player') or {};game=r.get('game') or {};team=r.get('team') or {}
            if not player.get('id') or not game.get('id'): continue
            raw=json.dumps(r,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
            box=dict(game_id=game['id'],player_id=player['id'],player=(player.get('first_name','')+' '+player.get('last_name','')).strip(),
                     date=str(game.get('date',''))[:10],team_id=team.get('id'),team=team.get('abbreviation',''),home=game.get('home_team_id'),away=game.get('visitor_team_id'),
                     status=game.get('status',''),minutes=minutes(r.get('min')),tip=game.get('datetime'),source=file.name,sourceLine=line,
                     sourceHash=hashlib.sha256(raw).hexdigest(),sourceFetchedAt=r.get('_stats_fetched_at',''))
            for key in ('pts','reb','ast','fg3m','oreb','dreb'): box[key]=r.get(key)
            key=(game['id'],player['id']);old=boxes.get(key)
            if old and old['status'].casefold()=='final' and box['status'].casefold()!='final': continue
            boxes[key]=box
        source_summary.append(dict(file=file.name,rows=count,fallback=file in fallback))
    excluded={}
    registry=root/'exclusions.jsonl'
    if registry.exists():
        for _,r in read(registry):excluded[r.get('id')]=r.get('reason','excluded quote')
    new_exclusions={}
    evidence=[]; reports={}
    now=datetime.now(timezone.utc).isoformat().replace('+00:00','Z')
    for name in ('settled.jsonl','combosettled.jsonl','v2settled.jsonl','v2_rotationsettled.jsonl'):
        file=root/name
        if not file.exists():continue
        fixed=[];counts=Counter();checks=[]
        for line,e in read(file):
            actual,reason,row=compare(e,boxes)
            new_result=result(e,actual) if actual is not None else 'REVIEW'
            old_time=time_value(e.get('source_fetched_at'))
            cache_time=time_value(row.get('sourceFetchedAt')) if row else None
            keep_existing=old_time is not None and (cache_time is None or cache_time<old_time) and e.get('result') in ('WIN','LOSS','PUSH','REVIEW')
            if keep_existing:
                new_result=e['result'];actual=e.get('actual') if new_result in ('WIN','LOSS','PUSH') else None
                reason='retained newer recorded source evidence; older/undated cache was not used to overwrite it'
            before=e.get('result','');previous=e.get('actual')
            changed=(new_result!=before or actual is not None and actual!=previous)
            counts['checked']+=1;counts['review' if actual is None else 'confirmed']+=1
            counts['outcome_changed']+=int(new_result!=before)
            counts['actual_changed']+=int(actual is not None and actual!=previous)
            counts['win_to_loss']+=int(before=='WIN' and new_result=='LOSS')
            counts['loss_to_win']+=int(before=='LOSS' and new_result=='WIN')
            qid=f"{e.get('date')}|{e.get('player_id')}|{e.get('stat')}|{e.get('side')}|{e.get('book')}|{float(e.get('line',0)):.1f}"
            tip_value=e.get('tip') or (row.get('tip') if row else None)
            log_time=time_value(e.get('logged_at'));tip_time=time_value(tip_value)
            after_tip=log_time is not None and tip_time is not None and log_time>=tip_time and not keep_existing
            excluded_reason=excluded.get(qid,'')
            if after_tip:
                counts['logged_after_tip']+=1
                excluded_reason='pick logged at/after recorded scheduled tip; final-cache schedule comparison'
                new_exclusions[qid]=dict(id=qid,reason=excluded_reason,audited_at=now)
            old=minimal=dict(log=name,lineNumber=line,date=e.get('date'),player=e.get('player'),playerId=e.get('player_id'),gameId=e.get('game_id'),
                          stat=e.get('stat'),side=e.get('side'),betLine=e.get('line'),tier=e.get('tier','PICK'),book=e.get('book'),marketPeriod=e.get('period','unrecorded'),quoteProvenanceVersion=e.get('audit_version',0),quoteExcludedReason=excluded_reason,quoteTiming='at_or_after_tip' if after_tip else 'before_recorded_tip' if log_time is not None and tip_time is not None else 'unknown',
                          oldResult=before,oldActual=previous,newResult=new_result,newActual=actual,changed=changed,reason=reason,
                          sourceFile=row['source'] if row else '',sourceLine=row['sourceLine'] if row else '',sourceHash=row['sourceHash'] if row else '',
                          sourceFetchedAt=row['sourceFetchedAt'] if row else '',loggedAt=e.get('logged_at'),originalSettledAt=e.get('settled_at'),tip=row.get('tip') if row else None)
            evidence.append(minimal)
            receipt=dict(id='|'.join(str(e.get(k,'')) for k in ('date','player_id','stat','side','book','line')),player=e.get('player'),date=e.get('date'),
                         playerId=e.get('player_id'),gameId=e.get('game_id'),stat=e.get('stat'),side=e.get('side'),line=e.get('line'),oldResult=before,oldActual=previous,
                         newResult=new_result,reason=reason)
            if actual is not None:receipt['newActual']=actual
            checks.append(receipt)
            updated=dict(e);updated.update(result=new_result,actual=actual if actual is not None else 0,profit=0,flat=0,
                 settlement_version=1 if actual is not None else 0,settlement_source='Uploaded cached final box score; not a fresh API verification',
                 settlement_reason=reason,source_fetched_at=row['sourceFetchedAt'] if row else '',source_hash=row['sourceHash'] if row else '',
                 source_player_id=row['player_id'] if row else 0,source_game_id=row['game_id'] if row else 0,source_team_id=row['team_id'] if row else 0,
                 source_player=row['player'] if row else '',source_date=row['date'] if row else '',settled_at=now)
            if actual is not None:
                win=payout(e.get('odds',-110));stake=e.get('units',0) or 1
                updated['flat']=win if new_result=='WIN' else -1 if new_result=='LOSS' else 0
                updated['profit']=stake*updated['flat']
            fixed.append(dict(e) if keep_existing else updated)
        reports[name]=dict(counts)
        if apply:
            backup=file.with_name(file.name+'.before-cache-audit-'+now.replace(':','').replace('.',''))
            shutil.copy2(file,backup)
            temp=file.with_name(file.name+'.cache-audit.tmp')
            temp.write_text(''.join(json.dumps(e,ensure_ascii=False,separators=(',',':'))+'\n' for e in fixed),encoding='utf-8')
            temp.replace(file)
            report=dict(at=now,log=name,checked=counts['checked'],confirmed=counts['confirmed'],review=counts['review'],staleSources=0,
                        checks=checks,changes=[c for c,x in zip(checks,evidence[-len(checks):]) if x['changed']],history=checks,
                        sourceMode='forensic uploaded cache; do not infer a fresh API timestamp')
            (root/(file.stem+'_audit.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
    if apply and new_exclusions:
        registry=root/'exclusions.jsonl'
        prior={r['id']:r for _,r in read(registry)} if registry.exists() else {}
        prior.update(new_exclusions)
        temporary=registry.with_name(registry.name+'.cache-audit.tmp')
        temporary.write_text(''.join(json.dumps(r,separators=(',',':'),ensure_ascii=False)+'\n' for r in prior.values()),encoding='utf-8')
        temporary.replace(registry)
    out=root/'cache_result_audit'
    out.mkdir(exist_ok=True)
    with (out/'all_results.csv').open('w',encoding='utf-8-sig',newline='') as f:
        w=csv.DictWriter(f,fieldnames=list(evidence[0]) if evidence else []);w.writeheader();w.writerows(evidence)
    with (out/'changed_results.csv').open('w',encoding='utf-8-sig',newline='') as f:
        w=csv.DictWriter(f,fieldnames=list(evidence[0]) if evidence else []);w.writeheader();w.writerows(e for e in evidence if e['changed'])
    summary=dict(at=now,sourceMode='uploaded cached final rows; no fresh API verification and no certification of legacy quote periods/prices',sources=source_summary,missingActiveStats=[p.name.split('.before-result-refresh-')[0] for p in fallback],logs=reports)
    (out/'summary.json').write_text(json.dumps(summary,indent=2),encoding='utf-8')
    print(json.dumps(summary,indent=2))
    return evidence,summary

if __name__=='__main__':
    ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--data',default='data');ap.add_argument('--apply',action='store_true');a=ap.parse_args();run(a.data,a.apply)
