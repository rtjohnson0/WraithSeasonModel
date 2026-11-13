#!/usr/bin/env python3
"""Merge generated state with remote updates and push without rebasing or force pushing.
JSONL conflicts are resolved by record identity with three-way deletion handling.
Remote edits to model/calibration files win over a concurrent generated copy.
Unrelated source conflicts stop with the merge aborted; generated commits remain local.
"""
import argparse
import json
import os
import re
from datetime import datetime
import subprocess
import time
from pathlib import Path

STATE_FILES = [
    'v2_rotationlog.jsonl', 'v2_rotationsettled.jsonl', 'v2_rotationsettled_audit.json',
    'settled_audit.json', 'combosettled_audit.json', 'v2settled_audit.json', 'side_audit.json', 'accuracy.json', 'quarterboard.json', 'quarterlog.jsonl', 'gameboard.json', 'board.json', 'picklog.jsonl', 'settled.jsonl', 'calibration.json', 'gamecal.json',
    'biasfix.json', 'minfix.json', 'wofix.json', 'runs.jsonl', 'more.json',
    'projlog.jsonl', 'model.json', 'combolog.jsonl', 'combosettled.jsonl',
    'combocorr.json', 'v2log.jsonl', 'v2settled.jsonl', 'v2.json', 'model_v2.json',
    'gamepicklog.jsonl', 'gamesettled.jsonl', 'combohealth.json', 'combocalibration.json', 'exclusions.jsonl', 'model_tail.json', 'calibration_tail.json',
]
MODEL_BUNDLE = ['data/' + x for x in ('model.json', 'model_v2.json', 'calibration.json',
                'combocalibration.json', 'gamecal.json', 'biasfix.json', 'minfix.json',
                'wofix.json', 'combocorr.json')]

TAIL_BUNDLE = ['data/model_tail.json', 'data/calibration_tail.json']

def git(*args, check=True):
    result = subprocess.run(['git', *args], text=True, encoding='utf-8', capture_output=True)
    if check and result.returncode:
        # Never print a remote URL: credentials can be embedded in it.
        raise RuntimeError('git ' + args[0] + ' failed: ' + result.stderr[-2000:])
    return result


def blob(ref, name):
    result = git('show', f'{ref}:{name}', check=False)
    return None if result.returncode else result.stdout


def is_input(name):
    # Some existing checkouts already track cached API inputs. Preserve those
    # updates, but do not force-add new raw downloads to the repository.
    return bool(re.fullmatch(
        r'data/(?:stats_\d+\.jsonl|advanced_\d+\.jsonl|players_active\.jsonl|'
        r'opening_(?:props|games|odds)_\d+\.jsonl|snapshots/[^\n]+\.jsonl)', name))


def record_key(name, row):
    if is_input(name):
        if name.startswith(('data/stats_', 'data/advanced_')):
            player = row.get('player_id', (row.get('player') or {}).get('id'))
            game = row.get('game_id', (row.get('game') or {}).get('id'))
            if player is None or game is None:
                raise ValueError(f'{name}: input row needs player and game identifiers')
            return (player, game, (row.get('period') or 0))
        if name == 'data/players_active.jsonl':
            if row.get('id') is None:
                raise ValueError(f'{name}: player row needs an id')
            return row['id']
        # Snapshot/opening rows can hold a whole response. Keep distinct captures
        # intact rather than applying the pick-log identity to unrelated rows.
        return json.dumps(row, sort_keys=True, separators=(',', ':'))

    if name.endswith('projlog.jsonl') or '/lines/market_' in name:
        return (row.get('date'), row.get('game_id', row.get('game')),
                row.get('player_id', row.get('pid')), row.get('stat'))
    if name.endswith('runs.jsonl'):
        return json.dumps(row, sort_keys=True, separators=(',', ':'))
    if name.endswith('exclusions.jsonl'):
        return row.get('id', json.dumps(row, sort_keys=True))
    if name.endswith(('gamepicklog.jsonl', 'gamesettled.jsonl')):
        return (row.get('date'), row.get('game_id'), row.get('market'))
    return (row.get('date'), row.get('game_id', row.get('game')),
            row.get('player_id', row.get('pid')), row.get('stat'), row.get('side'),
            row.get('book'), row.get('line'), row.get('period', 'game'))


def prefer_input(older, newer, field='_stats_fetched_at'):
    def stamp(row):
        try:
            value = datetime.fromisoformat(row.get(field, '').replace('Z', '+00:00'))
            return value.timestamp() if value.tzinfo else None
        except (ValueError, TypeError):
            return None
    a, b = stamp(older), stamp(newer)
    if a is not None and (b is None or a > b):
        return older
    return newer


