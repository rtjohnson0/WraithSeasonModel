# V2 position, presentation and combo diagnostics

## Position integrity

BALLDONTLIE stores Charles Bassey as G in active rosters and historical box scores. NBA lists Forward-Center; ESPN lists C. The sourced registry in `internal/data/position_corrections.json` requires the provider ID **and** exact name, and normalizes his loaded history to F-C. The raw download is retained, with original position and source in the loaded record. Feature generation, rotation roles, and exported matchup groups all use the same corrected history. No position is inferred from height or a recent stat performance.

A cross-check of 528 ESPN roster entries matched 514 provider players by unique normalized name and team. Bassey was the only guard-vs-big conflict among those matches. Hybrid positions were not treated as errors; unmatched identities were not corrected. This is a focused consistency check, not a claim that every role is verified. See `position-audit-2026-11-19.json`. NBA reference: https://www.nba.com/player/1629646 ; ESPN: https://site.api.espn.com/apis/site/v2/sports/basketball/nba/teams/9/roster .

Pregame starter slots that contradict a guard/big roster role are rejected rather than silently overwriting that role. Future confirmed corrections can be added to the registry with provenance. Historical logged forecasts and their grades are unchanged. Normal Daily retrains with the corrected inputs when the historical-input prerequisites are satisfied; normal model/calibration safeguards still apply.

## What yesterday's combo audit found

The new `cmd/comboaudit` inspects a chosen V2 policy's frozen, verified full-game forecasts, deduplicates using the public record's rule, and joins identity-matching final box scores. Revised actuals that disagree with the frozen settlement are excluded from decomposition. It never fits a correction or changes a record.

For November 18, 2026, 74 matched decisions: 35–39 (47.3%) against an average published estimate of 62.5%. These are correlated comparisons from **one date**, not 74 independent experiments.

| Side | Decisions | Hit rate | Estimated | Actual minus projected minutes | Actual minus projection | Minutes component | Rate component |
|---|---:|---:|---:|---:|---:|---:|---:|
| Over | 29 | 24.1% | 60.6% | -6.44 | -7.02 | -4.28 | -2.75 |
| Under | 45 | 62.2% | 63.8% | +4.09 | +2.99 | +2.93 | +0.06 |

At the forecast's aggregate per-minute rate, minutes component = (actual minutes − projected minutes) × projection / projected minutes. Rate component is the remaining actual-minus-projection error. The identity is arithmetic, not a causal claim; game scripts, role changes and selection can explain both. These results motivate better workload diagnosis, not a tuned blanket Over penalty. The full typed slices are in `combo-audit-2026-11-18.json`.

Combo cards now include lower/higher workload scenarios using the same shared-minutes simulation and correlation inputs, plus a visible workload note when projected minutes differ from typical by at least four. That threshold is an explanation trigger, not an eligibility rule. Leans and existing comparison eligibility are preserved. Research combo estimates still require independent validation before becoming selected plays. Scored/loaded quote totals now reflect the actual input counts.

PowerShell:

```powershell
go run ./cmd/comboaudit -policy rotation-v4 -date 2026-11-18
```

Normal Daily runs the read-only audit after V2 settlement. It saves `run_evidence/combo-audit/report.json` without editing forecasts or records.

## Frontend and explanations

- V2 board identifies itself as V2 and preserves backend model order by default, within each tip-time matchup. Optional probability/value sorting remains available.
- Board leans and additional leans are counted separately; the total includes both.
- The record prominently explains all-run inclusion and the existing highest-tier/earliest-quote deduplication rule.
- V2 singles now attach the existing fitted opponent-defense sensitivity calculation, holding other rate inputs fixed. Current-season rank percentages remain descriptive, not probability multipliers. Missing model inputs are not presented as measured effects.

No model confidence was increased or retroactively tuned to the November 18 outcomes.
