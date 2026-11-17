# Spreads, totals and combo evidence — November 17, 2026

This patch starts a separate game-model research track and improves combo evidence diagnostics. It does not claim the live spreads or totals are now strong, does not promote the challenger, and does not manufacture combo selections. Existing prop/game LEANs and combo comparisons remain available.

## Game-market findings and changes

The existing ratings use scoring averages, not measured possession efficiency or current injury-adjusted lineups. The second-stage regression freely weights the market and ratings forecast. In the saved calibration, the total's ratings coefficient is negative, so a higher raw ratings forecast can lower the blended total. That is a fitted historical association, not a reliable basketball mechanism.

A second issue is timing: the existing offseason reset occurs during the first new game's update, after that game's features/readiness were read. The opt-in candidate expires those ratings before featuring a new season. Its team scoring ratings also adjust for the opponent's strictly earlier long-run offense/defense, with both teams reading the same pre-update state.

Research history uses explicit final game scores. The audit found 16 games in the loaded history where summing the player download disagreed with official scores. Missing or conflicting official finals are excluded. The live grading path already uses explicit final scores; this change does not rewrite any result.

`FitAnchoredMix` fits a residual correction around the sportsbook line and constrains the ratings weight to [0,1]. It cannot reverse the ratings signal or amplify its disagreement. It still needs a quote-matched evaluation; it is not activated in daily prediction.

Whole-number spread and total backtests now use the same push-conditioned probabilities as published quotes. The daily spread CSV also uses that same whole-line correction. This fixes a probability consistency issue, not demonstrated predictive edge.

## Paired raw-forecast diagnostics

Each calendar month is fitted only on earlier games. Both models are evaluated on the same games and explicit final scores, requiring readiness under both policies. These tests compare the candidate's explicit finals, pregame season reset and schedule adjustment to the legacy raw model, **not the new market blend**.

| Season | Paired games | Market | Legacy MAE | Candidate MAE | Legacy RMSE | Candidate RMSE |
|---|---:|---|---:|---:|---:|---:|
| 2024 | 1,252 | Margin | 11.180 | 11.182 | 14.321 | 14.344 |
| 2024 | 1,252 | Total | 14.859 | 14.780 | 18.565 | 18.525 |
| 2025 | 1,252 | Margin | 11.532 | 11.506 | 14.675 | 14.676 |
| 2025 | 1,252 | Total | 14.870 | 14.763 | 18.847 | 18.780 |

Totals improve modestly on average, while spreads are mixed. Total bias also worsens: actual-minus-forecast changes from -0.483 to -0.865 in 2024 and +0.272 to +1.092 in 2025. In the first three weeks of 2025 (93 paired games), margin MAE improves 10.819→10.636, but total MAE worsens 12.327→12.533. This is not enough for promotion. Variants were explored while inspecting these seasons; treat these results as development diagnostics, not an untouched final confirmation.

Historical opening-game odds are not available in this workspace. Recovery from the available source archive did not contain them. Consequently the new market-blend candidate has **not** been evaluated against the sportsbook here. Do not translate these point-error changes into a hit-rate or profitability claim.

Reproduce locally with the historical stats and already downloaded opening odds:

```powershell
go run ./cmd/games -audit -season 2025
go run ./cmd/games -backtest -season 2025 -research
```

The audit writes report.json, report.md and forecasts.csv under run_evidence/game-research. The research backtest writes gamecal_research.json; daily prediction continues using gamecal.json. `-research` requires `-backtest` and cannot silently become a live policy. If the local opening file is missing, the existing download command is `go run ./cmd/pull gameodds -season 2025`.

## Why combos are not selected

The direct combo calibration file is absent. In the isolated November 17 readiness run, 2,301 unique market rows were loaded: 2,089 belong to an earlier model, and 212 are dated November 17 and cannot be evaluated before that date completes. There are zero eligible completed current-version rows and zero qualified combo types. The earlier model's results cannot validate the changed v6 projections.

Health reporting now distinguishes unfinished dates, different model versions, different feature configurations, invalid training cutoffs, missing quote provenance and invalid probabilities. It shows the calibration/evaluation progress for PRA, PR, PA and RA separately. No selection requirement is relaxed to fill the board.

The held-out comparison now clusters paired log-loss improvement by date: each slate is one observation rather than treating many correlated player/stat lines as independent evidence. Each combo type still requires 300 training rows, 200 held-out evaluation rows, 30 held-out dates, positive improvement over raw and sportsbook probabilities, and a date-level z of at least 2. Calibrations carry the evidence-policy identifier `date-clustered-combo-v1`; older evidence policies cannot silently qualify current selections.

The simulator, its probabilities and the posted forecasts are not made stronger simply by changing this gate. The change makes qualification more honest and explains why no combination qualifies. A fresh current-version record is still needed.

## Verification

Full Go suite, 28 frontend unit tests and production build passed. Regression tests cover pregame offseason expiration without double-resetting, explicit/conflicting finals, no future-game feature leakage, symmetric schedule updates, bounded non-inverting market blends, whole-line probability consistency, combo exclusion categories and slate-level evidence counts. Historical diagnostics and combo calibration checks ran in separate output directories without changing live data or model files.
