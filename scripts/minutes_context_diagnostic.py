#!/usr/bin/env python3
"""Private, observational minutes/usage subgroup diagnostic.

Only pregame context already captured in minutes-audit/report.json may define
subgroups. Never infer historical availability from postgame lineups, and never
fit production corrections from these results.
"""
import argparse
import json
import math
from collections import defaultdict
from pathlib import Path


def finite(value):
    return isinstance(value, (float, int)) and not isinstance(value, bool) and math.isfinite(value)


def groups(row):
    context = row.get("context")
    if not isinstance(context, dict):
        return ["context unavailable"]
    result = ["context captured"]
    confirmed = context.get("confirmedOut")
    assumed = context.get("assumedOut")
    question = context.get("questionable")
    doubtful = context.get("doubtful")
    if isinstance(confirmed, int) and confirmed > 0:
        result.append("confirmed teammate absent")
    elif confirmed == 0:
        result.append("no confirmed teammates absent")
    if isinstance(assumed, int) and assumed > 0:
        result.append("assumed teammate absent")
    if isinstance(question, int) and isinstance(doubtful, int) and question + doubtful > 0:
        result.append("uncertain teammate status")
    if context.get("returning") is True:
        result.append("player returning from absence")
    if context.get("b2b") is True:
        result.append("back-to-back")
    spread = context.get("spread")
    if finite(spread) and abs(spread) >= 10:
        result.append("spread absolute 10+")
    typical = row.get("typicalMinutes")
    projected = row.get("projectedMinutes")
    if finite(typical) and finite(projected) and typical > 0:
        if projected <= typical - 5:
            result.append("projection 5+ below typical")
        if projected >= typical + 5:
            result.append("projection 5+ above typical")
    return result


def diagnose(report):
    grouped = defaultdict(list)
    observations = report.get("observations", [])
    # One earliest pregame player/game per cohort, not one per line or stat.
    earliest = {}
    for row in observations:
        cohort = row.get("cohort", "unknown")
        key = (cohort, row.get("date"), row.get("gameId"), row.get("playerId"))
        projected, actual = row.get("projectedMinutes"), row.get("actualMinutes")
        if not all(finite(v) for v in [projected, actual]) or projected <= 0 or actual < 0:
            continue
        if key not in earliest or str(row.get("at", "")) < str(earliest[key].get("at", "")):
            earliest[key] = row
    for row in earliest.values():
        cohort = row.get("cohort", "unknown")
        for subgroup in ["all player-games", *groups(row)]:
            grouped[(cohort, subgroup)].append(row)
    result = []
    for (cohort, subgroup), sample in sorted(grouped.items()):
        errors = [r["actualMinutes"] - r["projectedMinutes"] for r in sample]
        days = len({r.get("date") for r in sample})
        n = len(errors)
        result.append({
            "cohort": cohort,
            "group": subgroup,
            "playerGames": n,
            "dates": days,
            "actualMinusProjectedMinutes": round(sum(errors) / n, 4),
            "minutesMAE": round(sum(abs(e) for e in errors) / n, 4),
            "underestimatedBy5Plus": sum(e >= 5 for e in errors),
            "overestimatedBy5Plus": sum(e <= -5 for e in errors),
            "insufficientEvidence": n < 30 or days < 10,
        })
    return {
        "schema": 1,
        "mode": "descriptive_only",
        "source": "frozen pregame minutes audit",
        "groups": result,
        "notes": [
            "Each player-game counted once per cohort/group; overlapping groups are not added.",
            "Legacy and verified replay cohorts are never pooled.",
            "Pregame injury/rotation context missing means unknown, not healthy.",
            "These are descriptive relationships, not causal adjustments, prediction validation, or evidence of profitability.",
            "30 player-games and 10 dates are diagnostic display thresholds, not formal proof of calibration."
        ],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=Path("run_evidence/minutes-audit/report.json"))
    parser.add_argument("--out", type=Path, default=Path("run_evidence/minutes-context/report.json"))
    args = parser.parse_args()
    summary = diagnose(json.loads(args.input.read_text(encoding="utf-8")))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
    print(f"Minutes context: {len(summary['groups'])} distinct cohort/subgroup summaries; no production changes.")


if __name__ == "__main__":
    main()
