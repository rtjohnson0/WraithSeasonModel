#!/usr/bin/env python3
"""Fail closed before publishing a slate without a complete *current* injury report.

The BDL injury endpoint covers players across the league, not merely the
bettor's selected player or home team. A zero-row response can be valid, so
the successful-fetch receipt—not row count—is authoritative. The schedule
must also include valid identities for BOTH home and away teams.
"""
import argparse
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

EASTERN = ZoneInfo("America/New_York")
MAX_AGE = timedelta(minutes=30)
CLOCK_SKEW = timedelta(minutes=2)


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("invalid injury snapshot timestamp")
    try:
        result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("invalid injury snapshot timestamp") from exc
    if result.tzinfo is None:
        raise ValueError("naive injury snapshot timestamp")
    return result.astimezone(timezone.utc)


def verify(snapshot_dir: Path, now: datetime):
    if now.tzinfo is None:
        raise ValueError("verification time must have a timezone")
    now = now.astimezone(timezone.utc)
    date = now.astimezone(EASTERN).strftime("%Y-%m-%d")
    receipt_path = snapshot_dir / f"injury_health_{date}.json"
    schedule_path = snapshot_dir / f"schedule_{date}.json"
    if not receipt_path.is_file():
        raise ValueError("No successful BDL injury report receipt for today's slate")
    if not schedule_path.is_file():
        raise ValueError("No complete schedule with both teams for today's slate")
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    schedule = json.loads(schedule_path.read_text(encoding="utf-8"))
    if not isinstance(receipt, dict) or not isinstance(schedule, dict):
        raise ValueError("invalid injury health or schedule record")
    if (receipt.get("date") != date or schedule.get("date") != date or
            receipt.get("injuries_complete") is not True):
        raise ValueError("Injury report wasn't completed for today's game date")
    at, schedule_at = timestamp(receipt.get("captured_at")), timestamp(schedule.get("captured_at"))
    for kind, checked in (("injury report", at), ("schedule", schedule_at)):
        if checked > now + CLOCK_SKEW or now - checked > MAX_AGE:
            raise ValueError(f"Stale or future {kind}; refresh injuries and both teams before publishing")
    if at != schedule_at:
        raise ValueError("Injury report and schedule aren't from the same capture")
    games = schedule.get("games")
    if not isinstance(games, list) or not games or receipt.get("games") != len(games):
        raise ValueError("No complete games with both teams in the pregame schedule")
    unique_games = set()
    home_teams, away_teams = set(), set()
    for game in games:
        if not isinstance(game, dict) or game.get("date") != date:
            raise ValueError("Missing game/date identity")
        game_id, home, away = game.get("id"), game.get("home"), game.get("visitor")
        if not all(type(x) is int and x > 0 for x in (game_id, home, away)) or home == away:
            raise ValueError("Missing home or away team identity")
        if game_id in unique_games:
            raise ValueError("Duplicate game in pregame schedule")
        unique_games.add(game_id)
        home_teams.add(home)
        away_teams.add(away)
    # No individual injury/player or team-level medical details are exposed.
    return {
        "date": date, "games": len(games),
        "home_teams": len(home_teams), "away_teams": len(away_teams),
        "injuries_complete": True,
        "age_minutes": round((now - at).total_seconds() / 60, 1),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--snapshots", type=Path, default=Path("data/snapshots"))
    args = parser.parse_args()
    try:
        result = verify(args.snapshots, datetime.now(timezone.utc))
    except (OSError, ValueError, KeyError, TypeError, json.JSONDecodeError) as exc:
        parser.exit(1, f"INJURY GATE BLOCKED: {exc}. Previous published board is retained.\n")
    print(
        "Injury gate PASSED: current league-wide report; "
        f"{result['games']} games with both home/away teams; "
        f"capture age {result['age_minutes']}m. "
        "Unknown or questionable players are not marked healthy."
    )


if __name__ == "__main__":
    main()
