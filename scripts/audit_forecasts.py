"""Diagnose saved forecasts against exact cached finals; never tune or alter results.
The saved forecasts are used as recorded. No features are rebuilt from future data.
"""
import argparse
import csv
import html
import json
import math
from collections import Counter, defaultdict
from pathlib import Path
from statistics import mean
from audit_results_cache import read, norm, minutes, time_value, compare, PARTS, payout


def load_boxes(directory):
    boxes = {}
    for path in sorted(directory.glob('stats_*.jsonl')):
        for _, r in read(path):
            if r.get('period') not in (None, 0, '0'):
                continue
            player, game, team = r.get('player') or {}, r.get('game') or {}, r.get('team') or {}
            k = game.get('id'), player.get('id')
            if not all(k):
                continue
            fetched = time_value(r.get('_stats_fetched_at'))
            previous = boxes.get(k)
            if previous and previous['fetched'] is not None and (fetched is None or fetched < previous['fetched']):
                continue
            boxes[k] = dict(game_id=k[0], player_id=k[1], player=(player.get('first_name', '')+' '+player.get('last_name', '')).strip(),
                date=str(game.get('date', ''))[:10], team_id=team.get('id'), team=team.get('abbreviation', ''),
                home=game.get('home_team_id'), away=game.get('visitor_team_id'), status=game.get('status', ''),
                minutes=minutes(r.get('min')), tip=game.get('datetime'), fetched=fetched, endpoint=r.get('_stats_endpoint'),
                source=path.name, **{s: r.get(s) for s in ('pts','reb','ast','fg3m','oreb','dreb')})
    return boxes


def timing(entry, box, field):
    at = time_value(entry.get(field))
    tip = time_value(box.get('tip')) if box else None
    return at is not None and tip is not None and at < tip


def summary(rows):
    out = {'n': len(rows), 'games': len({r['game'] for r in rows}), 'dates': len({r['date'] for r in rows})}
    if not rows:
        return out
    for key in ('actual_minus_projected', 'absolute_error', 'minutes_error', 'minutes_absolute_error', 'minutes_component', 'rate_component', 'typical_absolute_error'):
        if key in rows[0]:
            out[key] = mean(r[key] for r in rows)
    if 'probability' in rows[0]:
        nonpush = [r for r in rows if r['result'] != 'PUSH']
        out.update(nonpush=len(nonpush), pushes=len(rows)-len(nonpush))
        if nonpush:
            out.update(predicted=mean(r['probability'] for r in nonpush), hit_rate=mean(r['result']=='WIN' for r in nonpush),
                       break_even=mean(r['break_even'] for r in nonpush), flat_roi=mean(r['profit'] for r in nonpush),
                       brier=mean((r['probability']-(r['result']=='WIN'))**2 for r in nonpush))
    return out


def quote_provenance(e):
    at, captured, updated = [time_value(e.get(k)) for k in ('logged_at','quote_at','source_updated_at')]
    if e.get('audit_version', 0) < 2 or e.get('period') not in ('game','full_game') or e.get('book_count',0) < 3:
        return False
    return all(t is not None for t in (at,captured,updated)) and 0 <= at-captured <= 1800 and 0 <= at-updated <= 21600


