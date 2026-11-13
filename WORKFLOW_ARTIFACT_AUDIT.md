# Workflow artifact audit — November 13, 2026

Input: `model-run-43458527780-1.zip`. The original upload remains unchanged.

## Confirmed defect and fix

The frozen replay rows contain correct player/game IDs, but blank player names and team abbreviations. The final-stat identity check correctly rejects the blank name. This prevents these rows from contributing to forward accuracy diagnostics; it does not establish that the published bets had incorrect actuals.

`decisionIdentity` now saves the same player name and team used by the displayed pick, alongside the quote's game ID and date. A regression test checks those fields and verifies that capture does not mutate the original feature row. The minutes audit now reports missing forecast names explicitly rather than grouping them with unavailable finals. Existing settlement identity requirements remain unchanged.

## Supplemental analysis of November 12

For a private, separate diagnostic, names and team abbreviations were recovered from the original pre-tip all-market logs using exact date, game and player IDs, matching tip times and requiring an unambiguous name/team pair. Names were not obtained from final stats. Neither the original artifact nor live records were rewritten. These recovered rows are supplemental evidence, not originally complete replay records.

The earliest eligible forecast per player/game/stat gives 102 questions for 29 players across two games on one completed date. Minutes are counted once per player/game. The model checkpoint hash and numerical probabilities reproduce exactly.

| Measure | Result |
| --- | ---: |
| Mean projected minus actual minutes | +1.34 |
| Minutes mean absolute error | 4.87 |
| Typical-minutes baseline MAE, same players | 5.74 |
| Raw Under leans | 65 / 102 |
| Final Under leans | 69 / 102 |
| Under leans with calibration intercept set to zero, sensitivity only | 61 / 102 |
| Under leans with typical minutes substituted, sensitivity only | 63 / 102 |

Removing the intercept or substituting typical minutes changes some sides but still leaves an Under-heavy sample. This is not evidence those changes improve future bets. A right-skewed count distribution can lean Under even when its mean is above the line: that occurs in 21 of these raw Under questions. Probability/price thresholds further change which sides become picks.

| Largest absolute minutes errors | Projected | Actual |
| --- | ---: | ---: |
| P.J. Washington | 32.7 | 16.4 |
| Santi Aldama | 22.8 | 9.8 |
| Oso Ighodaro | 23.1 | 31.5 |
| Cooper Flagg | 35.0 | 27.4 |
| Grayson Allen | 26.9 | 34.1 |
| Jaden McDaniels | 32.4 | 25.3 |

These are observed misses, not inferred explanations. This sample does not establish whether an injury, rotation decision, foul trouble or game script caused them. November 12 replay rows lack saved availability context. November 13 has 422 new rows with context, but those games were not completed when this artifact was captured.

## Status of the two model improvements

1. **Minutes accuracy:** the capture defect is fixed, and diagnostics identify individual errors. No blanket minutes boost is supported: both this one-day sample and the larger legacy sample perform worse using typical minutes. An injury-aware adjustment needs evaluated forecasts with their actual pregame availability context.
2. **Under concentration:** reproduced and separated into raw-distribution and calibration stages. The artifact supports diagnosis but contains only two completed games; it does not validate a new probability correction. Model weights, calibration coefficients and pick thresholds remain unchanged.

All Go packages pass after the capture fix. This ZIP contains the source fix and audit findings, without downloaded data, altered checkpoints, recovered replay rows or changes to the public performance record.
