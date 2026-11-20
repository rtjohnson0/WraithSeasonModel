#!/usr/bin/env python3
"""Shadow-only analysis of ACTUAL rotation-v4 V2, not the primary-model replay.

Completed questions come exclusively from the Go rotation-v4 auditor, which
verifies signed pregame context, model hashes, timing, recomputation and final
identity. It writes eligible question rows temporarily to ignored private/.
This script aggregates them and deletes that file. Only aggregate statistics
are stored in run_evidence/. No live picks, lines, prices or identities exported.
"""
import argparse
import hashlib
import json
import math
import re
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

POLICY_ID = "rotation-v4-fixed-market-residual-v1"
STATS = {"points", "rebounds", "assists", "threes"}
MIN_GROUP = 8
RETAIN = 0.75
UNCERTAIN_RETAIN = 0.55


def finite(v):
    return isinstance(v, (int, float)) and not isinstance(v, bool) and math.isfinite(v)


def when(value):
    try:
        t = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        return t.astimezone(timezone.utc) if t.tzinfo else None
    except (ValueError, TypeError):
        return None


def implied(odds):
    if type(odds) is not int or (-100 < odds < 100):
        return None
    return 100 / (100 + odds) if odds > 0 else -odds / (100 - odds)


def uncertain(question):
    own = str(question.get("own_status") or "").strip().lower()
    return own in {"questionable", "doubtful", "probable", "day-to-day"} or (
        type(question.get("confirmed_out")) is int and
        question.get("confirmed_out", 0) > 0 and
        (question.get("allocation_applied") is False or question.get("rotation_supported") is False)
    )


def challenger(p_over, market_over, pregame):
    """Zero-stake fixed challenger; no fitting on outcomes and no Over quota."""
    if not all(finite(p) and 0 < p < 1 for p in (p_over, market_over)):
        raise ValueError("invalid probabilities")
    weight = UNCERTAIN_RETAIN if uncertain(pregame) else RETAIN
    return min(1 - 1e-6, max(1e-6, market_over + weight * (p_over - market_over)))


def valid_question(row, before):
    try:
        t = datetime.strptime(row["date"], "%Y-%m-%d")
        if t.strftime("%Y-%m-%d") >= before or row.get("stat") not in STATS:
            return False
        for field in ("p_over", "market", "line", "actual", "projected",
                      "projected_minutes", "actual_minutes"):
            if not finite(row.get(field)):
                return False
        if not (0 < row["p_over"] < 1 and 0 < row["market"] < 1 and
                row["projected_minutes"] > 0 and row["actual_minutes"] > 0 and
                row["actual"] != row["line"] and row["line"] >= 0):
            return False
        over, under = implied(row.get("over_odds")), implied(row.get("under_odds"))
        if over is None or under is None:
            return False
        if abs(row["market"] - over/(over+under)) > 1e-8:
            return False
        if not isinstance(row.get("model_id"), str) or not re.fullmatch(r"[0-9a-f]{64}", row["model_id"]):
            return False
        return True
    except (KeyError, ValueError, TypeError):
        return False


def read_go_verified(path, before):
    """Rows have already passed the stronger rotation-v4 Go audit. No backfill."""
    questions = []
    rejected = Counter()
    if not path.is_file():
        return questions, {"go_verified_private_input_missing": 1}
    with path.open(encoding="utf-8") as src:
        for line in src:
            if not line.strip():
                continue
            row = json.loads(line)
            if valid_question(row, before):
                questions.append(row)
            else:
                rejected["malformed_or_out_of_window_verified_input"] += 1
    return questions, dict(rejected)


def assess(row):
    p, m = row["p_over"], row["market"]
    shadow = challenger(p, m, row)
    side = "OVER" if p >= .5 else "UNDER"
    side_p = p if side == "OVER" else 1 - p
    odds = row["over_odds"] if side == "OVER" else row["under_odds"]
    selected = side_p >= .55 and side_p - implied(odds) >= .01
    over_won = row["actual"] > row["line"]
    won = over_won if side == "OVER" else not over_won
    minutes_delta = row["actual_minutes"] - row["projected_minutes"]
    rate = row["projected"] / row["projected_minutes"]
    minute_term = minutes_delta * rate
    rate_term = row["actual"] - row["actual_minutes"] * rate
    indicators = {
        "minutes_at_least_5_above": minutes_delta >= 5,
        "minutes_at_least_5_below": minutes_delta <= -5,
        "minutes_error_in_miss_direction":
            (side == "UNDER" and minutes_delta >= 5) or (side == "OVER" and minutes_delta <= -5),
        "rate_error_in_miss_direction":
            (side == "UNDER" and rate_term > 0) or (side == "OVER" and rate_term < 0),
        "rate_term_larger_than_minutes_term": abs(rate_term) > abs(minute_term),
        "pregame_confirmed_teammate_out": row.get("confirmed_out", 0) > 0,
        "pregame_relevant_teammate_out": row.get("relevant_out", 0) > 0,
        "pregame_unsupported_rotation": row.get("rotation_supported") is False,
        "pregame_allocation_not_applied": row.get("allocation_applied") is False,
        "pregame_fewer_than_3_matched_games": row.get("matched_games", 0) < 3,
        "pregame_own_status_uncertain": uncertain(row),
        "pregame_expected_lineup_present": row.get("pregame_lineups", 0) > 0,
    }
    payout = (odds / 100 if odds > 0 else 100 / -odds) if won else -1.
    return {"date": row["date"], "stat": row["stat"], "side": side,
            "checkpoint": row["model_id"], "p": p, "market": m,
            "challenger": shadow, "outcome": int(over_won),
            "selected": selected, "won": won, "payoff": payout,
            "minute_delta": minutes_delta,
            "minute_term": minute_term, "rate_term": rate_term,
            "indicators": indicators}


