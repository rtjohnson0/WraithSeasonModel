# Commit local work and synchronize with the bot using record-aware state merging.
param([string]$Message = "update")
$ErrorActionPreference = "Stop"
function Run-Git { & git.exe @args; if ($LASTEXITCODE -ne 0) { throw "git $($args[0]) failed" } }
$branch = (& git.exe branch --show-current).Trim()
if (-not $branch) { throw "Check out your branch before pushing." }
foreach ($kind in @("rebase-merge", "rebase-apply", "MERGE_HEAD")) {
    $statePath = (& git.exe rev-parse --git-path $kind).Trim()
    if (Test-Path $statePath) { throw "Finish or abort the existing merge/rebase first." }
}
Run-Git add -A
& git.exe diff --cached --quiet
if ($LASTEXITCODE -ne 0) { Run-Git commit -m $Message }
# Python validates and stages existing state files individually, preserves both logs,
# respects deliberate record deletions, and retries normal pushes without force.
& python scripts/sync_state.py --branch $branch --message $Message
if ($LASTEXITCODE -ne 0) { throw "Synchronization stopped; your local commits are retained. Read the error above." }
Write-Host "Pushed. Run the daily or site workflow to publish." -ForegroundColor Green
