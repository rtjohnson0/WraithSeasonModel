# Combo accuracy update — 2026-11-13

## What the supplied data shows

The uploaded data(1).zip contains combo selections through November 10, not live November 13 results. The audit below removes repeated entries and requires matching cached settlement player/game/date identity, a source hash, and an outcome consistent with the actual total. The historical quotes are incompletely documented: this is diagnostic data, not a certified live record.

There are 394 unders among 426 eligible unique outcomes. Actual totals exceed projections by 1.84 on average in this selected sample. PRA is the weakest observed segment: 25 wins in 55 decisions, with actual totals 5.74 above projections on average. This pattern warrants investigating low component/minutes forecasts; selected outcomes alone do not establish the cause or a correction for future games. Raw simulation confidence substantially exceeds observed hit rates. Do not treat a 70–80% raw estimate as a demonstrated chance of winning.

## Changes in this update

- The default combo tab contains selected plays only. Line comparisons remain accessible in a separate tab with the existing charts, history and expandable analysis.
- Uncalibrated comparisons are displayed in a stable game/player order rather than ranked by apparent simulated profit. Their probabilities have not been capped, replaced or adjusted to manufacture overs.
- Public selection now explicitly requires a direct calibration for the exact combo type. A manual experimental blend cannot bypass this requirement. The separate V2 research output remains separate.
- A player's unresolved own availability status prevents a qualified combo selection. Teammate history can still show a minutes-only scenario comparison; it does not establish a confirmed absence or a new injury forecast.
- Combo availability scenarios now reprice the combined distribution at the observed historical minutes. A comparison row uses raw probabilities consistently; a selected row uses its direct calibration. Neither scenario changes the headline selection probability.
- Qualified candidates are ranked by expected return at the quoted odds rather than probability-point edge alone.
- New market logs include team, projected total, projected minutes, minutes shift, and distinct book count. Previously those projection fields were zero, limiting diagnosis of under bias. Older logs are preserved.
- Added the read-only `combos audit` command with separate combo/side summaries, projection error and probability-versus-outcome comparisons. REVIEW rows and comparisons cannot enter this diagnostic sample.
- The midnight-UTC / 7 PM ET game-time fix from the previous update is retained.

## What this does not establish

No new forecasting coefficients have been fitted or promoted. The supplied directly logged combo-market data covers one date, and the existing calibration command declined to write a fit because it lacks sufficient earlier and later evidence. Existing promotion criteria additionally require 30 later evaluation dates and improvement against raw and sportsbook probabilities. A better menu or stricter eligibility rules do not prove improved betting returns. A calibration pass is also not a prospective profit guarantee.

The next accuracy study should evaluate every newly captured market, including rejected selections, with identity-checked final totals and minutes. Inspect minutes errors separately from component scoring rates and covariance by PRA/PR/PA/RA and side. Freeze any candidate correction before evaluating new future dates. Do not tune an arbitrary boost from the selected historical plays below or reuse an already viewed test as untouched evidence.

## Apply and refresh

Replace the project files while keeping your own data and local secrets. Push the update, then run the GitHub daily workflow in **pretip** mode to refresh quotes and recompute the boards. A code push uses site mode and does not fetch new picks. No retraining is needed to apply these selection/UI fixes.

For a local read-only diagnostic in PowerShell:

```powershell
go run ./cmd/combos audit
go run ./cmd/combos audit -legacy
```

The second command includes historical quotes for exploration only. Neither command changes results or the model. The full diagnostic below is included for review, not published on the customer dashboard.

## Validation

Go regression suite, 17 frontend tests and the production frontend build passed. This is software validation, not evidence of a future win rate.

---

# Combo diagnostics

426 unique binary outcomes. Read-only: no models, selections or public records changed.
Includes historical quotes with incomplete provenance. Cached settlement identities match; this is exploratory, not verified performance or evidence of a profitable strategy.

Positive actual-minus-projection bias means projections were too low. Raw confidence is the simulation before calibration; it is not a demonstrated hit rate. Selected plays alone cannot calibrate all available markets.

| Combo / side | Outcomes | Dates | Hit rate | Published probability | Raw confidence | Actual minus projection | Stat MAE |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| all / all | 426 | 13 | 55.6% | 57.4% | 72.6% | +1.84 | 4.57 |
| all / OVER | 32 | 8 | 53.1% | 57.1% | 83.2% | -4.52 | 5.86 |
| all / UNDER | 394 | 13 | 55.8% | 57.5% | 71.8% | +2.36 | 4.47 |
| pra / all | 55 | 12 | 45.5% | 57.2% | 74.4% | +5.74 | 8.01 |
| pra / OVER | 2 | 1 | 50.0% | 60.2% | 71.0% | -6.34 | 12.56 |
| pra / UNDER | 53 | 12 | 45.3% | 57.0% | 74.5% | +6.20 | 7.84 |
| pr / all | 51 | 10 | 62.7% | 58.5% | 74.0% | +1.11 | 6.69 |
| pr / OVER | 8 | 5 | 62.5% | 57.9% | 85.3% | -7.97 | 9.99 |
| pr / UNDER | 43 | 10 | 62.8% | 58.6% | 71.9% | +2.80 | 6.07 |
| pa / all | 18 | 7 | 61.1% | 60.2% | 60.5% | +0.20 | 6.77 |
| pa / UNDER | 18 | 7 | 61.1% | 60.2% | 60.5% | +0.20 | 6.77 |
| ra / all | 302 | 13 | 56.0% | 57.1% | 72.8% | +1.36 | 3.46 |
| ra / OVER | 22 | 6 | 50.0% | 56.5% | 83.6% | -3.11 | 3.75 |
| ra / UNDER | 280 | 13 | 56.4% | 57.2% | 72.0% | +1.71 | 3.43 |
