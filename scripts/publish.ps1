# Writes the website data from your logs and pushes it, which triggers the GitHub Pages build.
#   powershell -ExecutionPolicy Bypass -File .\scripts\publish.ps1
#   powershell -ExecutionPolicy Bypass -File .\scripts\publish.ps1 -Date 2026-10-24
# Run it after the daily run (and again after settle/review so yesterday's results show up).
param([string]$Date = "")
$ErrorActionPreference = "Stop"

$dateFlag = @()
if ($Date -ne "") { $dateFlag = @("-date", $Date) }

go run ./cmd/export @dateFlag
if ($LASTEXITCODE -ne 0) { throw "export failed" }

git add site/public/data/picks.json
git diff --cached --quiet
if ($LASTEXITCODE -eq 0) { Write-Host "No changes to publish."; exit 0 }
git commit -m "Update picks $(Get-Date -Format 'yyyy-MM-dd HH:mm')"
git push
Write-Host ""
Write-Host "Pushed. The site updates in about a minute (see the Actions tab)."
