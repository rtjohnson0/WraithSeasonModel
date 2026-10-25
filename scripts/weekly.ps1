# Weekly refit. Run once a week (or after a model change):  .\scripts\weekly.ps1
# Refits the calibration on last season's opening lines PLUS this season's saved daily snapshots,
# retrains the model with the current spread/total features, and refits the game-line blend.
$ErrorActionPreference = "Stop"
function Run {
    param([string]$Label, [scriptblock]$Cmd, [switch]$Optional)
    Write-Host ""
    Write-Host "== $Label =="
    & $Cmd
    if ($LASTEXITCODE -ne 0) {
        if ($Optional) { Write-Host "(skipped: $Label failed, continuing)"; return }
        throw "step failed: $Label"
    }
}
$now = Get-Date
$Last = if ($now.Month -ge 8) { $now.Year - 1 } else { $now.Year - 2 }

if (-not (Test-Path "data\opening_games_$Last.jsonl")) {
    Run "last season's opening game lines" { go run ./cmd/pull gameodds -season $Last }
}
Run "backtest + calibration (history + live snapshots)" { go run ./cmd/backtest -season $Last -live }
Run "retrain"                                           { go run ./cmd/train }
Run "game-line backtest"                                { go run ./cmd/games -backtest -season $Last } -Optional
Write-Host ""
Write-Host "Done. Paste the backtest scorecard to review it."
