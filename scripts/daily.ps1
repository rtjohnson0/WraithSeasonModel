# Daily pipeline for Windows PowerShell. Run from the project folder:
#   .\scripts\daily.ps1                 (refresh, grade, retrain, snapshot, props + game lines)
#   .\scripts\daily.ps1 -SkipTrain      (same, without retraining)
#   .\scripts\daily.ps1 -Full           (re-download the whole current season instead of only new games)
# Run it in the morning, then run .\scripts\pretip.ps1 an hour before tip-off.
param(
    [int]$Season = 0,
    [switch]$Lineup,
    [switch]$SkipTrain,
    [switch]$Full
)
$ErrorActionPreference = "Stop"
# UTF-8 Go/JSON logs and Git output must not use the Windows ANSI code page.
$env:PYTHONUTF8 = "1"
$rotationPolicy = if ($env:WRAITH_V2_ROTATION_POLICY) { $env:WRAITH_V2_ROTATION_POLICY } else { "rotation-v5" }

# Native commands do not stop the script on failure by themselves; this does.
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

if ($Season -eq 0) {
    $now = Get-Date
    $Season = if ($now.Month -ge 8) { $now.Year } else { $now.Year - 1 }
}
$lineupFlag = @()
if ($Lineup) { $lineupFlag = @("-lineup") }
$newFlag = @("-new")
if ($Full) { $newFlag = @() }

Run "refresh box scores ($Season)" { go run ./cmd/pull stats -seasons $Season @newFlag }
Run "refresh advanced stats"       { go run ./cmd/pull advanced -seasons $Season @newFlag } -Optional
Run "tracking opportunities" { go run ./cmd/pull context -seasons "$Season,$($Season - 1)" -feeds passing,rebounding -recentonly -maxminutes 2 } -Optional
Run "shooting and defense context" { go run ./cmd/pull context -seasons "$Season,$($Season - 1)" -feeds possessions,drives,catchshoot,pullupshot,defense_rim,defense_threes,shooting_distance,lineup_stats,matchups -maxminutes 1 } -Optional
Run "current rosters"              { go run ./cmd/pull players }
Run "audit quote provenance"       { python scripts/audit_quotes.py --data data --apply }
Run "grade past picks"             { go run ./cmd/settle } -Optional
Run "grade game markets" { go run ./cmd/games -settle } -Optional
Run "grade combo plays"            { go run ./cmd/combos settle } -Optional
Run "grade every logged line"      { go run ./cmd/lines } -Optional
Run "grade the V2 challenger"      { go run ./cmd/v2 settle } -Optional
Run "combo projection diagnostic" { go run ./cmd/comboaudit -policy $rotationPolicy } -Optional
Run "V2 single-stat diagnostic" { go run ./cmd/comboaudit -singles -policy $rotationPolicy -out run_evidence/v2-singles-audit/report.json } -Optional
Run "review yesterday (csv)"        { go run ./cmd/review } -Optional

if (-not $SkipTrain) {
    $refreshFlags = @("--season", "$Season")
    if ($Lineup) { $refreshFlags += "--lineup" }
    Run "refresh prop model and matching calibration" { python scripts/refresh_models.py @refreshFlags }
}

Run "calibrate game-v2" { python scripts/calibrate_games.py --force --season ($Season - 1) }
Run "snapshot lines and injuries"  { go run ./cmd/pull snapshot }
Run "pregame projected lineups" { go run ./cmd/pull context -live } -Optional
Run "extra books (SportsGameOdds)" { go run ./cmd/sgo snapshot } -Optional

Run "forward accuracy evaluation" { go run ./cmd/accuracy }
Run "offline accuracy report" { python scripts/accuracy_report.py }
Run "direct combo calibration" { go run ./cmd/combos calibrate } -Optional
Run "player props"                 { go run ./cmd/predict -rotationpolicy $rotationPolicy }
Run "minutes and Under-bias audit" { go run ./cmd/minutesaudit -legacy } -Optional
Run "Over and Under price comparison" { go run ./cmd/priceaudit } -Optional
Run "prior V2 rotation comparison" { go run ./cmd/rotationaudit -policy rotation-v2 -out run_evidence/rotation-v2 } -Optional
Run "absence fallback comparison" { go run ./cmd/rotationaudit -policy rotation-v3 -out run_evidence/absence-fallback } -Optional
Run "V2 context comparison" { go run ./cmd/rotationaudit -policy rotation-v4 -out run_evidence/context-audit } -Optional
Run "game lines"                   { go run ./cmd/games } -Optional
Run "separate game challenger"     { go run ./cmd/gamepace } -Optional

Write-Host ""
Write-Host "Done. Props: data\picks_<date>.csv   Game lines: data\gamepicks_<date>.csv"

Run "first-quarter points comparisons" { go run ./cmd/quarter } -Optional

Run "uncertainty shadow (separate experimental record)" { python scripts/run_tail_shadow.py } -Optional
