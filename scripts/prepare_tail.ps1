# Optional: rebuild the isolated research candidate from cached historical data.
# The daily workflow already records the bundled candidate; this is not needed daily.
$ErrorActionPreference = "Stop"
$env:PYTHONUTF8 = "1"
go run ./cmd/tailstudy -writecandidate
if ($LASTEXITCODE -ne 0) { throw "Candidate study failed. Primary model was not replaced." }
Write-Host "Candidate ready. Daily and pre-tip now record it separately. Freeze this pair during forward evaluation."
