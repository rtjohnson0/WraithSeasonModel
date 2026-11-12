# How we test the next model

Run from the project root on Windows PowerShell:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\compare.ps1 -Season 2025
```

This uses existing downloaded data and does not call paid APIs. You need stats history covering seasons before 2025, `data/opening_props_2025.jsonl`, and the existing trained model and calibration. Advanced stats and opening game odds are used when available. It can take several minutes. The script opens an HTML report when complete.

The command writes into a new timestamped `research` folder. It copies and hashes the current model and calibration, hashes the raw input files, checks the inputs did not change while running, and leaves production files alone. Research artifacts and questions stay out of Git and the public website.

## The comparison

The November 2026 checkpoint cannot honestly predict 2025 games it has already trained on. Instead, the historical screen reconstructs the current modeling method: each month's model trains only on prior months, using the existing pregame feature policy without realized absence features.

All variants answer the same player/game/stat/line questions at the same exact book prices. Three fixed policies are tested:

- Current method: existing minutes and dispersion.
- Minutes challenger: move projected minutes 25% toward typical minutes.
- Minutes and dispersion challenger: the same minutes adjustment, with conditional stat dispersion multiplied by 0.75.

The third policy is an experiment, not a claim that narrower distributions are correct. Narrowing can increase raw confidence; the matching calibration and held-out evaluation must justify it.

Each variant gets its own calibration fitted on the earliest 60% of questions, splitting on complete dates. The next 20% selects a challenger. The final 20% evaluates that one selection against the current-method reference and sportsbook baseline. Final-test outcomes never select the challenger. This reference is the current METHOD with chronological calibration, not a retrospective replay of today's exact checkpoint.

Do not tune a new policy after seeing the final test and keep calling it untouched. A redesign needs a new future test period. Cached historical stats and opening quotes are research evidence; this screen does not verify every historical quote's capture provenance or certify a launch hit rate.

## Initial result on the supplied cache

The fit period ends February 3, 2026, validation ends March 13, and final testing covers March 14–June 13. There are 12,906 non-push final-test questions.

The minutes/dispersion challenger had slightly better final log loss (0.68424 versus 0.68438) and minutes MAE (4.52 versus 4.55 minutes per prop question). It failed the overall screen because it was slightly WORSE on the validation period that had to justify selecting it. Its stat MAE also did not improve overall. No replacement was promoted. Raw counts of selected unders remained high, so this experiment does not establish a numerical fix for the under-heavy board.

See `audit_findings/model_comparison/comparison.html` for the initial report and per-stat results. The initial run froze the model/calibration but predated the input-hash check added to the command; that limitation is recorded in its manifest.

## What would justify a production change

A challenger must improve validation, then show sufficient final-test date coverage, improvement in log loss and Brier score, and date-clustered evidence against both the reference and sportsbook baseline. Passing this historical screen only earns a prospective shadow test. It does not automatically deploy a model.

If a candidate qualifies, implement its policy in the shared prediction/replay path, fit matching calibration on earlier captured data, keep it separate from the existing board, and collect new untouched forward outcomes. Promote only after reviewing those results. The existing `cmd/accuracy` report remains the forward evidence check.

## Combo percentages

Comparison-only combo headlines now show the projected stat total, not the uncalibrated 73–78% estimate. The raw simulation percentage stays inside the expanded analysis with its limitation. No artificial probability cap or unsupported confidence increase is applied. Qualified combos retain their separately calibrated probabilities.
