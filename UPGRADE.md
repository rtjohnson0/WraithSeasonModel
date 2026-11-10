# Model and workflow upgrade

## Windows UTF-8 hotfix

The Python audits now explicitly read UTF-8 (with optional BOM) and write UTF-8.
Git synchronization explicitly decodes UTF-8. Windows PowerShell wrappers also
set PYTHONUTF8=1 for child Python processes. This fixes cp1252 decoding failures
on player names such as Luka Dončić without changing the original logs.

If a run stopped at `audit quote provenance`, it stopped before training.
After replacing the updated files, rerun `scripts/daily.ps1` without SkipTrain,
then export and push. Previously downloaded history does not need downloading
again solely because of this encoding error.


This package updates the source and GitHub Actions workflow. It does not contain
or overwrite your real `data/` directory, API keys, trained models, or Git history.
Copy the project files into your existing checkout, preserving those items.

## What is fixed

### Quotes, periods and settlement

- SGO Over and Under must have the same bookmaker threshold, valid American
  prices, and neither side may explicitly be unavailable. Both sides' update
  timestamps are retained. The older timestamp controls freshness.
- New SGO snapshots retain pair verification, both thresholds, event identity,
  event start and capture time. Name matching also checks the event start.
  Legacy SGO snapshots without pair evidence cannot supply executable prices.
- The unmodified raw SGO events are saved as `data/snapshots/sgo_raw_DATE.jsonl`
  to make future pairing audits possible.
- Quarter/half/first-three-minute markets remain excluded from this full-game
  model. Explicit unknown periods are rejected.
- Quote selection rejects captures at/after tip, unknown start times, future
  timestamps, missing prices, stale snapshots and stale price updates. This
  happens before consensus, anchoring, selection and logging. Defaults are
  six hours for price updates and 30 minutes for response capture age; adjust
  `-maxquoteage` and `-maxcaptureage` deliberately for your feeds.
- The same physical sportsbook appearing through both feeds counts once.
- Historical daily prop evaluation uses the latest eligible pre-tip capture,
  in timestamp order even after a merge. Closing references require a capture
  within an hour before tip; otherwise CLV remains unknown.
- Live game spread/total inputs are filtered for pre-tip status and freshness.
  Historical opening-time features use opening game odds rather than late
  daily snapshots that could contain future information.
- Explicit in-progress box scores and partial-period stat rows cannot become
  full-game results. Legacy historical rows with no game status still load.
- New picks and market/projection rows carry quote audit version 2. This means
  the quote passed validation; it is not a claim of predictive skill.

### Records and historical repair

The report you pasted is not the underlying pick log or raw feed. It cannot
prove which historical prices were genuinely available. No historical win or
loss has been invented, deleted, or regraded in this package.

Default main/combo/V2 settlement reports, the line report, threshold sweep,
review, and the exported performance record exclude legacy unverified rows.
The original rows remain on disk. Main reports support `-legacy` for forensic
inspection. `cmd/export -legacy` also permits the legacy main/combo record; the
V2 website record and line website record stay verified-only.

A lower or initially empty verified record after upgrading is expected. It
separates new validated quotes from the old record; it does not turn old wins
into losses or declare all old results invalid. The website explains this.

Audit your actual files with:

```sh
python scripts/audit_quotes.py --data data
```

This writes a detailed `quote_audit_TIMESTAMP.json` and leaves source logs
unchanged. Missing evidence remains `unverified`. Line size alone is never
proof of a quarter market or a bad quote.

To apply explicit exclusions and rebuild the exported record:

```sh
python scripts/audit_quotes.py --data data --apply
go run ./cmd/export
```

The audit writes a backed-up `data/exclusions.jsonl` registry. Pick/settled
loaders honor it; the original rows are retained for inspection. Automatic
exclusions require explicit period/post-tip evidence, or a raw SGO pairing
failure tied to the exact recorded quote capture. Old snapshots often lack
that information, so they cannot be automatically certified.

The older `scripts/audit_periods.py` remains available for snapshot matching.
It distinguishes suspected from explicit partial periods and backs up logs
before quarantining explicit invalid rows. Stop scheduled writers before
running either audit manually with `--apply`.

### Prediction and validation

- A shared `predictor.Adjustments`/`Forecast`/`Price` path supports live scoring,
  walk-forward pricing and replay. Unvalidated live-only learned corrections
  and the big-minutes-cut overlay are now off by default. Explicit flags can
  still enable experimental overlays; their policies are captured in replay.
