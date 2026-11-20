# V2 workload and opportunity review

Reviewed November 19, 2026: 43 verified prop questions across 26 player-games, 23 wins and 20 losses. These are correlated questions, not 43 independent games. The JSON alongside this document preserves the arithmetic and coverage for every reviewed win and loss.

## What the evidence shows

- Four players (Strus, Kuminga, Vassell, Hachimura) account for seven losses where projected workload was materially below actual minutes. Using the earliest retained decision per player-game (lexical stat tie break), across the 26 unique player-games, the primary minutes forecast had 3.86 minutes MAE versus 4.42 for V2. Among those four allocation cuts, primary MAE was 5.14 versus 10.00. These are descriptive comparisons on one date, not sufficient to fit a replacement minutes model.
- Fox: 31.6 minutes projected, 32.1 actual; shooting opportunities (FGA + 0.44 FTA) were 11.8 expected versus 7.64 actual. Production was limited by opportunity volume, despite better-than-reference points per opportunity. The final projection also exceeded the frozen opportunity-only reference; that residual includes the primary rate model and contextual adjustments, not solely the absence adjustment.
- Jaquez: 31.6 minutes projected versus 22.1 actual. Shooting opportunities per actual minute were above the frozen reference, but points per opportunity were much lower. Both minutes and conversion contributed to the miss.
- McDaniels: fewer shooting opportunities than the reference, but substantially better conversion helped beat the Under. Reducing a usage forecast would not have prevented that miss reliably.
- Brunson: five 3PA versus 8.4 projected from frozen earlier attempt rate, but four makes. That Under miss came with high conversion, not extra three-point volume.
- Wembanyama: 27.76 shooting opportunities versus 24.9 reference, with slightly lower conversion than historical. The full projection (24.9 points) was also below the frozen opportunity-only reference (30.7 points). That difference needs to remain visible instead of labeling every miss a minutes or usage problem.
- Pregame spread >=10: 9–11 across 20 questions / 12 player-games. Actual final margin >=15: 6–3 across nine questions / six player-games. A large spread is not proof that a player sat early. These overlapping samples are too small to fit new blowout penalties.

## Changes

1. Add a primary-minutes sensitivity scenario to contextual V2. It retains V2 per-minute rates, the exact line and the market blend, changing only workload. For singles, if this makes the posted side less likely than 50%, retain an otherwise eligible play as a LEAN rather than CHECK or STRONG. Both estimates and a card-level note are visible. This is selection revision `minutes-sensitivity-v1`; central projections/probabilities and earlier records are unchanged. Existing eligibility and correlation caps still apply.
2. Extend the daily single-stat audit to split forecast error into minutes, opportunity volume, conversion and a context residual. Expected volume and conversion come only from the original frozen pregame signal; they are never rebuilt using later data. Compare wins and losses, with stat, side, confidence, absence and game-context slices. Include unique player-game counts.
3. Add `pull context -recentonly` for daily/pre-tip passing and rebounding refreshes. Its bounded request budget cannot be spent on historical windows. Background backfill keeps its existing full-history behavior.
4. Retain passing/rebounding inputs and progress with run artifacts so the exact observations used for reviews can be inspected later.

## Coverage and limits

Shooting opportunity decomposition covers all 24 points/threes questions. Same-game potential-assist/rebound tracking for the other 19 questions was unavailable in the accessible run artifacts and the earlier context backfill artifact. Those remain unavailable; no zeroes, proxies or causal explanations were substituted. The audit can use validated tracking once the next refresh supplies it.

Opportunity decomposition, with frozen opportunity rate q and conversion c:

- Minutes: (actual minutes - forecast minutes) × q × c
- Volume: (actual opportunities - actual minutes × q) × c
- Conversion: actual stat - actual opportunities × c
- Context residual: forecast minutes × q × c - final forecast

These sum exactly to actual minus forecast. This is an arithmetic attribution, not causal proof of injury, role, defense or teammate shooting. FGA + 0.44 FTA is a shooting-opportunity proxy, not official usage percentage. No new numerical multiplier is fitted from these 43 outcomes.

## Run locally

```powershell
go run ./cmd/pull context -seasons 2026 -feeds passing,rebounding -recentonly -maxminutes 2
go run ./cmd/comboaudit -singles -date 2026-11-19 -out run_evidence/v2-singles-audit/report.json
```

Normal daily runs generate both JSON and Markdown reports automatically. Merging alone does not regenerate the live board; the next daily/pre-tip prediction applies the new selection review.
