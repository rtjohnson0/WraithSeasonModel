#!/usr/bin/env python3
"""Overlay the latest *verified* pre-tip game markets onto the deployed dashboard.

No pregame player picks, original V2 predictions, settlements, or other current
site fields are rewritten. Missing/expired odds fail closed.
"""
import json
import argparse
from datetime import datetime, timezone, timedelta
from pathlib import Path
from urllib.request import Request, urlopen

PUBLIC_BOARD = "https://rtjohnson0.github.io/WraithSeasonModel/data/picks.json"
MAX_AGE = timedelta(minutes=30)

def utc(iso):
    result = datetime.fromisoformat(iso.replace("Z", "+00:00"))
    if result.tzinfo is None:
        raise ValueError("Missing timezone")
    return result.astimezone(timezone.utc)

def overlay(board, games, challenger, now=None):
    now = now or datetime.now(timezone.utc)
    if not isinstance(board, dict) or not isinstance(board.get("board"), dict):
        raise ValueError("Missing published pregame board")
    date = board["board"].get("date")
    if games.get("date") != date or challenger.get("date") != date:
        raise ValueError("Game market dates do not match published pregame board")
    if not isinstance(games.get("games"), list) or not isinstance(challenger.get("games"), list):
        raise ValueError("Invalid game model snapshots")
    if not any(g.get("quotes") for g in games["games"] if isinstance(g, dict)):
        raise ValueError("No priced current game markets; preserve old dashboard")
    for source in (games, challenger):
        captured = utc(source.get("at", ""))
        if captured > now + timedelta(minutes=2) or now-captured > MAX_AGE:
            raise ValueError("Game market snapshot older than 30 minutes")
    # Quote source timestamps must be before a game's tip. The Go loader also
    # applies these checks, but keep a publishing guard independent of it.
    for g in games["games"]:
        tip = utc(g.get("tip", ""))
        if now >= tip:
            raise ValueError("Game started; do not relabel any existing quote as a new pregame offer")
        for q in g.get("quotes", []):
            cap = utc(q.get("quoteAt", ""))
            upd = utc(q.get("sourceUpdatedAt", ""))
            if not (cap < tip and upd < tip) or now-cap > MAX_AGE or now-upd > timedelta(hours=6):
                raise ValueError("Stale, in-play, or undated market quote")
    updated = dict(board)
    updated["gameMarkets"] = games
    updated["gameChallenger"] = challenger
    updated["generatedAt"] = now.isoformat().replace("+00:00", "Z")
    return updated

def current_public(fallback):
    try:
        with urlopen(Request(PUBLIC_BOARD, headers={"Cache-Control":"no-cache","User-Agent":"Wraith-GameOdds/1"}), timeout=10) as f:
            data=f.read(25_000_001)
        if len(data)>25_000_000:
            raise ValueError("Public board too large")
        return json.loads(data)
    except (OSError, TimeoutError, ValueError):
        return json.loads(fallback.read_text(encoding="utf-8"))

def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--game",type=Path,default=Path("data/gameboard.json"))
    ap.add_argument("--challenger",type=Path,default=Path("data/gamechallenger.json"))
    ap.add_argument("--site",type=Path,default=Path("site/public/data/picks.json"))
    a=ap.parse_args()
    board=current_public(a.site)
    games=json.loads(a.game.read_text(encoding="utf-8"))
    challenger=json.loads(a.challenger.read_text(encoding="utf-8"))
    result=overlay(board,games,challenger)
    a.site.parent.mkdir(parents=True,exist_ok=True)
    a.site.write_text(json.dumps(result,ensure_ascii=False,separators=(",",":"))+"\n",encoding="utf-8")
    print(f"Published {len(games['games'])} priced game matchups and {len(challenger['games'])} V2 research matchups; player props untouched.")

if __name__=="__main__":
    main()