- Training produces model version 4. Minutes residuals are estimated from a
  chronological holdout, rather than the minutes regression's own fit rows.
  Small datasets get a broad residual prior.
- Version 4 estimates stat dispersion from held-out predictions and shrinks
  player-specific dispersion toward the stat-wide estimate. It does not add
  a new attempts/shooting model or establish higher real-world profitability.
- Existing version-3 models still load. They keep their old uncertainty model
  until you retrain. A version-4 model requires version-matched calibration.
- Calibration/evaluation splits keep entire dates together. Model comparisons
  use date-clustered uncertainty rather than counting every prop as independent.
- Median-line Brier baselines use only earlier dates, with shrinkage toward
  50% when little prior evidence exists. A tiny observed improvement is no
  longer printed as proof of an advantage.
- `cmd/lines` compares V1 and V2 on identical stored lines and timestamps.
  `cmd/v2 settle` labels its different selected sets as an unmatched strategy
  comparison. V2's stat/side gates require verified history, 20 dates, the
  sample threshold and a date-clustered gain before promotion.
- V2's live adjustments no longer learn from unverified projection logs.
- Historical `backtest -lineup` is explicitly exploratory: realized historical
  participation cannot establish what was known at opening time. It cannot
  write production calibration. Timestamped forward decisions are the source
  for legitimate lineup-model calibration.

To adopt the new default pregame model, use your existing real history:

```sh
go run ./cmd/train
go run ./cmd/backtest -season 2025
go run ./cmd/pull snapshot
go run ./cmd/sgo snapshot
go run ./cmd/predict
```

The backtest needs `data/opening_props_2025.jsonl` and appropriate earlier
stat history, as before. Do this locally and review the held-out results before
committing `data/model.json` and `data/calibration.json` together. Model versions
are not automatically retrained in the scheduled `daily -s` workflow.
The daily scripts without SkipTrain now refit calibration after training.

### Exact input replay

New runs save the primary model checkpoint, feature vectors (including missing
values), numerical adjustment policy, calibration coefficients and quoted
prices under `data/replay/`. These remain private workflow artifacts, not
public website data or routine Git commits.

```sh
go run ./cmd/replay
```

This reproduces captured primary-model probabilities, rejects mismatches and
then evaluates completed questions. Logged V2 probabilities are compared on
those same questions; V2 is not reconstructed from its own checkpoint here.
Timestamped input capture prevents later injury reports from silently replacing
what a run actually knew. It cannot reconstruct missing historical inputs.

For calibration from accumulated timestamped decisions:

```sh
go run ./cmd/replay -fitcal
```

Only matching model versions/configurations and the default numerical policy
qualify. It requires separated dates, sufficient samples and a positive
held-out improvement over market probabilities with date-clustered evidence.
Insufficient evidence leaves the existing calibration file unchanged. This is
also the supported route for a lineup model once enough forward data exists.

### Combined markets

Combos no longer borrow calibration coefficients from their component singles.
Until a combo has its own valid calibration it uses the market probability;
explicit model-weight flags remain experimental, and combos remain unstaked.
Every eligible median combo question is logged, including those not selected.

```sh
go run ./cmd/combos calibrate --before 2026-11-10
```

Choose the cutoff for your run. The command trains only on earlier dates,
checks separate later dates and saves only combo fits that improve held-out
log loss. It needs new verified combo-market captures; old selected combo
results alone are not a suitable calibration dataset.

## GitHub workflow and local publishing

The workflow now:

1. Serializes active workflow executions across the repository; schedules use
   Eastern 9:17 AM, 3:17 PM, 6:17 PM and 8:17 PM through both DST offsets.
   GitHub can replace an older pending execution; running work is not cancelled.
2. Runs tests before the model pipeline.
3. Caches reproducible inputs separately from authoritative logs/models.
   An old cache can no longer restore a corrected or deleted state file.
4. Uploads run state, raw quote evidence, replay inputs and CSVs as 90-day
   artifacts even when a later pipeline step or push fails.
5. Commits existing state paths individually, fetches the current branch and
   resolves data conflicts using three-way JSON record merging. Independent
   appends survive; explicit deletions are not resurrected; model/calibration
   updates made upstream are preserved as a coherent bundle.
6. Retries a normal push after fetching/merging a newer branch. It never
   force-pushes, repeatedly rebases an unresolved conflict, or removes conflict
   markers by blindly concatenating JSON files.
7. Stops on a real source conflict, aborts that merge and retains local commits
   plus the artifacts for recovery. Credentials/branch-protection failures
   cannot be fixed by bypassing repository permissions.
