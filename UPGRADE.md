# Model and workflow upgrade

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
