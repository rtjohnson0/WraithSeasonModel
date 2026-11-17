# Accuracy work and launch evaluation

A correct scoreboard and a high displayed confidence are not proof of an edge. This update adds forward evaluation and selection controls. It does not certify the current model as profitable or claim that the new experiments improved predictions.

## What changes

1. Frozen pregame forecasts are checked against their original model checkpoint, feature vector, model training cutoff, calibration training cutoff, exact player/game identity and explicitly final full-game stats. Old records without that evidence stay excluded. No future games are graded. One earliest eligible question per player/game/stat is selected before looking at the outcome. Repeated books and refreshes cannot inflate the sample.
2. Minutes uncertainty gets a ±4-minute scenario check. If the chosen side reverses, the play becomes CHECK with zero stake. An offline challenger learns how much to blend toward recent typical minutes on the earlier two-thirds of dates, then evaluates it on later dates. The same player-game is counted once. Its learned weight is not automatically applied: a changed minutes policy needs matching probability calibration and a subsequent forward test.
3. Existing separate points/rebounds/assists/threes models stay separate. New calibration experiments fit each stat on earlier raw forecasts and compare with frozen existing forecasts and sportsbook prices on later dates. No parameters are silently promoted from these experiments.
4. The report breaks results down by stat AND side, with projection bias, MAE, minutes error, probability reliability, ROI at exact recorded odds and date-clustered gain. It never forces equal numbers of overs and unders. A one-sided slate alone does not establish that the other side is better.
5. Win rate is evaluated alongside Brier score, log loss and the sportsbook's own vig-free probability at the same line. Selection now prioritizes estimated return at the exact price, then edge and probability, among candidates that already clear the probability/price rules. This ordering corrects the earlier confidence-first preference for favorites; it is not a validated profitability improvement. These probabilities still need forward validation. Closing-line value is not used as verified evidence because old closing lines lack sufficient timestamp/price provenance.
6. The combo simulator retains shared minutes and residual correlation. Direct combo calibration now requires final identity-checked stats, pregame timing, a proven earlier model cutoff and exact prices. A combo type must improve over both raw probabilities and the sportsbook on later dates with sufficient date coverage. Comparisons without direct calibration remain comparisons, not calibrated recommendations. Quarter/game-total models remain separate and are not certified by the full-game prop report.

## Public board

The default evidence gate only permits a staked PICK when that stat AND side meet provisional forward criteria: at least 200 non-push selected outcomes over 30 dates, positive date-clustered log-loss gain over the book with z >=2, improved Brier score, a descriptive Wilson lower win-rate bound above average break-even, and observed win rate within five percentage points of average stated confidence. Evidence must match model version and feature configuration, precede the predicted date and include results within the past 14 days.

These are conservative provisional rules, not proof against multiple comparisons, dependence, changing player roles, or future losses. Results describe a fixed 55% / 1 percentage point edge policy at median-market replay lines; live board selection and available book prices can differ. Further untouched forward evaluation is necessary before a launch performance claim. Report metrics do not establish that the current retrained checkpoint independently passed every check.

A failing/missing report downgrades staked picks to PAPER; it does not erase plays. The board still displays research plays and explanations. There is no promise of a minimum number of plays per day. Do not change thresholds based on whichever historical segment looks best.

## Run on Windows PowerShell

From the project directory:

```powershell
go run ./cmd/accuracy
if ($LASTEXITCODE -ne 0) { throw 'Accuracy evaluation failed' }
python scripts/accuracy_report.py
if ($LASTEXITCODE -ne 0) { throw 'Accuracy report failed' }
Start-Process .\data\accuracy_report.html
```

Daily and pretip scripts now run evaluation automatically. Keep using the one-process execution-policy bypass when launching scripts:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\daily.ps1 -SkipTrain
```

Use fresh final stats downloaded by the updated pull command. Existing models do not need retraining to run these checks. To test different features or retrain, rerun the existing walk-forward backtest with matching feature flags before prediction. Do not enable realized historical injury/absence features as if they were timestamped pregame knowledge.

The workflow preserves immutable decision captures in a separate cache and run artifacts so evaluation can accumulate across runners. No result logs or published boards are restored from this cache. A cache miss reduces available evidence rather than inventing results. `data/accuracy_report.html` stays offline/in workflow artifacts; the public website does not expose it.

The supplied historical cache does not prove all calibration training cutoffs or fresh final-source provenance. Zero accepted forward questions is an honest initial result. Run the updated daily/pretip pipeline to accumulate evidence; missing fields cannot be backfilled from hindsight. Before a launch accuracy claim, collect enough unique forward dates, inspect errors, and retain an untouched later evaluation period.