8. Validates merged source, regenerates site data from the merged records,
   builds the site and deploys Pages. Bootstrap only downloads inputs.
9. Rebuilds on source/data pushes to the default branch. Bot commits use
   `[skip ci]` to avoid triggering a publishing loop.

`scripts/push.ps1`, `publish.ps1` and `publish.sh` use the same Python
`scripts/sync_state.py` merge logic. Local source conflicts still require review;
the scripts do not pick a source-code version for you. Python 3 is required
locally. The workflow uses the runner's installed Python.

Keep your existing repository secrets. No key is included in this ZIP.
This package has not been pushed to your GitHub repository. After replacing
files and pushing, run the workflow's `site` mode to verify Pages configuration,
or `daily` to refresh data. A cold cache requires `bootstrap` first. Protected
branch rules and Pages settings remain repository configuration.

## Validation performed

- `go test ./...` and `go vet ./...`.
- Python audit and synchronization tests, including real temporary Git
  repositories, concurrent source/data changes, upstream model corrections,
  deliberate record deletions and a rejected push followed by a successful retry.
- GitHub workflow validation with actionlint; shell syntax checks.
- `npm ci` and the production site build.
- Synthetic end-to-end training, calibration, prediction, export and exact
  replay of 36 captured primary-model decisions.

Synthetic tests validate implementation, not a betting advantage. Your real
logs and trained files were not included, so their corrected totals and any
change in real predictive performance have not been measured. PowerShell
wrappers were reviewed but not executed on a Windows host.

## Current board and book accuracy

The published props board now comes from a replaceable `data/board.json`, not the
best edge ever logged that day. A new 6.5 quote replaces an old 8.5 quote; a
successful run with no qualifying picks clears the old board. Historical logs
remain available for audit and grading, separately from current opportunities.
Extras and V2 must match the current prediction run exactly before export.

The predictor requires three independent physical books by default and excludes
prop thresholds more than 0.5 units from the market median. If direct and SGO
feeds disagree on a book's available thresholds, that book is withheld. These
are conservative API checks, not direct verification of a sportsbook screen.
There is no forced pick per game or default extra-play filler.

The frontend shows exact book, threshold, odds, source update and capture times,
break-even probability and a per-book comparison. It hides stale, started and
unconfirmed prop snapshots by default, refreshes published data every minute,
and distinguishes publication time from the scoring run. Reloading the page
cannot refresh the sportsbook feed: run the pretip workflow to fetch new odds.

## Moneyline and game-total cards

`cmd/games` writes `data/gameboard.json` on every successful run, including empty
runs. It now uses timestamp-filtered pregame odds (capture within 30 minutes,
source update within six hours), keeps each book once, cleans discrepant lines,
and requires three books per displayed market. Export includes this separate
snapshot only for the selected date; the frontend expires each quote using its
own times and removes games at scheduled tip-off.

Cards include projected scores, total, an approximate 80% total range, both
moneyline sides, and Over/Under probabilities at each selected book's exact
total. Comparisons show break-even, no-vig book probability, probability edge
and model expected ROI. Whole-number totals include estimated push refunds;
probability and edge columns condition on no push, while ROI includes pushes.
The normal residual assumption and historical residual spread are estimates;
the displayed range has not been independently calibrated for coverage.
These cards stay labeled paper tracking and show backtest evidence separately.
They do not add model probabilities to the verified props performance record.

No prop-model retraining is needed for these display and quote changes. After
copying these files into your checkout, run `scripts/pretip.ps1`, then export and
push. The game cards need `data/gamecal.json`; if absent, train that calibration
with `go run ./cmd/games -backtest -season 2025` first. Workflow state syncing
includes both board snapshots, so concurrent changes cannot union old board
quotes into the latest run. The site build also checks quote expiration logic.


Period labels are now explicit on prop cards: “Full game · Points” (and the
other full-game stats). Missing legacy period metadata is labeled “Period
unverified”; it is not automatically presented as a verified full-game market.
The exporter rejects explicit partial-game entries even if one appears in a
saved board snapshot. Quarter picks are excluded from this full-game model because its features,
projections, recent form and settlement totals are full-game only. The separate
1Q points comparison below handles first-quarter data.


## Separate 1Q points comparisons

The site can now show both “1Q · Points” and “Full game · Points.”
`go run ./cmd/quarter` reads explicitly tagged BDL first-quarter point markets
from the existing snapshot. A small line is never used to guess the period.
The usual three-book consensus, pregame and freshness checks still apply.
Full-game and quarter parsers cannot admit one another's markets.

