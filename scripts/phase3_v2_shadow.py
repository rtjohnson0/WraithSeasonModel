#!/usr/bin/env python3
"""Phase 3: frozen, read-only V2 probability challenger and loss attribution.

Input: existing Go-verified replay outcomes from cmd/minutesaudit, paired with
the ORIGINAL replay quote. Never joins on names, current market lines or
postgame-reconstructed injury context. Writes aggregate-only research evidence.
"""
import argparse
import hashlib
import json
import math
import re
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path
from zoneinfo import ZoneInfo

POLICY_ID = "v2-market-residual-shrink-v1"
STATS = {"points", "rebounds", "assists", "threes"}
MIN_GROUP = 8
# Frozen, prospective research policy. NOT fitted to these outcome rows.
BASELINE_RESIDUAL_RETAIN = 0.75
UNCERTAIN_RESIDUAL_RETAIN = 0.55


def finite(x):
    return isinstance(x, (int, float)) and not isinstance(x, bool) and math.isfinite(x)


def when(x):
    try:
        v = datetime.fromisoformat(str(x).replace("Z", "+00:00"))
        return v.astimezone(timezone.utc) if v.tzinfo else None
    except (ValueError, TypeError):
        return None


def implied(odds):
    if type(odds) is not int or odds == 0:
        return None
    return 100 / (100 + odds) if odds > 0 else -odds / (100 - odds)


def context_uncertain(ctx):
    if not isinstance(ctx, dict):
        return False
    return (ctx.get("returning") is True or
            (type(ctx.get("questionable")) is int and ctx["questionable"] > 0) or
            (type(ctx.get("doubtful")) is int and ctx["doubtful"] > 0) or
            (type(ctx.get("assumedOut")) is int and ctx["assumedOut"] > 0) or
            str(ctx.get("ownStatus", "")).lower() in {"questionable", "doubtful", "probable"})


def challenger(p_over, market_over, context):
    """Prefixed conservative alternative to production's final P(Over).

    This does NOT impose an Over/Under quota or use outcomes. Existing market
    weighting is already in p_over; only the residual disagreement is shrunk.
    """
    if not all(finite(v) and 0 < v < 1 for v in (p_over, market_over)):
        raise ValueError("invalid probabilities")
    retain = UNCERTAIN_RESIDUAL_RETAIN if context_uncertain(context) else BASELINE_RESIDUAL_RETAIN
    return min(1 - 1e-6, max(1e-6, market_over + retain * (p_over - market_over)))


def question_key(date, game, player, stat, line, at):
    if not isinstance(date, str) or not isinstance(stat, str):
        return None
    if type(game) is not int or type(player) is not int or game <= 0 or player <= 0:
        return None
    if stat not in STATS or not finite(line) or not when(at):
        return None
    return (date, game, player, stat, float(line), when(at))


def obs_key(o):
    return question_key(o.get("date"), o.get("gameId"), o.get("playerId"),
                        o.get("stat"), o.get("line"), o.get("at"))


def replay_key(r):
    identity = (r.get("row") or {}).get("identity") or {}
    return question_key(r.get("date"), identity.get("GameID"), identity.get("PlayerID"),
                        r.get("stat"), r.get("line"), r.get("at"))


def replay_valid(r, data, digest_cache):
    if r.get("selection_policy", "confidence") not in ("", "confidence") or r.get("period") != "game":
        return False
    timestamp, tip, quote, updated = (when(r.get(k)) for k in ("at", "tip", "quote_at", "updated_at"))
    if not all((timestamp, tip, quote, updated)):
        return False
    if not (quote <= timestamp < tip and updated <= timestamp and
            0 <= (timestamp - quote).total_seconds() <= 1800 and
            0 <= (timestamp - updated).total_seconds() <= 21600):
        return False
    if not isinstance(r.get("calibration_before"), str) or r["calibration_before"] >= str(r.get("date")):
        return False
    if not all(finite(r.get(x)) and 0 < r[x] < 1 for x in ("p1", "market")):
        return False
    if type(r.get("book_count")) is not int or r["book_count"] < 3:
        return False
    mid = r.get("model_id", "")
    if not isinstance(mid, str) or not re.fullmatch(r"[0-9a-f]{64}", mid):
        return False
    if mid not in digest_cache:
        path = data / "replay" / ("model_" + mid + ".json")
        digest_cache[mid] = path.is_file() and hashlib.sha256(path.read_bytes()).hexdigest() == mid
    return digest_cache[mid]