def metrics(items):
    n = len(items)
    days = len({x["date"] for x in items})
    if n < MIN_GROUP:
        return {"n": n, "days": days, "status": "small_cohort_suppressed"}
    result = {"n": n, "days": days,
              "status": ("insufficient_forward_evidence" if n < 200 or days < 20
                         else "requires_predeclared_future_trial")}
    for name, field in [("current_v2", "p"), ("shadow_v2", "challenger"), ("devig_market", "market")]:
        result[name] = {
            "brier": round(sum((r[field] - r["outcome"])**2 for r in items) / n, 6),
            "log_loss": round(sum(-math.log(max(1e-12, min(1-1e-12,
                           r[field] if r["outcome"] else 1-r[field]))) for r in items) / n, 6)}
    result["shadow_minus_current_log_loss"] = round(
        result["shadow_v2"]["log_loss"] - result["current_v2"]["log_loss"], 6)
    return result


def loss_summary(items):
    selected = [r for r in items if r["selected"]]
    if len(selected) < MIN_GROUP:
        return {"selected_count": len(selected), "status": "small_cohort_suppressed",
                "loss_patterns": {}}
    missed = [r for r in selected if not r["won"]]
    out = {"selected_count": len(selected), "selected_days": len({r["date"] for r in selected}),
           "loss_count": len(missed), "wins": len(selected) - len(missed),
           "hypothetical_fixed_rule_flat_roi":
               round(sum(r["payoff"] for r in selected) / len(selected), 5),
           "loss_patterns": {}, "minimum_group_n": MIN_GROUP}
    for label, subset in [("all_losses", missed),
                          *[(f"{stat}_{side}_losses",
                             [r for r in missed if r["stat"] == stat and r["side"] == side])
                            for stat in sorted(STATS) for side in ("OVER", "UNDER")]]:
        if len(subset) < MIN_GROUP:
            continue
        out["loss_patterns"][label] = {
            "n": len(subset), "dates": len({r["date"] for r in subset}),
            "mean_actual_minus_projected_minutes":
                round(sum(r["minute_delta"] for r in subset) / len(subset), 4),
            "mean_minutes_contribution":
                round(sum(r["minute_term"] for r in subset) / len(subset), 4),
            "mean_rate_contribution":
                round(sum(r["rate_term"] for r in subset) / len(subset), 4),
            "indicators": {k: sum(bool(r["indicators"][k]) for r in subset)
                           for k in sorted(subset[0]["indicators"])},
        }
    return out


def live_shadow(data, before):
    """Ungraded live rotation-v4 capture; output only counts and mean shift."""
    folder = data / "v2_rotation" / "context" / "replay"
    seen = {}
    checks = {}
    for path in sorted(folder.glob("decisions_*.jsonl")):
        with path.open(encoding="utf-8") as src:
            for line in src:
                if not line.strip():
                    continue
                r = json.loads(line)
                date, at, tip, quote, updated = (r.get("date"), when(r.get("at")),
                                                 when(r.get("tip")), when(r.get("quote_at")),
                                                 when(r.get("updated_at")))
                if not isinstance(date, str) or date < before or not all((at, tip, quote, updated)):
                    continue
                if r.get("selection_policy") != "rotation-v4" or r.get("probability_method") != "linear" or r.get("period") != "game":
                    continue
                if not (quote <= at < tip and updated <= at and (at-quote).total_seconds() <= 1800
                        and (at-updated).total_seconds() <= 21600):
                    continue
                mid = r.get("model_id", "")
                if not isinstance(mid, str) or not re.fullmatch(r"[0-9a-f]{64}", mid):
                    continue
                if mid not in checks:
                    path_model = folder / f"model_{mid}.json"
                    checks[mid] = path_model.is_file() and hashlib.sha256(path_model.read_bytes()).hexdigest() == mid
                if not checks[mid]:
                    continue
                identity = ((r.get("row") or {}).get("identity") or {})
                key = (date, identity.get("GameID"), identity.get("PlayerID"), r.get("stat"))
                p, m = r.get("p1"), r.get("market")
                if not all(finite(v) and 0 < v < 1 for v in (p, m)) or type(key[1]) is not int or type(key[2]) is not int:
                    continue
                if key not in seen or at < seen[key][0]:
                    rotation = r.get("rotation") or {}
                    status = rotation.get("ownStatus", "")
                    ctx = {"own_status": status,
                           "confirmed_out": len(rotation.get("confirmedOut") or []),
                           "allocation_applied": rotation.get("allocationApplied"),
                           "rotation_supported": rotation.get("supported")}
                    seen[key] = (at, p, m, ctx)
    shifts = [(p, challenger(p, m, ctx)) for _, p, m, ctx in seen.values()]
    n = len(shifts)
    return {"questions_ungraded": n,
            "mean_absolute_probability_adjustment":
                round(sum(abs(p-c) for p,c in shifts)/n, 6) if n >= MIN_GROUP else None,
            "coverage_status": "small_cohort_suppressed" if n < MIN_GROUP else "not_outcome_evidence"}


