# Minutes and Under-selection audit — November 13, 2026

Source: public `rtjohnson0/WraithSeasonModel` data pinned to commit
`1b7869182cdb9620c3bc7b0b1524a50ae46cefce`.
Final stats extend through November 12; projection logs extend through November 13.

## Findings

The retained historical projection sample contains 667 unique player-games on 15 dates (October 27–November 12), covering 2,411 player/game/stat questions. These are exploratory legacy diagnostics: old projection logs do not contain enough frozen forecast provenance to certify forward performance.

| Minutes measure | Result |
| --- | ---: |
| Mean projected minus actual | -0.12 minutes |
| Model mean absolute error | 4.17 minutes |
| Saved typical-minutes baseline mean absolute error, same sample | 4.85 minutes |
| Underestimated by at least five minutes | 108 player-games |
| Overestimated by at least five minutes | 100 player-games |

The model has substantial individual minutes errors, but this sample does not support a blanket increase. For the 57 player-games projected at least five minutes below their saved typical workload, the model's minutes MAE was 4.47 versus 6.97 for the typical baseline. Historical availability context was not captured; confirmed-out, questionable and returning-player subgroup effects cannot be established from these rows.

Average actual-minus-projected production was -0.27 points, -0.06 rebounds, -0.07 assists and +0.06 threes. These averages do not establish broad underprojection. Averages can conceal player-specific errors and should not be used to justify an arbitrary adjustment.

The saved November 13 side summary contains 423 representative markets. Raw probabilities lean Under in 320 (75.7%); final probabilities lean Under in 302 (71.4%). Calibration reduces the overall concentration in that snapshot. None of its representative Over quotes meets both probability and price checks. Posted-play counts can differ because they can use another quote or include leans.

Distribution shape and exact-price filtering require further investigation. Count distributions can place more than half their probability below a line even when the mean is near or above it. Under concentration alone does not prove miscalibration or establish a profitable replacement.

## Changes included

- A read-only `cmd/minutesaudit` command and private HTML/JSON reports, automatically attempted after player-prop prediction in daily and pre-tip scripts.
- Unique player-game minutes scoring, a matched typical-minutes baseline, and an exact retrospective minutes/rate error decomposition.
- Separate strict replay and legacy diagnostic cohorts. Missing historical injury context stays unknown.
- New replay records capture pregame confirmed outs, assumed outs, questionable/doubtful teammates, own status, returning-player status, back-to-back context and spread.
- Future all-market logs save raw/book probabilities, odds, training metadata and quote timestamps alongside final probabilities.

No model weights, probability corrections, selection thresholds or public performance records were changed. The report is retained in workflow evidence rather than added to the public website.

## Run locally

From the repository root, in PowerShell or Bash:

```sh
go run ./cmd/minutesaudit -legacy
```

Open `run_evidence/minutes-audit/report.html`. JSON is saved beside it. `-legacy` explicitly enables the separate older-log diagnostic. Without that flag, only eligible frozen replay decisions are considered for outcome scoring. Forecast-stage forensic counts remain clearly labelled and are not verified outcomes.

## What is still needed

This public repository contains no frozen `data/replay` captures, so this audit has zero verified replay questions. Download the latest GitHub Actions `model-run-…` evidence artifact containing `data/replay/decisions_*.jsonl` and their matching `model_<sha>.json` checkpoints. Those original snapshots are needed to reproduce probabilities and separate minutes, distribution and calibration effects against final outcomes.

After reviewing them, define one fixed challenger and evaluate it on later, newly captured dates. Do not reuse the previously inspected final test for model selection, force an Over quota, or copy exploratory results into the public record.
