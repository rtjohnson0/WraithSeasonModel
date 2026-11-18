# V2 opportunity and pregame context

## Scope and data sources

V2 policy `rotation-v4` adds earlier potential assists, rebound chances, shot volume, pregame lineup status, spread/workload research and minutes sensitivity. V1's feature list, calibration and record remain unchanged. This policy has its own logs, settlement file and replay directory.

The supplemental downloader uses the official BALLDONTLIE endpoints:

| Endpoint | Captured inputs |
|---|---|
| `/nba/v1/stats/tracking` | passing, rebounding, possessions, drives, catchshoot, pullupshot |
| `/nba/v1/stats/defense` | less_than_6ft, 3_pointers |
| `/nba/v1/stats/shooting` | defender_distance |
| `/nba/v1/lineup_stats` | historical five-player lineup statistics |
| `/nba/v1/matchups` | historical player/defender matchups |
| `/nba/v1/projected_lineups` | expected/confirmed pregame starting five |

References: https://docs.balldontlie.io/#game-tracking-stats and https://docs.balldontlie.io/#projected-lineups. Tracking is full-game, delayed data. The ordinary `/lineups` endpoint is post-start and is deliberately not used for pregame features.

## Opportunity model

A scorer's opportunity volume and conversion are separate. The latest ten eligible games supply volume per minute; up to forty earlier games supply conversion. The features are potential assists × assist conversion, rebound chances × rebound conversion, 3PA × make conversion, and `(FGA + .44 × FTA)` × points per shooting opportunity. The latter is an estimate, not a measured possession count.

These expected per-minute rates are normalized against the player's strictly earlier baseline. Each stat uses its own opportunity input in a joint ridge regression alongside the existing fourteen lineup-composition features. Effects are not independent bonuses: components add, with a total ±8% cap and per-component ±5% caps. No forced direction or Over quota is used. The new policy orders higher estimated hit probability first within tiers; existing qualification checks and LEANs remain.

At least three covered recent games, 60% coverage of the recent window, coverage reaching one of the player's last two games, and recent volume within sixty days are needed. A fitted opportunity coefficient also needs 300 league player-games on fifteen earlier dates. These are league evidence floors, not repeated opponent or injury combinations. Unknown/null counts remain unknown; explicit zero remains zero. Inconsistent rebound chance totals are rejected, not repaired. All used player/game/team identities must match an explicit final box score, and tracking timestamps cannot exceed the forecast timestamp.

The fit uses only games before the forecast date. Its historical rotation inputs are observed final rotations, while live inputs are projected rotations. It is observational research and is not calibrated proof of a causal injury effect or a reliable hit rate.

## Pregame starters and competitiveness

Only timestamped pregame projected-lineup captures are accepted, checked against current rosters and the scheduled game. Each must contain five unique players/slots and retain `expected` versus `confirmed`. Post-tip captures, future revisions, stale captures and conflicts with fresh confirmed-Out reports are excluded. A provider's lineup slot can inform replacement positions; it does not guarantee minutes or identify a primary defender.

The spread/workload model fits earlier actual minutes relative to each player's earlier workload using historical **opening** game lines. It needs 1,000 player-games and thirty dates, caps the added change at ±3 minutes, and is suppressed when the primary minutes model already has a fitted spread effect. Missing historical game lines produce no extra adjustment. The separate large-margin diagnostic estimates a 15+ point final margin; it is not the chance a particular starter sits the fourth quarter.

Sensitivity scenarios come from the existing predictor's minutes distribution. They do not add another uncertainty penalty or claim to be confidence intervals. Return-to-play restrictions, exact defensive coverages and primary-defender assignments remain unknown when no reliable structured source provides them; they are not invented from an injury label.

## What is collected versus numerically fitted

Potential assists, rebound chances and box-score shooting opportunities enter the new conditional rate fit when their coverage floors are met. Touches, drives, catch-and-shoot/pull-up volume, shot spacing and measured FG allowed inside six feet/on threes appear as descriptive earlier-game profiles when covered. Five-player lineup ratings and individual matchups are retained for subsequent controlled experiments. Those additional defensive percentages and tiny matchup samples are **not** given arbitrary probability multipliers. Blocks in the existing composition model remain a limited proxy, not measured rim protection.

## Downloads and persistence

`pull context` saves pages and cursors after each response and round-robins feeds. Separate queue positions prevent a small download budget repeatedly starving later feeds. The budget is soft: it is checked between requests, so an API retry can extend it. Server rate limits are honored. Completed recent windows can be refreshed after three hours; new dates receive their own recent window. Current-season historical windows advance after completion.

Daily makes small bounded core and wider-context pulls; pre-tip refreshes core opportunities before the fresh quote/injury snapshot and captures pregame projected lineups afterward. Raw context remains in the input cache, not in git. No API key is written to data or evidence.

The separate **BDL context backfill** workflow starts on merge of this downloader/workflow and nightly, with a four-hour soft budget and a 270-minute job limit. It never publishes or replaces a board. Its progress is cached and each batch is uploaded with one-day retention. Daily imports the newest same-branch artifact before its own refresh, merging JSONL observations by identity and capture time so a background snapshot cannot overwrite newer daily data. The importer uses disk-backed merging, validates archive paths, and does not forward GitHub authorization to the storage redirect.

## Initial evidence

The checked-in `opportunity-study-2026.json` uses a fit cutoff of 2026-01-01 and later observed rotations through 2026-07-01. It is a conditional exposure study, **not a pregame backtest**. Points MAE changes 4.2299 → 4.1829; threes 1.0216 → 1.0134; rebounds worsen 1.8207 → 1.8307 and assists worsen 1.3881 → 1.3937. Improvements here are from box-score shooting-volume inputs. Historical potential-assist/rebound-chance inputs in the current partial download have zero eligible training cases for this historical cutoff, so these results do not validate the new tracking feeds.

Future immutable pregame decisions, matched with verified later finals, remain the meaningful live comparison. No claim of 60–62% accuracy is supported by this change.

## PowerShell

After merge, normal Daily handles bounded downloads and V2 prediction. A longer local download can be resumed with:

```powershell
go run ./cmd/pull context -seasons 2026,2025 -maxminutes 0
go run ./cmd/pull context -live
go run ./cmd/predict -rotationpolicy rotation-v4
go run ./cmd/rotationaudit -policy rotation-v4 -out run_evidence/context-audit
```

`0` means unlimited until completion; Ctrl+C leaves completed pages available for resuming. This downloads supplemental context, not opening props or the existing multi-year box scores again.
