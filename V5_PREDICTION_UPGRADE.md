# V2 rotation-v5: actual prediction changes, versioned and reversible

This is **prediction code**, not another reporting tab. On merge, the scheduled
daily and pre-tip runs default to `rotation-v5` for V2-only predictions and
combos; the primary Board's independently trained model is unchanged.

## Updated V2 predictions

### Minutes: supported only, capped

The current V2 rotation-v4 sometimes worsened minutes MAE on the initial
three-day study. V5 alters the minutes calculation itself:

1. A confirmed teammate must be marked Out by a fresh pre-tip report.
2. The team allocation must be valid and supported, with >=3 complete
   earlier games matching the absent rotation. Players named Out are
   never projected to participate.
3. Unsupported or poorly evidenced allocations retain the primary-model
   pregame minutes forecast.
4. Even a supported rotation may change the primary-model minutes
   estimate by no more than **±2.5 minutes**.
5. The standalone, unverified game-spread minutes adjustment is not added
   to V5 (the trained primary minutes model already incorporates that
   context when available).

All of these conditions are evaluated **before tip**. A final box score
cannot alter the forecast. The same `predictor.Adjustments` is saved in each
replay row and independently recomputed by the Go auditor.

### Three-pointers: attempts × conversion, not just made-threes history

V4 already collects earlier `fg3a` and `fg3m` but the experimental
opportunity correction can be noisy. V5 explicitly uses

```
recent_attempts_per_minute × earlier_long_run_3PM_per_attempt
```

from strictly earlier, shooting-complete games. It requires >=5 recent
games, >=15 observed attempts, >=60 observed minutes and a valid
conversion rate. The opportunity/made-threes rate disagreement is
strongly shrunk based on attempt count, and its incremental correction
is capped to ±10%; combined V5 3PM rate scaling is capped to ±12%.

The V4 opportunity-production 3PM factor is disabled **within V5 only**
to avoid stacking the same 3PA information twice. Other production-rate
effects remain from the existing rotation context.

This is a conservative, **testable hypothesis**, not proof that we can
beat 3PM lines immediately.

## Independent results and rollback

V5 has its own frozen decision path `data/v2_rotation/guarded/` and
`data/v2_guardedlog.jsonl`, `data/v2_guardedsettled.jsonl`.
Older V4 checkpoints remain untouched.

The existing V2.1 Lab now includes an independent `Guarded V2 predictions`
panel sourced from `run_evidence/guarded-v5/report.json`. The public
`scripts/publish_v5_lab.py` exports only aggregate counts and >=5-date
error scores to `site/public/data/v5-lab.json`. It never copies specific
player/game lines, bookmaker odds, bets or rotation records. The historical
four-variant V4 cards are explicitly relabeled as a **prior V4 sample**,
not the new live V5 sample. These cannot be compared as if their evaluation
dates were matched.

After merge, current V2 website picks will use the V5 projections. This
is a deliberate experimental live V2 rollout, **not** a claim of proven
profitable selections; no primary-model selection, Whop data delivery,
or game-market picks are changed.

Rollback any time without reverting source code:

```
Repository Settings → Secrets and variables → Actions → Variables
WRAITH_V2_ROTATION_POLICY = rotation-v4
```

To return to V5, set the variable to `rotation-v5`. The workflow
uses V5 as the default if the variable isn't set. GitHub Actions must
complete the next daily/pre-tip run for the site to reflect the switch.

Historical V5 and V4 selections must not be pooled. Compare their
verified same-question Brier/log loss, minutes MAE, 3PM attempts
diagnostics, hit rate and recorded-odds ROI across *future* games.
Strong positive evidence is needed before treating this as an
accuracy improvement; if V5 underperforms, roll back.

## ML, spreads and totals

Those markets already have their own independently tracked game-pace
challenger and a separate V2.1 Lab scoreboard. **This PR does not adjust
those game predictions**. Next, upgrade the actual game-market prediction
components (lineup-aware win probability, margin dispersion and
possession/shot-mix totals) with the same frozen-pre-tip audit discipline,
rather than mixing their records with player props.

## Tests

```bash
go test ./...
python3 -m unittest discover -s scripts -p 'test_*.py'
cd site && npm ci && npm test && npm run build && npm run build:private
```

No improvement should be advertised until independent forward results
demonstrate it. Existing Pages data privacy limits still apply.
