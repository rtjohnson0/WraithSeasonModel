# Turns the latest V2 board (data\v2.json, written by `go run ./cmd/predict`) into a post you can paste.
# Run from the project folder, after predict:
#   .\scripts\post-v2.ps1                       (Strong and Lean plays, games that have not tipped off)
#   .\scripts\post-v2.ps1 -Tiers STRONG         (Strong plays only)
#   .\scripts\post-v2.ps1 -Tiers STRONG,V2LEAN,CHECK -Max 20
#   .\scripts\post-v2.ps1 -MaxJuice 200 -MinEdge 0.01   (loosen the price and edge filters)
#   .\scripts\post-v2.ps1 -IncludeStarted       (also list games that have already tipped off)
#   .\scripts\post-v2.ps1 -NoClipboard          (print only; do not copy)
# It prints the post, saves it to data\v2_post.txt, and copies it to the clipboard.
# Nothing here posts anywhere by itself: you paste it into X, Discord or wherever you post.
#
# Plays are ranked by edge (probability minus the price's break-even), not by probability: a heavy
# favorite at -190 has a high probability from the price alone. Plays priced worse than -MaxJuice
# or with less than -MinEdge are left out.
param(
    [string[]]$Tiers = @("STRONG", "V2LEAN"),
    [int]$Max = 12,
    [int]$MaxJuice = 160,
    [double]$MinEdge = 0.02,
    [string]$Data = "data",
    [switch]$IncludeStarted,
    [switch]$NoClipboard
)
$ErrorActionPreference = "Stop"

$path = Join-Path $Data "v2.json"
if (-not (Test-Path $path)) { throw "$path not found. Run: go run ./cmd/predict" }
$v2 = Get-Content $path -Raw -Encoding UTF8 | ConvertFrom-Json
if (-not $v2.plays) { Write-Host "V2 has no plays on its board for $($v2.date)."; return }

$statName = @{ points = "PTS"; rebounds = "REB"; assists = "AST"; threes = "3PM"; pra = "PTS+REB+AST"; pr = "PTS+REB"; pa = "PTS+AST"; ra = "REB+AST" }
$tierName = @{ STRONG = "STRONG"; V2LEAN = "LEAN"; CHECK = "RISKY (check status and line first)" }
$now = (Get-Date).ToUniversalTime()

function Odds([int]$o) { if ($o -gt 0) { "+$o" } elseif ($o -lt 0) { "$o" } else { "" } }

# Break-even probability at American odds, for plays logged without an edge.
function EdgeOf($p) {
    if ($p.edge) { return [double]$p.edge }
    $o = [double]$p.odds
    if ($o -eq 0) { return 0 }
    $be = if ($o -lt 0) { -$o / (-$o + 100) } else { 100 / ($o + 100) }
    return [double]$p.prob - $be
}

$all = @($v2.plays | Where-Object { $Tiers -contains $_.tier })
$started = 0; $juiced = 0; $thin = 0
$plays = @()
foreach ($p in $all) {
    if (-not $IncludeStarted -and -not [string]::IsNullOrEmpty($p.tip) -and ([datetime]::Parse($p.tip).ToUniversalTime() -le $now)) { $started++; continue }
    if ($p.odds -lt 0 -and (-$p.odds) -gt $MaxJuice) { $juiced++; continue }
    if ((EdgeOf $p) -lt $MinEdge) { $thin++; continue }
    $plays += $p
}
if ($plays.Count -eq 0) {
    Write-Host "No plays left: $started already started, $juiced priced worse than -$MaxJuice, $thin under a $([int]($MinEdge * 100))% edge."
    Write-Host "Loosen it with -MaxJuice, -MinEdge, -Tiers STRONG,V2LEAN,CHECK or -IncludeStarted."
    return
}

# Most trusted tier first, then edge. Capped at -Max.
$rank = @{ STRONG = 0; CHECK = 1; V2LEAN = 2 }
$plays = @($plays | Sort-Object @{ Expression = { $rank[$_.tier] } }, @{ Expression = { EdgeOf $_ }; Descending = $true } | Select-Object -First $Max)

$day = ([datetime]::Parse($v2.date)).ToString("ddd MMM d")
$out = New-Object System.Collections.Generic.List[string]
$out.Add("WraithPicksSZN V2 plays: $day")
$out.Add("V2 is the test model. Tracked, never staked, graded in public.")

foreach ($t in @("STRONG", "CHECK", "V2LEAN")) {
    $group = @($plays | Where-Object { $_.tier -eq $t })
    if ($group.Count -eq 0) { continue }
    $out.Add("")
    $out.Add($tierName[$t])
    foreach ($p in $group) {
        $stat = if ($statName.ContainsKey($p.stat)) { $statName[$p.stat] } else { $p.stat.ToUpper() }
        $side = if ($p.side -eq "OVER") { "O" } else { "U" }
        $team = if ($p.team) { " ($($p.team))" } else { "" }
        $price = Odds $p.odds
        $line = "{0}{1}: {2}{3} {4}" -f $p.player, $team, $side, $p.line, $stat
        if ($price) { $line += " $price" }
        $line += " | {0:N0}% (edge {1:N1})" -f ($p.prob * 100), ((EdgeOf $p) * 100)
        $out.Add($line)
    }
}

$out.Add("")
$out.Add(("{0} shown. Left out: {1} priced worse than -{2}, {3} under a {4}% edge." -f $plays.Count, $juiced, $MaxJuice, $thin, [int]($MinEdge * 100)))

$text = $out -join "`n"
$file = Join-Path $Data "v2_post.txt"
[System.IO.File]::WriteAllText((Join-Path (Get-Location) $file), $text + "`n", (New-Object System.Text.UTF8Encoding($false)))

Write-Host ""
Write-Host $text
Write-Host ""
if (-not $NoClipboard) {
    Set-Clipboard -Value $text
    Write-Host "(copied to the clipboard and saved to $file)"
} else {
    Write-Host "(saved to $file)"
}