def summarize(questions, rejected, live):
    assessed = [assess(q) for q in questions]
    selected = [r for r in assessed if r["selected"]]
    by_stat = {}
    for stat in sorted(STATS):
        part = [r for r in selected if r["stat"] == stat]
        if len(part) >= MIN_GROUP:
            by_stat[stat] = metrics(part)
    counts = Counter(r["checkpoint"] for r in assessed)
    return {
        "schema": 1, "policy_id": POLICY_ID, "source_cohort": "go_verified_rotation-v4",
        "shadow_policy": {"method": "fixed_shrink_of_v2_vs_market",
                          "normal_residual_retention": RETAIN,
                          "uncertain_residual_retention": UNCERTAIN_RETAIN,
                          "trained_on_this_audit": False,
                          "production_behavior_modified": False},
        "verified_questions": len(questions),
        "skipped_private_rows": rejected,
        "distinct_model_checkpoints": len(counts),
        "checkpoint_counts_at_least_8": {k[:12]: v for k,v in sorted(counts.items()) if v >= MIN_GROUP},
        "all_same_questions": metrics(assessed),
        "same_fixed_rule_subset": metrics(selected),
        "by_stat_fixed_rule_subset": by_stat,
        "loss_analysis": loss_summary(assessed),
        "ungraded_live_shadow": live,
        "notes": [
            "Actual V2 rotation-v4 provenance, not main-model replay or a legacy retrospective pick log.",
            "Go rotationaudit validates pregame checkpoint identity/recompute, quote, policy, captured rotation effects, final source and team identity before exporting to temporary private storage.",
            "Fixed illustrative selection >=55% probability and >=1 percentage point vs recorded offered odds is NOT the actual V2 posting/ranking policy; ROI is hypothetical and cannot be advertised as track record.",
            "The shadow variant never changes posted selections, minutes forecasts or stake units. No model is refitted using these outcomes.",
            "The pregame rotation context used for shadow confidence is frozen; actual minutes and realized stat rates are used only for retrospective arithmetic attribution.",
            "Injury, minute, and rate indicators overlap and do not establish causes. Groups smaller than 8 are suppressed.",
            "Only aggregate outputs; question-level inputs are deleted after aggregation and never placed in tracked data/ or run_evidence/."
        ],
    }


def run(data, private_file, out, before):
    try:
        questions, rejected = read_go_verified(private_file, before)
        result = summarize(questions, rejected, live_shadow(data, before))
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(json.dumps(result, indent=2, allow_nan=False) + "\n", encoding="utf-8")
        return result
    finally:
        # Never retain question-level betting information in uploaded artifacts.
        private_file.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--private-input", type=Path, default=Path("private/phase3_rotation_v4.jsonl"))
    parser.add_argument("--out", type=Path, default=Path("run_evidence/phase3/v2_challenger_loss_analysis.json"))
    args = parser.parse_args()
    # The default private input is exclusively produced by the verified Go path.
    # Reject arbitrary public-data arguments to avoid copying identifiable picks.
    if args.private_input.as_posix() != "private/phase3_rotation_v4.jsonl":
        parser.error("private input must be the fixed ignored path private/phase3_rotation_v4.jsonl")
    before = datetime.now(ZoneInfo("America/New_York")).strftime("%Y-%m-%d")
    result = run(args.data, args.private_input, args.out, before)
    print(f"V2 rotation-v4 shadow: {result['verified_questions']} Go-verified questions; "
          f"{result['loss_analysis']['selected_count']} fixed-rule research selections; "
          f"aggregates only: {args.out}")


if __name__ == "__main__":
    main()
