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
Run "prepare game-v2 calibration" { python scripts/calibrate_games.py }
$now = Get-Date
$Season = if ($now.Month -ge 8) { $now.Year } else { $now.Year - 1 }
Run "tracking opportunities" { go run ./cmd/pull context -seasons "$Season" -feeds passing,rebounding -maxminutes 1 } -Optional
Run "snapshot lines and injuries"  { go run ./cmd/pull snapshot }
Run "pregame projected lineups" { go run ./cmd/pull context -live } -Optional
Run "extra books (SportsGameOdds)" { go run ./cmd/sgo snapshot } -Optional
Run "forward accuracy evaluation" { go run ./cmd/accuracy }
Run "offline accuracy report" { python scripts/accuracy_report.py }
Run "direct combo calibration" { go run ./cmd/combos calibrate } -Optional
Run "player props"                 { go run ./cmd/predict -rotationpolicy rotation-v4 }
Run "minutes and Under-bias audit" { go run ./cmd/minutesaudit -legacy } -Optional
Run "Over and Under price comparison" { go run ./cmd/priceaudit } -Optional
Run "prior V2 rotation comparison" { go run ./cmd/rotationaudit -policy rotation-v2 -out run_evidence/rotation-v2 } -Optional
Run "absence fallback comparison" { go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback } -Optional
Run "V2 context comparison" { go run ./cmd/rotationaudit -policy rotation-v4 -out run_evidence/context-audit } -Optional
Run "game lines"                   { go run ./cmd/games } -Optional

Run "first-quarter points comparisons" { go run ./cmd/quarter } -Optional

Run "uncertainty shadow (separate experimental record)" { python scripts/run_tail_shadow.py } -Optional
