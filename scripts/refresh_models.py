#!/usr/bin/env python3
"""Resumable historical opening odds and isolated model research (never auto-promote).

All production model/calibration updates are intentionally disabled here until
a future separately reviewed promotion policy has verified forward evidence.
Daily picks keep using the existing, matching model-calibration pair.
"""
import argparse
import json
import re
import subprocess
import tempfile
from pathlib import Path

from calibrate_games import current_season
from prop_history_cache import coverage


def cached_games(season):
    c=coverage(Path("data"),season)
    return c["cached_games"],c["required_games"]


def matching_pair(paths):
    try:
        model,cal=(json.loads(p.read_text(encoding="utf-8")) for p in paths)
        return (type(model.get("version")) is int and
                model["version"] == cal.get("model_version") and
                (not cal.get("config") or isinstance(cal["config"],str)))
    except (OSError,ValueError,TypeError):return False


def research_candidate(season,lineup=False):
    """Build model and walk-forward calibration only in a disposable directory.

    The Go backtest retrains monthly on strictly earlier games and fits its
    calibration on earlier questions before judging later questions. This
    diagnostic is historical research, NOT validated rotation-v4 performance.
    """
    data=Path("data")
    with tempfile.TemporaryDirectory(prefix="wraith-research-model-") as tmp:
        directory=Path(tmp)/"data"
        directory.mkdir()
        for pattern in ("stats_*.jsonl","advanced_*.jsonl",
                        "opening_games_*.jsonl","opening_props_*.jsonl"):
            for source in data.glob(pattern):
                (directory/source.name).symlink_to(source.resolve())
        flags=["-lineup"] if lineup else []
        try:
            subprocess.run(["go","run","./cmd/train","-data",str(directory),*flags],
                           check=True,capture_output=True,text=True)
            backtest=subprocess.run(
                ["go","run","./cmd/backtest","-data",str(directory),
                 "-season",str(season-1),*flags],
                check=True,capture_output=True,text=True)
        except subprocess.CalledProcessError as exc:
            raise RuntimeError("isolated candidate train or chronological backtest failed") from exc
        model_path=directory/"model.json"
        cal_path=directory/"calibration.json"
        if not matching_pair([model_path,cal_path]):
            raise RuntimeError("candidate model/backtest did not produce a matching calibration")
        model=json.loads(model_path.read_text(encoding="utf-8"))
        cal=json.loads(cal_path.read_text(encoding="utf-8"))
        score=parse_holdout(backtest.stdout or "")
        if score is None:
            raise RuntimeError("candidate backtest missing required chronological market holdout score")
        summary={
            "schema":1,
            "mode":"isolated_historical_candidate_only",
            "season":season-1,
            "model_version":model["version"],
            "model_trained_before":model.get("trained_before"),
            "calibration_trained_before":cal.get("trained_before"),
            "calibration_config":cal.get("config"),
            **score,
            "passes_exploratory_historical_floor":(
                score["held_out_questions"]>=400 and
                score["gain_over_market_log_loss"]>0 and
                score["date_clustered_z"]>=2),
            "production_changed":False,
            "eligible_for_automatic_promotion":False,
            "note":"Historical pregame opening-line walk-forward only; model/calibration are deleted after testing. Rotation-v4 must demonstrate independent, frozen, forward accuracy before any separately reviewed promotion."
        }
        target=Path("run_evidence/backfill/candidate_readiness.json")
        target.parent.mkdir(parents=True,exist_ok=True)
        target.write_text(json.dumps(summary,indent=2)+"\n",encoding="utf-8")
        return summary


def parse_holdout(log):
    lines=re.search(r"\((\d+) lines the calibration never saw\)",log)
    score=re.search(
        r"What the model adds beyond the sportsbook.s price alone:\s*"
        r"([+-]?\d+(?:\.\d+)?) log loss per line, z =\s*([+-]?\d+(?:\.\d+)?)",
        log)
    if not lines or not score:
        return None
    return {
        "held_out_questions":int(lines.group(1)),
        "gain_over_market_log_loss":float(score.group(1)),
        "date_clustered_z":float(score.group(2)),
    }


def refresh(season=None,lineup=False,backfill_minutes=5):
    season=season or current_season()
    paths=[Path("data/model.json"),Path("data/calibration.json")]
    if not matching_pair(paths):
        raise RuntimeError("Production model/calibration mismatch; refusing unsafe publishing")
    download_failed=False
    try:
        subprocess.run(["go","run","./cmd/pull","props","-season",str(season-1),
                        "-maxminutes",str(backfill_minutes)],check=True)
    except subprocess.CalledProcessError:
        download_failed=True
    done,total=cached_games(season-1)
    status={
        "status":"backfilling",
        "downloadFailed":download_failed,
        "cachedGames":done,"totalGames":total,"season":season-1,
        "productionModelUnchanged":True,
    }
    if total and done==total and not download_failed:
        try:
            report=research_candidate(season,lineup)
            status["status"]="candidate_evaluated"
            status["historicalCandidatePassesExploratoryFloor"]=report["passes_exploratory_historical_floor"]
            print("Isolated historical candidate evaluated; production model/calibration unchanged.",flush=True)
        except (RuntimeError,OSError,ValueError) as exc:
            # No candidate failure should interrupt normal live predictions.
            # Never print subprocess stdout/stderr: it may contain raw context.
            status["status"]="candidate_evaluation_failed"
            status["candidateError"]=str(exc)[:200]
            print("Isolated candidate unavailable; existing production pair retained.",flush=True)
    elif download_failed:
        print(f"Historical opening odds download encountered an error; {done}/{total} cached. "
              "Production model and calibration retained.",flush=True)
    else:
        print(f"Historical opening odds {done}/{total} cached. Continuing collection; "
              "production model and calibration retained.",flush=True)
    target=Path("data/model_refresh.json")
    target.write_text(json.dumps(status,indent=2)+"\n",encoding="utf-8")
    return status["status"]=="candidate_evaluated"


if __name__=="__main__":
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--season",type=int)
    parser.add_argument("--lineup",action="store_true")
    parser.add_argument("--backfill-minutes",type=float,default=5,
                        help="Time budget for this run's opening odds collection")
    options=parser.parse_args()
    refresh(options.season,options.lineup,options.backfill_minutes)
