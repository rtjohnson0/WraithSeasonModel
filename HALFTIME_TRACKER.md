# Wraith In-Game / Halftime Tracker

Use the **In-Game** tab in the same Wraith dashboard. It retains the
original published **full-game** player-prop line and pick (Board and V2),
and displays independently collected current cumulative NBA box scores.

An actual complete halftime capture also saves **PTS, REB, AST, 3PM**.
That first-half box remains visible when subsequent 3Q/4Q updates arrive
without replacing the original pregame betting record.

Example: **Over 6.5 assists**, **4 at halftime** = **3 more assists**
needed to finish Over 6.5. This comparison is not a new available wager.

## Live sportsbook side-by-side

If BDL actually provides a paired, *post-tip updated*, *<=3-minute-old*
full-game O/U market, show its over/under book price and de-vigged implied
probability, ranking those independently of the pregame probability.
Never recycle a pregame quote as a live price, treat period 2H as full game,
or label implied book probability as Wraith's live model probability.
When current offerings cannot be verified, the screen explicitly says
so rather than inventing line 6/7 or any confidence percentage.

**No in-game model trained/validated yet.** These are comparisons, not
new Wraith betting picks or projections of second-half performance.

## Run in the short halftime window

GitHub Actions → **Wraith halftime quick check** → **Run workflow**.
A separate *best effort* five-minute evening schedule also checks
ongoing games, but GitHub Actions queue/deployment latency is not
guaranteed to meet an exact halftime window.

The new job only:
- checks out the saved pregame state;
- queries the league's live game status + cumulative stats for
  **in-progress games from today's original saved slate**;
- makes a bounded query for currently offered full-game sportsbook
  prices, *only during a reported halftime*;
- patches the already deployed public site's JSON in memory without
  changing old lines/picks, and builds/deploys the existing frontend.

It does NOT run Daily, pull a season, refresh model weights, settle
picks, or rebuild historical odds. Missing live stats never mean zero.
A stale snapshot (over eight minutes) is hidden from the In-Game tab.
A game with no usable live box is skipped; an API failure does not
overwrite the previous deployment.

## Access and privacy

The halftime action is **public Pages only**; for `WRAITH_DELIVERY=whop`
it does not execute, because it cannot publish player-level comparisons
into a public static file when private access is required. The frontend
In-Game view uses the same authenticated `/api/picks` response as other
screens if a protected delivery implementation is later added.

Raw live feed responses, model inputs and pregame decision histories are
not committed by the standalone halftime workflow. No live odds are
added to the immutable original pregame selection records.

## Validation

```sh
go test ./cmd/halftime ./cmd/export ./internal/boardstate
python3 -m unittest discover -s scripts -p test_publish_in_game.py -v
cd site && npm ci && npm test && npm run build && npm run build:private
```

Initial release is a **comparison tool only**, not a forward-tested
second-half model. The next enhancement is to learn an independently
validated conditional 2H model based on halftime score, role, minutes,
fouls, pace, rotation/injuries and current quoted betting lines.
