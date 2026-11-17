# Run from the project root. No production model or calibration is overwritten.
param([int]$Season = 2025, [string]$Data = 'data', [string]$Output = 'research')
$ErrorActionPreference = 'Stop'
& go run ./cmd/compare -season $Season -data $Data -out $Output
if ($LASTEXITCODE -ne 0) { throw 'Historical comparison failed; production files were not changed.' }
& python scripts/comparison_report.py --root $Output
if ($LASTEXITCODE -ne 0) { throw 'Comparison completed, but HTML rendering failed.' }
$latest = Get-ChildItem -Directory $Output | Where-Object { Test-Path (Join-Path $_.FullName 'study.json') } | Sort-Object Name | Select-Object -Last 1
if ($latest) { Start-Process (Join-Path $latest.FullName 'comparison.html') }
