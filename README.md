# NBA player-prop model (Go)

Predicts the probability that a player goes OVER or UNDER a sportsbook line for **points, rebounds
or assists**, for players averaging 20+ minutes (or projected for 18+). It posts a pick only when
the calibrated probability is **>= 60% (OVER) or <= 40% (UNDER) (raised from 55/45 on 2026-10-27 after live results: 55-60% plays hit ~52%, 60%+ hit ~64%)**, sizes a small stake, logs
every pick permanently, then grades it and measures closing line value (CLV).

**Read this first.** Sportsbook lines already contain most of what box scores say. This project
gives you a model, a fair way to test it, and a live scorecard. It does not give you an edge by
itself: the backtest on *your* data decides that. If `go run ./cmd/backtest` shows no gain over a
coin flip, do not bet it.

## Setup (once)

1. Install Go 1.22+ and open this folder in VS Code.
2. Copy `.env.example` to `.env` and paste your keys (no quotes needed). `.env` and `data/` are
   git-ignored. Keys that have been pasted into a chat should be rotated.
3. Pull history (a few minutes; BALLDONTLIE GOAT allows 600 requests/minute):

```
go run ./cmd/pull stats    -seasons 2022,2023,2024,2025
go run ./cmd/pull advanced -seasons 2022,2023,2024,2025
go run ./cmd/pull props    -season 2025
```

4. Test the model honestly, then train it:

```
go run ./cmd/backtest -season 2025 -ablate
go run ./cmd/train
```

## Daily use

```
.\scripts\daily.ps1          # Windows (refresh, retrain, snapshot, picks, grade)
./scripts/daily.sh           # macOS / Linux
.\scripts\pretip.ps1         # quick refresh near tip-off
```

or run the steps yourself: `pull stats` -> `train` -> `pull snapshot` -> `sgo snapshot` ->
`predict` -> (next day) `pull stats` -> `settle`.

Run `pull snapshot` several times on game day (`go run ./cmd/pull snapshot -every 30m`) so
`settle` has a closing line to measure CLV against.

## Dry run with no API (simulated league)

```
go run ./cmd/synth -out demo_data
go run ./cmd/backtest -data demo_data -season 2025 -ablate -lineup
go run ./cmd/train    -data demo_data -lineup
go run ./cmd/predict  -data demo_data -date <date synth prints>
```

The simulated league has real structure (opponents, pace, injuries, blowouts, streaks) and a
simulated sportsbook that only partly sees it. It proves the code finds structure that is there.
It says **nothing** about real sportsbooks, which are far sharper. The positive ROI it shows is
a property of that toy book, not a forecast.

## How the model works

1. **Minutes model.** Ridge regression on recent minutes (EWMA, last game, 5/30-game averages,
   starter share), rest, home court, strength gap, and which rotation teammates are out. Residuals
   are kept as empirical quantiles per predicted-minutes bucket, so the minutes distribution is
   skewed and fat-tailed (blowouts, early exits). Most prop surprise is minutes surprise.
2. **Stat model.** Negative binomial regression per stat, `mean = minutes * exp(x.beta)`, fitted
   with the minutes actually played as exposure, so it learns a per-minute rate that depends on
   context. Dispersion alpha is estimated from the data.
3. **Probability.** P(over) averages the negative binomial tail over the 15-point minutes
   distribution. Whole-number lines remove the push probability.
4. **Calibration.** A logistic correction (per stat when there is enough data) fitted on
   out-of-sample backtest results, which shrinks overconfident probabilities toward 50%.
   This is also the "blend with the market" step: the line sits near 50/50.
5. **Posting and staking.** 60% / 40% rule, best price across BALLDONTLIE and SportsGameOdds
   books, at most 4 picks per game and 2 per player (they are correlated), stake = quarter
   Kelly in units of 1% of bankroll, capped (default 2u). Start at 0.5u flat while paper trading.

### Features (groups can be switched off for ablation)

