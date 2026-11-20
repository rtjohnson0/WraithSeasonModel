# Phase 2: near-tip quote coverage and minutes/usage diagnostics

This is a **read-only evidence** change. It does not change the player prop model,
calibration, selection policy, staking, market pricing, game predictions, or
Whop/public publishing configuration.

## Near-tip odds

The `near-tip-odds.yml` workflow is separate from the daily model workflow.
It schedules a sportsbook-only `go run ./cmd/sgo snapshot` on the default
branch every 20 minutes during UTC hours 21:00–05:59, guarded by actual
America/New_York time (17:00–00:59 ET). The extra UTC hours account for DST.
GitHub scheduled jobs may be delayed, dropped, or run after the last usable quote.

Snapshot history is retained in a dedicated rolling GitHub Actions cache and
30-day artifacts. The regular daily workflow restores the isolated
`data/near_tip_snapshots` cache **after** its usual input restore and runs an
idempotent append-only merge into `data/snapshots`. This prevents replacing
an earlier market capture with a newer copy. The existing CLV script still refuses a
snapshot at/after tip, more than 45 minutes before tip, stale over 15 minutes,
wrong sportsbook, wrong player, or different handicap.

Do not interpret the **last observed pre-tip** quote as the true market closing
quote if the bookmaker stopped updating early. CLV coverage will reveal how
much measurable evidence exists. No CLV is manufactured when lines move.

The quote-only job needs `SPORTSGAMEODDS_API_KEY` already configured in
GitHub Actions. It uses provider API requests and GitHub Actions minutes,
so monitor cost/quotas and adjust cadence if necessary.

## Minutes and usage context

After `cmd/minutesaudit -legacy`, the new
`scripts/minutes_context_diagnostic.py` reads the private audit's frozen
observations. It summarizes error by:
- confirmed/assumed teammate absences and uncertainty at forecast time;
- returning players, back-to-backs, large pregame spreads;
- projected minutes at least five above/below the saved typical workload;
- rows with unavailable pregame context (reported separately).

Samples count **one earliest forecast per player/game/cohort**, not all the
points/rebounds/assists/threes questions for the same player. Verified replay
and exploratory legacy cohorts are never pooled. Fewer than 30 player-games
or 10 dates receive an insufficient-evidence warning. The diagnostics are not
a learned correction and do not assume missing context means no absences.

Output: `run_evidence/minutes-context/report.json` (ignored by Git).
These reports can contain player-level data in the underlying minutes audit;
keep them in restricted workflow artifacts and never publish to the site.

## Tests

```bash
python3 -m unittest discover -s scripts -p test_near_tip_minutes_context.py -v
python3 -m unittest discover -s scripts -p test_forward_clv_audits.py -v
```

The existing production tests are unchanged. Do not merge until the
`Audit integrity checks` workflow passes.

## Known limitations

* The quote-only job does not backfill gaps or prove that a captured quote was
  the final sportsbook quote; compare actual quote coverage before claiming CLV.
* GitHub cache retention/eviction or concurrent scheduled runs may leave gaps.
  Snapshot lines are deduplicated when imported; cache presence is not proof of
  complete market coverage.
* The context analysis does not capture player-level opportunity changes
  (e.g. potential assists/touches) unless present in the frozen audit report.
  These should be tracked prospectively before any usage correction is tested.