def replay_index(data):
    """Pair only exactly matching Go-verified questions; ties are ambiguous."""
    index = defaultdict(list)
    digests = {}
    for path in sorted((data / "replay").glob("decisions_*.jsonl")):
        with path.open(encoding="utf-8") as handle:
            for line in handle:
                if not line.strip():
                    continue
                r = json.loads(line)
                key = replay_key(r)
                if key and replay_valid(r, data, digests):
                    index[key].append(r)
    return index


def matched_rows(report, index):
    """Go has independently checked outcomes/identity and replayed checkpoints."""
    drops = Counter()
    rows = []
    for o in report.get("observations", []):
        if o.get("cohort") != "verified replay":
            drops["non_strict_cohort"] += 1
            continue
        key = obs_key(o)
        if not key or not all(finite(o.get(k)) for k in
                              ("finalOver", "actual", "projection", "projectedMinutes", "actualMinutes", "line")):
            drops["incomplete_verified_observation"] += 1
            continue
        if o["actualMinutes"] <= 0 or o["actual"] == o["line"]:
            drops["dnp_or_push"] += 1
            continue
        candidates = [r for r in index.get(key, [])
                      if abs(r.get("p1", -100) - o["finalOver"]) <= 1e-9]
        # Multiple bookmaker markets at the same logged second must NOT be
        # silently combined or priced with a randomly chosen book.
        unique = {(r.get("book"), r.get("line"), r.get("over_odds"),
                   r.get("under_odds"), r.get("model_id"), r.get("market"))
                  for r in candidates}
        if len(unique) != 1:
            drops["missing_or_ambiguous_original_quote"] += 1
            continue
        r = candidates[0]
        over_odds, under_odds = r.get("over_odds"), r.get("under_odds")
        if implied(over_odds) is None or implied(under_odds) is None:
            drops["invalid_recorded_odds"] += 1
            continue
        market = r["market"]
        # Equivalent to the original model's market-over from two-sided prices,
        # with de-vig by normalization (no new price or bookmaker substituted).
        p0 = implied(over_odds)
        p1 = implied(under_odds)
        if abs(market - p0 / (p0 + p1)) > 1e-8:
            drops["market_odds_inconsistent"] += 1
            continue
        rows.append({"obs": o, "quote": r})
    return rows, drops


def take_side(p_over):
    return ("OVER", p_over) if p_over >= 0.5 else ("UNDER", 1 - p_over)


def payoff(odds, won):
    if not won:
        return -1.
    return odds / 100 if odds > 0 else 100 / -odds


