# Versioned forward evidence and closing-line audit (experimental)

These changes do not modify the prediction, betting, selection, calibration,
settlement, public dashboard, or Whop-delivery algorithms. They are read-only
diagnostics written to ignored `run_evidence/` and retained as private
workflow artifacts subject to the repository's artifact-access controls.

## Forward validation

Run `python3 scripts/archive_forward_evidence.py` after `go run ./cmd/accuracy`.
This archives the existing strict `data/accuracy.json` report beneath
`run_evidence/forward_versions/v<version>/<config>/`, keyed by a content hash
and evaluation timestamp. It retains historical versions without treating
legacy rows as currently verified results.

`candidate_observations` and `strict_qualifying_picks` are deliberately
different. The checkpoint inventory counts captured rows without backfilling
validity from a later model or changing the strict replay gates. Model version
and config separation is supported; within-version checkpoint re-training
should be evaluated as separate baselines in a future phase.

## Closing-line coverage

Run `python3 scripts/closing_line_audit.py` after the SGO snapshot and picks.
The report is saved to `run_evidence/closing_line_summary.json`.

An original SGO-origin pick is paired only if a snapshot has an identical
player, game tip, sportsbook, stat, full-game period, and handicap, with capture
after the pick and before tip-off, at most 45 minutes from tip. The source quote
must have been updated no more than 15 minutes before the closing snapshot.
This comparison uses implied break-even probabilities at the *same* line.
If the handicap changes, the record remains unpaired and contributes to
coverage diagnostics only. This avoids inventing a probability conversion for
a different line.

**Important:** The existing workflow's fixed pre-tip schedule does not
guarantee a snapshot within 45 minutes of each game. A low coverage rate
is expected until per-game near-tip capture is scheduled. This change
does not claim closing lines that were not actually collected.

Both reports are best-effort non-public diagnostic outputs and never
overwrite public pick files or change eligible plays. No private tokens
or customer information should be inserted into them.

## Test

```sh
python3 -m unittest discover -s scripts -p test_forward_clv_audits.py -v
```

Tests use fabricated players and local temporary files only. Full production
pipeline tests are separate, and must pass before merging.