| group | what it captures |
|---|---|
| minutes, base | minutes history; baseline per-minute rate (recency-weighted, shrunk to the league) |
| form | last-10 rate vs long run; shot volume trend |
| slump | recent FG% vs the player's norm (shooting luck tends to revert; volume persists) |
| opp | opponent's recent points/rebounds/assists allowed vs league; game pace |
| game | strength gap (blowout risk cuts starters' minutes) |
| rest | back-to-back, days of rest, games in the last 7 days |
| home | home court |
| teammates | rotation players out: their minutes, starters out, and their per-game production |
| usage | advanced-stat trends: usage, assist share, rebound share, passes per minute (the double-team proxy: usage down, passing up) |

The backtest's `-ablate` retrains once per group and reports whether each helped. Keep only
groups that help on *your* data.

## Honest limitations

- **Teammates-out is flattered by a backtest on opening lines.** For history it is derived from
  who actually played, which a live run would know from the injury report near tip-off, but the
  opening line had not seen late scratches. So `backtest` leaves that group off by default
  (feature set "pregame"). Use `-lineup` (and `train -lineup`) only when you run live with
  injury snapshots, and judge that version by `settle` (live CLV), not by the backtest.
- **No double-team data.** Doubles are inferred from usage and passing trends, not tracking.
- **No Vegas spread/total feature yet.** The strength gap is a proxy. BALLDONTLIE odds are
  captured by `pull snapshot` going forward; historical depth is unverified, so it is not wired in.
- **SportsGameOdds is unproven.** Field names follow the provider's docs but I could not call
  the API. Run `go run ./cmd/sgo sample` and check the printed lines. The BALLDONTLIE field shapes
  are also from docs; if `pull` or `backtest` shows odd counts, send me the first lines of
  `data/stats_*.jsonl` and `data/opening_props_2025.jsonl`.
- **Model is refit monthly in the backtest, daily live.** Early season relies on prior seasons.
- **Lineup news speed is where books beat models.** A model that reads a stale injury report
  loses to one that acts on the late scratch. Re-run `predict` close to tip-off.
- A backtest with 55%+ hit rate over a few hundred picks is promising, not proof. Paper-trade
  with the pick log until `settle` shows positive average CLV over 100+ picks.

## Layout

```
cmd/pull       download stats, advanced stats, opening props, injuries, snapshots (BALLDONTLIE)
cmd/sgo        SportsGameOdds sample + snapshot
cmd/backtest   walk-forward test, calibration fit, ablation, scorecard
cmd/train      fit the model -> data/model.json
cmd/predict    score today's lines, post picks, log them
cmd/settle     grade logged picks, CLV, live record
cmd/synth      simulated data for a dry run
internal/feat  feature engine (no look-ahead; tested)
internal/predictor, glm, linalg   minutes + negative binomial models
internal/eval  walk-forward, metrics, calibration, ablation
internal/model, proj, backtest    probability math, baseline projection, baseline backtest
internal/props, sgo, bdl, data    data access
internal/bets, picklog            stake sizing, permanent pick log
scripts        daily pipeline (PowerShell and bash)
```

`go test ./...` runs the unit tests, including a test that deletes the future and checks no
feature changes (look-ahead), and an ablation test on the simulated league.

## Game markets (spread, moneyline, total)

    go run ./cmd/pull gameodds -season 2025          # once: last season's opening lines
    go run ./cmd/games -backtest -season 2025        # tests the team model against them, saves data/gamecal.json
    go run ./cmd/pull snapshot                       # today's odds
    go run ./cmd/games                               # best play per game per market (PICK or LEAN)

Game lines are far sharper than player props. The model blends the sportsbook line with a
team-ratings projection, weighted by what worked on past games. If the backtest's z is under
about 2, the ratings add nothing the line did not already know: treat every play as a paper bet.
Game picks are not yet in picklog/settle; they are saved to data/gamepicks_DATE.csv.

## Pick tiers and guards (predict)

- PICK: cleared 60%/40% and the edge rule. Staked (capped at 0.5u).
- CHECK: cleared the bar, but projected minutes are 8+ away from his recent average (-maxshift). Staked 0
  until you confirm the minutes (minutes limit, role change, stale average).
- LEAN: fill-in so every game has plays (-pergame). Staked 0.
All three are written to data/picklog.jsonl with their tier; `settle` reports full picks as the headline
record and CHECK/LEAN beside it, so you can see whether the guard and the 55% bar earn their keep.
Predict also warns when 75%+ of the board is one side or projected minutes shift as a group.

PAPER: plays on stats outside -stakestats (default: rebounds only, from the 2025 backtest where assists lost
money and points posted nothing). They are logged with 0 units so live results can confirm or overturn that.

Warm-up: while a team has played fewer than -warmup (default 8) games this season, its plays are PAPER.
Early in a season the inputs are last season's averages and nearly empty rolling windows, which the backtest
(fitted mostly on mid-season games) never validated. Use -warmup 0 to override.

## Model upgrades (season phase, minutes volatility)

- `season` feature group: `early` (1 at the opener, 0 after 15 team games) and `early_m` (early x 30-game minutes),
  so the model can learn to trust last season's averages differently early on. It is an ablation group like the others.
- Per-player minutes volatility: each player's minutes spread is scaled by a ratio (0.75 to 1.35) learned from
  his own past misses, shrunk toward 1 with 20 pseudo-games. `go run ./cmd/backtest -novol` measures what it adds.
- Ablation fix: each ablation run now keeps the base configuration's disabled groups off (before, the teammates
  group was silently ON in every ablation run, so every group looked harmful). Earlier ablation output is invalid.
- More history helps the minutes and rate models (not the calibration, which needs odds):
  `go run ./cmd/pull stats -seasons 2018,2019` (a long pull; 2020 and 2021 have unusual bubble/COVID schedules).

## Game environment, injury status, live calibration