def records(name, content):
    result = {}
    for line in (content or '').splitlines():
        if not line.strip():
            continue
        row = json.loads(line.lstrip('\ufeff'))
        if not isinstance(row, dict):
            raise ValueError(f'{name}: expected a JSON object')
        key = record_key(name, row)
        prev = result.get(key)
        if prev is None:
            result[key] = row
        elif is_input(name):
            result[key] = prefer_input(prev, row)
        elif name.endswith('projlog.jsonl') or '/lines/market_' in name:
            if row.get('at', '') >= prev.get('at', ''):
                result[key] = row
        elif row.get('logged_at', '') < prev.get('logged_at', ''):
            result[key] = row
    return result


def merge_jsonl(name, base, ours, theirs):
    b, o, t = (records(name, x) for x in (base, ours, theirs))
    merged = []
    for key in dict.fromkeys([*t, *o, *b]):
        bv, ov, tv = b.get(key), o.get(key), t.get(key)
        if ov == bv:
            value = tv
        elif tv == bv:
            value = ov
        elif ov is None or tv is None:
            # A deliberate deletion of an existing record must not be resurrected.
            value = None if bv is not None else (tv or ov)
        elif ov == tv:
            value = ov
        elif is_input(name):
            value = prefer_input(ov, tv)
        elif name.endswith('gamesettled.jsonl'):
            value = prefer_input(ov, tv, field='sourceAt')
        elif name.endswith('gamepicklog.jsonl'):
            value = min((ov, tv), key=lambda r: r.get('at', ''))
        elif name.endswith('settled.jsonl'):
            value = prefer_input(ov, tv, field='source_fetched_at')
        elif name.endswith('projlog.jsonl') or '/lines/market_' in name:
            value = max((ov, tv), key=lambda r: r.get('at', ''))
        else:
            # Remote corrections win; identical/new append-only records are unioned.
            value = tv
        if value is not None:
            merged.append(value)
    merged.sort(key=lambda r: (r.get('date', ''), str(record_key(name, r))))
    return ''.join(json.dumps(r, separators=(',', ':'), ensure_ascii=False) + '\n' for r in merged)


def merge_result_audit(local, upstream):
    a, b = json.loads(local), json.loads(upstream)
    newest = dict(a if a.get('at', '') > b.get('at', '') else b)
    history = {}
    for report in (b, a):
        for change in report.get('history', report.get('changes', [])):
            history[json.dumps(change, sort_keys=True, ensure_ascii=False)] = change
    newest['history'] = list(history.values())[-2000:]
    return json.dumps(newest, ensure_ascii=False, indent=2)


def state_paths():
    names = {'data/' + x for x in STATE_FILES}
    names.update(str(p).replace(os.sep, '/') for p in Path('data/lines').glob('*.jsonl'))
    names.update(str(p).replace(os.sep, '/') for p in Path('data/quarter_games').glob('*.json'))
    names.update(p for p in git('ls-files', '-z', '--', 'data').stdout.split('\0') if is_input(p))
    return sorted(names)


def is_state(name):
    return is_input(name) or name in {'data/' + x for x in STATE_FILES} or (
        name.startswith('data/lines/') and name.endswith('.jsonl')) or (
        name.startswith('data/quarter_games/') and name.endswith('.json'))


def save_initial(message):
    paths = [p for p in state_paths() if Path(p).is_file()]
    for p in paths:
        # Stage existing paths one at a time; one absent optional file cannot skip the whole batch.
        git('add', '-f', '--', p)
    for p in git('ls-files', '--deleted', '-z').stdout.split('\0'):
        if p and is_state(p):
            git('add', '-u', '--', p)
    staged = [p for p in git('diff', '--cached', '--name-only').stdout.splitlines() if p]
    if any(not is_state(p) for p in staged):
        raise RuntimeError('Non-state changes are staged. Commit source changes first.')
    if staged:
        git('commit', '-m', message)
    # Generated tracked site data is rebuilt after synchronization, never stashed across a merge.
    exported = 'site/public/data/picks.json'
    if git('ls-files', '--error-unmatch', exported, check=False).returncode == 0:
        git('restore', '--', exported)
    dirty = git('diff', '--name-only').stdout.splitlines()
    if dirty:
        raise RuntimeError('Uncommitted tracked changes remain: ' + ', '.join(dirty))


