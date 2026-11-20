# Phase 3 — rotation-v4 shadow challenger and loss analysis

**Status:** research-only; no calibrated improvement is claimed.

## What this actually evaluates

This phase evaluates **the real V2 `rotation-v4` pregame forecasts**, not
the primary-model `data/replay` history. `cmd/rotationaudit` already verifies:
frozen model hashes and exact recomputed probability; capture timestamps;
period/market identity; rotation and opportunity provenance; final box-score
team and player identity; and prior-game training evidence. It selects the
earliest eligible question per date/game/player/stat.

When launched with `-policy rotation-v4 -phase3`, that Go auditor additionally
writes its independently verified questions to an ignored, permission-0600
`private/phase3_rotation_v4.jsonl`. The existing Go audit report explicitly
**excludes** this question-level field from JSON via `json:"-"`.

Immediately afterward `scripts/phase3_v2_shadow.py` consumes the temporary
questions, writes **aggregate-only** loss/confidence comparison statistics under
ignored `run_evidence/phase3/v2_challenger_loss_analysis.json`, and deletes
the private questions in a `finally` block even if analysis raises an error.
The default CI artifact paths, static site builder, and normal source sync
never include this temporary `private/` file.

## Frozen challenger policy

The new challenger is a simple, deliberately conservative sensitivity model,
**not a trained replacement**:

```text
P_shadow(over) = P_market(over) + weight × (P_rotation_v4(over) − P_market(over))
weight = 0.75 normally; 0.55 for captured uncertain rotation
```

`uncertain` is declared from decision-time status and documented rotation
support/allocation flags—not final minutes, losses or postgame injuries.

Both production V2 probability and shadow are compared on **identical
verified questions** using log loss and Brier score against the same de-vigged
market baseline. By-stat and loss groups with fewer than eight observations
are suppressed.

To directly test whether **fewer, better-filtered plays** improve accuracy, the
shadow report also compares a predeclared **precision research filter**:
at least 62% predicted confidence, at least 3 percentage points advantage
over the original price, at least 22 projected minutes, no uncertain own
status, and verified rotation support when a relevant teammate is out.
It reports its selection count versus the fixed baseline and hides its hit
rate/ROI until at least eight questions qualify. It is NOT the live model's
selection logic and is not applied automatically. A higher historical hit
rate could be selection noise, so any promising filter requires a later,
prospective holdout and a comparison with market break-even prices.

The hypothetical baseline selection filter is confidence >=55% and at least
one percentage point edge at the recorded quote. It is **not the full live
V2 selection/ranking policy** (which has its own gates). The resulting flat ROI
is illustrative research only, **not** actual published-pick ROI.

## Loss explanations and privacy

Each miss is decomposed into:

```text
actual − projection =
 (actual minutes − projected minutes) × projected stat-per-minute
 + (actual − actual minutes × projected stat-per-minute)
```

Additionally we count recorded pregame teammate absences, relevant absences,
unsupported/missing rotation allocation, limited matched rotations, player
status uncertainty, and captured expected lineups. These indicators **overlap**
and are correlations, not proof of why the pick lost.

Actual minutes and game outcomes are used exclusively in the after-game
diagnostic. Neither is a forward prediction input. No model is fitted to
evaluated outcomes. The private report contains **no player name, ID, game ID,
sportsbook, line, quote, odds or individual pick**.

The ungraded shadow path may summarize captured `rotation-v4` replay
probability differences. It never outputs picks or scores. No automatic
promotion is possible; prospective testing on held-out dates is still
required before considering a production correction.

## Current website and Whop

**Your GitHub Pages site remains unchanged.** Existing public picks will
continue appearing via the current publishing workflow. This phase does NOT
protect historically public predictions or implement Whop member gating.
Securing paid picks requires separating public site assets from private
member-authorized prediction responses.

## Run and test

The morning/pre-tip workflows already invoke the rotation-v4 audit.
After this PR they run:

```sh
go run ./cmd/rotationaudit -policy rotation-v4 -phase3 -out run_evidence/context-audit
python3 scripts/phase3_v2_shadow.py
```

The new CI job runs:

```sh
go test ./cmd/rotationaudit
python3 -m unittest discover -s scripts -p test_phase3_v2_shadow.py -v
```

`go test ./...` and the existing Python suites must also pass before merge.

**Safety:** The repository is already public and still distributes the
existing paid prediction files. No additional question-level betting
data should ever be committed to this repository or attached to a public PR.
