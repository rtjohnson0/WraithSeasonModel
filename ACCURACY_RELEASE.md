# Accuracy and product update — November 12, 2026

The under-heavy selection issue is not yet solved. There is no supported claim that the new candidate is more accurate on unseen games or ready to sell as a proven profitable model.

## What changed

The V5 research model learns an explicit early-exit minutes component and projected-mean-specific stat dispersion. It uses a chronological prefix model to estimate later residuals, then refits the central model on all available earlier data. Singles and combo simulation use the same weighted minutes distribution. Each stat has its own calibration family, chosen on development dates. A symmetric market-residual challenger cannot introduce a blanket under intercept.

A separate price-policy experiment compares both offered sides by expected return, rather than demanding that the selected side be the likelier outcome. For example, a 45% over at +150 can have forecast value even though the under is likelier. It keeps the probability at 45%. This policy is always zero-stake research and is excluded from the old fixed-55% accuracy certification.

The public board continues using its existing calibrated model. The supplied `model_tail.json` and `calibration_tail.json` are a separate candidate, trained through November 9 from the supplied cache. They do not replace `model.json` or `calibration.json`. Keep the pair frozen while collecting new decisions.

The frontend preserves the brand, compact expandable picks, form graphs, minutes charts and default historical view. Refresh has a busy state, failures preserve loaded plays, navigation returns to the top, filtering applies consistently to leans, and active filters have a clear reset. Expanded graphs use the full row and stack on small screens. Keyboard focus and reduced-motion preferences are supported.

## Development evidence

The study used 38,314 calibration outcomes through February 3, followed by 12,714 non-push development outcomes through March 13. Monthly forecasts trained strictly before the prediction month. The previously examined March 14–June 13 final period was excluded.

| Method | Development log loss | Selected overs / unders |
|---|---:|---:|
| Reference V4 | 0.682966 | 8 / 218 |
| V5 standard calibration | 0.682957 | 7 / 206 |
| V5 per-stat development selection | 0.682815 | 7 / 203 |
| Same V5, experimental price policy | 0.682815 | 25 / 242 |

The improvement is small. Minutes MAE slightly worsened (4.549 to 4.552); stat MAE was essentially unchanged (2.50745 to 2.50707). Directional imbalance remains. Price policy changes selection, not forecasting accuracy. These policies were designed after diagnosis and their calibration family was selected using these development dates: this is not a fresh holdout. Cached historical quote provenance is not independently certified. Research selection totals do not apply production game/player caps or guarantee executable sportsbook prices. See `audit_findings/tail_development/study.json` for the complete results.

## Run it

Extract over your existing project; preserve your existing data, `.env` and Git history. Do not delete your data folder. The ZIP includes only the separate candidate pair under `data`.

Run the existing daily action. Both daily and pre-tip scripts now also run `scripts/run_tail_shadow.py`, using already downloaded inputs, with zero units and separate outputs under `data/tail_shadow` and `data/value_shadow`. They never publish an experimental board. Missing candidates skip cleanly; an experiment failure does not discard the primary board. Workflow caches retain their decision history, and run artifacts retain decisions and source inputs. Shadow histories are not committed as public picks.

On Windows, from the project folder:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\daily.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\push.ps1 -Message "Add uncertainty research and polish board"
```

To rebuild the candidate deliberately, after collecting the required historical inputs:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare_tail.ps1
```

Rebuilding starts a different candidate; do not mix its forward results with the old pair when deciding promotion. The study writes timestamped research output. Re-running it does not create fresh evidence.

## Before promoting the candidate

Collect new pregame decisions and final identity-matched box scores after this change. Compare probability calibration, loss, projection error and exact-price returns by stat and side, with dates as clusters. Review anomalous high-confidence predictions individually. Check quarter and full-game markets independently. A missing or uncertain source result stays outside confirmed performance. Use the retained replay hashes to identify the frozen model/calibration pair. Do not promote solely for more overs or higher development ROI.

November 16 is a product target, not statistical proof. A polished public beta can collect feedback, but the current evidence does not justify advertising a verified hit rate or profitability.

## Validation performed

Go package tests and vet, 23 Python regressions (including a real Git race/rejected push and isolated shadow output tests), 16 frontend logic tests, the production frontend build, and rendered React checks pass. Rendered checks cover collapsed picks, preserved SVG graphs, default history and game-pick headings. A real desktop/mobile browser review is still needed; the local browser could not start in this environment. No API refresh, GitHub push or deployment was performed here.
