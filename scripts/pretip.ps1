# Quick refresh close to tip-off (no retrain): newest lines and injuries, then picks.
#   .\scripts\pretip.ps1
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
Run "snapshot lines and injuries"  { go run ./cmd/pull snapshot }
Run "extra books (SportsGameOdds)" { go run ./cmd/sgo snapshot } -Optional
Run "forward accuracy evaluation" { go run ./cmd/accuracy }
Run "offline accuracy report" { python scripts/accuracy_report.py }
Run "player props"                 { go run ./cmd/predict }
Run "game lines"                   { go run ./cmd/games } -Optional

Run "first-quarter points comparisons" { go run ./cmd/quarter } -Optional
