# V2 rotation experiment

This update makes confirmed teammate absences affect V2's minutes and prop probabilities across all teams. It does not claim an established accuracy improvement. V1's numerical prediction and selection settings are unchanged.

## What changes

- **Team allocation:** regulation minutes total 240. Complete historical overtime games are normalized back to 240; incomplete or conflicting boxes are rejected.
- **Recent baseline:** up to ten strictly earlier complete same-team games within 180 days, weighted toward recent games. Current roster membership is required; insufficient coverage keeps the model baseline and marks the case for review.
- **Confirmed absences:** only fresh, timestamped, unambiguous injury reports affect the allocation. BDL and ESPN reports are checked for time and identity. Currently absent players' minutes are redistributed jointly, not as independent bonuses. Long-term absences with no recent minutes do not create another minutes boost.
- **Replacement evidence:** earlier team rotations with explicit zero-minute rows for all relevant currently absent players inform the shares. Missing box-score rows do not establish absence. Conditional shares are shrunk toward recent available-player workload using N/(N+5). Fewer than three matching games stays review-only; these thresholds are experimental choices, not accuracy guarantees.
- **Physical constraints:** no allocation exceeds 48 regulation minutes per player. At least eight available players with observed team workload and five earlier complete team games are required. No synthetic reserve player or invented minutes is added.
- **Uncertain players:** questionable and doubtful teammates stay in the main allocation. Cards show separate scenarios if each sits, without inventing a probability of absence.
- **Production:** the current primary model supplies per-minute rates. Scoring, rebounding and assist usage are not automatically boosted. Extra opportunities from minutes change the distribution, raw probability and V2 model/book probability estimate at the exact offered line.
- **No forced Overs:** the same numerical rules apply to both sides. New-policy singles require the configured posting probability (55% by default) and price edge (1pp by default), plus existing quote/depth/price checks. The old V2 exception for lower-probability plus-money sides is disabled for this version.

## Policy and record separation

The new default is `rotation-v1`, enabled by `-v2rotation=true`. It uses the current primary model's rates, not the stale optional `model_v2.json`. There is no need to train that older secondary checkpoint to use this update.

The old V2 level anchor, star-minutes correction and stat multipliers are not applied to this policy. V2 retains its explicit model/book blend (50/50 by default), which is **not newly fitted calibration**. It remains paper tracking. There is no mechanism that automatically promotes it to V1 or labels it proven from the older V2 results.

New selected forecasts are stored in `data/v2_rotationlog.jsonl` and graded in `data/v2_rotationsettled.jsonl`. The V2 page's new-version record reads those files. Earlier V2 logs/results remain in their original files; they are not deleted or counted toward this version. `-v2rotation=false` restores the legacy V2 path.

All representative single-stat forecasts—not just selected picks—save their exact model checkpoint, features, target minutes, quote, injury context and probability rule under `data/v2_rotation/replay`. Team allocations/scenarios are retained beside them. GitHub Actions retains this directory in its decision cache and downloadable evidence artifact; it is not added to the public website.

## On the cards

Expand a V2 play to see model-baseline minutes, minutes actually used, matching-rotation count, confirmed outs and alternate uncertain-player scenarios. Existing player pictures, outcome graphs and recent-performance charts remain. The full explanation is under How it works.

## Run

After copying the source into the repository, run the normal daily workflow. Its roster/stat/injury refresh and prediction steps produce the new V2 automatically.

PowerShell:

```powershell
go test ./...
if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
powershell -ExecutionPolicy Bypass -File .\scripts\daily.ps1
```

Bash:

```bash
go test ./... && bash scripts/daily.sh
```

The existing GitHub workflow handles site export/deployment. To inspect only the new paired evaluation locally:

```sh
go run ./cmd/rotationaudit
```

The daily and pre-tip scripts also run that audit automatically. Private reports are in `run_evidence/rotation-audit`. No completed eligible forecasts means evaluation is unavailable, not a 0% hit rate.

## Validation and limits

Tests cover all 30 teams, joint absences, no future-game leakage, overtime normalization, rejected partial boxes, limited-history handling, uncertain-player scenarios, baseline immutability, newly eligible bench players, exact numerical replay and separate records. An opt-in archived-snapshot integration produced 18 valid team allocations and 384 exactly reproducible representative single-stat questions. That is an engineering check, not a historical performance result.

Forward evaluation pairs the same frozen model and same quote with and without team allocation. It measures unique player-game minutes errors and per-stat Brier/production errors on completed dates. The first version does not fit extra usage effects, confirmed starting-lineup changes, substitutions, coach-specific replacement rules or an entirely new opponent-absence model. Historical with/without splits describe associations, not proof of a replacement role. Check these limitations before treating a percentage as reliable.
