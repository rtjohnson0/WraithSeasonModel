# Market analytics and presentation update — November 13, 2026

Apply the files in this source-only update over your existing propsmodel folder. Keep your existing data folder and GitHub secrets. This zip does not contain replacement models, live picks, results or the sample data used for UI checks. These changes are not deployed to your GitHub repository automatically.

## What changed

- Spreads now reach the game board. The previous code computed spread probabilities but only serialized moneyline and total quotes. Whole-number spreads include push probability; away handicaps use the opposite sign of the home line.
- Game directions are saved before tip at their original book, line, price and probability. The record separates all model directions (including paper/lean rows) from selected plays. The first valid pregame direction per game/market is retained; reruns do not change its historical line. No old results are reconstructed from later prices or final scores.
- Grading reads the provider's explicit final home/away scores, matching date and both team IDs. Missing finals remain pending; conflicting or mismatched sources are withheld. Flat units are comparison units, not a statement that money was staked.
- Team scoring ranks and defensive points/rebounds/assists/threes ranks use complete current-season final box scores before the forecast date. Team points must reconcile with the provider's final score. At least three games and two eligible teams are required. The displayed denominator is the number of eligible teams, not automatically 30.
- Position defense uses feed position groups (guards, wings, forwards, big forwards, centers), not invented exact five-position assignments. Games need at least 95% position coverage by opponent minutes. Missing positions are not guessed. Combo analysis uses the actual components of PRA/PR/PA/RA. Full-game rankings are not substituted for quarter defense.
- Matchup explanations distinguish supporting and opposing evidence. Rankings describe past per-game production, including overtime; they are not pace-adjusted efficiency ratings or automatic model multipliers.
- V2's long methodology moved to How it works. The V2 header is a compact status strip. Game-market and combo cards have consistent headers, prices and visible expand controls. Existing combo form/minutes graphs remain available.
- Daily and pretip scripts now attempt direct combo calibration before prediction. The site shows calibration readiness, while qualified selections remain separate from raw comparison estimates. The selection floor is 55%, with the existing price, availability and fit checks still applied.
- New game logs and calibration status are included in state synchronization and workflow evidence. Market identity prevents spread/total/moneyline rows from overwriting one another. Concurrent settlement merges retain the newer source correction; decision merges retain the earlier pregame quote.

## Why combos were empty in the supplied cached data

The supplied November 10 cache had no combocalibration.json. Its 267 saved combo market forecasts yielded no eligible calibration rows: 250 were waiting for final actuals or identity review, and 17 failed model/date/provenance compatibility. That cache is not your current live data. The new readiness report explains exclusions on future runs.

Direct combo calibration still requires sufficient earlier fitting rows and later-date evaluation, including at least 30 evaluation dates, plus individual combo-type improvement over both the raw probabilities and sportsbook baseline. Wiring calibration into the daily run does not guarantee it passes. Until it does, line comparisons are available, but raw 70–80% simulations are not promoted into selected bets. This update does not claim a demonstrated increase in hit rate or profitability.

## Windows: refresh after applying the update

Run from your existing propsmodel directory:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\daily.ps1 -SkipTrain
if ($LASTEXITCODE -ne 0) { throw "Daily refresh failed" }
go run ./cmd/export
if ($LASTEXITCODE -ne 0) { throw "Export failed" }
```

Or run the GitHub daily workflow after pushing the changed source. The workflow's existing export and build steps publish the refreshed site. Game-market records start with the next valid pregame run; final scores are checked during subsequent morning runs. No retraining is needed for the presentation/ranking/record changes.

Optional read-only diagnostic:

```powershell
go run ./cmd/combos audit
```

Do not delete the existing data folder or generated logs when applying this update.

## Validation

Regression coverage includes spread publication and pushes, exact score/team/date matching, rejection of quarter/missing-provenance results, immutable first decisions, missing-position handling, strict historical cutoffs, combo-component explanations, paper-versus-selected record separation, and concurrent state merges. Go tests, frontend tests, Python tests and the production frontend build were run. The browser blocked the local preview, so desktop/mobile visual appearance was not verified in-browser here.

## Player-card follow-up

Player and combo cards now include headshots matched against the official NBA roster by exact normalized full name. NBA IDs are kept separate from provider IDs. Names without an exact match and failed image loads show initials. The roster snapshot is dated November 13, 2026; image availability is dependent on the NBA CDN.

The compact matchup line explicitly says whether the matchup supports or works against the shown side. When a saved forecast includes the fitted defense-rate audit, its breakdown displays that measured effect separately from the descriptive allowance/rank percentage. Older forecasts without that audit do not get an invented effect. Below-55% directions are visibly labelled low-confidence leans. No fitted coefficient or probability was changed in response to a single game result.

For example, the supplied cache includes a Towns rebound under whose descriptive defense note says second-most allowed (+36%); its measured fitted defense-rate sensitivity is +1.2%, with projected minutes three below his usual workload. Those are different quantities. This cached example is not a verification of the screenshot's current forecast.
