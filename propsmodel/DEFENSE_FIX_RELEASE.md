# Defense and selection consistency update

This release fixes the omitted made-three totals in team/position features,
keeps one selected direction per game/player/stat, and makes over research
comparisons use the same exact-quote analytics record as normal props.
Opposite-side comparisons, nonpositive modeled edge, and incomplete legacy
watch records are not promoted into comparison cards.

V2 experimental combos have their own `COMBORESEARCH` tier and never enter the
selected combo tab. V1 retains direct combo calibration requirements and now
exports rejection counts. A zero selected-combo count can be correct: missing
calibration is not repaired by inventing confidence or lowering the evidence
requirements. Rebounds still use learned opponent weights; ranks are not
forced into probability multipliers. Analysis distinguishes current-season
descriptive ranks from the rolling, shrunk inputs actually used by the fit.

## Required model migration

The corrected base model is version 6; its optional tail challenger is version
7. Older versions remain readable for historical replay but cannot be used for
new live prediction. Old calibration must not be reused with the corrected fit.
Older historical records are retained without retrospective rescoring.

From the repository root, with Go 1.22+ and the downloaded historical inputs:

```bash
bash scripts/rebuild_defense_model.sh
bash scripts/pretip.sh
```

The rebuild stages the new model/calibration and only installs them after both
fit successfully. It needs the original `stats_*.jsonl` and historical opening
prop quotes, normally held in the workflow input cache. They are not included
in this public checkout. Restore that cache or download the original data using
the repository's existing input commands. Never replace historical quotes with
today's sportsbook lines. The optional second model file needs `cmd/train -v2`
before selecting the non-rotation legacy V2 policy again.

Scheduled `daily.sh -s` and `pretip.sh` deliberately do not retrain. Complete this
one-time migration in the input-enabled environment before releasing those
jobs. Until migration succeeds, new predictions fail clearly and the previously
published board/history can still be exported. Do not claim the old saved
projections have already incorporated the correction.

## Validation

Regression coverage checks nonzero three-point team/position totals, finite
opponent feature ratios and rank direction; exact-quote over analytics;
negative-edge/incomplete/opposite-side exclusions; same-player different-game
isolation; and separation of V2 research combos from qualified selections.
These correctness checks do not establish improved predictive performance.
That requires subsequent untouched-date evaluation after retraining.

## Included fitted candidate

`validation/defense-candidate/model.json` is a separately fitted version-6
candidate using 94,514 historical player-games through November 15, 2026.
Both three-point defense coefficients and their training variances are now
nonzero. It has not replaced `data/model.json` and has no newly fitted probability
calibration. Do not copy it over the live model without fitting matching
calibration. `validation.json` records the completed checks and remaining limit.

Validation completed: the entire Go suite, 28 frontend tests, 23 UI interaction
checks, production frontend build, and shell syntax. The new aggregate regression
was also run against the original source: it fails there and passes after the fix.
