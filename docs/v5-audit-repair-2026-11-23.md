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

## Prop review additions before merge

- Extend the existing workload-reversal lean rule to rotation-v5. V5 was capturing the primary-minutes sensitivity but the selection helper accepted only V4. Eligible singles that reverse below 50% at primary minutes remain visible as LEANs for either side; no probability or projection is inflated or rewritten.
- Freeze exact-quote selection metadata in new replay records: posted, excluded with reason, or another quote/side posted. Captures match player, game, stat, line, book and capture time. The immutable historical record stays unknown when this metadata did not exist. The audit groups these first eligible saved decisions by stat, side, status and reason; this is a descriptive comparison, not a replay certification of new selection rules.
- Public lab comparisons allowlist only aggregate selection fields. Hit rates stay hidden below 30 questions/five dates and for unknown historical selection. No player, line, book or private reason string can enter the public summary.
- Daily combo and single-stat diagnostics now explicitly use the active rotation policy, rather than silently reading V4 settlements. On the saved current V5 cohort they match all 106 posted questions, 67–39. Shooting-opportunity decomposition covers 50; actual passing/rebound-chance tracking remains unavailable for 56 questions. Missing tracking is not replaced with zero or a causal explanation.
- PowerShell Daily and pre-tip now default to V5 like Bash/GitHub Actions, with the same WRAITH_V2_ROTATION_POLICY rollback override. Previously both PowerShell prediction steps were hard-coded to V4.

Run normal Daily after merging. Existing leans stay available, past records stay unchanged, and the new selection groups accumulate prospectively from saved exact quotes. This is not a claim that selection changes have demonstrated improved accuracy.