def sync(branch, message, attempts=5, delay=2):
    git('check-ref-format', '--branch', branch)
    if git('diff', '--name-only', '--diff-filter=U').stdout.strip():
        raise RuntimeError('An unresolved merge is already in progress.')
    for kind in ('MERGE_HEAD', 'rebase-merge', 'rebase-apply'):
        location = Path(git('rev-parse', '--git-path', kind).stdout.strip())
        if location.exists():
            raise RuntimeError('Finish or abort the existing merge/rebase first.')
    save_initial(message)
    for attempt in range(attempts):
        git('fetch', 'origin', f'+refs/heads/{branch}:refs/remotes/origin/{branch}')
        remote = git('rev-parse', f'refs/remotes/origin/{branch}').stdout.strip()
        head = git('rev-parse', 'HEAD').stdout.strip()
        base = git('merge-base', head, remote).stdout.strip()
        merged = git('merge', '--no-commit', '--no-ff', remote, check=False)
        merge_head = Path(git('rev-parse', '--git-path', 'MERGE_HEAD').stdout.strip())
        try:
            conflicted = git('diff', '--name-only', '--diff-filter=U').stdout.splitlines()
            if any(not is_state(p) for p in conflicted):
                raise RuntimeError('Source conflict needs review: ' + ', '.join(p for p in conflicted if not is_state(p)))
            if merged.returncode and not conflicted:
                raise RuntimeError('Merge failed without resolvable state conflicts.')
            for name in conflicted:
                before, local, upstream = blob(base, name), blob(head, name), blob(remote, name)
                path = Path(name)
                if name.endswith('.jsonl'):
                    content = merge_jsonl(name, before, local, upstream)
                else:
                    # Remote explicitly changed configuration; keep that change.
                    content = upstream
                    if (name.startswith('data/quarter_games/') or name in {'data/settled_audit.json', 'data/combosettled_audit.json', 'data/v2settled_audit.json', 'data/side_audit.json', 'data/accuracy.json', 'data/quarterboard.json', 'data/gameboard.json', 'data/board.json', 'data/more.json', 'data/v2.json'}) and local and upstream:
                        a, b = json.loads(local), json.loads(upstream)
                        if a.get('at', '') > b.get('at', ''):
                            content = local
                        if name in {'data/settled_audit.json', 'data/combosettled_audit.json', 'data/v2settled_audit.json'}:
                            content = merge_result_audit(local, upstream)
                if content is None:
                    git('rm', '-f', '--ignore-unmatch', '--', name)
                else:
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text(content, encoding='utf-8')
                    git('add', '-f', '--', name)
            board_path = 'data/board.json'
            candidates = [(ref, blob(ref, board_path)) for ref in (head, remote)]
            candidates = [(ref, json.loads(raw)) for ref, raw in candidates if raw]
            if candidates:
                winner, board = max(candidates, key=lambda item: item[1].get('at', ''))
                for name in ('data/board.json', 'data/more.json', 'data/v2.json'):
                    content = blob(winner, name)
                    if content is not None:
                        Path(name).parent.mkdir(parents=True, exist_ok=True)
                        Path(name).write_text(content, encoding='utf-8')
                        git('add', '-f', '--', name)
                    elif Path(name).exists():
                        git('rm', '-f', '--ignore-unmatch', '--', name)
            # Preserve a coherent trained bundle if a user retrained while this run worked.
            for bundle in (MODEL_BUNDLE, TAIL_BUNDLE):
                if any(blob(base, p) != blob(remote, p) for p in bundle):
                    for name in bundle:
                        content = blob(remote, name)
                        if content is not None:
                            Path(name).parent.mkdir(parents=True, exist_ok=True)
                            Path(name).write_text(content, encoding='utf-8')
                            git('add', '-f', '--', name)
                        elif blob(base, name) is not None:
                            git('rm', '-f', '--ignore-unmatch', '--', name)
            if merge_head.exists():
                git('commit', '-m', 'Merge upstream state and source updates' + (' [skip ci]' if '[skip ci]' in message else ''))
        except Exception:
            if merge_head.exists():
                git('merge', '--abort', check=False)
            raise
        # A normal push protects concurrent work. A rejection triggers a fresh merge next time.
        pushed = git('push', 'origin', f'HEAD:refs/heads/{branch}', check=False)
        if pushed.returncode == 0:
            print('State synchronized with the remote branch.')
            return
        print(f'Push attempt {attempt + 1} rejected; fetching the latest branch before retrying.')
        if attempt + 1 < attempts:
            time.sleep(delay)
    raise RuntimeError('Could not push after retries; local commits and workflow artifacts retain the generated state.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--branch', required=True)
    parser.add_argument('--message', default='data: picks and results [skip ci]')
    args = parser.parse_args()
    try:
        sync(args.branch, args.message)
    except (RuntimeError, ValueError) as exc:
        parser.exit(1, str(exc) + '\n')