def run(directory, output):
    directory, output = Path(directory), Path(output)
    boxes = load_boxes(directory)
    exclusions = Counter()
    forecasts = {}
    for _, p in read(directory/'projlog.jsonl'):
        box = boxes.get((p.get('game_id'),p.get('player_id')))
        if not timing(p, box, 'at'):
            exclusions['projection_missing_final_or_not_pregame'] += 1
            continue
        key = p.get('date'),p.get('game_id'),p.get('player_id')
        if key not in forecasts or p['at'] < forecasts[key]['at']:
            forecasts[key] = p
    projection_rows, minutes_rows = [], []
    for p in forecasts.values():
        box = boxes[(p['game_id'],p['player_id'])]
        pm, actualm, typical = p.get('proj_min',0), box['minutes'], p.get('typ_min',0)
        if pm <= 0 or actualm <= 0:
            exclusions['projection_participation_missing'] += 1
            continue
        accepted = False
        for stat, sp in p.get('stats',{}).items():
            e = dict(p, stat=stat, line=sp.get('line',0), side='OVER',period='game')
            actual, reason, _ = compare(e, boxes)
            if actual is None:
                exclusions['projection_'+reason] += 1
                continue
            pred = sp.get('proj')
            if not isinstance(pred,(float,int)) or not math.isfinite(pred):
                continue
            accepted = True
            mr = (actualm-pm)*pred/pm
            projection_rows.append(dict(date=p['date'],game=p['game_id'],player_id=p['player_id'],player=p['player'],stat=stat,
                predicted=pred,actual=actual,minutes=pm,actual_minutes=actualm,typical_minutes=typical,
                actual_minus_projected=actual-pred,absolute_error=abs(actual-pred),minutes_error=actualm-pm,
                minutes_absolute_error=abs(actualm-pm),minutes_component=mr,rate_component=actual-pred-mr,
                typical_absolute_error=abs(actual-pred*typical/pm) if typical>0 else abs(actual-pred)))
        if accepted:
            minutes_rows.append(dict(date=p['date'],game=p['game_id'],player=p['player'],predicted=pm,actual=actualm,
                actual_minus_projected=actualm-pm,absolute_error=abs(actualm-pm),typical_absolute_error=abs(actualm-typical) if typical>0 else abs(actualm-pm)))
    picks = {}
    for _, e in read(directory/'picklog.jsonl'):
        if e.get('tier') == 'LEAN':
            continue
        box = boxes.get((e.get('game_id'),e.get('player_id')))
        actual, reason, _ = compare(e,boxes)
        if actual is None:
            exclusions['pick_'+reason] += 1
            continue
        if not timing(e,box,'logged_at'):
            exclusions['pick_not_pregame_or_tip_missing'] += 1
            continue
        # Date/stat/player/game define one outcome; never select using its result.
        key = e['date'],e['game_id'],e['player_id'],e['stat'],quote_provenance(e)
        if key not in picks or e['logged_at'] < picks[key]['logged_at']:
            picks[key] = e
    pick_rows=[]
    for e in picks.values():
        actual,_,box = compare(e,boxes)
        odds=e.get('odds',0)
        if not isinstance(odds,int) or not (odds>=100 or odds<=-100):
            exclusions['pick_invalid_price']+=1
            continue
        p=e.get('prob',0)
        if not isinstance(p,(int,float)) or not 0<p<1:
            exclusions['pick_invalid_probability']+=1
            continue
        result='PUSH' if actual==e['line'] else 'WIN' if (actual>e['line'])==(e['side']=='OVER') else 'LOSS'
        pick_rows.append(dict(date=e['date'],game=e['game_id'],player_id=e['player_id'],player=e['player'],stat=e['stat'],side=e['side'],
            line=e['line'],actual=actual,probability=p,raw_probability=e.get('prob_raw'),odds=odds,
            break_even=100/(100+odds) if odds>0 else -odds/(100-odds),result=result,
            profit=0 if result=='PUSH' else payout(odds) if result=='WIN' else -1,
            quote_provenance=quote_provenance(e),market_period_proven=e.get('period') in ('game','full_game'),actual_source_stamped=box['fetched'] is not None and box['endpoint']=='/nba/v1/stats'))
    report=dict(note='Diagnostic use of saved pregame forecasts and exact cached final identities. Unknown legacy quote periods remain unproven; their apparent returns cannot establish model accuracy and may compare the wrong market period. Quote-provenance groups are separated. No model, result log, or probability is changed. A stat error is actual minus predicted. Rate/minutes error components are hindsight attribution, not replacement forecasts. One earliest pregame decision per player/game/stat; one minutes observation per player/game.',
                final_through=max((b['date'] for b in boxes.values() if b['status']=='Final'),default=''),
                projection_summary=summary(projection_rows),minutes_summary=summary(minutes_rows),exclusions=dict(exclusions),
                projection_by_stat={},projection_by_date={},picks_by_stat_side={},picks_by_probability={},picks_by_provenance={})
    for stat in ('points','rebounds','assists','threes'):
        report['projection_by_stat'][stat]=summary([r for r in projection_rows if r['stat']==stat])
        for side in ('OVER','UNDER'):
            report['picks_by_stat_side'][stat+'/'+side]=summary([r for r in pick_rows if r['stat']==stat and r['side']==side])
    for date in sorted({r['date'] for r in projection_rows}):
        report['projection_by_date'][date]=summary([r for r in projection_rows if r['date']==date])
    for lo,hi in ((0,.55),(.55,.60),(.60,.65),(.65,.70),(.70,1.01)):
        report['picks_by_probability'][f'{lo:.2f}–{hi:.2f}']=summary([r for r in pick_rows if lo<=r['probability']<hi])
    for name,keep in [('documented_quotes',lambda r:r['quote_provenance']),('legacy_quotes',lambda r:not r['quote_provenance'])]:
        report['picks_by_provenance'][name]=summary([r for r in pick_rows if keep(r)])
    output.mkdir(parents=True,exist_ok=True)
    (output/'summary.json').write_text(json.dumps(report,indent=2),encoding='utf-8')
    for name,rows in [('projections.csv',projection_rows),('minutes.csv',minutes_rows),('picks.csv',pick_rows)]:
        with (output/name).open('w',newline='',encoding='utf-8') as f:
            if rows:
                w=csv.DictWriter(f,fieldnames=list(rows[0]));w.writeheader();w.writerows(rows)
    esc=lambda s:html.escape(str(s))
    document='<!doctype html><html lang="en"><meta charset="utf-8"><title>Saved forecast audit</title><style>body{font:16px system-ui;max-width:1100px;margin:30px auto;padding:20px;line-height:1.6;background:#110e19;color:#eee}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#211a30;padding:20px}h2{margin-top:35px}</style><h1>Saved forecast audit</h1><p>'+esc(report['note'])+'</p>'
    for key,value in report.items():
        if key!='note':document+='<h2>'+esc(key.replace('_',' ').title())+'</h2><pre>'+esc(json.dumps(value,indent=2))+'</pre>'
    (output/'report.html').write_text(document+'</html>',encoding='utf-8')
    return report

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--data',default='data');parser.add_argument('--out',default='forecast_audit');args=parser.parse_args()
    report=run(args.data,args.out)
    print(json.dumps({k:report[k] for k in ('final_through','projection_summary','minutes_summary','projection_by_stat','picks_by_provenance','picks_by_probability')},indent=2))
