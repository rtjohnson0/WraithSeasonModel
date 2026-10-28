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
Run "current rosters"              { go run ./cmd/pull players }
Run "grade past picks"             { go run ./cmd/settle } -Optional
Run "grade combo plays"            { go run ./cmd/combos settle } -Optional
Run "grade every logged line"      { go run ./cmd/lines } -Optional
Run "review yesterday (csv)"        { go run ./cmd/review } -Optional

if (-not $SkipTrain) {
    Run "retrain" { go run ./cmd/train @lineupFlag }
}

Run "snapshot lines and injuries"  { go run ./cmd/pull snapshot }
Run "extra books (SportsGameOdds)" { go run ./cmd/sgo snapshot } -Optional

Run "player props"                 { go run ./cmd/predict }
Run "game lines"                   { go run ./cmd/games } -Optional

Write-Host ""
Write-Host "Done. Props: data\picks_<date>.csv   Game lines: data\gamepicks_<date>.csv"
