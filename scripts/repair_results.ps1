# Apply a forensic cache repair without pretending old quotes are verified bets.
param([int]$Season = 0, [switch]$Refresh)
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
$statsPath = "data/stats_$Season.jsonl"
if (-not (Test-Path $statsPath) -or (Get-Item $statsPath).Length -eq 0) {
    $backup = Get-ChildItem -Path data -Filter "stats_$Season.jsonl.before-result-refresh-*" -ErrorAction SilentlyContinue | Sort-Object Name -Descending | Select-Object -First 1
    if ($backup) { Copy-Item -LiteralPath $backup.FullName -Destination $statsPath }
    else {
        $recovery = "audit_findings/recovery/stats_$Season.jsonl"
        if (Test-Path $recovery) { Copy-Item -LiteralPath $recovery -Destination $statsPath }
    }
}
if ($Refresh) {
    Run-Step "Refresh final stats safely" { go run ./cmd/pull stats -seasons $Season }
    Run-Step "Verify current-source results" { go run ./cmd/resultaudit }
} else {
    Run-Step "Compare and repair grades against cached final box scores" { python scripts/audit_results_cache.py --data data --apply }
}
Run-Step "Repair missing replay game identities where exact saved evidence agrees" { python scripts/repair_replay_identity.py --data data --apply }
Run-Step "Export results and audit details" { go run ./cmd/export }
Write-Host "Repaired derived results. Backups and CSVs are in data/cache_result_audit. Historical quote validation remains separate."