The quarter command fetches BALLDONTLIE play-by-play, final game quarter totals
and confirmed starting lineups for the last ten prior full-game appearances of
each quoted player. Data begins in the provider's 2025 season. Scoring is
attributed only when the participant ID, player name and team agree. All Q1
scoring must reconcile with both official team Q1 totals, and there must be
quarter-completion evidence. Ambiguous scorers, incomplete lineups and missing
quarter totals leave that game unavailable. Nothing is divided by four.

Actual zero-point quarters are retained when quarter participation is confirmed
by the starting lineup or a Q1 play. Nonparticipants are not treated as a
zero-point performance. Missing data is never substituted with full-game points.
The latest five verified 1Q performances and average are shown, with missing
recent-game coverage reported. The initial quarter average and smoothed hit
rate are descriptive paper estimates, not the trained full-game model.
A smoothed directional estimate requires at least eight confirmed quarter
appearances and no missing games in the recent window. It has no stake or
claimed calibration and needs a separate chronological backtest before use
as a calibrated probability model.

Quarter paper entries and grading live in `data/quarterlog.jsonl`. They use
period-specific identities and actual Q1 points only. No confirmed Q1
participation means REVIEW, not a win, loss, or automatic sportsbook void.
Whole-number point lines push on equality. The record counts quote outcomes;
multiple books/refreshes are correlated and do not represent independent bets.
It never enters full-game training or the full-game record. Old mislabeled
historical results are not retroactively relabeled as quarter results.

Normalized verified quarter games are cached under `data/quarter_games/`, and
recent games are refreshed for corrections. These small normalized files and
the separate board/log participate in the existing conflict-safe state sync.
A failed quarter refresh clears its current board; it cannot reuse a full-game
board or retain an old quarter board as if it were new. API access failures
appear under quarter data coverage. The command can run with `-offline` against
previously validated cache. No external API calls were needed for the fixture
and regression checks; live coverage depends on the account's endpoint access.

Both daily and pretip wrappers now call the quarter command automatically.
For an immediate Windows refresh, run `scripts/pretip.ps1`, then export and
push. New quarter data needs a first download, but no full-game model retrain.


## Daily sync fix for previously tracked downloaded inputs

Some older checkouts track `data/stats_2026.jsonl` despite the ignore rule.
Daily refreshed that file, but the state synchronizer only staged prediction
outputs and then stopped on the remaining tracked change. The synchronizer
now includes already tracked stats, advanced stats, active-player lists,
opening lines and snapshot JSONL inputs. New untracked downloads remain out of
Git. Concurrent stats edits merge by player/game/period identity; distinct
players cannot collapse under the pick-log key, and the last row in a local
input file wins before the three-way merge. Concurrent remote corrections
retain priority, with deliberate deletions still respected. Unknown source
changes continue to stop synchronization.

A local bare-remote regression reproduces tracked stats updates, concurrent
source and model changes, conflicting stats corrections, new player rows and
a rejected first push. It verifies a clean tracked worktree, all player rows,
remote corrections and successful retry without adding untracked raw inputs.
After pushing this script, start a new Daily workflow run on the updated
branch; rerunning the old failed job uses its original source revision.

## Corrected final-stat settlement and searchable result audit

Settlement previously skipped every pick already in the settled log. A result
saved from an incorrect or older actual therefore survived later box-score
corrections. The generic feature loader also converts absent numeric fields to
zero, which is unsafe as settlement evidence. New `internal/settlement` reads
raw box-score fields with presence checks and keeps them separate from features.

Individual, combo and V2 settlement now rechecks all saved full-game results.
It requires the exact player/game IDs, matching player name, game date, explicit
Final status, actual game team, logged team when present, positive recorded
minutes, and a present nonnegative integer for each requested stat/component.
When available, total rebounds must equal offensive plus defensive rebounds.
Explicit partial-period rows cannot supply full-game actuals. Missing players
or zero minutes do not prove a sportsbook DNP void. These cases become REVIEW
and contribute neither wins/losses nor profit. Unknown stat names cannot turn
into zero. Source player/game/team IDs, date, name and raw-row SHA-256 accompany
confirmed results. Quote provenance alone no longer verifies a settled result.

