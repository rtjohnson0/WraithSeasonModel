# Commit your changes and push them to GitHub, safely. Run from the project folder:
#   powershell -ExecutionPolicy Bypass -File .\scripts\push.ps1 "what changed"
# It adds your code, force-adds only the small state files (never the big downloaded data), pulls the
# bot's latest commits first so the push is not rejected, and stops with a clear message on a conflict.
param([string]$Message = "update")
$ErrorActionPreference = "Stop"

function Run-Git { & git.exe @args; if ($LASTEXITCODE -ne 0) { throw "git $($args -join ' ') failed" } }

# A rebase left half done from an earlier conflict has to be finished or aborted first.
if (Test-Path ".git\rebase-merge" -PathType Container) {
    Write-Host "A rebase is still in progress. Fix the conflicts, 'git add' the files, 'git rebase --continue', then run this again." -ForegroundColor Red
    exit 1
}

Run-Git add -A
# State files the workflow and site rely on. They are gitignored, so they need -f.
foreach ($f in @("data/model.json", "data/calibration.json", "data/gamecal.json", "data/biasfix.json",
                 "data/picklog.jsonl", "data/settled.jsonl", "data/runs.jsonl")) {
    if (Test-Path $f) { Run-Git add -f $f }
}

& git.exe diff --cached --quiet
if ($LASTEXITCODE -ne 0) {
    Run-Git commit -m $Message
} else {
    Write-Host "Nothing new to commit."
}

# Bring in what the scheduled runs committed, then push.
& git.exe pull --rebase --autostash
if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "The pull hit a conflict. Run 'git status' to see the files. For a file where your copy is the right one:" -ForegroundColor Yellow
    Write-Host "  git checkout --theirs <file>; git add <file>; `$env:GIT_EDITOR='true'; git rebase --continue" -ForegroundColor Yellow
    Write-Host "(In a rebase, --theirs is YOUR version.) For data\picklog.jsonl, settled.jsonl or runs.jsonl keep BOTH sides instead:" -ForegroundColor Yellow
    Write-Host "open the file, delete only the <<<<<<<, ======= and >>>>>>> lines, then git add it and continue." -ForegroundColor Yellow
    exit 1
}
Run-Git push
Write-Host ""
Write-Host "Pushed. Open the Actions tab and run 'daily' or 'site' if you want it live now." -ForegroundColor Green
