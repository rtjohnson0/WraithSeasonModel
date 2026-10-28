# Commit your changes and push them to GitHub, safely. Run from the project folder:
#   powershell -ExecutionPolicy Bypass -File .\scripts\push.ps1 "what changed"
# It adds your code, force-adds only the small state files (never the big downloaded data), pulls the
# bot's latest commits first so the push is not rejected, and settles conflicts in the data files by
# itself (see Resolve-DataConflicts). A conflict in a code file stops it with a clear message.
param([string]$Message = "update")
$ErrorActionPreference = "Stop"

function Run-Git { & git.exe @args; if ($LASTEXITCODE -ne 0) { throw "git $($args -join ' ') failed" } }

# Settles every conflicted file under data/ and stages it. In a rebase, --theirs is YOUR version.
#   append-only logs (picklog, settled, runs, combolog, combosettled, v2log, v2settled, data/lines/*.jsonl): keep BOTH
#   sides by deleting only the conflict marker lines (written as UTF-8 without a BOM)
#   everything else under data/ (more.json, projlog.jsonl, model, calibration...): rewritten on every
#   run, so take your version
# Returns $false when a conflicted file is not a data file; that one needs a human.
function Resolve-DataConflicts {
    $files = @(& git.exe diff --name-only --diff-filter=U)
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    foreach ($f in $files) {
        $name = $f.Replace('\', '/')
        if (-not $name.StartsWith("data/")) {
            Write-Host ""
            Write-Host "Conflict in $name, which is not a data file, so it needs you." -ForegroundColor Red
            Write-Host "Open it, fix the <<<<<<< ======= >>>>>>> section, then: git add $name ; `$env:GIT_EDITOR='true'; git rebase --continue ; and run this script again." -ForegroundColor Yellow
            return $false
        }
        $path = Join-Path (Get-Location) $name
        $appendOnly = ($name -match '^data/(picklog|settled|runs|combolog|combosettled|v2log|v2settled)\.jsonl$') -or ($name -match '^data/lines/.*\.jsonl$')
        if ($appendOnly) {
            $lines = [System.IO.File]::ReadAllLines($path, $utf8)
            $keep = @($lines | Where-Object { $_ -notmatch '^(<<<<<<<|=======|>>>>>>>)' })
            [System.IO.File]::WriteAllLines($path, [string[]]$keep, $utf8)
            Write-Host "kept both sides of $name"
        } else {
            & git.exe checkout --theirs -- $name
            Write-Host "took your version of $name"
        }
        & git.exe add -- $name
    }
    return $true
}

# A rebase left half done from an earlier conflict has to be finished or aborted first.
if (Test-Path ".git\rebase-merge" -PathType Container) {
    Write-Host "A rebase is still in progress. Fix the conflicts, 'git add' the files, 'git rebase --continue', then run this again." -ForegroundColor Red
    exit 1
}

Run-Git add -A
# State files the workflow and site rely on. They are gitignored, so they need -f.
foreach ($f in @("data/model.json", "data/calibration.json", "data/gamecal.json", "data/biasfix.json", "data/minfix.json", "data/wofix.json",
                 "data/picklog.jsonl", "data/settled.jsonl", "data/runs.jsonl", "data/more.json", "data/projlog.jsonl",
                 "data/combolog.jsonl", "data/combosettled.jsonl", "data/combocorr.json", "data/lines",
                 "data/v2log.jsonl", "data/v2settled.jsonl", "data/v2.json", "data/model_v2.json")) {
    if (Test-Path $f) { Run-Git add -f $f }
}

& git.exe diff --cached --quiet
if ($LASTEXITCODE -ne 0) {
    Run-Git commit -m $Message
} else {
    Write-Host "Nothing new to commit."
}

# Bring in what the scheduled runs committed, settle any data-file conflicts, then push.
& git.exe pull --rebase --autostash
$tries = 0
while ($LASTEXITCODE -ne 0 -and $tries -lt 40) {
    $tries++
    $rebasing = (Test-Path ".git\rebase-merge" -PathType Container) -or (Test-Path ".git\rebase-apply" -PathType Container)
    $unmerged = @(& git.exe diff --name-only --diff-filter=U)
    if (-not $rebasing -and $unmerged.Count -eq 0) {
        Write-Host "The pull failed for a reason other than a conflict (network? sign-in?). Read the message above." -ForegroundColor Red
        exit 1
    }
    if (-not (Resolve-DataConflicts)) { exit 1 }
    if ($rebasing) {
        $env:GIT_EDITOR = "true"
        & git.exe rebase --continue
    } else {
        break
    }
}
$left = @(& git.exe diff --name-only --diff-filter=U)
if ($left.Count -gt 0 -or (Test-Path ".git\rebase-merge" -PathType Container)) {
    Write-Host ""
    Write-Host "Could not finish the rebase on its own. Run 'git status', fix what it lists, 'git rebase --continue', then run this again." -ForegroundColor Red
    exit 1
}
Run-Git push
Write-Host ""
Write-Host "Pushed. Open the Actions tab and run 'daily' or 'site' if you want it live now." -ForegroundColor Green