Before changing settlement files, the command saves an exact original backup.
Corrections, old/new values and reasons are written to `settled_audit.json`,
`combosettled_audit.json` and `v2settled_audit.json`. Audit history survives later
runs and concurrent state sync. It is searchable on the Record page by player,
date, game ID, stat or reason. The immutable original pick logs are not edited.
Old results lacking final-stat evidence are withheld from the default record
until checked. Historical unverified results remain an explicitly separate
forensic view, not the verified headline record.

The same strict final actuals now feed the CSV review, live projection check,
and all-line calibration diagnostics. Diagnostics compare logged player names,
dates and teams when both are present. They cannot silently use a missing
requested component as zero. These checks validate provider row consistency;
they are not an independent confirmation that the provider's final box score
agrees with another statistics service.

To audit current-season results in Windows PowerShell after copying this ZIP:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\audit_results.ps1 -Season 2026
```

This refreshes the selected season, regrades every saved individual/combo/V2
result and exports the site data. `go run ./cmd/resultaudit` can instead use the
existing downloaded files without a model or retraining. A regression fixture
proves a saved Naz Reid Under 9.5 WIN with actual 1 changes to LOSS at actual 13;
that fixture is not an audit of an unseen user's production data. To establish
which source row caused a particular production error, supply the current pick
logs, settled logs, stats and model/calibration files. Screenshots alone cannot
establish the stored player/game identities or provider row contents.

## Clearer individual, combo, moneyline and total explanations

Individual prop cards now distinguish evidence supporting the side from
counterarguments. A defense allowing a lot of points is explicitly against an
Under, even if lower projected minutes or other factors still favor that side.
A minutes sensitivity check warns when using the normal workload at the same
rate would move the implied total across the quoted line. It is labeled a
sensitivity, not a second calibrated forecast. Defense already enters the
learned `opp_allow` and `pos5_allow` rate features; its raw rank percentage is
not applied as an extra multiplier. New run notes report the actual fitted
rate sensitivity to league-neutral defense inputs and the largest fitted
minutes terms, with their reference and limitations. Disabled defense features
are identified explicitly instead of claiming they influenced the projection.

Moneyline/total cards name the projected winner or central Over/Under lean,
then separately name a price-based paper value lean, or PASS when neither
side has positive expected value. They explain projection versus exact line,
probability versus break-even, and the ratings-only and market-consensus inputs
to the fitted blend. A likely winner is not automatically a worthwhile price.

A cause of absent combos was the conservative no-calibration fallback: book
probabilities alone generally cannot beat the price's break-even probability.
The site now shows available, validated combo comparisons even when none clear
the tracking gates. Comparison-only rows are labeled COMBOWATCH, never staked,
and never appended to the qualified combo paper log. Qualified combos retain
their own tracking and grading. Feed, freshness, consensus and pricing counts
explain where combo lines disappeared. No synthetic combo threshold is built
from unrelated single-stat sportsbook lines. Direct combo calibration remains
required to claim calibrated model confidence; singles' calibration is not
borrowed to create picks.

## Under/Over selection diagnostics

Every new scoring run writes `data/side_audit.json`. The board shows one quote
near the median per player/game/stat, raw-model Over/Under counts, calibrated
counts, probability-bar counts, price-edge-bar counts, final posted counts,
mean projected-minus-line gaps and minutes shifts. It highlights Under-heavy
raw predictions, calibration majority reversals, rejected Over probabilities
or prices, and repeated low-minute Under projections. This diagnoses where
asymmetry arises without counting every duplicate book as a new market.
It does not force a 50/50 split, refit weights on current games, or establish
which component caused a production skew without the actual saved model and
inputs. Correct actual-stat errors first; evaluate minutes/rates/calibration
on strictly later dates before accepting a numerical bias correction.

New stats downloads include their full-game endpoint and UTC fetch timestamp.
Strict settlement requires this provenance; existing unstamped files need the
one-time season refresh in `audit_results.ps1`. A newer verified result cannot
be reversed by an older cached download. Stat-input conflict resolution also
prefers a newer stamped API correction over an older competing row, while
retaining the existing deliberate-deletion behavior. The audit reports older
cached sources that were ignored. This prevents a cache restore or concurrent
pull from silently undoing a correction on the next daily run.

The audit page also has “Show every checked result,” so search covers all
current checks rather than only changed outcomes. It retains bounded correction
history alongside those current receipts. Confirmed settlement conflicts use
their provider-download times: a concurrent older WIN cannot replace a newer
verified LOSS, and newer evidence requiring REVIEW withholds the result.
The full-season audit wrapper backs up the original stats file before download,
so the pre-refresh source remains available for investigating an incorrect row.
