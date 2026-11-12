#!/usr/bin/env python3
"""Record isolated, zero-stake V5 decisions using already downloaded inputs."""
import argparse
import shutil
import subprocess
from pathlib import Path


def run(data, go='go'):
    data = Path(data).resolve()
    if not all((data / n).is_file() for n in ('model_tail.json', 'calibration_tail.json')):
        print('Uncertainty shadow skipped: no candidate pair installed.')
        return
    for name, policy in [('tail_shadow', 'confidence'), ('value_shadow', 'value')]:
        target = data / name
        target.mkdir(exist_ok=True)
        # Copies deliberately prevent prediction or settlement from modifying inputs.
        for pattern in ('stats_*.jsonl', 'advanced_*.jsonl', 'opening_games_*.jsonl', 'players_active.jsonl'):
            for source in data.glob(pattern):
                dest = target / source.name
                if not dest.exists() or source.stat().st_mtime_ns != dest.stat().st_mtime_ns or source.stat().st_size != dest.stat().st_size:
                    shutil.copy2(source, dest)
        if (data / 'snapshots').is_dir():
            shutil.copytree(data / 'snapshots', target / 'snapshots', dirs_exist_ok=True)
        for source, dest in [('model_tail.json', 'model.json'), ('calibration_tail.json', 'calibration.json')]:
            shutil.copy2(data / source, target / dest)
        subprocess.run([go, 'run', './cmd/settle', '-data', str(target)], check=True)
        args = [go, 'run', './cmd/predict', '-data', str(target), '-v2=false', '-combos=false', '-pergame', '0', '-maxunits', '0', '-selectionpolicy', policy]
        if policy == 'value':
            args += ['-minedge', '0.02', '-valueminprob', '0.40', '-valueminroi', '0.04']
        subprocess.run(args, check=True)


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--data', default='data')
    p.add_argument('--go', default='go')
    a = p.parse_args()
    run(a.data, a.go)
