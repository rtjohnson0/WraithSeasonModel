# Fix missing plays for 7 PM Eastern games

The old start-time parser discarded every timestamp at 00:00 UTC as a date placeholder. A genuine 7 PM Eastern tip in November is 00:00 UTC the following day. That erased the game's start time and caused both the pregame snapshot loader and the executable-quote checks to reject its props. The complete schedule still displayed the game, producing the combination of “Tip time pending” and zero plays.

The parser now checks the supplied NBA slate date in America/New_York. A real next-day midnight-UTC tip is retained; a date-only midnight placeholder falls on the preceding Eastern day and is rejected. Offset timestamps and summer daylight saving time are covered. Rows without a usable date keep the previous conservative handling. No start time is guessed from a time-only string.

The export also fills a missing game-card tip from its current schedule and puts unknown times after timed games. Empty cards explain missing schedule data separately from a game that did not pass quote/model checks.

## Install and refresh

Copy the source files from this ZIP over your project. Preserve your local .env and data. No model retraining is required for this timing fix.

From PowerShell in the project folder:

```powershell
go test ./...
if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\push.ps1 -Message "Fix midnight UTC tips hiding evening props"
if ($LASTEXITCODE -ne 0) { throw "Push failed" }
```

Then open GitHub Actions → WraithPicksSZN daily → Run workflow → mode **pretip**. A push alone selects **site**, which republishes saved state and does not refresh the feed or re-evaluate the props.

For a local refresh instead, after installing the files:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\pretip.ps1
if ($LASTEXITCODE -ne 0) { throw "Refresh failed" }
go run ./cmd/export
if ($LASTEXITCODE -ne 0) { throw "Export failed" }
```

Run while the games are still pregame. Started games correctly stay closed. New quote collection and prediction are necessary; re-exporting an old board does not regenerate picks.

## Validation

- `go test ./...`: passed, including a fixture that retains the valid 7 PM pregame quote, rejects a capture after tip, and closes the quote at tip.
- Schedule tests check that both midnight-UTC games retain their start times even with incomplete prop coverage.
- Export regression checks missing tip recovery and sorting.
- `npm test`: all 16 frontend tests passed.
- `npm run build`: passed.

This fixes a reproducible code defect matching the screenshot's missing-tip symptom. The latest live snapshots were not supplied or re-fetched here, so this is not a claim that specific TOR–PHI or DET–WAS bets now qualify. Fresh lines, player history, availability and model criteria still determine the plays. No thresholds were lowered and no historical or live lines were promoted to fill a game.
