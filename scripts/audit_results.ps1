# Refresh a season, recheck all individual/combo/V2 results, then export the audit UI.
param([int]$Season = 0, [switch]$SkipRefresh)
$ErrorActionPreference = "Stop"
$env:PYTHONUTF8 = "1"
function Run-Step([string]$Label, [scriptblock]$Action) {
    Write-Host "== $Label =="
    & $Action
    if ($LASTEXITCODE -ne 0) { throw "step failed: $Label" }
}
if ($Season -eq 0) {
    $now = Get-Date
    $Season = if ($now.Month -ge 8) { $now.Year } else { $now.Year - 1 }
}
if (-not $SkipRefresh) {
    $statsPath = "data/stats_$Season.jsonl"
    if (Test-Path $statsPath) {
        $stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssfffZ")
        Copy-Item -LiteralPath $statsPath -Destination "$statsPath.before-result-refresh-$stamp"
    }
    Run-Step "Refresh final box scores for season $Season" { go run ./cmd/pull stats -seasons $Season }
}
Run-Step "Audit all saved individual, combo and V2 results" { go run ./cmd/resultaudit }
Run-Step "Export corrected records and audit details" { go run ./cmd/export }
Write-Host "Audit complete. Reports: data/settled_audit.json, data/combosettled_audit.json, data/v2settled_audit.json"
