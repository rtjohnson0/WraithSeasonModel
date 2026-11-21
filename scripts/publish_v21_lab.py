#!/usr/bin/env python3
"""Allowlisted public export of AGGREGATE V2.1 research metrics only.

No picks, player identifiers, sportsbook quotes, individual outcomes, lines,
market prices, or unknown fields are ever copied into GitHub Pages.
"""
import argparse
import json
import math
from pathlib import Path

EXPECTED_POLICY = "v21-minute-guard-and-cap-20261121"
IDS = ("base-minutes", "rotation-v4", "supported-only", "capped-shift")


def positive_integer(value):
    return type(value) is int and value >= 0


def finite_number(value):
    return type(value) in (int, float) and math.isfinite(value)


def sanitize(report):
    if not isinstance(report, dict) or report.get("schema") != 1 or report.get("policy") != EXPECTED_POLICY:
        raise ValueError("unknown or invalid V2.1 evidence")
    candidates = report.get("candidates")
    if not isinstance(candidates, list) or [x.get("id") for x in candidates if isinstance(x, dict)] != list(IDS):
        raise ValueError("candidate order or schema mismatch")
    result = {
        "schema": 1,
        "policy": EXPECTED_POLICY,
        "generatedAt": str(report.get("generatedAt", ""))[:32],
        "through": str(report.get("through", ""))[:10],
        "status": str(report.get("status", "research_only"))[:60],
    }
    for key in ("dates", "playerGames", "questions"):
        val = report.get(key)
        if not positive_integer(val):
            raise ValueError("invalid nonnegative evidence count: " + key)
        result[key] = val
    result["candidates"] = []
    for entry in candidates:
        item = {"id": entry["id"]}
        item["label"] = {
            "base-minutes": "Baseline minutes",
            "rotation-v4": "Current V2 rotation-v4",
            "supported-only": "Supported absences only",
            "capped-shift": "Supported + capped ±2.5m",
        }[item["id"]]
        for key in ("playerGames", "questions"):
            if not positive_integer(entry.get(key)):
                raise ValueError("invalid candidate count: " + key)
            item[key] = entry[key]
        # Scores are intentionally zero for suppressed small cohorts;
        # the UI checks sample counts before displaying them.
        for key in ("minutesMAE", "minutesBias", "brier", "logLoss", "statMAE"):
            value = entry.get(key)
            if not finite_number(value) or abs(value) > 1000:
                raise ValueError("invalid aggregate score: " + key)
            item[key] = round(value, 6)
        result["candidates"].append(item)
    # Positive assertion: ONLY these properties enter the public artifact.
    return result


def publish(source, destination):
    summary = sanitize(json.loads(Path(source).read_text(encoding="utf-8")))
    target = Path(destination)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=Path("data/v21_lab.json"))
    parser.add_argument("--out", type=Path, default=Path("site/public/data/v21-lab.json"))
    args = parser.parse_args()
    if not args.source.exists():
        print("No saved V2.1 scorecard yet. The Lab tab will show a pending-data message.")
        return
    result = publish(args.source, args.out)
    print(f"Published aggregate-only V2.1 Lab: {result['playerGames']} verified player-games, "
          f"{result['questions']} identical stat questions, {result['dates']} dates.")


if __name__ == "__main__":
    main()
