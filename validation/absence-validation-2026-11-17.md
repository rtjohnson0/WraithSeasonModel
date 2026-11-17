# Timestamped absence validation, November 17, 2026

## Source and reproducibility

Authenticated source: `rtjohnson0/WraithSeasonModel`, successful Actions run `44109947683`, artifact `14341228393` (`model-run-44109947683-1`). Artifact digest: `sha256:40cffc050a28a115f1b7ceb2b0f1dec833d2d15a9726208a7a13601591ecffd8`.

The artifact contains November 13–17 immutable V1 and rotation captures, their frozen model checkpoints, timestamped injury/quote snapshots, and final 2026 box scores under `run_evidence/stats_2026.jsonl`. The final stats were copied into an isolated extraction's data directory for settlement. Production files were not changed.

The existing paired audit was reproduced exactly: 378 completed player-games, four dates (November 13–16), and 1,376 non-push binary prop questions. Checkpoint hashes, pre-tip decision and quote timing, original probability reproduction, earlier training cutoff, and final player/game/team identity are checked. Minutes are counted once per player/game. Correlated prop questions are not independent observations.

## What was actually tested

This is the existing `rotation-v1` experimental team-minutes allocation versus the same frozen model and the same 50/50 model/book blend WITHOUT that allocation. It is not a comparison against V1's production calibrated probabilities, not a fitted causal injury model, and not a test of enabling the old `-lineup` feature groups.

The completed evaluated questions all used model version 4, trained through November 9. Version 6 captures appear only on November 17, which was held out as incomplete. The active repository model is version 6, trained through June 13, 2026 (91,006 feature rows). This training cutoff should be checked when preparing the next candidate, but does not itself invalidate strictly earlier training or prove a performance problem.

## Results

Lower minutes MAE is better. A zero-sized cohort has unavailable accuracy, not zero error.

| Frozen cohort | Player-games | Dates | Prop questions | Baseline minutes MAE | Rotation minutes MAE |
|---|---:|---:|---:|---:|---:|
| All | 378 | 4 | 1,376 | 4.660 | 5.438 |
| No confirmed out | 32 | 3 | 118 | 5.073 | 5.988 |
| Confirmed outs with no recent minutes to redistribute | 55 | 4 | 189 | 4.429 | 4.742 |
| Unsupported/fallback absence allocations | 299 | 4 | 1,069 | 4.635 | 5.497 |
| Supported joint absences, at least three matching earlier rotations | 0 | 0 | 0 | unavailable | unavailable |

Across all questions, Brier scores (baseline -> rotation) are assists 0.25264 -> 0.25271; points 0.25675 -> 0.25302; rebounds 0.24704 -> 0.24756; threes 0.25590 -> 0.25254. Mixed probability results across four dates do not justify promoting the complete policy, and the worse aggregate minutes error argues against blindly applying its fallback.

## Decision

Do not describe all 378 examples as validated injury adjustments. Most are fallback allocations; 'supported' with no relevant rotation absences is also not evidence for an injury effect. The actual joint-absence path has no completed eligible examples in this archive.

The earlier uploaded-pick scenario analysis (12 questions, 10 player-games, approximately 0.7-minute MAE improvement) examined a different historical with/without-minutes sensitivity calculation. It is not interchangeable with this larger frozen rotation audit and does not validate all teammate or opponent features.

Opponent-absence per-minute-rate features remain UNTESTED by this comparison: the captured model checkpoints disable `teammates`, `oppout`, and `misses`. A zero-weight feature ablation cannot establish that an opponent effect helps. Enabling those groups in a historically realized-participation backtest would not repair the pregame information mismatch.

## Audit improvements

`cmd/rotationaudit` now reports checkpoint model versions and accepts `-cohort`. Cohorts are assigned AFTER freezing the earliest eligible question, avoiding selection of a later injury observation for the same question. Confirmed-out evidence must have a player ID, explicit Out status, and a nonfuture timestamp no older than 24 hours at decision time. Missing, questionable, future, and stale observations are excluded from absence validation.

Supported means at least three matching earlier rotations plus the stored support flag. This is a sample-availability label, not statistical performance certification. The 'irrelevant-outs' cohort follows the existing stored rotation reason; future capture schemas should record an explicit relevant-out list instead of requiring reason-text interpretation.

## Next experiment

Keep recording complete pre-tip inputs/checkpoints for both baseline and candidate, including confirmed and uncertain status scenarios. Fit candidate absence/role features only from earlier timestamp-valid records, using dates/games rather than prop rows as the separation unit. Compare minutes and stat-rate forecasts on matched later player-games, then evaluate separately fitted probabilities on untouched quotes. Report relevant-absence support and coverage along with errors; do not assume all present absences imply the same role change. Neither blanket boosts nor forced over counts are justified by these results.

This PR changes only diagnostics and records findings. It does not enable V1 features, replace model/calibration files, alter leans, rewrite history, or deploy a new prediction policy.

## Reproduce

After downloading the identified run evidence and placing the retained final stats in its isolated `data` directory:

```powershell
go run ./cmd/rotationaudit -data PATH_TO_EVIDENCE\data -before 2026-11-17 -cohort all -out run_evidence\absence-validation\all
go run ./cmd/rotationaudit -data PATH_TO_EVIDENCE\data -before 2026-11-17 -cohort supported-joint-absence -out run_evidence\absence-validation\supported
go run ./cmd/rotationaudit -data PATH_TO_EVIDENCE\data -before 2026-11-17 -cohort unsupported-fallback -out run_evidence\absence-validation\fallback
```
