#!/usr/bin/env python3
"""Audit quote provenance without inventing historical odds or changing outcomes.
--apply writes exclusions only for explicit period, post-tip, or raw SGO pairing
failures. Missing evidence stays unverified; large line gaps alone are not proof.
"""
import argparse
import json
import re
from datetime import datetime, timezone
from pathlib import Path

STATS={'points','rebounds','assists','threes','pra','pr','pa','ra'}
SGO_STATS={'threePointersMade':'threes','points+rebounds':'pr','points+assists':'pa',
           'rebounds+assists':'ra','points+rebounds+assists':'pra'}
FULL={'','game','full_game','full game','full','0'}
LOGS=['picklog.jsonl','settled.jsonl','combolog.jsonl','combosettled.jsonl','v2log.jsonl','v2settled.jsonl']


def read(path):
    if not path.exists(): return []
    return [json.loads(x.lstrip('\ufeff')) for x in path.read_text(encoding="utf-8-sig").splitlines() if x.strip()]


def timestamp(value):
    try:
        at=datetime.fromisoformat(value.replace('Z','+00:00'))
        return at if at.tzinfo else None
    except (ValueError,TypeError,AttributeError):return None


def norm(name):
    return ' '.join(x for x in re.sub(r"[^a-z0-9 ]",' ',name.lower()).split()
                    if x not in {'jr','sr','ii','iii','iv','v'})


def identity(row):
    return f"{row.get('date','')}|{row.get('player_id',0)}|{row.get('stat','')}|{row.get('side','')}|{row.get('book','')}|{float(row.get('line',0)):.1f}"


def raw_evidence(directory):
    evidence={}
    for path in (directory/'snapshots').glob('sgo_raw_*.jsonl'):
        for snap in read(path):
            captured=timestamp(snap.get('captured_at'))
            event=snap.get('event',{});odds=event.get('odds',{})
            for oid,over in odds.items():
                parts=oid.split('-')
                if len(parts)!=5 or parts[2:]!=['game','ou','over']:continue
                stat=SGO_STATS.get(parts[0],parts[0])
                if stat not in STATS:continue
                name=event.get('players',{}).get(parts[1],{}).get('name')
                if not name:name=' '.join(parts[1].split('_')[:-2])
                under=odds.get('-'.join(parts[:4])+ '-under',{})
                for book,o in over.get('byBookmaker',{}).items():
                    u=under.get('byBookmaker',{}).get(book)
                    if not u:continue
                    try:line=float(o['overUnder']);under_line=float(u['overUnder'])
                    except (KeyError,ValueError,TypeError):continue
                    for side,price in [('OVER',o.get('odds')),('UNDER',u.get('odds'))]:
                        try:price=int(price)
                        except (ValueError,TypeError):continue
                        # Mirrors the OLD parser: Under price attached to Over threshold.
                        key=(norm(name),book.lower(),stat,line,side,price)
                        reason=None
                        if line!=under_line:reason=f'raw SGO Over line {line} differs from Under line {under_line}'
                        elif u.get('available') is False or o.get('available') is False:reason='raw SGO selection unavailable'
                        evidence.setdefault(key,[]).append((captured,reason,str(path)))
    return evidence


def audit(directory,apply=False):
    evidence=raw_evidence(directory);report=[];exclude={}
    for name in LOGS:
        for row in read(directory/name):
            reason=None
            if row.get('stat') not in STATS or any(str(row[k]).strip().lower() not in FULL
                for k in ('period','period_id','periodID') if row.get(k) is not None):
                reason='explicit partial period or unsupported stat'
            tip=timestamp(row.get('tip'));logged=timestamp(row.get('logged_at'));quote=timestamp(row.get('quote_at'))
            if tip and ((logged and logged>=tip) or (quote and quote>=tip)):
                reason='pick or quote captured at/after tip-off'
            book=row.get('book','')
            if not reason and book.startswith('sgo/'):
                key=(norm(row.get('player','')),book[4:].lower(),row.get('stat'),
                     float(row.get('line',0)),row.get('side'),row.get('odds'))
                # Require the precise recorded capture time; matching a nearby market
                # does not prove it was the source of a legacy pick.
                matches=[e for e in evidence.get(key,[]) if quote and e[0]==quote]
                if matches and all(e[1] for e in matches):reason=matches[-1][1]
            status='exclude' if reason else ('verified' if row.get('audit_version',0)>=2 else 'unverified')
            entry={'file':name,'id':identity(row),'status':status,'reason':reason,'entry':row}
            report.append(entry)
            if reason:exclude[entry['id']]={'id':entry['id'],'reason':reason,'audited_at':datetime.now(timezone.utc).isoformat()}
    for status in ('verified','unverified','exclude'):
        print(status,sum(r['status']==status for r in report))
    stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    report_path=directory/f'quote_audit_{stamp}.json'
    report_path.write_text(json.dumps(report,indent=2)+'\n', encoding='utf-8')
    if apply:
        path=directory/'exclusions.jsonl'
        existing={r['id']:r for r in read(path)}
        existing.update(exclude)
        if path.exists():path.with_name(path.name+'.'+stamp+'.bak').write_bytes(path.read_bytes())
        temp=path.with_suffix('.tmp')
        temp.write_text(''.join(json.dumps(r,separators=(',',':'))+'\n' for r in existing.values()), encoding='utf-8')
        temp.replace(path)
    print(f"Audit saved: {report_path}. Original pick and settlement rows are retained.")
    return report


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--data',type=Path,default=Path('data'));p.add_argument('--apply',action='store_true')
    args=p.parse_args()
    if not args.data.is_dir():p.error('data directory does not exist')
    audit(args.data,args.apply)
