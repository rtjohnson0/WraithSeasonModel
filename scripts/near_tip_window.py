#!/usr/bin/env python3
"""Schedule guard for quote-only GitHub workflow, using Eastern local time."""
from datetime import datetime, time
from zoneinfo import ZoneInfo


def in_window(now):
    local = now.astimezone(ZoneInfo("America/New_York"))
    # Games typically begin after 17 ET; preserve the late-night West Coast window.
    return local.time().replace(tzinfo=None) >= time(17) or local.time().replace(tzinfo=None) < time(1)


if __name__ == "__main__":
    import sys
    if not in_window(datetime.now(ZoneInfo("UTC"))):
        print("Outside Eastern evening window; no SportsGameOdds API request.")
        sys.exit(1)
    print("In Eastern evening window; capturing quotes.")
