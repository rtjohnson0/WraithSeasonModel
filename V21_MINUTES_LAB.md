# V2.1 Minutes Lab — public aggregate dashboard tab

## The UI

After merging and a successful GitHub Pages deployment, visit:

`https://rtjohnson0.github.io/WraithSeasonModel/#/lab`

The header has a **V2.1 Lab** tab, and the current V2 page links to it.
The tab displays **four aggregates from exactly the same independently
verified rotation-v4 questions**:

1. Baseline minutes (V4 production rates held constant).
2. Current V2 rotation-v4.
3. Supported-only absences: use V4's minutes when a relevant player is
   confirmed out, matching prior rotation games >=3, allocation applied and
   supported; otherwise revert to baseline minutes. Uncertain own status
   also reverts.
4. Supported-only with an additional ±2.5-minute cap on adjustment size.

The Go rotation auditor replays frozen pregame checkpoints, verifies the
underlying odds, injury context, model timestamps, prediction recomputation
and final player/team identity before including a question. The experimental
minute shifts alter neither the existing pregame usage-rate forecast nor the
recorded book prices.

**This is not a second feed of today's picks.** It is a shadow research
scoreboard of settled, pregame-recorded forecasts. It has no bets, stake
units or per-player recommendations, and does not change production V2.
Performance differences are observational and correlated across questions;
the tab does not declare a winner until an independent prospective trial.

## Evaluation metrics

- Minutes MAE: average absolute minutes prediction error, one first
  verified forecast per player/game, no repeated credit for multiple props.
- Minutes bias: actual minus forecast minutes; positive = too low.
- Brier score and log loss: estimated P(Over) vs actual at the *same*
  original full-game line and the identical saved market blend.
- Stat MAE: mean absolute error for the same recorded stat questions.

Values with fewer than 30 player-games or 30 binary stat questions are hidden.
The tab warns about fewer than 20 independent evaluation dates or fewer
than 200 player-games. This cutoff is a minimum evidence indicator, **not**
a statistical proof of superior predictions or positive ROI.

## Publishing / privacy

The Go auditor produces `data/v21_lab.json`, a safe aggregate-only record.
`scripts/publish_v21_lab.py` explicitly allowlists the four candidate IDs
and numeric aggregate metrics into `site/public/data/v21-lab.json`;
unknown fields, private picks, game/player identities, lines and prices are
not copied. The private/Whop build deliberately excludes static data
altogether, preserving its secure-delivery boundary.

The Pages tab refreshes the aggregate scorecard every minute and explains
when no verified report exists yet. To populate the initial report on a
source-only deployment, the existing job restores immutable replay evidence
and independently regenerates the aggregate, then carries it through the
normal state synchronization. The morning/pre-tip jobs also produce it.

## Tests

```sh
go test ./cmd/rotationaudit
python3 -m unittest discover -s scripts -p test_publish_v21_lab.py -v
cd site
npm ci
npm test
npm run build
npm run build:private
```

Do not merge until the audit-integrity PR workflow passes and the public
scorecard contains only aggregates. If a future change introduces live
experimental per-player picks, they must remain behind authorized
membership-based delivery, not new static Pages files.
