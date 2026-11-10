# Full-game market safeguards

This model trains on full-game statistics. Quarter, half and first-three-minute
markets must never be priced or graded with those totals.

## What changed

- BALLDONTLIE still uses an exact allowlist of full-game prop types; explicit
  `period`, `period_id` and `periodID` metadata on a prop or its market is now
  checked too. Unknown explicit periods are rejected.
- SportsGameOdds still requires `game` in the odd ID. New SGO snapshots retain
  `period: "game"`; loading snapshots rejects explicit partial periods.
- New pick, combo and V2 log entries persist `period: "game"`. Appending a
  non-full-game pick fails before writing any entries from that batch.
- Shared pick/settled loaders exclude explicit partial periods and unsupported
  stat labels. All three settlement commands and their exported records use
  these loaders. Grade also refuses to grade a partial entry, and summaries
  exclude it rather than treating it as a win, loss, push or DNP void.
- Legacy entries with absent period metadata retain full-game compatibility.
  A legacy quarter pick already mislabeled as plain `points` cannot be
  identified with certainty from the pick alone. A low points line is not proof.

## Check existing records

Run from the project root, with the scheduled pipeline stopped:

```sh
python scripts/audit_periods.py --data data
```

The audit checks picklog/settled, combo and V2 logs against saved BALLDONTLIE
`data/snapshots/props_*.jsonl` snapshots. Statuses:

| Status | Meaning | Action with --apply |
| --- | --- | --- |
| exclude | Explicit partial period or unsupported stat on the saved pick | Quarantine |
| suspected | Exact game/player/book/base-stat/line matches a partial market, with no matching full market found | Keep for review |
| ambiguous | Both full and partial market evidence matches | Keep for review |
| full | Matching full-game snapshot evidence | Keep |
| unmatched | No matching snapshot evidence | Keep for review |

Snapshot matching cannot prove the source of a legacy pick. Check suspected and
ambiguous rows against the original capture at the pick time before correcting
their period metadata. Do not change a loss to a win using a full-game total.
This audit does not reconstruct quarter results or a quarter-trained model.
It does not infer periods from old SGO snapshots whose period was discarded.

To quarantine explicit invalid rows, preserve backups, and save a detailed JSON
report:

```sh
python scripts/audit_periods.py --data data --apply
go run ./cmd/export
```

Every changed log gets a timestamped `.bak` copy and an `.excluded` file holding
its removed rows. Restore the `.bak` copy to undo the file change. The audit
validates all input JSON before changing any logs. Republishing the site through
your usual workflow is required to update its displayed record. Other cached
reports and model fits are not rewritten by the audit; regenerate them through
your existing workflow after reviewing the source logs.

## Validation

```sh
go test ./...
python -m unittest discover -s scripts -p 'test_audit_periods.py'
```

Both passed. Regression cases cover mixed full-game/quarter markets, partial
combos, metadata aliases, explicit unknown periods, SGO snapshot overwrite
prevention, settlement exclusion, legacy compatibility, persisted full-game
metadata, low legitimate lines, ambiguous evidence, dry run and backup/quarantine.

The supplied ZIP contains source code and sample website data, but no real
`data/` logs or snapshots. No historical win/loss correction has been performed.
The current feed sample verified `points_1q` is separate from `points`; the
original ZIP already filtered that suffix, so the specific historical entry
point remains unverified. API keys are not included in this package.
