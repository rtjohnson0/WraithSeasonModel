# Availability and minutes comparison

Run Daily or pre-tip after installing this update. No retraining is required for the comparison panel. Expand an individual prop or combo to see relevant teammate availability and the history behind it.

The panel separates current model projections from recorded historical minutes. It does not apply an automatic injury boost or replace the headline probability.

- Injury listings require a timestamp no more than 30 minutes old at prediction time. Newer observations win over file order; future, untimed, stale and conflicting listings cannot drive this comparison. A feed's Out listing is distinguished from Questionable or Doubtful. This is a BALLDONTLIE feed listing, not independent verification of the official injury report.
- Historical comparisons require explicit Final status, player/team/game identities, and matching game-team membership. Team minute totals must be close to a complete regulation or overtime box with at least eight participants; partial downloads cannot be treated as absence evidence.
- Only earlier same-team games within 180 days qualify. The player needs at least five appearances in the preceding ten team games. The teammate needs at least three recent appearances and one within the preceding six team games to establish rotation membership.
- Numeric comparisons require at least five games together and three without recorded teammate playing time. These thresholds permit a descriptive comparison; they do not prove a reliable injury effect. Samples, game IDs and minutes are retained with the published pick, and users can inspect the source games in the dropdown.
- Minutes shown in the history are observed averages, including any overtime. They are not an invented +minutes coefficient. The scenario shifts the existing minutes distribution toward each average while retaining its bounds and production rates. Minutes used by that distribution can therefore differ slightly from the historical average.
- Single-stat percentages reprice the exact same side, line and paired prices with the existing calibration. They are minutes-only sensitivity estimates, not a newly validated conditional forecast or a guaranteed hit rate. A teammate's likelihood of sitting is not assigned or blended. Increased minutes can work against an under, and the panel says so.
- Combo comparisons show minutes and projected totals; a conditional percentage is withheld. V2 single-stat picks do not borrow the main model's probability scenarios. The full-game layer does not create quarter comparisons from full-game history.
- Multiple teammate scenarios are evaluated separately, never added together. Historical games without playing time do not establish whether the reason was injury, rest, suspension, or a coaching decision. Other simultaneous absences and role changes can explain part of the difference. An absence association is not a causal replacement-role model.

The main model, picks, stake rules, recorded results and existing graphs are preserved. New panels appear only on newly scored boards with fresh relevant injury data. Existing archived picks are not given retroactive injury explanations.

Validation includes final-status/team provenance, chronology, incomplete boxes, unknown final status, stale/future/conflicting injury captures, thin-history suppression, selected-under direction, and preservation of the headline prediction. Frontend rendered checks cover scenario percentages, source games and withheld unsupported values. These checks establish implementation behavior, not future forecasting accuracy.

## Line and board size

The sportsbook sets the line. A minutes scenario changes the model's stat projection and estimated probability at that same exact line and price; it does not change the sportsbook's number. The availability comparison itself removes no selections and changes no headline probabilities.

The board now targets four main rows per game where positive-edge leans exist, plus up to eight extra leans. Extra leans are expanded by default and count in the stat/side filters. They remain labeled Lean, zero-stake and outside the pick-performance record. Own questionable/doubtful status and nonpositive edges cannot become lean filler. Actual pick probability/edge requirements and per-game/per-player staking limits remain unchanged.

Combo display permits up to 60 tracked picks and line comparisons combined, with 24 visible initially. Comparison rows are spread across games and limited to two per player, keeping their comparison label. Tracking thresholds and limits remain unchanged. Quarter and moneyline/total boards already display all their eligible markets; no extra picks are manufactured there. Missing fresh quotes or supporting history can still leave a game short.
