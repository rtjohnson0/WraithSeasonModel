#!/usr/bin/env python3
"""Attach a same-slate in-game snapshot to the currently published public board.

Do not regenerate the original slate or alter the pregame lines. Never publish
a current book percentage under a model probability label. Only the existing
public Pages board can be used; protected Whop mode is intentionally unsupported
until an authorized in-game gateway is provided.
"""
import argparse
import json
from datetime import datetime, timezone
from pathlib import Path
from urllib.request import Request, urlopen

PUBLIC_BOARD = "https://rtjohnson0.github.io/WraithSeasonModel/data/picks.json"


def accept(board, halftime):
    if (not isinstance(board, dict) or not isinstance(halftime, dict)
            or halftime.get("schema") != 1 or not isinstance(halftime.get("plays"), list)):
        raise ValueError("Invalid board or verified halftime format")
    date = halftime.get("date")
    if not isinstance(board.get("board"), dict) or board["board"].get("date") != date:
        raise ValueError("Public board must match current halftime game date")
    at = halftime.get("capturedAt")
    try:
        instant = datetime.fromisoformat(at.replace("Z", "+00:00"))
    except (ValueError, AttributeError) as exc:
        raise ValueError("Missing verified halftime capture time") from exc
    if instant.tzinfo is None:
        raise ValueError("Naive halftime capture timestamp")
    if abs((datetime.now(timezone.utc)-instant).total_seconds()) > 8*60:
        raise ValueError("Refusing to publish an outdated in-game snapshot")
    # Work with a fresh JSON object, preserving the entire original public
    # dashboard: records, source timestamps, historical picks, game markets.
    out = dict(board)
    out["halftime"] = halftime
    out["generatedAt"] = instant.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")
    return out


def load_public_board(local):
    # A live deployed board is preferable to the repository's older saved
    # baseline, because it preserves the latest player charts / pick record.
    try:
        with urlopen(Request(PUBLIC_BOARD, headers={"Cache-Control":"no-cache",
                     "User-Agent":"Wraith-Halftime/1"}), timeout=8) as response:
            data = response.read(25_000_001)
        if len(data) > 25_000_000:
            raise ValueError("Public board exceeds safe input size")
        return json.loads(data)
    except (OSError, TimeoutError, ValueError, json.JSONDecodeError):
        return json.loads(Path(local).read_text(encoding="utf-8"))


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--source", type=Path, default=Path("data/halftime.json"))
    ap.add_argument("--local", type=Path, default=Path("site/public/data/picks.json"))
    ap.add_argument("--out", type=Path, default=Path("site/public/data/picks.json"))
    args = ap.parse_args()
    latest = json.loads(args.source.read_text(encoding="utf-8"))
    report = accept(load_public_board(args.local), latest)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, ensure_ascii=False, separators=(",", ":"))+"\n", encoding="utf-8")
    print(f"Published {len(latest['plays'])} live tracked props. Original board/pregame prices unchanged.")


if __name__ == "__main__":
    main()
