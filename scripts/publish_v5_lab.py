#!/usr/bin/env python3
"""Strictly aggregate-only public evidence for the NEW rotation-v5 cohort.

Never copy source Go reports verbatim: they may gain fields in future versions.
The allowlist exposes no single game, player, book, price, or selection.
"""
import argparse
import json
import math
from datetime import datetime, timezone
from pathlib import Path

STAT_IDS = ("points", "rebounds", "assists", "threes")

def numeric(x):
    return type(x) in (float, int) and math.isfinite(x) and 0 <= x < 10000

def count(x):
    return type(x) is int and x >= 0

def sanitize(raw):
    if not isinstance(raw, dict) or raw.get("Policy") != "rotation-v5":
        raise ValueError("report must be the independent guarded V5 cohort")
    pg, questions, days = (raw.get(x) for x in ("PlayerGames", "Questions", "Days"))
    if not all(map(count, (pg, questions, days))):
        raise ValueError("invalid V5 cohort sizes")
    output = {"schema":1, "policy":"rotation-v5", "researchOnly":True,
              "generatedAt":datetime.now(timezone.utc).isoformat(),
              "through":str(raw.get("Through") or "")[:10],
              "playerGames":pg, "questions":questions, "days":days,
              "minutesMAE":None, "minutesBias":None, "stats":[]}
    exclusions=raw.get("Excluded") or {}
    output["audit"]={}
    for dest,source in (("pending","date not completed"),("rejectedProduction","production provenance unavailable"),("missingFinals","final identity/source unavailable")):
        value=exclusions.get(source,0)
        if not count(value): raise ValueError("invalid audit exclusion count")
        output["audit"][dest]=value
    # Go reports exclude all private Phase3 questions by design.
    if pg >= 30 and days >= 5:
        ma=raw.get("RotationMinutesMAE")
        if not numeric(ma):
            raise ValueError("invalid minutes error")
        output["minutesMAE"]=round(ma,5)
    slices=raw.get("Slices")
    if slices is None: slices=[]
    if not isinstance(slices,list):
        raise ValueError("bad per-stat aggregate")
    for row in slices:
        if not isinstance(row,dict) or row.get("Stat") not in STAT_IDS:
            raise ValueError("unexpected V5 stat")
        n=row.get("N")
        sd=row.get("Days")
        if not count(n) or not count(sd):
            raise ValueError("invalid stat sample count")
        item={"stat":row["Stat"],"n":n,"days":sd,"brier":None,"statMAE":None}
        if n >= 30 and sd >= 5:
            b, e=row.get("RotationBrier"),row.get("RotationMAE")
            if not numeric(b) or not numeric(e):
                raise ValueError("invalid stat score")
            item["brier"]=round(b,6)
            item["statMAE"]=round(e,5)
        output["stats"].append(item)
    return output


def publish(source,destination):
    result=sanitize(json.loads(Path(source).read_text(encoding="utf-8")))
    destination=Path(destination)
    destination.parent.mkdir(parents=True,exist_ok=True)
    destination.write_text(json.dumps(result,indent=2,allow_nan=False)+"\n",encoding="utf-8")
    return result


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--source",type=Path,default=Path("run_evidence/guarded-v5/report.json"))
    ap.add_argument("--out",type=Path,default=Path("site/public/data/v5-lab.json"))
    args=ap.parse_args()
    if not args.source.exists():
        print("V5 has not completed any verified games yet; no report to publish.")
        return
    result=publish(args.source,args.out)
    print(f"V5 separate forward cohort: {result['playerGames']} player-games, {result['questions']} questions across {result['days']} dates.")
if __name__=="__main__":
    main()
