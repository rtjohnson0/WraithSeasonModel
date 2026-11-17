# Supported-only rotation research

The legacy rotation experiment can replace baseline minutes even when it has no relevant confirmed absence or too little matching history. The new opt-in `rotation-v2` policy retains baseline minutes in those cases. V1, its selection rules and LEAN fill-ins are unchanged. The default V2 policy remains `rotation-v1`; this patch does not activate a new live policy.

## What changes

A numerical minutes adjustment requires a valid allocation, a relevant confirmed absence, and at least three earlier matching joint rotations. The selected player must not be confirmed out and must have a finite target above zero and at most 48 minutes. Existing roster and fresh-injury checks still apply. Unsupported teams keep the complete baseline forecast, including its minutes distribution. Per-minute rates do not change.

Questionable and doubtful teammates are not assumed out. Their hypothetical scenarios use the same support requirement; unsupported scenarios retain baseline minutes and are labelled with limited matching history. Three matching games is an evidence floor, not proof of predictive accuracy.

The cards distinguish a proposed team allocation from the minutes actually applied. Replay captures store a target only when it was used, so the forecast can be reproduced exactly.

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

This runs the normal prediction command and switches its V2 output to the new policy; it is not a read-only audit. Normal V1 outputs are still written as usual. No retraining is required for this minutes-only research policy. To inspect completed strict-policy captures without writing predictions:

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

Current historical matching requires explicit zero-minute records to establish nonparticipation. Missing player rows are not silently treated as medical absences. This limits support and needs separate historical-data work. Opponent-absence rate effects, teammate usage effects and disabled learned feature groups are not enabled by this change.

## Checks

Tests cover unsupported and healthy-team fallback, supported three-game targets, invalid targets, exact baseline replay with no injected target, unchanged baseline models, and separate export records. Validation completed: `go test ./...` passed; all 28 site tests passed; the Vite production build passed. Collect new pregame captures before considering this policy for promotion.
