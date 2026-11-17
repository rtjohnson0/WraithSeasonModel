# Confidence-first board selection

The default board now ranks qualifying candidates by their estimated probability of the selected outcome. Offered-price return breaks probability ties. This ranking applies before game/player caps, competing-direction resolution, comparison ordering and CSV ranking. The explicit `-selectionpolicy value` research mode retains return-first ranking and remains paper-only.

LEAN options remain enabled by default with the existing target of four total board rows per game when eligible positive-edge candidates are available. Qualified picks are selected first; lower-confidence LEAN rows only fill remaining space. Leans retain their labels and zero-unit status, and are not appended to the permanent pick log. The target is not a guarantee of four rows: existing bad-line, uncertain-status and player limits still apply. Explicit `-pergame 0` is an optional way to disable fill-ins.

Probability estimates, model fitting, calibration, eligibility thresholds, injury/quote checks, forward-evidence gates and paper/check labels are unchanged. Prices still gate eligibility and appear in analytics. This implements a preference for hit probability among qualifying forecasts; it does not prove better predictive accuracy, guarantee wins, force overs, or manufacture 70% probabilities. The model and calibration do not need retraining for this source change.

## Install and refresh

Merge the enclosed `propsmodel` folder into `C:\Users\kingo\Documents\propsmodel`, replacing source files. The package excludes data, snapshots, models, calibration, credentials and generated public board data.

From that project root in PowerShell:

```powershell
& {
    go run ./cmd/pull snapshot
    if ($LASTEXITCODE -ne 0) { throw "Snapshot failed" }
    go run ./cmd/predict
    if ($LASTEXITCODE -ne 0) { throw "Prediction failed" }
    go run ./cmd/export
    if ($LASTEXITCODE -ne 0) { throw "Export failed" }
}
```

The frontend retains chronological game ordering. This source package is not a new fitted model. Older records remain as recorded; selection-policy comparisons must use untouched dates and distinguish releases.

## Checks

Tests cover a higher-probability qualifying candidate surviving a cap despite a lower modeled return; return-first research retaining its previous preference; equal treatment of over and under directions; unchanged probability values; preservation of qualified cards when leans fill remaining space; and an explicit zero fill target adding no weak cards.
