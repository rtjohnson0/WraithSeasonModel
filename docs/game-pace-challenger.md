# Separate possessions and efficiency game challenger

## What ships

`go run ./cmd/gamepace` runs after the existing game model in Daily and pre-tip, on Bash and PowerShell. It writes `data/gamechallenger.json` and freezes the first fresh pregame direction per game/market in `data/gamepace_picklog.jsonl`. The regular game settlement command grades its own `data/gamepace_settled.jsonl` alongside, never into, game-v2's record. Export exposes the board and record in a new **V2 → Game markets** tab with same-game score comparisons and access to the original model’s ML, spread, total and record views. Main game picks, player props and their records are preserved. The current game markets are also restored inside V2 prop cards’ matchup sections. Challenger rows can never become selected stakes, even if an evidence flag is accidentally present.

No API calls or full historical downloads are added. Existing box scores already contain the required fields; unknown/null possession fields are rejected instead of filled with zero.

## Construction and timing

A game needs explicit full-game final scores, consistent game/team identity, five or more played appearances per team, complete FGA/FTA/offensive rebounds/turnovers for every played row, matching summed final points, and complete team minutes. Estimated possessions average both teams' `FGA + .44 × FTA − OREB + TOV`. This is an estimate, not measured play-by-play possessions. Team minutes identify regulation/overtime clock length; pace is normalized to 48 minutes and scoring efficiency uses all full-game possessions.

Three ridge regressions fit possessions, home points per possession, and away points per possession. Inputs are earlier team offense/defense/pace, rest and back-to-backs, plus projected player-composition features for scoring, rebounding, creation, threes, turnovers and blocks. These are jointly fitted box-score associations, not measured rim protection, on/off effects, or causal player values. Expected overtime scoring is an average learned only from earlier games, added separately to the regulation score. It is not a prediction that this particular game reaches overtime.

Historical player workloads average the team's previous ten games, counting nonappearances as zero, and production rates use up to twenty earlier appearances within 180 days. At least six earlier team games and 80% player-rate coverage are required. Same-day games are all featured before any same-day outcome is absorbed; team profiles expire across gaps over 90 days. Forecast guards bound pace to 80–120 and efficiency to .8–1.5 points per possession. Ridge penalty is 50; model fitting needs 300 earlier complete games.

Live lineup context uses only the fresh `rotation-v4` full-game allocation captured by the prop pipeline, at most 30 minutes old and never future-dated. Team/date, current roster IDs, valid roles, 0–48 minutes per player and a 240-minute total are checked. Valid projected workloads change the model's composition inputs; their combined effect is bounded at ±3 points per team versus the earlier-workload baseline. No separate missing-star bonus or sum of player prop projections is added. Unknown or insufficient lineup evidence retains baseline workloads explicitly. Questionable-player alternatives remove only the named player alongside confirmed outs and remain sensitivity scenarios, not probabilities of sitting. Fresh expected starter information can inform V2's rotation roles; starting is not treated as guaranteed workload.

## Probabilities and records

Forecast errors supply separate margin/total scales, with conservative floors of 8/12 points. Live uses monthly out-of-fold error scales when at least 300 forecasts across 30 earlier dates exist; training residual scales otherwise remain an unvalidated fallback. Spread/total prices use continuity-corrected normal tails at the exact quoted line, including pushes. Moneylines use the home-win tail of the margin distribution. These initial distributional choices require forward validation; they are not calibrated skill claims.

A separate symmetric market-offset logistic fit is maintained for each of moneyline, spread and total. It learns only from this policy's own frozen, identity-checked, graded earlier decisions, with at least 300 games and 30 dates. There is no home/Over intercept and no borrowing from props, game-v2, or another market. Until those requirements are met, published probabilities are raw experimental estimates. These are selected-direction records, not an exhaustive sample of every available quote; readiness to fit is not evidence of an edge.

Forward probability evaluation refits on strictly earlier dates and reports log loss against the raw model and sportsbook baseline separately by market. Calibrated comparisons use the same later evaluation subset, never their training outcomes. Empty or insufficient samples are unavailable, not zero loss or successful calibration. No automatic promotion to stakes is included.

Quotes are reused from the current game-v2 slate after its existing cleaning, with paired prices and at least three books. They are rechecked for the exact parent tip, a pre-tip run, nonfuture captures/revisions, a 30-minute capture limit and six-hour price limit. No substitute market lines are synthesized. Frozen decisions retain exact quote, raw/market probabilities, model coefficients, features, lineup assumptions and calibration state for reproducibility. Later runs cannot replace the first decision. Policies, boards and settlement files are isolated.

## Actual initial result

The reproducible November 19 cutoff produced 3,687 monthly walk-forward forecasts across 711 dates. On those same games:

| Raw forecast | Current game-v2 ratings MAE | Challenger MAE |
|---|---:|---:|
| Winning margin | 11.1901 | 11.3417 |
| Game total | 14.8337 | 14.9939 |

Lower is better. **The challenger is slightly worse on both measures.** This earns experimentation and transparent comparison, not replacement or an accuracy claim. The historical test uses earlier projected workloads and does not reconstruct historical pregame injury labels. It therefore does not establish that the live absence override improves forecasts. The point-error study is not a sportsbook-edge backtest and supplies no historical moneyline/spread/total hit-rate claim. See `gamepace-study-2026-11-19.json` for coverage and exclusions (16 conflicting/incomplete box scores and 972 incomplete workloads).

## Commands

Normal Daily and pre-tip include this step after `cmd/games`. To run only the challenger locally, after fresh props/rotations and game lines:

```powershell
go run ./cmd/gamepace
```

Read-only historical point study, with a fixed reproducible cutoff:

```powershell
go run ./cmd/gamepace -audit -date 2026-11-19 -out run_evidence/game-pace
```

This writes only the requested diagnostic directory. It does not reconstruct a historical live board or backdate a posted pick. Pending and review results are excluded from accuracy. The runtime always uses the actual clock for quote validity.
