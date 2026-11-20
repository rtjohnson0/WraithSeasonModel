#!/usr/bin/env python3
"""Merge timestamped SportsGameOdds snapshots without replacing earlier evidence.

Only sgo_YYYY-MM-DD.jsonl and sgo_raw_YYYY-MM-DD.jsonl are included.
The merger is idempotent and never removes existing rows. No picks are read.
"""
import argparse
import re
from pathlib import Path


PATTERN = re.compile(r"sgo_(?:raw_)?[0-9]{4}-[0-9]{2}-[0-9]{2}\.jsonl\Z")


def merge(source, target):
    target.mkdir(parents=True, exist_ok=True)
    summary = {"files": 0, "new_rows": 0}
    if not source.exists():
        return summary
    for path in sorted(source.iterdir()):
        if not path.is_file() or not PATTERN.fullmatch(path.name):
            continue
        dest = target / path.name
        original = dest.read_bytes().splitlines() if dest.exists() else []
        additions = path.read_bytes().splitlines()
        seen = set(original)
        new = [line for line in additions if line.strip() and not (line in seen or seen.add(line))]
        if not new:
            continue
        # Existing lines are preserved in their original order.
        with dest.open("ab") as handle:
            if dest.exists() and dest.stat().st_size:
                with dest.open("rb") as verify:
                    verify.seek(-1, 2)
                    if verify.read(1) != b"\n":
                        handle.write(b"\n")
            for line in new:
                handle.write(line + b"\n")
        summary["files"] += 1
        summary["new_rows"] += len(new)
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--target", type=Path, required=True)
    args = parser.parse_args()
    print("Near-tip snapshot merge:", merge(args.source, args.target))


if __name__ == "__main__":
    main()