- `market` feature group: the game's spread and total (`abs_spread`, `fav_spread`, `game_total`) are model inputs
  for minutes and for every stat (blowout risk cuts starters' minutes; the total tracks pace). History comes from
  `pull gameodds` (last season's opening lines) and the daily `snapshots/odds_*.jsonl`; where a game has no line the
  features are empty and treated as average. Today's lines are loaded by predict from the snapshot.
- Injury status: a player listed questionable / doubtful / day-to-day is tiered CHECK (0 units) with
  "HIS STATUS" in the why column; uncertain teammates are listed too. Re-run `scripts\pretip.ps1` after lineups are known.
- Live calibration: `go run ./cmd/backtest -season 2025 -live` also scores every saved daily props snapshot once its
  games are final, so the calibration learns from current-season lines. `scripts\weekly.ps1` does the weekly refit.
  Keep data\snapshots\ (the daily script writes it); it is the only source of this season's lines.

## Game-line sanity checks

`go run ./cmd/games` ignores prediction markets (Kalshi, Polymarket) and drops odds rows that do not look like real, current
sportsbook lines: off the half-point grid, prices outside -200/+200 or with a sum that makes no sense, spreads more than 1.5
or totals more than 2.5 from the median, a number no second book posts, a moneyline favourite that disagrees with the spread,
and rows over 4 hours older than the newest in that game. Plays where the model sits more than 4 points (side) or 7 points
(total) from the books are tiered CHECK: the model has no injury or lineup information, so a big gap is usually its blind spot.

## Tracking features: the `chances` group

Rebound chances, passes, touches and secondary assists come from `pull advanced` (the raw
`/nba/v2/stats/advanced` rows already hold them, so no re-pull is needed for seasons you have
pulled since the API added the fields). The `chances` feature group turns them into:

| stat | feature | meaning |
| --- | --- | --- |
| rebounds | `rch_tr` | rebound chances per minute, last 10 games vs last 40 (role/opportunity change) |
| rebounds | `rch_conv` | rebounds per chance, last 10 vs last 40 (luck that tends to correct) |
| rebounds | `rch_off` | share of chances that are offensive, recent vs norm |
| assists | `pass_conv` | assists per pass, recent vs norm |
| assists, points | `touch_tr` | touches per minute, recent vs norm |
| assists | `sec_tr` | secondary plus free-throw assists per minute, recent vs norm |

All are shrunk log ratios against the player's own history and are empty (treated as average)
when tracking data is missing. Whether they help is a question for the data, not for me:
`go run ./cmd/backtest -season 2025 -ablate` lists `chances` with a log-loss change. Keep it only if
removing it makes the model worse. The feature set changed, so run train and backtest again before predict.

## Line sanity in predict

* A line with no price on one side is skipped (the calibration would treat the missing side as a
  coin flip and invent an edge). The skipped count is printed.
* `-maxgap 1.0`: a line more than 100% of max(projection, 2) away from the projection is marked
  CHECK with a note, since it is more often a bad or alternate line than a real edge. `-maxgap 0` turns it off.

## Next-day review: what hit, what missed, and why

```powershell
go run ./cmd/review                      # yesterday (daily.ps1 does this right after settle)
go run ./cmd/review -date 2026-10-22     # a specific day
go run ./cmd/review -all                 # every logged day in one file
```

Writes `data\review_DATE.csv` (every tier: PICK, CHECK, PAPER, LEAN), one row per player, stat and side
(the same play logged twice counts once). Columns include the result, pick, line, actual, how far past the
line it landed, the model's projected stat and minutes, the minutes actually played, the game margin,
`reason_type`, a plain-English `reason`, and `why_picked` (the model's own notes at pick time, saved
from now on). The terminal summary shows W-L by tier, stat and side, why the losses lost, and whether the
minutes and stat projections ran high or low.

Reason types for a loss: `minutes` (blowout, left early, or more minutes than projected), `close_call`
(missed by a point or less), `projection_miss` (stat far from the projection with minutes on track: the
model misread the player) and `variance` (projection was close). The reason is a diagnosis from the box
score, not proof. One game cannot separate skill from luck, so read the counts across many days.

## One game: `-game`

```powershell
go run ./cmd/predict -game "ORL CHA"      # or ORL@CHA, or one abbreviation, or a game id
go run ./cmd/predict -game ORL -top 30
```

Prints the game's market spread and total (context only), its best lines in any tier (`LOOK` means
below the 55% bar), and then a **projection table with no restrictions**: every player and stat, at
the consensus line, starters first, with the projection, the calibrated lean and probability, the edge
against the price, and a plain-language read ("slight lean: inside the noise", "stat unproven in
backtest" and so on). Writes `data\game_<teams>_<date>.csv` and `..._projections.csv`. Nothing from this
view is logged. A projection above the line is not an edge: books use the same information.

## Position matchups: five groups

`pos5_allow` (points, rebounds and assists) measures how much the opponent has allowed to this
player's position group over its last 30 games, against the league average. The feed only has G, F, C
and hybrids, so the groups are guard (G), wing (G-F, F-G), forward (F), big forward (F-C, C-F) and
center (C). It replaced the older three-group `pos_allow`, so the feature set changed: run
`go run ./cmd/backtest -season 2025 -ablate` and `go run ./cmd/train` before predict. The ablation lists
`matchup` with a log-loss change; keep the group only if removing it makes the model worse.

## The website (`site/`)

React + Vite + SCSS, built to plain static files and published with GitHub Pages. It reads one file,
`site/public/data/picks.json`, which `go run ./cmd/export` writes from your pick logs. The code
repo can be private; the published site and that JSON file are public, so the file contains only what
a visitor should see: players, lines, picks, grades, tiers and results.

Pages: **Board** (today's plays by matchup with letter grades, filters, and a "why" on each play),
**Record** (every graded play, by group, by stat and side, a check that the percentages hold up, and
recent results) and **How it works**. Grades come from the calibrated probability, and only staked
plays can earn an A; everything else is capped at B+.

### Running it on GitHub (scheduled, hands-off)

`.github/workflows/daily.yml` runs the model on a schedule and publishes the site. Four runs a day, Eastern:
9:17 am (the daily pipeline: box scores, grade, retrain, lines, picks), then 3:17 pm, 6:17 pm and
8:17 pm (newest lines and injuries, picks; the 8 pm one is for the late games). GitHub schedules are
in UTC, so after the clocks go back on Nov 1 they run an hour earlier on the Eastern clock; edit the
hours in `daily.yml` if you want the Eastern times to stay put.

One-time setup
1. `cd site; npm install` (creates `package-lock.json`: commit it).
2. Make a **private** repo and push (commands below). `data/` is not pushed except four files:
   `picklog.jsonl` (every posted pick), `settled.jsonl` (graded results), `calibration.json`
   (from `go run ./cmd/backtest -season 2025 -ablate`) and `model.json` (from `go run ./cmd/train`),
   the last two made on your PC where the full history is. The scheduled runs do not retrain: they
   use the committed model. Retrain locally from time to time, then `git add -f data/model.json
   data/calibration.json`, commit and push.
3. Repo Settings > Secrets and variables > Actions > New repository secret:
   `BALLDONTLIE_API_KEY` and `SPORTSGAMEODDS_API_KEY` (the same values as your `.env`).
4. Settings > Pages > Source: **GitHub Actions**. Pages from a private repo needs a paid GitHub plan
   (Pro, Team or Enterprise); on the Free plan the repo must be public to use Pages.
5. Actions tab > "WraithPicksSZN daily" > Run workflow > mode **bootstrap**. It downloads the last two
   seasons of box scores (expect up to an hour or so). After that the two daily runs go on their own.

Run by hand any time with mode `daily`, `pretip`, or `site` (rebuild the site after a code change).
Each run saves the downloaded data to the Actions cache and commits the three state files. The CSVs
(`picks_*.csv`, `review_*.csv`) are attached to each run as a downloadable artifact for 14 days.
Local preview: `cd site; npm run dev`. The old `publish` scripts are no longer needed.

### Play names and icons on the site

The site shows four play names instead of the internal tiers: **Max play** (the only staked kind), **Great play** (58%+ but unstaked: unproven stat or early season), **Risky play** (needs a check: minutes, status or odd line) and **Lean** (smaller edge, not a recommendation). Small icons show weak or tough defense against the position, hot or cold form, minutes up or down, back-to-back, teammates out, status and odd lines. Green helps the pick, red works against it. The names describe the evidence, not a guarantee.

### Blowout-adjusted minutes

Starters sit early in lopsided games (final margin of 18 or more). Before the minutes averages are taken, a short night in a blowout is raised to the player's normal minutes (the average of his non-blowout games), so a 24-minute night in a 25-point game does not make the model expect fewer minutes next time. A night where he still played more than normal is left alone. A new feature, `m_blowcut`, tells the minutes model how much was added. Because the features changed, rerun `go run ./cmd/backtest -season 2025 -ablate` and then `go run ./cmd/train` before the next `predict`.

### Team names on the site

`go run ./cmd/export` now fills in "AWAY @ HOME" for plays that were logged with only a game number, using that day's snapshot and the stats history.

### Three-pointers made

The model now also predicts three-pointers made (threes), using the `threes` props from BALLDONTLIE and `threePointersMade` from SportsGameOdds. Its features are his three-point attempt volume and percentage trends over the last 10 games against his own norm, plus form, opponent, home/away and matchup. It is new and unvalidated: it is tiered PAPER (never staked) until a backtest on your own data shows it holds up. Run `go run ./cmd/backtest -season 2025 -ablate` and `go run ./cmd/train` before the next `predict`; the feature list changed, so the old calibration no longer applies.

### Minutes projection: what changed and how to check it

New minutes features: `m_med10` (median of his last 10 games, so one injury exit does not drag it), `m_low10` (share of the last 10 games well below his normal), and `star_spread` (the game's spread scaled by how big a part of the game he plays, since a lopsided spread costs starters minutes). Together with the blowout correction these are the only changes.

`go run ./cmd/backtest -season 2025 -ablate` now starts with a **MINUTES CHECK** table: out-of-sample minutes error and bias for the model against the plain recent average, split by projected minutes and for the first three weeks of a season. Bias is actual minus projected, so a positive number means the projection was too low (what the Oct 24 review showed). To measure what the blowout correction adds, run the backtest once with `-noblowout` and compare the two tables. If the new features or the correction do not lower the minutes error, remove them.

### One-sided boards (all unders) and threes

- `go run ./cmd/bias` projects every eligible player-game of the season with a model trained on earlier months only and compares with what happened, for minutes and every stat, for the first three weeks and after. Nothing is selected, so it shows whether the model is biased low (a positive bias, and 60%+ of games above projection). The next-day review cannot show this: it only sees the plays the model chose, which are the ones where it happened to project low.
- `predict` now warns when more than 70% of the day's scored lines lean the same way (`-skew 0.70`, 0 turns it off) and marks plays on that crowded side CHECK, not staked.
- Threes are on the board again (`-stats points,rebounds,assists,threes`) but are never staked until a backtest supports them. Check the threes row in the backtest's BY STAT table. `-game` always shows every stat.
- **Off-market lines.** When 4+ books list the same player and stat, a book whose line is 1.0 or more away from the median is flagged in the why text and with an icon on the site. If the off-market line favors the pick (a higher line for an under, a lower one for an over) it is a green "soft line" badge. This does not depend on the model: it is a stale or shaded number you can simply take.

### Line movement

`predict` adds a note to a play's why text when the market line has moved since the first snapshot of the day (for example "market line moved with the pick 24.5 to 25.5 since 9:17 am (6 of 8 books changed...)"), and the site shows an arrow icon: green if the market moved toward the pick, red if away. `go run ./cmd/moves` lists the day's biggest movers. It needs at least two snapshots in a day, so the four scheduled runs (or running `go run ./cmd/pull snapshot` again later) are what feed it. The feed has no bet counts, so it shows that a line moved, not why: several books moving together is usually news, one book alone is usually stale or shaded, and public money versus a sharp bettor cannot be told apart. `go run ./cmd/pull props -season 2026` would add true opening lines but is not wired in yet.

### Opponent absences

A missing rim protector or rebounder changes what the other team's players can do. For every player the model now looks at the opposing team's rotation players who are out: `oppabs_min` (their minutes), `oppabs_big` (minutes of bigs), `oppabs_blk` (blocks per game they are not on the floor to make) and `oppabs_reb` (rebounds per game). Historically "out" means he did not play in that game's box score; live it means ruled out on the injury report. They feed the points, rebounds, assists and threes models, not the minutes model. `predict` also shows "opponent out: ..." in the why text, and the site shows an icon, whether or not the model has the features on.

These features are in the same group of switches as teammates-out: **they are off unless you run with `-lineup`**, for the same reason. A backtest against opening lines knows who sat out from the box score, which the opening lines did not, so it flatters these features and its verdict on them cannot be trusted. To use them live:

    go run ./cmd/backtest -season 2025 -ablate -lineup
    go run ./cmd/train -lineup

and commit the new `data/model.json` and `data/calibration.json` for the scheduled runs. Judge them by the live record and closing line value (`go run ./cmd/settle`), not by that backtest. Do not run the model both ways and mix the files: `predict` refuses a calibration that does not match the model's feature set.

## Any missing starter, and early-season game lines

Opponent-out features now count every missing opposing rotation player, not just bigs: how many starters (28+ min) are out, plus the points and assists that are not on the floor. The "opponent out" note marks starters. Like the teammates-out group, it is off unless you pass `-lineup` to backtest/train/predict, and it is unvalidated.

Game lines needed both teams at 8 games of history, so they were skipped early in the season. `go run ./cmd/games` now defaults to `-mingames 6` (an offseason counts as 4 games, so two games in). Early ratings lean on last year's teams, and the game-line backtest has never beaten the book, so everything stays PAPER. Skipped games now name the teams and say how many games each has.

## Bias correction, and what the live record can tell you

**Bias correction.** `go run ./cmd/bias -write -lineup` projects every eligible player-game of the current season with a model trained on earlier months only, compares with what happened, and writes `data/biasfix.json`: one multiplier per stat on the projected mean. It applies a stat's multiplier only when the bias is both clear (z of 2 or more, at least 300 player-games) and is shrunk toward 1 on small samples and clamped to 0.85-1.10, so a few noisy games cannot move projections. **It is off by default (2026-10-26):** `predict -bias` applies it. On the live board it pushed projections to 14% under the lines (points x0.915 on top of a model already ~5% low against lines that `cmd/linecheck` showed were accurate), which is what made the board all Unders. The file was fitted on the backtest path, not the live path, and on only 655 games. The calibration was fitted without this correction, so treat it as a stopgap for a drifting season, not a validated improvement.

**Situations report.** `go run ./cmd/settle` now ends its record with a SITUATIONS table across every tier: plays with teammates out, opponent out, an opponent starter out, back-to-back, and OVER vs UNDER, each next to its "no ..." counterpart, with hit rate and closing line value in stat points and as a share of the line (comparable across stats). It reads the reasoning note already stored with every logged pick, so it works on past picks too. A situation only matters if it beats its "no ..." row across a few hundred plays.

## Run-to-run changes

Every `predict` run (9 AM, 3 PM, 6 PM, 8 PM) saves its board to `data/runs.jsonl` and compares it with the run before it that day. The terminal prints CHANGES SINCE THE 9 AM RUN: each dropped play with the reason (he is ruled out, the line moved, the model now leans the other way, probability or edge fell below the bar, minutes or projection changed, or it lost its place to stronger plays), the new plays, and tier changes. `-game` and `-nolog` runs are not recorded. The site shows the latest run's board, a "What changed between runs today" panel with the same reasons, and an amber "Back again" mark (plus a filter) on plays that were on the board in more than one run. If the model keeps choosing a play as lines and lineups change, that is worth more than a play that appeared once.

## Minutes: cut-short nights

A night cut short by an injury exit or foul trouble no longer drags a player's minutes down. Before the minutes averages are taken, an isolated night under 55% of his normal (the median of his last 30 games, rotation players only) is raised to normal. Two or more short nights in his last five are left alone, since that is a real change (a benching, a minutes limit, a new role). Blowouts are handled separately, as before. `-noexitfix` on `backtest` and `bias` turns it off so you can measure what it does; compare the MINUTES CHECK table with and without it before trusting it. The posting guard and the "minutes vs his normal" note now compare the projection with his typical minutes (median of his last ten after both corrections), not a raw 15-game average. On the site, "normal minutes" is the median of his last five with a cut-short night left out, and that night is faded in the minutes chart.

## Site look

The board is flat and quiet: one purple accent, a cropped banner, a one-line strip on each play (the line, his last five games as dots, the projection as a triangle), a bar for confidence, and filters that show one choice at a time. Games are listed by tip-off time.

## Are the books high too? (linecheck)

`go run ./cmd/linecheck` takes every saved prop line (the median across books) for games that have finished, and compares it with what the player did: average line, average actual, and the share that went over and under, by stat and by day. Nothing is selected, so it shows whether the market itself is running high early in the season. Read it next to `go run ./cmd/bias`, which measures the model against results.

## Tip-off times and defense by position

Each game on the board shows its tip-off time (Eastern), and games are listed in tip-off order. The time comes from the game snapshot (`datetime`, or `status` for games not yet played) and is saved with each logged pick; older picks are filled in from the day's snapshot when the site data is exported.

The old "soft defense" / "tough defense" note is replaced by a ranking against the player's position group (guard, wing, forward, big forward, center) in the stat being picked, for example "Allows the 3rd-most rebounds to centers (+9%)". Teams are ranked on what players at that position produced against them, using this season's games once every team has played three, and last season's games too before that (the line then says "early"). Each team's number is blended with the league average as if it had six extra average games, so a few games cannot put a team first or last. Red or green shows whether the ranking helps or hurts the pick. The badge icon appears only for the top eight at either end.

## Why is the board all Unders? (`cmd/audit`)

Run `go run ./cmd/audit` after a pull. It checks the line data before blaming the model:
alternate lines per book (the loader now keeps the main line, the one with the most even over/under prices),
each book's offset from the others, which side the books price as more likely, and how the lines sit against each
player's last-10 and season averages. If most lines sit well above recent averages, the books are running high
early in the season and an all-Unders board is the market's doing. Pair it with `go run ./cmd/linecheck`.

## Play reasoning

Expanding a play shows the arithmetic (minutes x per-minute rate = projection vs line), three probabilities
(model alone, what the odds imply, the blended percentage shown), evidence for and against, and the live record on
that stat and side with a plain note on how much a record of that size can be trusted. Built in `cmd/export/casefile.go`.

## Level check: `predict -anchor`

Books' lines are accurate on average (`cmd/linecheck`), so if the model's average projection sits well off the average line across the whole slate, the level is off. `-anchor` rescales each stat so the slate's average projection equals its average line (only the level moves; which players the model likes more or less than the book still picks the plays). It prints the scale it used (on 2026-10-26: points x1.05, rebounds x1.04, assists x1.06, threes x1.01, so the raw model is about 5% low). Not backtested. The centre check at the end of `predict` shows why Overs are rarer than Unders even with a balanced model: counting stats are right-skewed, so a line set at the mean goes under more often (54% of lines this season), and an Over needs a bigger move to reach the posting bar.

## More plays and earlier picks

- **More plays:** each game has a "More plays" button showing up to 5 more lines (at most 2 per player, positive edge, best price per line) that did not make the board. `predict` writes them to `data/more.json` on every real run (not with `-nolog` or `-game`), and `export` attaches them to the game. They are leans: they did not clear the posting bar and are not part of the pick log or the record.
- **Earlier picks still at 58% or better:** a run that drops a play now also records where that player and stat stand (`now` in `data/runs.jsonl`). The site lists dropped plays whose same side is still rated 58% or more, with the reason they left the board. Plays for players who are now ruled out are left out.
- `data/more.json` is committed like the other state files and is rewritten each run, so on a git conflict take your version.

## Results on the site

`export` now includes up to 600 graded plays (`-recent`). The Record page lists them by day with each day's W-L, what he finished with and what the model projected. The Board page has a "Results from <last graded day>" card listing every play graded that day: not only the final board, but plays earlier runs posted and later dropped, and the lean fill-ins. That is why a day can show 127 graded plays against 44 on the board.

## Live check on every player (`cmd/livecheck`)

The posted picks are the plays where the model disagrees with the book, so grading only them hides the model's real bias (a 2026-10-26 review showed minutes 3 above projection, but only for the plays the model chose). `predict` now saves what it projected for every player it scored to `data/projlog.jsonl` (minutes, each stat's raw and used projection, the books' median line, back-to-back, teammates and opponents out, his own status). After the games are in your box scores, `go run ./cmd/livecheck` compares those with what happened for everyone, split by back-to-back, teammates out, projected minutes and how far the model cut his minutes below normal. Run `predict` without `-nolog` before the games so the day is logged. The file keeps one row per player per day (the latest run).

## Game context on the board

Each game on the board now shows its spread, total and the score those imply (for example LAL -2.5, total 234.5, POR 116 to LAL 118.5), with how far the total sits above or below the day's average, and two plain reads: how close the game should be (tight, competitive, lopsided, blowout risk) and how fast (more or fewer possessions). A tight game means starters keep their minutes; a big spread means they may sit the fourth quarter. The same market feeds each play's "What supports it" and "What could go wrong". `predict` writes the consensus spread and total to `data/more.json`, and also logs them in `data/projlog.jsonl`, so `go run ./cmd/livecheck` can show whether minutes really run long in close games and short in lopsided ones.

## Keeping the data files small

`pull advanced -new` re-downloads the last 7 days on every run and appended them, so `data/advanced_2026.jsonl` collected duplicates until it passed GitHub's 100 MB limit and pushes were rejected. Pulls now compact their files afterwards: one whole-game row per player and game, only the columns the model reads, and `stats_*.jsonl` loses repeated rows. `go run ./cmd/pull compact` does it for every stats and advanced file on demand. Nothing the loaders read changes.

## Site extras

- **Top plays strip**: the five strongest plays on the board; tap one to jump to it.
- **Outcome curve**: expanded plays show the model's full distribution with the line marked.
- **Line movement**: a small sparkline of the line since open, coloured by whether it moved with or against the pick.
- **Lineup news**: per game, who is out or uncertain, taken from the injury feed.
- **Countdowns**: each game shows time to tip; started games dim and stop taking new reads.
- **Record charts**: hit rate against what the model said, and a day-by-day grid.
- **Add to home screen**: the site ships a manifest and a small service worker (network first for `data/picks.json`). On iPhone use Share, Add to Home Screen.
- **New picks banner**: an open tab checks for a newer `picks.json` every 5 minutes and on return; tap Reload.

## Choosing the cutoff

`go run ./cmd/sweep` (optionally `-stat rebounds`) reads `data/settled.jsonl` and, per side, shows plays per day, hit rate, break-even and flat units at each edge cutoff. Pick the lowest cutoff where the hit rate clears break-even with a real sample. A side with no graded plays cannot be tuned yet.

## Who is out, not just how many (2026-10-27)

The model already used the total minutes and starters missing from a team. It now also knows which teammates are out:

- `miss_pos`: absent minutes at his own position group (the next group over counts half), so a guard out helps the guards.
- `wo_min`: from his own box scores, how his minutes changed in past games when each absent teammate was out, against games when that teammate played. Only games after the teammate joined the team, only games before the one being projected, and each difference is shrunk toward zero (one game without him says little).
- `wo_rate` (per stat): the same comparison for his per-minute output, capped at +/-0.3 in log terms per teammate.

These are new inputs, so the existing `model.json` does not use them. Retrain (`go run ./cmd/train`), then compare `go run ./cmd/backtest` before and after, and watch the "teammates out" rows in `go run ./cmd/livecheck`. External sites such as StatMuse have no API and are not needed: the with/without splits come from the box scores already in `data/`.

## Minutes blend (2026-10-27)

`go run ./cmd/bias` now also prints how the model's minutes cuts below a player's normal fared on every game this season (nothing selected), and a table that moves the projection 0, 25, 50, 75 or 100% of the way to his normal minutes and scores each. The share with the lowest minutes error is the recommendation; apply it with `go run ./cmd/predict -minblend <share>`. Default 0 (off). It changes only minutes; per-minute rates are untouched. It is fitted on this season's games so far, so re-run `bias` as games accumulate.

## What three days of graded plays showed (2026-10-27)

218 graded plays from 10-24 to 10-26 (`review_*.csv`):

- **Overs were being posted and doing fine** (15 graded, 10-5, +3.0u) at the 55% bar. Raising the bar to 60% on 10-27 removed nearly all of them, so it was put back: **posting is 55%/45% again; staking needs 60%/40%** (`predict -stakebar`, default 0.60). Plays between the two bars are tiered PAPER with 0 units and a note, and are graded like everything else.
- **Game closeness drives minutes.** Plays in games that finished within 5 points ran +5.5 minutes over projection (53% had a +5 surge); blowouts ran -1.7. Unders hit 51% in close games and 64% in blowouts. The model has the spread as an input but under-reacts, so `go run ./cmd/bias` now prints the minutes miss by pre-game spread and fits a correction.
- **Teammates out was a good sign for the Unders** (65% on 66 plays; starters 74%), because books already raise the lines. The misses were stars absorbing a missing co-star (Wagner with Banchero out, Murray with three out).

`go run ./cmd/bias -write` saves `data/minfix.json` (minutes per point of spread, centred so average minutes do not move). `predict` applies it only when the slope is clear (z of 2.5+, the right sign) and prints a line saying so; `-nominfix` turns it off.

**Correction to the spread note above (2026-10-27, later).** On all 775 eligible player-games the pre-game spread did not predict the minutes miss clearly (slope -0.09 per point, z -1.7), so `minfix.json` stays inactive and `predict` ignores it. The "close games ran over" result came from the final margin of the plays that were picked, which is only known after the game and is partly selection. The same unselected table showed the model's biggest minutes cuts (6+ below normal) recover about a quarter of the way (+2.2 on 97 games), so `predict -bigcut` (default 0.25) moves only those back toward normal. `predict` also prints an `OVERS:` table of the day's strongest Overs with the reason each is not on the board (probability, price, or a cap).

## Who gains when a teammate is out (`cmd/onoff`)

`go run ./cmd/onoff` reads today's injuries snapshot and, for each rotation player ruled out (and each questionable one, labelled "if he sits"), lists the teammates whose minutes, points, rebounds, assists and threes were higher in games without him, with the number of games behind each figure. `-player "Duren"` asks about any player. It is built from the box scores only, over the last two seasons and since the player joined his team; samples are small, so read the game counts (`thin` means under 3 games without him). It informs the model's `wo_min`/`wo_rate` features but does not change any projection by itself.

`go run ./cmd/bias` also prints a minutes range check: how often actual minutes land inside the model's 10th-90th percentile band, for bench, rotation and starters.

## With-and-without correction (tested before it is used)

`feat` computes, for each player, how his minutes and per-minute output changed in past games when today's absent teammates were out (`wo_min`, `wo_rate`; shrunk, capped). These were only reachable through `train -lineup`, which made unselected minutes worse. Instead of retraining, `go run ./cmd/bias` now tests them directly: every 2026 game is projected without the history, then grouped by what the history says, and the miss is regressed on it. The table prints the miss by group and a slope (1 would mean the history is right as it stands, 0 noise). `bias -write` saves `data/wofix.json`; `predict` applies the minutes shift and per-stat factor only for slopes clearly above zero (z of 2.5+), shrunk by sample size, and prints a line when it does. `-nowofix` turns it off. Backtest absences come from box scores (which include rest days), live ones from the injury report, so expect the live effect to be a little smaller.

`go run ./cmd/onoff` now lists only teams playing that date (`-all` for every team) and treats a player as a rotation player from his last ten games, however long ago, so long-term injured starters (Duren, Sharpe) appear.

## Off-market lines (2026-10-27)

On 10-26 the 35 graded plays priced at a book whose line sat a point or more from the 4+-book median went 26-9 (74%, +10.8u); the other 89 plays went 55% (+0.2u). One day, so a lead, not a result. The same gap works in both directions: a high line makes the Under cheap and a low line makes the Over cheap. `predict` now prints an `OFF-MARKET LINES` table (Overs and Unders together, biggest gap first, best price per player/stat/side) with the model's probability and whether it agrees. Overs on this table are the mirror of the Under edge, so they do not depend on the model's Over probability reaching 55%.

The with-and-without fit on 775 games (`bias`) found the minutes history is real (slope +0.43, z +8.6), so `wofix.json` now applies a minutes shift of 0.31 times the history. The per-stat factors were not clear enough (points z +2.5, just under the cutoff) and are not applied.

## Overs watch on the site

`predict` (without `-nolog`) saves the day's Overs closest to qualifying, plus Overs at books priced a point or more below the market median, into `data/more.json`. `export` puts them in `picks.json` and the Board page shows them as "Overs watch" above the results: player, the bet, price and book, the model's probability, edge, projection, time to tip, and a plain reason it is not a pick (probability under the bar, price leaves no edge, or the low-line note). Off-market Overs come first. They are not picks, are not logged to the pick record, and are hidden once the game starts.

## Combo lines: PRA, PR, PA, RA (2026-10-27)

Books list points + rebounds + assists (PRA), points + rebounds (PR), points + assists (PA) and rebounds + assists (RA) as their own markets. Both feeds carry them (BALLDONTLIE as `points_rebounds_assists` and so on, SportsGameOdds as `points+rebounds+assists-…`), and `go run ./cmd/sgo snapshot` now asks for them. The parsers keep them apart from the single stats on purpose (`ParseResponse`/`ParseEvent` return singles only; `ParseCombos`/`LoadSnapshotCombos`/`ParseEventAll` return the combos), so nothing that indexes by single stat can see one.

**How they are priced.** You cannot add three probabilities. `internal/combo` simulates the sum: each of the 15 minutes nodes is shared by all three stats (a blowout cuts points, rebounds and assists together), each stat is drawn from its negative binomial at that node, and the leftover link (a hot night brings assists too) is a small Gaussian-copula correlation. Defaults are +0.08 points-rebounds, +0.12 points-assists, +0.05 rebounds-assists; `go run ./cmd/combos fit` measures them from the season's games and saves `data/combocorr.json`. The single-stat corrections (bias file, with-and-without, anchor) are applied to each part first. The simulation is fixed-seed, 30,000 draws per player, so the same inputs always give the same answer.

**What they are not.** There is no graded history on combos, so they have no calibration of their own. Instead each combo borrows the single-stat calibration its parts earned (`calibrate` in `cmd/predict/combos.go`): the intercept, model weight and book weight of points, rebounds and assists, weighted by each part's share of the projected total. A PRA is mostly points, and the points calibration trusts the model very little (model weight 0.03, book 0.85 on the 10-27 run), so a PRA stays close to the book unless a line is genuinely off. `-comboweight 0.25` (any 0 to 1) replaces this with a plain model/book blend. The tier is its own `COMBO` (never PICK, PAPER, CHECK or LEAN), nothing is staked, and plays are logged to `data/combolog.jsonl`, apart from `picklog.jsonl`. The pick record, the sweep and the review do not see them. They are tracked at 53%/47% (`-combobar`), looser than the 55% posting bar, because they are never staked and need a graded sample; the printout says how many reach 55%. One play per player (`-comboperplayer`): PRA and PR on the same player are one idea. The site shows them in a separate "Combo lines" section with a stat filter and an Over/Under filter.

**Commands.**

- `go run ./cmd/combos count` shows, for the last three snapshot days, which prop types the raw feed sent and how many combo lines, players and books each source has. Run this first: it settles whether the feeds really deliver the markets and how many books quote them (the off-market check needs 4+).
- `go run ./cmd/combos fit` and `go run ./cmd/combos check` fit the correlations and test the combo distributions against the season's games (no lines needed): bias, spread ratio (above 1 means too sure), probability-integral-transform tenths, and the 10-90% band coverage.
- `go run ./cmd/combos settle` grades `combolog.jsonl` into `combosettled.jsonl` and prints the record by combo, side and probability band. The daily script runs it after `settle`.

`predict -combos=false` turns the section off; `-combomax` caps how many are tracked per run (30); at most two per player.

**Reading the results.** Wait for about 100 graded combo plays. Until then, compare the hit rate with the predicted column. If combos hit more often than predicted, the model has earned more trust on them than the singles got; only then consider a higher `-comboweight` or staking. The combos inherit the single stats' biases (points, assists and threes are over-projected by 6 to 7% this season), so a combo Under can look better than it is until those are settled.

## Grading every line, not just the picks (2026-10-28)

The pick log grades about 50 plays a day, so a two or three point edge takes weeks to show. The model scores about 1,900 lines a day. `predict` (without `-nolog`) now logs one row per player and stat at the **median line across books** to `data/lines/market_DATE.jsonl`: the model's calibrated P(over) at that line, projection, projected minutes, minutes shift, teammates and opponents out, back-to-back, how many books, the lowest and highest line, and the day's opening median (from the snapshot history). A later run replaces the same player and stat and keeps rows from earlier runs it no longer lists. Games that have already tipped are left out, because the feed can carry live lines. Off-market lines (a book a point or more from the median of 4+ books) go to `data/lines/offmarket.jsonl`, append-only and deduplicated, at the price and side they were offered. Nothing here changes a pick.

`go run ./cmd/lines` grades both files against final box scores and prints:

- calibration at the median line by probability band, and the Brier score against always guessing the day's base rate (does the model carry any information at all?);
- slices (a teammate out, back-to-back, minutes cut or raised 4+, the median line moved 0.5+ in either direction, books disagreeing, 12+ books, each stat) with the Over rate against what the model expected. A gap of 5 points on a few hundred lines is a real miss; one day is noise;
- the off-market record at each book's own price: Under at a high line, Over at a low one, by gap size, number of books, whether the model agreed, and by stat, in flat units and ROI.

The same tables are on the site's Record page ("Every line, not just the picks" and "Lines priced off the market"). With `-since 2026-10-27` it only counts days from that date. Expect the first useful read after about a week (2,500+ graded lines); the slices need more.

## Expected missed shots (opt-in, 2026-10-28)

Rebounds come from missed shots. A new rebound feature, `exp_miss`, is the log ratio of the expected misses in the game (each team's recent misses blended with what the other side forces, both teams summed) to a typical game (94). It is a feature group of its own (`misses`) and **off by default**: the live model, `calibration.json` and its fingerprint are unchanged, and an old `model.json` loads exactly as it was fitted. To test it:

1. `go run ./cmd/backtest` (normal) and note the rebounds log loss, then `go run ./cmd/backtest -missed`. The second run saves `data/calibration_misses.json` and leaves the live calibration alone.
2. If rebounds improve by a clear margin (a few thousandths of log loss on thousands of lines), `go run ./cmd/train -missed`, copy `calibration_misses.json` over `calibration.json`, and run `go run ./cmd/bias -missed -write`. The feature set name changes (`...-misses`), so predict refuses a calibration from the other set.
3. If it does not help, leave it off. Nothing needs undoing.

## ESPN injury cross-check (2026-10-28)

`go run ./cmd/pull snapshot` also saves ESPN's injury list to `data/snapshots/espn_injuries_DATE.json`, and `predict` folds it into the BALLDONTLIE report: a player ESPN lists as Out whom BALLDONTLIE does not is treated as out (no pick on him, and his teammates' lineup features know), and a player ESPN lists as day-to-day with no status of his own becomes uncertain, so his plays are CHECK and not staked. The printout lists who changed (`ESPN INJURY CHECK`). The endpoint was checked against a live response, but not from the machine that runs the workflow: if it is blocked there the fetch fails quietly and predict uses BALLDONTLIE alone. Look for `espn injuries saved` in the snapshot step of a workflow run.

## Not built

Referee tendencies, travel and altitude, and the opponent's three-point attempts allowed are not in the model. None of them could be checked against real data from here, and the model's rebound, assist and points biases are still larger than what those usually move. The graded line log above is what will say where the model is off first. More history is a data pull, not code: `go run ./cmd/pull stats -seasons 2022,2023,2024,2025` and `go run ./cmd/pull advanced -seasons 2022,2023,2024,2025`, then retrain and rerun the backtest.

## push.ps1 settles data conflicts

When the scheduled runs and your machine both changed a data file, `scripts/push.ps1` now resolves it: append-only logs (`picklog`, `settled`, `runs`, `combolog`, `combosettled`, `data/lines/*.jsonl`) keep both sides, everything else under `data/` takes your copy, and a conflict in a code file stops with a message. Duplicate market rows from a merge are collapsed on read.
