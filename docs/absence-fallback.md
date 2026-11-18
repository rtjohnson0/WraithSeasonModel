# V2 projected lineups and pooled role production

## What changes

`rotation-v3` is the new V2 experiment. Daily/pre-tip scripts select it after this PR is merged. V1 scoring, its record and visible LEANs remain unchanged. V3 uses separate record files and immutable evidence rather than relabeling prior V2 results. The older `rotation-v2` can still be selected explicitly.

Fresh confirmed outs trigger a projected full-game replacement rotation. Supported historical joint rotations are retained when available. Otherwise vacated workload is distributed toward players with recorded similar positions, using recent same-team workload. No repeated matchup or exact injury combination is required. This is not a confirmed starting five or a stint-level lineup simulation.

Replacement allocation conserves 240 regulation minutes, excludes confirmed outs and adds at most 12 minutes per player, with a 40-minute ceiling for increases (a pre-existing higher baseline is retained up to 48). It cannot invent reserves with no observed workload. Missing positions, incomplete team coverage or an infeasible allocation retain the old baseline minutes rather than forcing a replacement estimate. Coarse C/B and F/W compatibility is an explicit heuristic, not a learned coaching rotation.

## Production model

A ridge model learns league-wide associations between rotation composition and per-minute points, rebounds, assists and threes relative to each player's strictly earlier per-minute baseline. Each game must be an explicit final with complete player stat fields and complete two-team minute totals. Player profiles use at most ten earlier player appearances within 730 days, carrying player production profiles across trades and the offseason, with three required. Fits use games strictly before the forecast date, from the preceding 730 days.

Features compare projected and recent-baseline composition: scoring, rebounding, creation, threes, blocks, center/big minutes and guard minutes, separately for teammates and opponents. Teammate exposure excludes the target player's own contribution. Exposures normalize overtime to 240 minutes. Profile coverage must cover at least 90% of the rotation's minutes, with recorded positions; unknown production is never filled with zero. Feature deltas are bounded and ridge regularization is fixed at 100 with a no-change anchor (no intercept). Targets are clipped to [-1, 2] relative rate change to limit extreme low-baseline games.

At least 500 pooled player-games and 30 dates are required per stat, not 500 repeated injury matchups. The model may increase or decrease a rate. Missing a center does not automatically grant all his rebounds to his opponent. Opponent rebound capacity and blocks are observed box-score roles, not measured defensive rebound percentage, rim protection, height or defensive rating. Shot-attempt/possession redistribution is not explicitly conserved.

Teammate and opponent components are capped at ±5% each and ±8% combined. They replace the older individual/joint production adjustment for this policy, never stack with it. Enabled learned absence feature groups suppress the corresponding extra component. Minutes and production changes remain separate.

Historical training uses observed final rotation minutes. Live inference uses projected rotations. These historical associations can contain co-absence, coaching, schedule, foul-trouble and game-state confounding; they are not isolated causal injury effects. The cards show projected replacements, baseline/projected minutes, pooled sample sizes, and applied production changes.

## Archived inputs and provenance

Complete archived finals can train the role model even if their old downloads lack capture timestamps. `BoxScoreComplete` requires explicit points/rebounds/assists/threes/blocks and final scores. This is separate from `AnalyticsReady`: archived rows are not promoted into verified forward evidence or settlement. Fresh injury timestamps, unambiguous identities and paired pregame quote checks remain required for decision captures. Questionable/doubtful players remain in the main scenario, with separate out scenarios.

## Validation

`go run ./cmd/lineupstudy -before 2026-01-01 -through 2026-07-01` fits earlier games and evaluates later *observed rotations*. The checked-in result is `docs/lineup-study-2026.json`. It is a conditional diagnostic, **not a pregame backtest**: it does not measure whether projected minutes/lineups were correct, pick accuracy, confidence calibration, or ROI.

That initial held-out check is mixed. Points MAE improves marginally (4.22994 to 4.22954); rebounds worsen (1.82071 to 1.83074), assists worsen (1.38808 to 1.39366), and threes worsen (1.02161 to 1.02631). This does not demonstrate an accuracy improvement and does not support a 60–62% claim. The user-requested expansion runs in the separate V2 experiment, not V1 or staked promotion.

The existing rotation audit evaluates immutable pregame decisions against later verified finals. Its baseline and candidate use the same frozen primary model, quote and blend, with minutes/production changes isolated. Daily/pre-tip now audit the active candidate instead of silently auditing legacy rotation-v1. Reports split signed stat residuals into minutes and rate contributions and group minutes errors by the captured pregame spread, not the final margin. The spread breakdown diagnoses competitiveness; it is not a fitted blowout-probability mixture.

Behavioral tests cover temporal exclusion, archived completeness versus provenance, unknown profiles, no-change composition, minute conservation/caps, replacement-role preference, frozen composition provenance, and policy/shadow isolation. Exact input/model captures are retained in the existing workflow evidence cache and artifacts under `data/v2_rotation/fallback/replay`.

## Commands

After merge, normal Daily/pre-tip runs select the new V2 policy automatically. For local testing:

```powershell
go run ./cmd/predict -rotationpolicy rotation-v3
go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback
go run ./cmd/lineupstudy -before 2026-01-01 -through 2026-07-01
```

Old V2 can be compared explicitly with `go run ./cmd/predict -rotationpolicy rotation-v2`. That command also captures a role-composition shadow without replacing the old V2 board. `-nolog` disables recording. Directly selecting V3 updates the local V2 board and uses its separate picks/settlement history.
