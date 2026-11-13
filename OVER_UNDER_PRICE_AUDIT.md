# Over/Under price comparison — November 13, 2026

Source: the supplied `model-run-43458527780-1.zip`, not a newly refreshed odds feed. These are diagnostic counts and examples, not current betting recommendations.

## What the saved slate shows

Both sides of 423 frozen representative markets were recomputed at the same line, book and forecast. The checkpoint hash and final probability must match. The latest eligible pre-tip capture is used for today's slate. A book-favored side means its two-price, no-vig probability is greater than 50%.

| Book-favored side | Markets | Clears 55% and 1pp edge using raw probability | Clears both after calibration |
| --- | ---: | ---: | ---: |
| Over | 178 | 14 | 0 |
| Under | 236 | 102 | 5 |

The other nine markets have equal no-vig side probabilities. Calibration filters both sides. The raw model starts with substantially more Under qualifiers, so this cannot be explained solely by the calibration intercept. Across all 423 Overs, 34 reach the final 55% probability bar, but none also meets the exact-price edge bar at these representative quotes. Two Overs meet the edge bar alone but fall below 55%.

Representative saved examples:

| Over | Price | Raw | Final | Price break-even | Rejection |
| --- | ---: | ---: | ---: | ---: | --- |
| Zion Williamson 2.5 assists | -122 | 64.8% | 53.2% | 55.0% | Both final probability and price |
| Alex Caruso 6.5 points | -120 | 64.3% | 50.3% | 54.5% | Both final probability and price |
| Dejounte Murray 4.5 assists | -140 | 67.3% | 56.2% | 58.3% | Final price edge |

Names in these older replay rows are recovered for display from exact pre-tip date/game/player logs. Numerical probabilities and prices remain the original frozen values. No row is promoted into the public performance record.

The supplied main-board snapshot has 30 rows, all Under: 17 PAPER and 13 LEAN. It is not the user's later 54-row view. Displayed rows include fill-in leans and paper forecasts; that count is not a count of validated staking recommendations. Current representative Under quotes at +150 or longer do not meet the numerical selection bars. A particular displayed +150 Under could be an earlier/alternate quote; it needs its exact player, line, book and timestamp traced before judging it.

## Historical outcome comparison and its limits

The tool groups both sides by stat and offered-price band. Completed replay questions use the earliest eligible forecast; current questions use the latest. The November 12 supplemental cohort has 102 questions across 29 players and two games. Five book-favored Overs clear the raw bars there; three win and two lose. None clears both final bars. This is too small and already inspected to establish that removing calibration improves accuracy.

Posted history is separately regraded from identity-checked final stats and deduplicated to the earliest posted player/game/stat/side on a date. PICK, PAPER, CHECK and LEAN stay separate. It remains selection-biased. Old rows lacking original quote-period/timing evidence are explicitly labelled `legacy quote provenance`; correct final player stats do not prove the original betting market was full-game. The tool never treats those rows as clean evidence for refitting calibration. No historical record is rewritten.

## Included tool

The daily and pre-tip scripts now attempt a private report after prediction:

```sh
go run ./cmd/priceaudit
```

Outputs: `run_evidence/price-audit/report.md` and `report.json`. The JSON includes each side's raw/final/book probability, price break-even, edge and numerical rejection reason. Reports are retained with workflow evidence and are not added to the public site. The fixed diagnostic bars are 55% probability and 1pp price edge, before other eligibility, injury, history and concentration checks.

For explicit supplemental recovery of missing historical names from exact pre-tip logs:

```sh
go run ./cmd/priceaudit -recoveridentities
```

## Decision

Do not install raw probabilities, force Overs onto the board or lower thresholds merely to balance it. The measurable next model comparison is per-stat probability calibration and distribution behavior at similar prices. This audit makes rejected raw-qualified Overs visible and supplies the cases for that comparison. Current model weights, calibration coefficients, pick thresholds and website layout are unchanged.
