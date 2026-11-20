#!/usr/bin/env python3
"""Safely merge resumable historical *opening* NBA prop quotes between caches.

Never reads or writes picks. Keep the first valid opening response per game;
a later API response cannot silently rewrite the original betting snapshot.
Invalid trailing JSON is skipped (interrupted write); malformed middle rows fail.
"""
import argparse
import json
import os
import tempfile
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo


def season_now():
    now = datetime.now(ZoneInfo("America/New_York"))
    return now.year if now.month >= 8 else now.year - 1


def game_id(row):
    if not isinstance(row, dict):
        return None
    number, response = row.get("_game_id"), row.get("response")
    if type(number) is not int or number <= 0 or not isinstance(response, dict):
        return None
    if not isinstance(response.get("data"), list):
        return None
    return number


def load(path):
    """Return game-indexed rows; reject corrupt history rather than losing it."""
    parsed = {}
    stats = {"valid": 0, "duplicate": 0, "conflicts": 0, "trailing_incomplete": 0}
    if not path.exists():
        return parsed, stats
    with path.open("r", encoding="utf-8-sig") as stream:
        lines = stream.readlines()
    last = max((i for i, x in enumerate(lines) if x.strip()), default=-1)
    for i, raw in enumerate(lines):
        if not raw.strip():
            continue
        try:
            record = json.loads(raw)
        except (json.JSONDecodeError, UnicodeDecodeError) as exc:
            if i == last:
                stats["trailing_incomplete"] += 1
                continue
            raise ValueError(f"{path}: invalid non-terminal record {i + 1}") from exc
        key = game_id(record)
        if key is None:
            raise ValueError(f"{path}: invalid opening response at line {i + 1}")
        stats["valid"] += 1
        if key in parsed:
            stats["duplicate"] += 1
            if parsed[key]["response"] != record["response"]:
                stats["conflicts"] += 1
            continue
        parsed[key] = record
    return parsed, stats


def merge(source, destination):
    source, destination = Path(source), Path(destination)
    source_rows, source_stats = load(source)
    dest_rows, dest_stats = load(destination)
    added = 0
    conflicts = 0
    for key, row in sorted(source_rows.items()):
        if key in dest_rows:
            if dest_rows[key]["response"] != row["response"]:
                conflicts += 1
            continue
        dest_rows[key] = row
        added += 1
    if added or (dest_stats["duplicate"] or dest_stats["trailing_incomplete"]):
        destination.parent.mkdir(parents=True, exist_ok=True)
        tmp = None
        try:
            with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=destination.parent,
                                             prefix=".opening_props_", suffix=".tmp",
                                             delete=False) as file:
                tmp = Path(file.name)
                for key in sorted(dest_rows):
                    json.dump(dest_rows[key], file, separators=(",", ":"), ensure_ascii=False)
                    file.write("\n")
                file.flush()
                os.fsync(file.fileno())
            os.replace(tmp, destination)
        finally:
            if tmp is not None:
                tmp.unlink(missing_ok=True)
    return {"source_games": len(source_rows), "destination_games": len(dest_rows),
            "added": added, "conflicting_responses_not_overwritten": conflicts,
            "invalid_trailing_source_rows": source_stats["trailing_incomplete"]}


def coverage(data, season):
    stats_path = data / f"stats_{season}.jsonl"
    expected = set()
    if stats_path.exists():
        with stats_path.open(encoding="utf-8-sig") as rows:
            for line in rows:
                if not line.strip():
                    continue
                raw = json.loads(line)
                game = raw.get("game") or {}
                if type(game.get("id")) is int and game["id"] > 0 and                         (game.get("season") or season) == season:
                    expected.add(game["id"])
    collected, invalid = load(data / f"opening_props_{season}.jsonl")
    matched = len(set(collected) & expected)
    return {"season": season, "cached_games": matched, "required_games": len(expected),
            "coverage_fraction": round(matched / len(expected), 6) if expected else None,
            "unrecognized_cached_games": len(set(collected) - expected),
            "trailing_incomplete_rows": invalid["trailing_incomplete"],
            "ready_for_full_season_backtest": bool(expected) and matched == len(expected)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=["import", "stage"], required=True)
    parser.add_argument("--season", type=int, default=season_now() - 1)
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--output", type=Path, default=Path("run_evidence/backfill/coverage.json"))
    args = parser.parse_args()
    if args.season < 2000 or args.season > 2100:
        parser.error("invalid season")
    canonical = args.data / f"opening_props_{args.season}.jsonl"
    isolated = args.data / "prop_backfill" / canonical.name
    source, target = (isolated, canonical) if args.mode == "import" else (canonical, isolated)
    merged = merge(source, target)
    report = coverage(args.data, args.season)
    report.update({"operation": args.mode, "merge": merged})
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(f"Season {args.season}: {report['cached_games']}/{report['required_games']} "
          f"opening games cached ({args.mode}, {merged['added']} new); "
          "no picks or model parameters modified")


if __name__ == "__main__":
    main()
