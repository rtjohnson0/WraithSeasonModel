# NBA player-prop model (Go)

Predicts the probability that a player goes OVER or UNDER a sportsbook line for **points, rebounds
or assists**, for players averaging 20+ minutes (or projected for 18+). It posts a pick only when
the calibrated probability is **>= 55% (OVER) or <= 45% (UNDER)**, sizes a small stake, logs
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
5. **Posting and staking.** 55% / 45% rule, best price across BALLDONTLIE and SportsGameOdds
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

- PICK: cleared 55%/45% and the edge rule. Staked (capped at 0.5u).
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

Books' lines are accurate on average (`cmd/linecheck`), so if the model's average projection sits well off the average line across the whole slate, the level is off. `-anchor` rescales each stat so the slate's average projection equals its average line (only the level moves; which players the model likes more or less than the book still picks the plays). It prints the scale it used (on 2026-10-26: points x1.05, rebounds x1.04, assists x1.06, threes x1.01, so the raw model is about 5% low). Not backtested. The centre check at the end of `predict` shows why Overs are rarer than Unders even with a balanced model: counting stats are right-skewed, so a line set at the mean goes under more often (54% of lines this season), and an Over needs a bigger move to reach the 55% bar.

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
