#!/usr/bin/env python3
"""Publish ONLY aggregate, separately evaluated moneyline/spread/total metrics.

Input gamechallenger.json contains individual matchups and per-game predictions,
so never copy the input object or its games list into GitHub Pages. A strict
allowlist produces an entirely new summary and rejects invalid evaluation data.
"""
import argparse
import json
import math
from pathlib import Path

MARKETS = ("moneyline", "spread", "total")


def clean_number(value):
    return value if type(value) in (int, float) and math.isfinite(value) and 0 <= value <= 100 else None


def positive_count(value):
    return type(value) is int and value >= 0


def sanitize(report):
    if not isinstance(report, dict) or report.get("policy") != "game-pace-v1":
        raise ValueError("unsupported game challenger schema")
    evaluation = report.get("evaluation")
    if not isinstance(evaluation, dict):
        raise ValueError("game challenger missing market-separated verification")
    out = {
        "schema": 1,
        "policy": "v21-game-markets-read-only-v1",
        "generatedAt": str(report.get("at") or "")[:32],
        "date": str(report.get("date") or "")[:10],
        "researchOnly": True,
        "markets": [],
    }
    for market in MARKETS:
        row = evaluation.get(market)
        if not isinstance(row, dict):
            raise ValueError("missing market " + market)
        n, days, calibrated = row.get("N"), row.get("Dates"), row.get("CalibratedN")
        if not all(positive_count(x) for x in (n, days, calibrated)) or calibrated > n:
            raise ValueError("bad market sample sizes")
        entry = {
            "id": market,
            "n": n,
            "days": days,
            "calibratedN": calibrated,
            "rawLogLoss": clean_number(row.get("RawLogLoss")),
            "bookLogLoss": clean_number(row.get("BookLogLoss")),
            "calibratedLogLoss": clean_number(row.get("CalibratedLogLoss")),
            "calibrationRawLogLoss": clean_number(row.get("CalibrationRawLogLoss")),
            "calibrationBookLogLoss": clean_number(row.get("CalibrationBookLogLoss")),
        }
        if n > 0 and (entry["rawLogLoss"] is None or entry["bookLogLoss"] is None):
            raise ValueError("missing independently evaluated model/book scores")
        if calibrated > 0 and (entry["calibratedLogLoss"] is None or entry["calibrationBookLogLoss"] is None):
            raise ValueError("incomplete matched calibration evaluation")
        out["markets"].append(entry)
    return out


def sanitize_point_study(study):
    if not isinstance(study,dict) or study.get("policy")!="game-residual-study-v1":
        raise ValueError("unsupported point study")
    out={"policy":study["policy"]}
    for k in ("games","dates"):
        if not positive_count(study.get(k)): raise ValueError("invalid point study counts")
        out[k]=study[k]
    for k in ("rawMarginMAE","correctedMarginMAE","rawTotalMAE","correctedTotalMAE"):
        out[k]=clean_number(study.get(k))
        if out[k] is None: raise ValueError("invalid point error")
    return out


def publish(source, destination, study=None):
    report = sanitize(json.loads(Path(source).read_text(encoding="utf-8")))
    if study is not None:
        report["pointStudy"]=sanitize_point_study(study)
    destination = Path(destination)
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(report, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=Path("data/gamechallenger.json"))
    parser.add_argument("--out", type=Path, default=Path("site/public/data/v21-games.json"))
    parser.add_argument("--study",type=Path,default=Path("run_evidence/game-pace/report.json"))
    args = parser.parse_args()
    if not args.source.exists():
        print("Game market research results not available; no scores published.")
        return
    study=json.loads(args.study.read_text()).get("residualStudy") if args.study.exists() else None
    report = publish(args.source, args.out, study)
    print("Aggregate-only game market lab:", ", ".join(
        f"{r['id']}={r['n']} verified results over {r['days']} dates" for r in report["markets"]))


if __name__ == "__main__":
    main()
