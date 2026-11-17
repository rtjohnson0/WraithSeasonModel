#!/usr/bin/env python3
"""Audit saved picks against BDL snapshots. Dry run unless --apply is supplied.
Explicit partial-period rows are moved to timestamped quarantine files, with
backups of every changed log. Ambiguous/unmatched legacy rows are only reported.
Run with the daily pipeline stopped, then re-export the site data.
"""
import argparse
import json
import re
import shutil
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path

STATS = {'points', 'rebounds', 'assists', 'threes', 'pra', 'pr', 'pa', 'ra'}
ALIASES = {'points_rebounds_assists': 'pra', 'points_rebounds': 'pr',
           'points_assists': 'pa', 'rebounds_assists': 'ra'}
FULL = {'', 'game', 'full_game', 'full game', 'full', '0'}
LOGS = ['picklog.jsonl', 'settled.jsonl', 'combolog.jsonl', 'combosettled.jsonl',
        'v2log.jsonl', 'v2settled.jsonl','v2_rotationlog.jsonl','v2_rotationsettled.jsonl']


def partial_meta(row):
    return any(str(row[k]).strip().lower() not in FULL
               for k in ('period', 'period_id', 'periodID')
               if row.get(k) is not None)


def base_stat(value):
    # Never infer a period from a low line. Only explicit provider labels count.
    base = re.sub(r'_(?:[1-4]q|[12]h|first\d+min)$', '', value)
    return ALIASES.get(base, base)


def signature(row, fallback=0):
    return (int(row.get('game_id') or fallback), int(row.get('player_id') or 0),
            str(row.get('vendor', row.get('book', ''))).lower(),
            base_stat(row.get('prop_type', row.get('stat', ''))),
            float(row.get('line_value', row.get('line'))))


def read_rows(path):
    # Fail before any write if a file is malformed. Do not silently discard rows.
    return [(line, json.loads(line.lstrip("\ufeff"))) for line in path.read_text(encoding="utf-8-sig").splitlines(True)
            if line.strip()]


def evidence(directory):
    found = defaultdict(set)
    for path in sorted((directory / 'snapshots').glob('props_*.jsonl')):
        for _, snap in read_rows(path):
            for p in snap.get('response', {}).get('data', []):
                if p.get('market', {}).get('type') != 'over_under':
                    continue
                stat = p.get('prop_type', '')
                if base_stat(stat) not in STATS:
                    continue
                is_partial = (base_stat(stat) != ALIASES.get(stat, stat)
                              or partial_meta(p) or partial_meta(p.get('market', {})))
                found[signature(p, snap.get('game_id', 0))].add(
                    'partial' if is_partial else 'full')
    return found


def classify(row, found):
    if partial_meta(row) or row.get('stat') not in STATS:
        return 'exclude'
    matches = found.get(signature(row), set())
    if matches == {'partial'}:
        return 'suspected'
    if matches == {'full', 'partial'}:
        return 'ambiguous'
    if matches == {'full'}:
        return 'full'
    return 'unmatched'


def audit(directory, apply=False):
    found = evidence(directory)
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    plans, report = [], []
    for name in LOGS:
        path = directory / name
        if not path.exists():
            continue
        keep, excluded = [], []
        counts = defaultdict(int)
        for line, row in read_rows(path):
            status = classify(row, found)
            counts[status] += 1
            if status == 'exclude':
                excluded.append(line)
            else:
                keep.append(line)
            if status != 'full':
                report.append({'file': name, 'status': status, 'entry': row})
        print(f'{name}: ' + ', '.join(f'{k}={v}' for k, v in sorted(counts.items())))
        if excluded:
            plans.append((path, keep, excluded))
    if apply:
        # Back up every input before the first replacement. No WIN/LOSS is regraded.
        for path, _, _ in plans:
            shutil.copy2(path, path.with_name(path.name + '.' + stamp + '.bak'))
        for path, keep, excluded in plans:
            quarantine = path.with_name(path.name + '.' + stamp + '.excluded')
            quarantine.write_text(''.join(excluded), encoding='utf-8')
            temp = path.with_name(path.name + '.' + stamp + '.tmp')
            temp.write_text(''.join(keep), encoding='utf-8')
            temp.replace(path)
        report_path = directory / ('period_audit_' + stamp + '.json')
        report_path.write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
        print(f'Applied; backups, quarantine and audit report saved in {directory}')
    else:
        for item in report:
            if item['status'] in {'exclude', 'ambiguous', 'suspected'}:
                r = item['entry']
                print(item['status'], item['file'], r.get('date'), r.get('player'),
                      r.get('stat'), r.get('side'), r.get('line'), r.get('book'))
        print('Dry run. Use --apply to quarantine explicit partial markets.')
    print('Suspected, ambiguous and unmatched entries remain unchanged and need review; '
          'this audit cannot prove the period when the source metadata is missing.')
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--data', type=Path, default=Path('data'))
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    if not args.data.is_dir():
        parser.error('data directory does not exist')
    audit(args.data, args.apply)
