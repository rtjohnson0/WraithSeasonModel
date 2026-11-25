# Fast Pregame Refresh — cached stats, fresh both-team injuries

## Use it

Go to **GitHub → Actions → WraithPicksSZN daily → Run workflow**.
Choose **fast** (the new default). The normal **daily** mode remains for
full background data collection, calibration research, grading, and audits.

Scheduled afternoon/evening `pretip` runs also use the same fast path.
Morning `daily` stays comprehensive so caches keep improving independently.

Fast mode:

1. Restores prior `data/stats_*.jsonl`, advanced/context/tracking,
   active rosters, and existing trained model/calibration from GitHub
   Actions cache and tracked state.
2. **Does not re-download the season, historical opening odds, or BDL
   context feeds; does not retrain or rerun costly settlement audits.**
3. Fetches **new** same-day NBA schedule, player props, game-market odds
   and the league-wide BallDontLie injury endpoint in one snapshot.
4. Requires the injury endpoint to finish successfully. The fetch writes
   `snapshots/injury_health_DATE.json` only after a complete, successful
   injury pagination. It is metadata only, not a public medical report.
5. `verify_pregame_injuries.py` checks the receipt is within 30 minutes,
   is for the exact current Eastern game day, matches the schedule's
   capture time, and includes valid **home AND away team identities**
   for each scheduled game.
6. Refreshes projected/confirmed lineups when available and fetches
   optional additional SportsGameOdds quotes. An optional feed failure
   cannot manufacture a confirmed player status.
7. Generates V2 props/combos with the existing `rotation-v5` policy
   (or explicit `WRAITH_V2_ROTATION_POLICY=rotation-v4` rollback)
   and refreshes moneyline, spread and total boards.
8. Publishes through the same existing Github Pages/Whop and state
   synchronization stages. Existing pregame snapshot freshness and
   odds validations still apply.

### Fix for inaccurate absent-player treatment

The previous main predictor used append-only `injuries_DATE.jsonl`
and retained each player's last seen status. If a player was marked
`Out` in the morning and **disappeared from the newer complete
report**, the old `Out` status could remain in the prediction.

Now the predictor reads **only the most recently completed capture**.
A status from a previous capture cannot make V5 adjust workload. The
same captured BDL statuses are used for both home and visiting teams,
while fresh, team-matched ESPN reports remain an additional signal.

The injury feed contains injured players, *not* explicit healthy
statuses for every roster player. Absence from the latest report means
"not listed as injured in this provider's latest complete response",
not a guarantee the player will play. Questionable/doubtful players
remain uncertain and V5 won't assume guaranteed extra minutes.

### Safe failure

If fresh injuries cannot be fetched (including HTTP 429), the run
**fails before predictions**, rather than creating a fresh-looking board
from an incomplete, empty, or old report. The last successfully
published board remains in place and is not silently overwritten.
If no usable historical cache/model exists, use full Daily once.

No earlier stats, odds, prediction logs, model weights, or evaluation
cohorts are removed; the new injury receipt is non-sensitive and
lives with normal cached pregame snapshots.

### Verify

```sh
go test ./...
python3 -m unittest discover -s scripts -p test_workflow_mode.py -v
python3 -m unittest discover -s scripts -p test_verify_pregame_injuries.py -v
python3 -m unittest discover -s scripts -p test_fast_pregame.py -v
bash -n scripts/fast_pregame.sh
```

The goal is to shorten pregame refreshes substantially, **not** to
promise a specific run duration. Time still depends on the API,
rate-limits and GitHub queue/deployment latency. A missing/failing
fresh injury fetch must not be hidden by a fast publish.
