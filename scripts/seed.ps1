# One-time: commit the box scores you already downloaded so GitHub Actions starts with them
# instead of re-downloading (the bootstrap run is slow). Run from the project folder:
#   .\scripts\seed.ps1
# It refuses any file over 95 MB (GitHub rejects 100 MB), force-adds the data, and pushes.
$ErrorActionPreference = "Stop"
$limit = 95MB

$patterns = @("stats_*.jsonl", "advanced_*.jsonl", "advanced_*.cursor", "players_active.jsonl")
$files = @()
foreach ($p in $patterns) {
    $files += Get-ChildItem -Path "data" -Filter $p -ErrorAction SilentlyContinue
}
# Model, calibrations and the pick record ride along (they must match each other).
foreach ($n in @("model.json", "calibration.json", "gamecal.json", "picklog.jsonl", "settled.jsonl")) {
    $f = Get-Item -Path (Join-Path "data" $n) -ErrorAction SilentlyContinue
    if ($f) { $files += $f }
}
if (-not ($files | Where-Object { $_.Name -like "stats_*" })) {
    Write-Host "No data\stats_*.jsonl found. Run from the project folder." -ForegroundColor Red
    exit 1
}

$total = 0
foreach ($f in $files) {
    $mb = [math]::Round($f.Length / 1MB, 1)
    $total += $f.Length
    if ($f.Length -gt $limit) {
        Write-Host ("{0} is {1} MB, over the limit; not adding it" -f $f.Name, $mb) -ForegroundColor Yellow
        continue
    }
    Write-Host ("adding {0} ({1} MB)" -f $f.Name, $mb)
    git add -f $f.FullName
}
Write-Host ("total {0} MB" -f [math]::Round($total / 1MB, 1))

git diff --cached --quiet
if ($LASTEXITCODE -eq 0) { Write-Host "nothing new to commit"; exit 0 }
git commit -m "seed data for GitHub Actions"
git pull --rebase
git push
Write-Host "Pushed. In Actions, run 'daily' (no bootstrap needed)." -ForegroundColor Green
