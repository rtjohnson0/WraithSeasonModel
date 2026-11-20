# Prop calibration refresh: BDL context vs sportsbook opening odds

This is an evidence-only improvement. **Existing production predictions and
GitHub Pages publication are unchanged.** No candidate calibration or
retrained model is automatically adopted.

## Why the current pipeline was blocked

Two independent BDL datasets are involved:

1. `BDL context backfill` (`.github/workflows/context-data.yml`):
   tracks passing, rebounding, shot quality/opportunities, drives and defensive
   context for the current and previous NBA seasons. It keeps cursor files
   in the `data/context` Actions cache. It is **not** the opening odds feed.
2. Historical **player prop opening lines**:
   `go run ./cmd/pull props -season 2025` fetches
   `/nba/v2/odds/player_props/opening`, one historical game per request.
   This is the source used by `cmd/backtest` to fit the primary model's
   market-aware calibration with chronological evaluation.

The last saved `data/model_refresh.json` before these changes showed
214 / 1,315 games for historical opening props and `backfilling`.
The November 20 workflow logs confirmed the fetch reached 214 and paused at
its configured five-minute budget, with other live-odds requests receiving
HTTP 429 rate limits. **This does not imply BDL context downloads are absent.**

## New independent, resumable downloader

`.github/workflows/prop-history-backfill.yml` runs separately once daily
with a 15-minute default download budget (override with repository variable
`WRAITH_PROP_BACKFILL_MINUTES`, 1–24). It:

- Restores the normal input cache to seed already downloaded games.
- Restores a dedicated rolling `data/prop_backfill` cache.
- Imports unique valid opening responses into the normal raw odds file.
- Resumes the actual BALLDONTLIE opening-lines endpoint (no model or picks).
- Saves the independent rolling cache and uploads **aggregate coverage only**.

Import/staging uses `scripts/prop_history_cache.py`: the first valid response
for a game is retained, newer conflicting copies never rewrite it, truncated
final lines are ignored, and unexpected corruption fails closed. Raw odds are
kept in ignored Actions caches, not source commits or public site JSON.

GitHub scheduled jobs may be delayed/skipped; API rate limits and cache
eviction can still slow progress. Treat status `backfilling` as real.

## Safer training after historical odds are complete

Previously `scripts/refresh_models.py` could overwrite `data/model.json`
and `data/calibration.json` after historical odds reached 100%, before a
separate current-version prospective comparison.

Now daily refresh **never auto-promotes** model parameters. After the complete
backfill, it creates an isolated temporary directory containing symlinks to
historical feature data and sportsbook prices, runs `cmd/train` and the
existing month-wise `cmd/backtest`, and verifies a matching model/calibration.
The backtest fits on earlier dates and evaluates later held-out questions,
including a devigged-market log-loss comparator.

Only an aggregate readiness summary is retained at
`run_evidence/backfill/candidate_readiness.json`. The candidate binaries
and calibration are deleted when the temporary directory closes.

A candidate's retrospective pass is **not** proof of live edge. A separate
reviewed promotion path should require sufficiently many independent frozen
post-change V2 rotation-v4 forecasts with proper odds/line matching and
date-clustered gain over the current production model and market baseline.

**There is no automatic promotion in this PR.** Normal production continues
using the existing paired model and calibration. If the historical candidate
fails, daily prediction continues; if the current production pair itself is
incompatible, the refresh fails closed instead of silently switching models.

## Test commands

```sh
python3 -m unittest discover -s scripts -p test_prop_history_cache.py -v
python3 -m unittest discover -s scripts -p test_model_refresh.py -v
python3 -m unittest discover -s scripts -p 'test_*.py'
go test ./cmd/pull/...
```

The first two test files use synthetic data; no secrets or paid picks are used.
Provider usage and cache retention must be monitored after merging.
