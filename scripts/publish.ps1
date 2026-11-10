# Publish saved state through the same conflict-safe synchronization as the workflow.
param([string]$Date = "")
$ErrorActionPreference = "Stop"
# UTF-8 Go/JSON logs and Git output must not use the Windows ANSI code page.
$env:PYTHONUTF8 = "1"
$dateFlag = @()
if ($Date -ne "") { $dateFlag = @("-date", $Date) }
go run ./cmd/export @dateFlag
if ($LASTEXITCODE -ne 0) { throw "export failed" }
$branch = (& git.exe branch --show-current).Trim()
if (-not $branch) { throw "Check out your branch before publishing." }
python scripts/sync_state.py --branch $branch --message "Publish saved records"
if ($LASTEXITCODE -ne 0) { throw "Could not synchronize state; local commits are retained." }
Write-Host "Saved records synchronized. New commits trigger the site workflow on the default branch."
