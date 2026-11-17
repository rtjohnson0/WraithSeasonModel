# Rotation and lineup production research

The legacy rotation experiment can replace baseline minutes even when it has no relevant confirmed absence or too little matching history. The new opt-in `rotation-v2` policy retains baseline minutes in those cases. V1, its selection rules and LEAN fill-ins are unchanged. The default V2 policy remains `rotation-v1`; this patch does not activate a new live policy.

## What changes

A numerical minutes adjustment requires a valid allocation, a relevant confirmed absence, and at least three earlier matching joint rotations. The selected player must not be confirmed out and must have a finite target above zero and at most 48 minutes. Existing roster and fresh-injury checks still apply. Unsupported minutes allocations keep the baseline minutes distribution. Separate teammate-production and opponent-allowance estimates can adjust per-minute rates under their own evidence rules, described below.

Questionable and doubtful teammates are not assumed out. Their hypothetical scenarios use the same support requirement; unsupported scenarios retain baseline minutes and are labelled with limited matching history. Three matching games is an evidence floor, not proof of predictive accuracy.

The cards distinguish a proposed team allocation from the minutes actually applied. Replay captures store a target only when it was used, so the forecast can be reproduced exactly.

## Offense and defense production

- **Teammates missing:** compare the selected player's per-minute points, rebounds, assists and threes under the entire relevant missing-player set. Minutes and production rates are separate, so more court time is not counted as increased usage.
- **Opponents missing:** compare position-specific allowed output, normalizing historical opposing players by their own strictly earlier per-minute rates. This reduces schedule-strength distortion; it does not remove all confounding. These observed allowances can reflect changes in defensive personnel, offensive misses and transition/game conditions; the code does not identify those mechanisms separately.
- Each stat requires at least five earlier joint-out games and five games with every relevant player present. Partial combinations are excluded rather than stacking individual-player bonuses. Relevant players have at least eight average minutes in their last ten earlier appearances. Current-roster coverage must reach 90% of historical team minutes.
- The conditional rate is compared with mixed-history production already represented by the baseline, then shrunk by `without / (without + 20)`. Each component is capped at ±5%; their product is capped at ±8%. These are conservative experimental design settings, not parameters fitted or validated on forward results. Effects can be positive, negative or unchanged independently for each stat.
- Both sides need complete final box scores, complete modeled stat fields and recorded download provenance. Historical opponent-rate references use up to ten appearances strictly before that historical game, requiring at least three. Unknown positions do not receive a guessed matchup adjustment. Only the prior 180 days are used.
- Cards show missing players' recent minutes, points, rebounds, assists and blocks, sample counts and applied rate changes. These are descriptive contributions, not net losses after replacements. Total rebounds are not defensive rebounds; blocks are not a measured rim-protection rating. Shot-location, matchup tracking and possession data are not available in this path, so no interior-defense, true usage-percentage or pace coefficients are invented.
- Do not double count learned absence groups: a component is disabled when the loaded model already includes its corresponding learned absence features. New V2 singles and combo comparisons share the same adjusted distributions. V1 and full-game spread/total/moneyline models are not modified.
- Fresh confirmed Out reports drive the main forecast. Questionable/doubtful reports remain hypothetical scenarios. Captures retain status timestamps, component scales and the exact combined replay policy. The audit rejects missing/future/stale production provenance or inconsistent scales.

These associations are not validated causal injury estimates. Collect fresh pregame captures and compare the candidate to baseline before promotion.

## Separate evidence

| Item | Existing policy | New policy |
|---|---|---|
| Policy | `rotation-v1` | `rotation-v2` |
| Pick log | `v2_rotationlog.jsonl` | `v2_rotation_strictlog.jsonl` |
| Settlement | `v2_rotationsettled.jsonl` | `v2_rotation_strictsettled.jsonl` |
| Replay directory | `v2_rotation/replay` | `v2_rotation/strict/replay` |

The export and V2 settlement command choose the matching record from `data/v2.json`. Switching policies does not import the other experiment's wins. The active V2 page still shows one policy at a time.

For an intentional research run from the repository root:

```powershell
go run ./cmd/predict -rotationpolicy rotation-v2
```

This runs the normal prediction command and switches its V2 output to the new policy; it is not a read-only audit. Normal V1 outputs are still written as usual. No retraining is required for this post-fit research policy. To inspect completed strict-policy captures without writing predictions:

```powershell
go run ./cmd/rotationaudit -policy rotation-v2
```

## Retrospective diagnostic, not forward validation

Source: retained artifact `model-run-44109947683-1`, artifact ID `14341228393`, SHA-256 `40cffc050a28a115f1b7ceb2b0f1dec833d2d15a9726208a7a13601591ecffd8`. Frozen model version 4, pregame questions from November 13–16, evaluated before November 17, 2026. The earliest valid pregame capture is selected per player/game/stat, with frozen-model and quote checks from the merged absence audit.

| Minutes forecast | Mean absolute error |
|---|---:|
| Frozen baseline | 4.66 min |
| Existing rotation experiment | 5.44 min |
| Retrospective supported-only fallback | 4.66 min |

There are 378 player-games across four dates and 1,376 binary prop questions. There are **zero qualifying supported joint-absence cases**. The fallback therefore equals baseline on this sample. This rule was chosen after inspecting these results; it is a diagnostic of unsupported adjustments, not independent validation or evidence of better betting results. Some existing rotation probability scores improved despite its worse minutes error; this patch does not claim every metric improves.

Baseline means the same frozen model and model/book blend without team allocation, not the calibrated production V1 probability. No completed version-6 evidence is represented here.

Historical matching requires explicit zero-minute records to establish nonparticipation. The standard training loader discarded those records, so this candidate adds a separate research loader that retains explicit numeric zeros. Missing/null minutes, omitted players and conflicting duplicate records remain unknown. Normal training and V1 feature history do not change. Nonparticipation alone does not establish a historical medical injury. The disabled learned feature groups remain disabled; production associations are a separate research policy. The frozen four-date result above evaluates only the minutes fallback, not these new production effects.

## Checks

Tests cover unsupported and healthy-team fallback, supported three-game targets, own/opponent production directions, thin samples, unknown positions, future-data exclusion, omitted/DNP rows, conflicting duplicates, invalid targets, exact numerical replay with rate changes, no stacking with learned effects, unchanged V1 models, fresh capture provenance and separate export records. Validation completed: `go test ./...` passed; all 28 site tests passed; the Vite production build passed. Collect new pregame captures before considering this policy for promotion.