def assess(row):
    o, r = row["obs"], row["quote"]
    p = o["finalOver"]
    market = r["market"]
    candidate = challenger(p, market, o.get("context"))
    y = 1 if o["actual"] > o["line"] else 0
    side, p_side = take_side(p)
    odds = r["over_odds"] if side == "OVER" else r["under_odds"]
    selected = p_side >= 0.55 and p_side - implied(odds) >= 0.01
    won = (y == 1) == (side == "OVER")
    minute_delta = o["actualMinutes"] - o["projectedMinutes"]
    rate = o["projection"] / o["projectedMinutes"]
    minute_term = minute_delta * rate
    rate_term = o["actual"] - o["actualMinutes"] * rate
    ctx = o.get("context")
    indicators = {
        "minutes_5plus_above": minute_delta >= 5,
        "minutes_5plus_below": minute_delta <= -5,
        "miss_direction_minutes": (side == "UNDER" and minute_delta >= 5) or
                                  (side == "OVER" and minute_delta <= -5),
        "miss_direction_rate": (side == "UNDER" and rate_term > 0) or
                               (side == "OVER" and rate_term < 0),
        "rate_term_larger_abs_than_minutes_term": abs(rate_term) > abs(minute_term),
        "pregame_context_missing": not isinstance(ctx, dict),
        "pregame_uncertain_rotation": context_uncertain(ctx),
        "pregame_confirmed_teammate_out": isinstance(ctx, dict) and
                                          type(ctx.get("confirmedOut")) is int and ctx["confirmedOut"] > 0,
        "pregame_back_to_back": isinstance(ctx, dict) and ctx.get("b2b") is True,
        "pregame_large_spread": isinstance(ctx, dict) and finite(ctx.get("spread")) and abs(ctx["spread"]) >= 10,
    }
    return {
        "date": o["date"], "stat": o["stat"], "side": side,
        "checkpoint": r["model_id"], "p": p, "market": market,
        "challenger": candidate, "won": won, "selected": selected,
        "payoff": payoff(odds, won), "minute_delta": minute_delta,
        "minute_term": minute_term, "rate_term": rate_term,
        "actual_minus_projection": o["actual"] - o["projection"],
        "outcome": y, "indicators": indicators,
    }


def safe_logloss(p, y):
    p = min(1 - 1e-12, max(1e-12, p))
    return -math.log(p if y else (1 - p))


def metrics(rows):
    if not rows:
        return {"n": 0, "days": 0, "status": "no_verified_outcomes"}
    n = len(rows)
    out = {"n": n, "days": len({r["date"] for r in rows}),
           "status": "exploratory_not_promotable" if n < 200 or len({r["date"] for r in rows}) < 20 else "requires_predeclared_forward_trial"}
    for name, field in (("production", "p"), ("challenger", "challenger"), ("market", "market")):
        out[name] = {
            "brier": round(sum((r[field] - r["outcome"]) ** 2 for r in rows) / n, 6),
            "log_loss": round(sum(safe_logloss(r[field], r["outcome"]) for r in rows) / n, 6),
        }
    out["challenger_minus_production_log_loss"] = round(out["challenger"]["log_loss"] - out["production"]["log_loss"], 6)
    return out


def loss_summary(rows):
    """Aggregate categories, no pick/player, date, line, book or price exports."""
    misses = [r for r in rows if r["selected"] and not r["won"]]
    selected = [r for r in rows if r["selected"]]
    output = {"selected_count": len(selected), "loss_count": len(misses),
              "wins": len(selected) - len(misses),
              "selected_days": len({r["date"] for r in selected}),
              "flat_roi_at_recorded_odds": round(sum(r["payoff"] for r in selected) / len(selected), 5) if selected else None,
              "loss_patterns": {}}
    for label, subset in [("all_losses", misses),
                          *[(f"{stat}_{side}_losses",
                             [r for r in misses if r["stat"] == stat and r["side"] == side])
                            for stat in sorted(STATS) for side in ("OVER", "UNDER")]]:
        if len(subset) < MIN_GROUP:
            continue
        output["loss_patterns"][label] = {
            "n": len(subset), "days": len({r["date"] for r in subset}),
            "avg_actual_minus_projected_minutes": round(sum(r["minute_delta"] for r in subset) / len(subset), 4),
            "avg_minutes_production_term": round(sum(r["minute_term"] for r in subset) / len(subset), 4),
            "avg_rate_production_term": round(sum(r["rate_term"] for r in subset) / len(subset), 4),
            "indicators": {k: sum(bool(r["indicators"][k]) for r in subset)
                           for k in sorted(subset[0]["indicators"])},
        }
    output["suppression_min_loss_count"] = MIN_GROUP
    return output


