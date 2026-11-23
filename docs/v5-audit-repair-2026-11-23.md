# V5 audit repair and game-model study

The live V2 policy is rotation-v5. Its forecast capture intentionally differs from V4: unsupported minutes are retained from the primary forecast, supported changes are capped at ±2.5 minutes, uncertain own status cannot receive an allocation boost, the old threes opportunity effect is removed, and a bounded attempt/conversion factor is applied instead. The old audit rejected the valid new captures because it still required V4 evidence shapes and rate-scale math.

## Repair

V5 is validated independently. Shared lineage, pregame timing, lineup, absence and fitted-effect checks are retained through the V4 evidence validator, then the actual V5 scales are reconstructed from the frozen primary rates and opportunity signals. The captured policy scales and rotation scales must match. Minutes targets must match the capped allocation, with supported joint-absence metadata and no uncertain own status. Applied minutes must match the frozen policy. No acceptance by version label alone; no forecast, grade, stake or earlier record changes.

On the retained November 23 morning run, the old report had zero questions and 522 production-provenance exclusions. Replaying those saved captures against identity-checked finals now evaluates **503 questions / 142 player-games / two dates**, through November 22. **19** remain excluded because matching final data is unavailable. The 765 current-date captures remain pending. This cohort includes scored forecasts, not just posted picks.

V5 minutes MAE: **4.568**, versus the matched primary forecast **4.582**. This small two-date difference does not establish improvement. Public scores remain suppressed until the existing five-date floor; counts and audit health are visible immediately. Public audit output allowlists named aggregate counters only.

## Spread / total work

Add a fixed, read-only recency correction study to the existing game challenger. It learns margin and total residuals from strictly earlier monthly out-of-fold forecasts within 180 days, with a 45-day half-life and 100 neutral pseudo-games. Evaluation requires at least 200 earlier games; games on the same date never inform one another. No latest-result tuning, sportsbook-based success claim or auto-promotion.

On **2,741 matching games / 538 dates**:

| Point error (MAE) | Existing challenger | Residual candidate |
|---|---:|---:|
| Margin | 11.560 | 11.582 |
| Total | 15.042 | 15.017 |

The margin candidate is worse and the total change is tiny. Neither is promoted to live forecasting. The aggregate comparison appears in the existing V2.1 game-market lab after the next daily run. Existing leans remain visible.

## Run

```powershell
go run ./cmd/rotationaudit -policy rotation-v5 -out run_evidence/guarded-v5
go run ./cmd/gamepace -audit -out run_evidence/game-pace
```

Normal Daily already invokes both model diagnostics and the public lab exporters. After merge, run Daily to regenerate the live lab. This repairs the evaluation, not the earlier betting record. Public outputs contain no per-player or per-book audit details.
