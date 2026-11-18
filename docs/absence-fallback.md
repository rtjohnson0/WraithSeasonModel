# Absence fallback and minutes diagnostics

The current rotation-v2 policy requires five earlier games with the whole relevant absence set missing and five with the whole set present before changing a production rate. This often leaves a known absence numerically untreated.

## Candidate policy

`rotation-v3` preserves supported joint estimates. For an unsupported stat with multiple relevant outs, it computes each absent player's own historical association using the same complete-final-box-score, explicit-zero-minute, roster-coverage, position and date checks. Each individual component still needs five out and five present games. Teammate estimates are specific to the scorer. Opponent estimates normalize historical scorers by their strictly earlier per-minute rates.

Supported individual effects are averaged, never added or multiplied. Weights are the missing players' recent minutes; opponent-rebound weights use their recent rebounding production. These weights measure relevance and do not transfer missing stats to an opponent. Unsupported shares stay neutral instead of being redistributed to supported players. Individual estimates receive another 50% shrinkage and the fallback is capped at ±2.5%. Existing joint components retain their ±5% cap; the existing combined own/opponent ±8% cap remains.

This does not identify a causal injury effect. Co-absences, historical replacements and game conditions remain confounders. It can increase or decrease a rate. No evidence means no adjustment. There is deliberately no invented rim-protection rating or automatic center-out bonus. Unknown positions remain unsupported. The existing fresh, confirmed-Out requirement and protection against double-counting enabled learned absence features remain.

## Automatic prospective comparison

Daily and pre-tip prediction continue publishing rotation-v2. Beside it, prediction records all eligible rotation-v3 single-stat market questions using the same fresh quotes, frozen model, features and timestamp. Shadow capture does not publish candidate picks or replace active board files. `-nolog` disables this capture. Workflow evidence cache/artifacts already retain the `data/v2_rotation` directory, including the new `fallback/replay` subdirectory. This is needed to evaluate the candidate on later finals without reconstructing what injuries were known.

Daily/pre-tip audits now explicitly evaluate the active rotation-v2 policy (previously they defaulted to legacy rotation-v1), plus the new fallback shadow. Unsupported/empty evidence is reported as unavailable, not 0% accuracy.

Both reports include an exact signed decomposition:

- Minutes contribution: `(actual minutes - projected minutes) * projected stat / projected minutes`.
- Rate contribution: `actual stat - actual minutes * projected stat / projected minutes`.
- Sum: `actual stat - projected stat`.

They also group minutes bias/MAE by captured pregame spread: 0–3.5, 3.5–7, 7–10, 10+, and unknown. Final margins never define those cohorts. These are diagnostics, not a newly fitted close-game/blowout minutes model.

## Commands (PowerShell or shell)

Normal scheduled runs automatically capture and audit the shadow after this change is merged. Read-only audits:

```powershell
go run ./cmd/rotationaudit -policy rotation-v2
go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback
```

Explicit candidate publication for local testing is available with `go run ./cmd/predict -rotationpolicy rotation-v3`. This overwrites the local V2 board, so it is not the normal Daily setting. If explicitly selected, V3 uses separate pick/settlement files and evidence directory; old history is not reassigned.

## Validation and remaining work

Behavioral tests cover missing rows, future/same-day exclusion, duplicate IDs, neutral unknown shares, joint precedence, both production paths, bounded changes, policy isolation and component provenance. A prospective win-rate improvement has **not** been demonstrated. No historical winners were deleted or reclassified.

Next comparisons must use the same earliest pregame questions on later dates and report date-level uncertainty, coverage, stat MAE, Brier score and selected-play results. Separate absence, minutes and joint candidates before combining them. This change does not yet implement cross-team role pooling, an explicit replacement-lineup model, a fitted competitiveness mixture, new three-point calibration or selection changes. Those require separate data and chronological evaluation; passing software tests does not establish a 60–62% hit rate.
