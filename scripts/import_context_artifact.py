"""Merge the latest same-branch BDL backfill artifact into a cached context directory.
Never replace newer captures with an older background snapshot. GitHub credentials
are sent only to api.github.com, never to its signed storage redirect.
"""
import datetime as dt
import json
import os
from pathlib import Path
import re
import shutil
import sqlite3
import tempfile
import urllib.error
import urllib.request
import zipfile


def timestamp(value):
    return dt.datetime.fromisoformat(value.replace('Z', '+00:00')).timestamp()


def merge_jsonl(path, source):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmp:
        db = sqlite3.connect(str(Path(tmp) / 'rows.db'))
        db.execute('create table records (k text primary key, at real, line text)')
        def add(stream):
            for line in stream:
                if not line.strip():
                    continue
                obj = json.loads(line)
                row = obj['row']
                key = json.dumps([row.get(k) for k in ('id', 'game_id', 'player_id', 'team_id', 'type', 'range')])
                at = timestamp(obj['captured_at'])
                db.execute('insert into records values (?,?,?) on conflict(k) do update set at=excluded.at,line=excluded.line where excluded.at>=records.at',
                           (key, at, json.dumps(obj, separators=(',', ':'))))
        if path.exists():
            with path.open(encoding='utf-8') as stream:
                add(stream)
        add(source)
        dest = path.with_suffix('.merge.tmp')
        count = 0
        with dest.open('w', encoding='utf-8', newline='\n') as stream:
            for (line,) in db.execute('select line from records order by k'):
                stream.write(line + '\n')
                count += 1
        db.close()
        dest.replace(path)
    return count


def merge_progress(path, incoming):
    existing = json.loads(path.read_text(encoding='utf-8')) if path.exists() else {}
    for key, value in incoming.items():
        old = existing.get(key)
        rank = lambda x: (x.get('End', ''), bool(x.get('Complete')), x.get('Rows', 0), x.get('Refreshed', ''))
        if old is None or rank(value) >= rank(old):
            existing[key] = value
    path.write_text(json.dumps(existing, indent=2), encoding='utf-8')


def merge_archive(archive, root):
    root = Path(root)
    root.mkdir(parents=True, exist_ok=True)
    total = 0
    with zipfile.ZipFile(archive) as z:
        if sum(f.file_size for f in z.infolist()) > 2_000_000_000:
            raise ValueError('context artifact exceeds expected cache size')
        for info in z.infolist():
            parts = Path(info.filename).parts
            if info.is_dir() or '..' in parts or Path(info.filename).is_absolute():
                continue
            name = parts[-1]
            if re.fullmatch(r'[a-z_]+_20\d\d\.jsonl', name):
                with z.open(info) as source:
                    total += merge_jsonl(root / name, source)
            elif name == 'progress.json':
                merge_progress(root / name, json.loads(z.read(info)))
    return total


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    repo, token = os.getenv('GITHUB_REPOSITORY'), os.getenv('GITHUB_TOKEN')
    branch = os.getenv('GITHUB_REF_NAME', 'main')
    if not repo or not token:
        print('Context artifact import skipped outside GitHub Actions; local cache retained.')
        return
    if not re.fullmatch(r'[\w.-]+/[\w.-]+', repo):
        raise ValueError('invalid repository')
    headers = {'Authorization': 'Bearer ' + token, 'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28'}
    base = 'https://api.github.com/repos/' + repo
    req = urllib.request.Request(base + '/actions/artifacts?name=bdl-context-data&per_page=20', headers=headers)
    with urllib.request.urlopen(req, timeout=30) as response:
        data = json.load(response)
    artifacts = [a for a in data['artifacts'] if not a['expired'] and a.get('workflow_run', {}).get('head_branch') == branch]
    if not artifacts:
        print('No background context artifact yet; bounded downloads continue.')
        return
    artifact = max(artifacts, key=lambda a: a['id'])
    root = Path('data/context')
    marker = root / 'imported_artifact.json'
    if marker.exists() and json.loads(marker.read_text())['id'] >= artifact['id']:
        print('Latest background context batch already imported.')
        return
    req = urllib.request.Request(base + '/actions/artifacts/' + str(int(artifact['id'])) + '/zip', headers=headers)
    opener = urllib.request.build_opener(NoRedirect())
    try:
        response = opener.open(req, timeout=30)
    except urllib.error.HTTPError as error:
        if error.code not in (301, 302, 303, 307, 308):
            raise
        location = error.headers['Location']
        if not location.startswith('https://'):
            raise ValueError('unexpected artifact redirect')
        response = urllib.request.urlopen(location, timeout=90)  # No Authorization header forwarded.
    with response, tempfile.TemporaryDirectory() as tmp:
        archive = Path(tmp) / 'context.zip'
        with archive.open('wb') as out:
            shutil.copyfileobj(response, out)
        count = merge_archive(archive, root)
    marker.write_text(json.dumps({'id': artifact['id'], 'rows': count}), encoding='utf-8')
    print(f'Imported/merged background context batch: {count} cached observations; newest captures retained.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, zipfile.BadZipFile) as error:
        print('Context artifact import unavailable (' + type(error).__name__ + '); cached data retained.')
        raise SystemExit(1)
