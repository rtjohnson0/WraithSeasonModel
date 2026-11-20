#!/usr/bin/env python3
"""Read-only closing-line audit. Never infers or fabricates missing market quotes.

Requires the same player, sportsbook, stat, period and exact handicap.
Only SGO-origin picks can be paired to SGO snapshots. Outputs aggregate
coverage/CLV to run_evidence, not a public board or committed pick log.
"""
import argparse
import json
import re
from collections import Counter
from datetime import datetime, timezone, timedelta
from pathlib import Path


def instant(value):
    try:
        result = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        return result.astimezone(timezone.utc) if result.tzinfo else None
    except (ValueError, TypeError):
        return None


def name_key(name):
    parts = re.sub(r"[^a-z0-9 ]", " ", str(name).lower()).split()
    return " ".join(p for p in parts if p not in {"jr", "sr", "ii", "iii", "iv", "v"})


def book_key(name):
    return str(name).lower().removeprefix("sgo/").strip()


def implied(odds):
    if not isinstance(odds, int) or isinstance(odds, bool) or odds == 0:
        return None
    return 100 / (100 + odds) if odds > 0 else -odds / (100 - odds)


def rows(path):
    if not path.exists():
        return
    with path.open(encoding="utf-8-sig") as handle:
        for line in handle:
            if line.strip():
                yield json.loads(line)


def snapshot_index(folder):
    index = {}
    for path in sorted(folder.glob("sgo_*.jsonl")):
        if path.name.startswith("sgo_raw_"):
            continue
        date = path.stem.removeprefix("sgo_")
        for row in rows(path):
            # The Go SnapshotRow embeds PlayerLine with exported PascalCase fields.
            at = instant(row.get("captured_at"))
            tip = instant(row.get("event_start") or row.get("EventStart"))
            updated = instant(row.get("updated_at") or row.get("UpdatedAt"))
            if not at or not tip or not updated or at >= tip or updated > at:
                continue
            if row.get("pair_verified", row.get("PairVerified")) is not True:
                continue
            line = row.get("Line", row.get("line"))
            if line is None or line != row.get("under_line", row.get("UnderLine")):
                continue
            if str(row.get("period", row.get("Period", "game"))).lower() != "game":
                continue
            over = row.get("OverOdds", row.get("over_odds"))
            under = row.get("UnderOdds", row.get("under_odds"))
            if implied(over) is None or implied(under) is None:
                continue
            key = (date, name_key(row.get("Name", row.get("name", ""))),
                   str(row.get("Stat", row.get("stat", ""))).lower(),
                   book_key(row.get("Book", row.get("book", ""))))
            index.setdefault(key, []).append((at, tip, updated, float(line), over, under))
    return index


def evaluate(picks, snapshots, close_window_minutes=45, quote_age_minutes=15):
    index = snapshot_index(snapshots)
    counts = Counter()
    measurements = []
    seen = set()
    for path in sorted(picks):
        for row in rows(path):
            counts["input_rows"] += 1
            if row.get("period", "game") not in ("game", "") or str(row.get("side", "")).upper() not in ("OVER", "UNDER"):
                counts["unsupported_period_or_side"] += 1
                continue
            book = str(row.get("book", ""))
            if not book.lower().startswith("sgo/"):
                counts["not_sgo_origin"] += 1
                continue
            logged = instant(row.get("logged_at"))
            tip = instant(row.get("tip"))
            quote = instant(row.get("quote_at"))
            original_odds = row.get("odds")
            if not logged or not tip or not quote or not quote <= logged < tip or implied(original_odds) is None:
                counts["invalid_original_provenance"] += 1
                continue
            key = (row.get("date"), row.get("game_id"), row.get("player_id"), row.get("stat"),
                   row.get("side"), book_key(book), row.get("line"), original_odds)
            if key in seen:
                counts["duplicate_original"] += 1
                continue
            seen.add(key)
            counts["eligible_originals"] += 1
            lookup = (row.get("date"), name_key(row.get("player", "")),
                      str(row.get("stat", "")).lower(), book_key(book))
            candidates = []
            wrong_line = False
            for at, start, updated, line, over, under in index.get(lookup, []):
                # Never pair another game or a post-tip quote, even if the names match.
                if start != tip or not logged <= at < tip:
                    continue
                if tip - at > timedelta(minutes=close_window_minutes):
                    continue
                if at - updated > timedelta(minutes=quote_age_minutes):
                    continue
                if abs(float(row.get("line", -1)) - line) > 1e-9:
                    wrong_line = True
                    continue
                odds = over if row["side"].upper() == "OVER" else under
                candidates.append((at, odds))
            if not candidates:
                counts["changed_handicap_unpaired" if wrong_line else "closing_quote_missing"] += 1
                continue
            at, close_odds = max(candidates, key=lambda c: c[0])
            original = implied(original_odds)
            closing = implied(close_odds)
            # Positive: our locked price has less implied break-even risk than the close.
            edge = closing - original
            measurements.append(edge)
            counts["paired_same_handicap"] += 1
    n = len(measurements)
    return {
        "schema": 1,
        "metric": "closing_implied_break_even_change",
        "coverage": dict(sorted(counts.items())),
        "paired_count": n,
        "coverage_rate": n / counts["eligible_originals"] if counts["eligible_originals"] else None,
        "mean_probability_point_clv": sum(measurements) / n if n else None,
        "positive_clv_share": sum(v > 0 for v in measurements) / n if n else None,
        "notes": [
            "CLV is closing implied probability minus original implied probability at the identical handicap; positive indicates price improvement.",
            "Different handicaps are explicitly unpaired, not converted to synthetic CLV.",
            "Same SGO sportsbook, player, game tip, full-game stat and pre-tip snapshot required.",
            "This is a price-movement diagnostic, not verified profitability; no public pick details are exported."
        ],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, default=Path("data"))
    parser.add_argument("--out", type=Path, default=Path("run_evidence/closing_line_summary.json"))
    args = parser.parse_args()
    report = evaluate([args.data / "picklog.jsonl", args.data / "v2_rotationlog.jsonl"],
                      args.data / "snapshots")
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print("Closing-line audit:", report["paired_count"], "same-handicap matches;",
          report["coverage"].get("eligible_originals", 0), "eligible originals")


if __name__ == "__main__":
    main()
