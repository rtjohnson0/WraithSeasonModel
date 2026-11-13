# V1 board and Combos redesign — November 13, 2026

The daily board opens with player props. Each card includes its matchup and tip, player portrait, offered line, model probability, price, recent form, projected minutes, projected stat and price edge. Detailed analysis and existing charts remain available.

Moneyline, spread and total projections now live inside the relevant prop's **Matchup & game markets** panel. Games without player props remain reachable under **Rest of the slate**. Quarter comparisons keep their own period and paper history below the full-game board.

Overs watch is visible without opening a dropdown, with six cards initially displayed. A shortcut beside the player-props heading scrolls to it. Watch entries remain clearly labeled as research, rather than selected picks.

Combos use matching portrait cards, search and stat/side filters. Selected combos can be sorted by model value or probability. Research comparisons show their projected total; uncalibrated simulation probabilities remain inside the explanation. Player photos also appear on Overs-watch and quarter cards. Unavailable images fall back to initials.

The shared shell has a wider desktop layout, rounded violet cards, responsive grids, touch controls, improved type hierarchy and a smaller mobile hero. Keyboard focus and reduced-motion preferences remain supported.

## Preview

Open `preview/ModelWraith-preview.html` in a modern browser. It contains explicitly labeled sample data and supports the V1 and Combos tabs. It is a layout preview, not a live board. Player photos and optional web fonts need internet access; photo failures show initials. The deployed app continues to load real exported data or the authenticated private API.

## Install

Copy the updated source into the existing repository, keeping your local data and secrets. From `site`:

```sh
npm ci
npm test
npm run test:ui
npm run build
```

For the Whop-protected build, use `npm run build:private` instead of `npm run build`. The existing daily workflow still handles exports and deployment. Nothing has been published by this update.

## Verification

- Full Go suite passed, including rotation allocation, alternate scenarios, exact replay and separated V2 records.
- 27 frontend unit tests passed, including matchup linking and preservation of games without props.
- 21 component interaction checks passed, covering portrait URLs/fallbacks, visible watch cards, filters/search, empty states, charts, embedded markets, closed quotes and combo research labeling.
- Public and private production builds passed.
- Live browser rendering could not be verified in this environment because browser preview access was restricted. Component checks do not establish pixel-level layout quality or actual external-photo availability.

## V2 calculation check

The previous rotation update already feeds adjusted minutes into V2 production and probability calculations and exports those same projections and rotation details. Questionable-player scenarios and the paired evaluation are also already present. This update verifies those existing tests; it does not promote the experiment or change V1 selection rules, model parameters, predictions, or grading.

## Visual polish — second pass

- Removed the manual Refresh board control and its unused state. The existing 60-second publication check and return-to-tab check remain; they retrieve published data rather than run the model.
- Larger Over/Under line tickets and explicit model-probability captions make each card easier to scan.
- Team-color portrait lighting, small team badges, subtle grid textures, gradient headings and more distinct selected-card borders add visual depth.
- Numeric recent-form tiles show actual historical values against the current line, including separate push styling. They are context, not graded historical bets.
- Desktop hover lift and image zoom respect reduced-motion preferences and do not apply on touch-only devices.
- Added component checks for prominent lines, numeric form and exact-line pushes. Both production builds still pass. Browser visual verification remains unavailable here.

## Schedule order

V1 and Combos keep games grouped by matchup in chronological tip-time order, with explicit Eastern-time dividers. Model-value and probability sorting operate within each game. Simultaneous tips use a stable matchup order; unknown start times appear last. Overs watch and games without props also follow tip time. Filters retain this grouping. Tests cover same-time games staying together, within-game sorting and missing or invalid start times.
