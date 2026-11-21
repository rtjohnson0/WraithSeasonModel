# V2.1 Game Markets: equal research for moneylines, spreads and O/U

The public V2.1 Lab now has two research panels:
**Player props & minutes**, and **Moneylines · Spreads · O/U**.

The game-market panel consumes only the independent `game-pace-v1`
per-market audit already produced by the daily model workflow. Each
moneyline, spread and total is scored against its **same-book, pregame
sportsbook probability** using proper log loss. Their calibration
cohorts remain separate, and calibration is not displayed as
validated until there are matched older-fitted forecasts to assess.

### What does and does not change

- This adds visibility and audit guardrails, **not** a winning-pick claim
  or a production probability update.
- The existing game-market picks remain **PAPER at 0 units** until a
  predeclared later-date market edge is demonstrated.
- No additional game/player-level predictions, prices, betting lines,
  private outcomes or experiment picks are copied to GitHub Pages.
- The script `scripts/publish_v21_games.py` *only allowlists*
  `evaluation.moneyline`, `evaluation.spread` and `evaluation.total`
  numeric aggregates into `site/public/data/v21-games.json`.
- Default UI suppresses scores until 30 outcomes across five separate
  dates. It does not certify a positive edge merely when the thresholds
  are met; 200 outcomes and 20 dates remain only basic diagnostic
  requirements, not proof of profitability.

### Next research steps (separately evaluated, never auto-promoted)

1. **Moneyline:** test lineup-adjusted team strength, starting rotation
   changes and home/away/rest effects; score same-game log loss and
   calibration versus book implied win probabilities.
2. **Spread:** test opponent-adjusted scoring-margin distribution,
   lineup scenarios and late-game variance instead of just projecting
   the winning team; measure cover calibration and line-error.
3. **Totals:** test possession count, shot mix, offensive efficiency,
   defensive context, injury effects and end-game foul/pace variance;
   grade O/U separately by price and available line.
4. **Player threes:** decompose minutes × three-point attempts/minute ×
   conversion rate. Diagnose opportunity errors versus makes variance
   before attempting to increase/force Over bets.

A challenger is promoted only after its *predeclared* model and training
cutoff beat the original model on later dates and pass market-baseline
checks. Raising the hit rate by playing very expensive favorites is not
equivalent to a profitable model.

### Local verification

```sh
python3 -m unittest discover -s scripts -p test_publish_v21_games.py -v
cd site
npm ci
npm test
npm run build
npm run build:private
```

The GitHub Audit Integrity PR workflow runs these checks along with the
existing model safety tests. No change to live selection, calibration,
publishing `picks.json`, or Whop authentication is authorized.