def live_shadow(index, today):
    """Shadow-only checks on ungraded pregame decisions, never player-level exports."""
    originals = {}
    for key, records in index.items():
        date, game, player, stat, line, at = key
        if date < today:
            continue
        for r in records:
            tip = when(r.get("tip"))
            if not tip or not at < tip:
                continue
            # A future market/question can be captured repeatedly. Keep the first
            # recorded decision; do not manufacture a new post-selection line.
            identity = (date, game, player, stat)
            current = originals.get(identity)
            if current is None or at < current[0]:
                originals[identity] = (at, r)
    probs = []
    for _, r in originals.values():
        p, market = r["p1"], r["market"]
        cand = challenger(p, market, r.get("audit_context"))
        probs.append((p, cand))
    n = len(probs)
    return {
        "ungraded_shadow_questions": n,
        "mean_abs_probability_change": round(sum(abs(p - c) for p, c in probs) / n, 6) if n else None,
        "over_side_flips": sum((p >= 0.5) != (c >= 0.5) for p, c in probs),
        "not_evidence_of_results": True,
    }

def summarize(report, index, today=None):
    matched, rejected = matched_rows(report, index)
    assessed = [assess(row) for row in matched]
    selected = [r for r in assessed if r["selected"]]
    by_group = {}
    for stat in sorted(STATS):
        part = [r for r in selected if r["stat"] == stat]
        if len(part) >= MIN_GROUP:
            by_group[stat] = metrics(part)
    checkpoint_counts = Counter(r["checkpoint"] for r in selected)
    if today is None:
        today = datetime.now(ZoneInfo("America/New_York")).strftime("%Y-%m-%d")
    return {
        "schema": 1,
        "policy_id": POLICY_ID,
        "ungraded_live_shadow": live_shadow(index, today),
        "mode": "frozen_read_only_research",
        "fixed_challenger": {"method": "shrink_final_model_market_residual",
                             "normal_retain": BASELINE_RESIDUAL_RETAIN,
                             "uncertain_rotation_retain": UNCERTAIN_RESIDUAL_RETAIN,
                             "trained_on_outcomes": False},
        "verified_join_count": len(assessed),
        "excluded": dict(sorted(rejected.items())),
        "checkpoint_count": len(checkpoint_counts),
        "checkpoint_cohorts": [{"checkpoint_sha_prefix": key[:12], "strict_questions": n}
                               for key, n in sorted(checkpoint_counts.items()) if n >= MIN_GROUP],
        "fixed_baseline_policy": "baseline p >= .55 and edge >= .01, one earliest frozen full-game stat question; pushes/DNP excluded",
        "same_questions_all": metrics(assessed),
        "same_baseline_selected": metrics(selected),
        "stat_on_baseline_selected": by_group,
        "loss_analysis": loss_summary(assessed),
        "notes": [
            "The challenger is evaluated on the SAME historical verified questions and SAME baseline selections, not cherry-picked new bets.",
            "This is a fixed challenger, not calibrated to the outcomes in this report. No auto-promotion, staking, production edits, or adjusted pick board.",
            "Historical comparisons are exploratory. Predeclare future evaluation dates and keep policy frozen before making any profitability claim.",
            "Observed minutes and realized stat rates are used only AFTER games for arithmetic attribution, never as pregame features.",
            "Loss indicators overlap and are correlations, not causal explanations. Missing availability context is unknown.",
            "Reports are aggregate only; no player, game, book, line, odds, or per-pick candidate values are exported."
        ],
    }


def run(data, audit_path, out):
    report = json.loads(audit_path.read_text(encoding="utf-8"))
    result = summarize(report, replay_index(data))
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(result, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--audit", type=Path, default=Path("run_evidence/minutes-audit/report.json"))
    parser.add_argument("--out", type=Path, default=Path("run_evidence/phase3/v2_challenger_loss_analysis.json"))
    args = parser.parse_args()
    result = run(args.data, args.audit, args.out)
    print("Phase 3 shadow audit:", result["verified_join_count"], "exact verified joins;",
          result["loss_analysis"]["selected_count"], "baseline qualifying selections;",
          "wrote aggregates only to", args.out)


if __name__ == "__main__":
    main()
