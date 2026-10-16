$ErrorActionPreference = 'Stop'

$promptArgs = ($env:HARVEY_PROMPT ?? '').Trim() -split '\s+', 1
$dbPath = if ($promptArgs.Count -ge 1 -and $promptArgs[0]) { $promptArgs[0] } else { '' }

if (-not $dbPath) {
    $dbPath = if ($env:DB_PATH) { $env:DB_PATH } else { 'agents/knowledge.db' }
}

$kbCmd = Get-Command kb -ErrorAction SilentlyContinue
if ($kbCmd) {
    $kb = $kbCmd.Source
} else {
    $kb = Join-Path $HOME 'Laboratory/knowledge/bin/kb'
    if (-not (Test-Path $kb)) {
        Write-Error "kb not found on PATH or at $kb — build it: cd ~/Laboratory/knowledge && make build"
        exit 1
    }
}

& $kb --db $dbPath project list

Write-Host "Database ready: $dbPath"
