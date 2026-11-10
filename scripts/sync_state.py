#!/usr/bin/env python3
"""Merge generated state with remote updates and push without rebasing or force pushing.
JSONL conflicts are resolved by record identity with three-way deletion handling.
Remote edits to model/calibration files win over a concurrent generated copy.
Unrelated source conflicts stop with the merge aborted; generated commits remain local.
"""
import argparse
import json
import os
import subprocess
import time
from pathlib import Path

STATE_FILES = [
    'quarterboard.json', 'quarterlog.jsonl', 'gameboard.json', 'board.json', 'picklog.jsonl', 'settled.jsonl', 'calibration.json', 'gamecal.json',
    'biasfix.json', 'minfix.json', 'wofix.json', 'runs.jsonl', 'more.json',
    'projlog.jsonl', 'model.json', 'combolog.jsonl', 'combosettled.jsonl',
    'combocorr.json', 'v2log.jsonl', 'v2settled.jsonl', 'v2.json', 'model_v2.json',
    'combocalibration.json', 'exclusions.jsonl',
]
MODEL_BUNDLE = ['data/' + x for x in ('model.json', 'model_v2.json', 'calibration.json',
                'combocalibration.json', 'gamecal.json', 'biasfix.json', 'minfix.json',
                'wofix.json', 'combocorr.json')]


def git(*args, check=True):
    result = subprocess.run(['git', *args], text=True, encoding='utf-8', capture_output=True)
    if check and result.returncode:
        # Never print a remote URL: credentials can be embedded in it.
        raise RuntimeError('git ' + args[0] + ' failed: ' + result.stderr[-2000:])
    return result


def blob(ref, name):
    result = git('show', f'{ref}:{name}', check=False)
    return None if result.returncode else result.stdout


def record_key(name, row):
    if name.endswith('projlog.jsonl') or '/lines/market_' in name:
        return (row.get('date'), row.get('game_id', row.get('game')),
                row.get('player_id', row.get('pid')), row.get('stat'))
    if name.endswith('runs.jsonl'):
        return json.dumps(row, sort_keys=True, separators=(',', ':'))
    if name.endswith('exclusions.jsonl'):
        return row.get('id', json.dumps(row, sort_keys=True))
    return (row.get('date'), row.get('game_id', row.get('game')),
            row.get('player_id', row.get('pid')), row.get('stat'), row.get('side'),
            row.get('book'), row.get('line'), row.get('period', 'game'))


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
        elif name.endswith('projlog.jsonl') or '/lines/market_' in name:
            value = max((ov, tv), key=lambda r: r.get('at', ''))
        else:
            # Remote corrections win; identical/new append-only records are unioned.
            value = tv
        if value is not None:
            merged.append(value)
    merged.sort(key=lambda r: (r.get('date', ''), str(record_key(name, r))))
    return ''.join(json.dumps(r, separators=(',', ':'), ensure_ascii=False) + '\n' for r in merged)


def state_paths():
    names = {'data/' + x for x in STATE_FILES}
    names.update(str(p).replace(os.sep, '/') for p in Path('data/lines').glob('*.jsonl'))
    names.update(str(p).replace(os.sep, '/') for p in Path('data/quarter_games').glob('*.json'))
    return sorted(names)


def is_state(name):
    return name in {'data/' + x for x in STATE_FILES} or (
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
                    if (name.startswith('data/quarter_games/') or name in {'data/quarterboard.json', 'data/gameboard.json', 'data/board.json', 'data/more.json', 'data/v2.json'}) and local and upstream:
                        a, b = json.loads(local), json.loads(upstream)
                        if a.get('at', '') > b.get('at', ''):
                            content = local
                if content is None:
                    git('rm', '-f', '--ignore-unmatch', '--', name)
                else:
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text(content, encoding='utf-8')
                    git('add', '-f', '--', name)
            # Preserve a coherent trained bundle if a user retrained while this run worked.
            if any(blob(base, p) != blob(remote, p) for p in MODEL_BUNDLE):
                for name in MODEL_BUNDLE:
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
