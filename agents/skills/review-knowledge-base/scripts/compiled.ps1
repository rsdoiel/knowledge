$ErrorActionPreference = 'Stop'

$promptArgs = ($env:HARVEY_PROMPT ?? '').Trim() -split '\s+', 2
$dbPath = if ($promptArgs.Count -ge 1 -and $promptArgs[0]) { $promptArgs[0] } else { '' }
$focus = if ($promptArgs.Count -ge 2 -and $promptArgs[1]) { $promptArgs[1] } else { '' }

if (-not $dbPath) {
    $dbPath = if ($env:DB_PATH) { $env:DB_PATH } else { 'agents/knowledge.db' }
}
if (-not $focus) {
    $focus = if ($env:FOCUS) { $env:FOCUS } else { 'all' }
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

# This skill needs kb >= 0.0.19: qualified record references (0.0.17), and accepting a
# record or promoting a summary being a person's act, enforced by kb (0.0.18, 0.0.19).
$kbMin = [version]'0.0.19'
$kbVer = ((& $kb -version 2>$null) -join ' ' -split '\s+')[1]
if (-not $kbVer -or [version]$kbVer -lt $kbMin) {
    Write-Error "this skill needs kb >= $kbMin; found $(if ($kbVer) { $kbVer } else { 'none' }). Upgrade: irm https://rsdoiel.github.io/knowledge/installer.ps1 | iex"
    exit 1
}

if (-not (Get-Command sqlite3 -ErrorAction SilentlyContinue)) {
    Write-Error "sqlite3 not found on PATH — install SQLite3 and retry."
    exit 1
}

# The root walked by the index check. Derived from the database path so the
# check covers the workspace the report is about, not whichever directory
# this happened to be invoked from.
$indexRoot = Split-Path -Parent (Split-Path -Parent ((Resolve-Path $dbPath).Path))
if (-not $indexRoot) { $indexRoot = Join-Path $HOME 'Laboratory' }

$today = Get-Date -Format 'yyyy-MM-dd'
Write-Host "## Knowledge Base Report  ($today)`n"

# LEFT JOIN observation_relations so a corrected observation is labelled
# rather than reported as current (kb v0.0.8, DR-0023).
$obsQuery = "SELECT o.id, p.name AS project, o.kind, o.created_at, CASE WHEN sup.to_id IS NULL THEN '' ELSE 'superseded by ' || sup.from_id END AS status, substr(o.body, 1, 80) AS body_preview FROM observations o JOIN projects p ON p.id = o.project_id LEFT JOIN observation_relations sup ON sup.to_id = o.id AND sup.relationship = 'supersedes' ORDER BY o.created_at DESC LIMIT 20;"

if ($focus -eq 'all') {
    Write-Host "### Projects`n"
    & $kb --db $dbPath project list
    Write-Host "`n### Recent Observations  (last 20)`n"
    sqlite3 -header -column $dbPath $obsQuery
    Write-Host "`n### Concepts`n"
    & $kb --db $dbPath concept list
    Write-Host "`n### Decision Records`n"
    & $kb --db $dbPath record list --all
    Write-Host "`n### Documents`n"
    & $kb --db $dbPath document list
    Write-Host "`n### Review Queue`n"
    & $kb --db $dbPath document review list
    Write-Host "`n### Index Check`n"
    # --check never writes. Without it, --all rewrites every index.md it
    # finds, which is not a review action. Exit 1 means stale, which is the
    # finding, so it must not abort the report. Above 1 the check itself failed
    # (a malformed record, an unreadable file): not a finding, so say so.
    # index never takes --db: it walks record files, not the database, and since
    # kb v0.0.13 it refuses one (exit 2).
    & $kb index $indexRoot --all --check
    if ($LASTEXITCODE -gt 1) { Write-Warning "the index check failed: kb exited $LASTEXITCODE, see above" }
    Write-Host "`n### Candidate Concepts  (suggestions, not findings)`n"
    & $kb --db $dbPath concept suggest --limit 10
    if ($LASTEXITCODE -ne 0) { Write-Warning "candidate concepts unavailable: kb exited $LASTEXITCODE, see above" }
} elseif ($focus -eq 'projects') {
    Write-Host "### Projects`n"
    & $kb --db $dbPath project list
} elseif ($focus -eq 'observations') {
    Write-Host "### Recent Observations  (last 20)`n"
    sqlite3 -header -column $dbPath $obsQuery
} elseif ($focus -eq 'concepts') {
    Write-Host "### Concepts`n"
    & $kb --db $dbPath concept list
} elseif ($focus -eq 'concept-candidates') {
    Write-Host "### Candidate Concepts  (suggestions, not findings)`n"
    & $kb --db $dbPath concept suggest --limit 20
} elseif ($focus -eq 'records') {
    Write-Host "### Decision Records`n"
    & $kb --db $dbPath record list --all
} elseif ($focus -eq 'documents') {
    Write-Host "### Documents`n"
    & $kb --db $dbPath document list
} elseif ($focus -eq 'review-queue') {
    Write-Host "### Review Queue`n"
    & $kb --db $dbPath document review list
} elseif ($focus -eq 'index-check') {
    Write-Host "### Index Check  (root: $indexRoot)`n"
    & $kb index $indexRoot --all --check
} elseif ($focus -like 'search:*') {
    $term = $focus.Substring(7)
    Write-Host "### Search results for `"$term`"`n"
    # kb search exits 1 when it finds nothing: "no results", a normal answer.
    # 2 or more is a failure (a bad term, a broken database, no workspace) and
    # is passed on, not reported as "no results".
    & $kb --db $dbPath search $term
    if ($LASTEXITCODE -gt 1) { exit $LASTEXITCODE }
} else {
    Write-Host "## Project: $focus`n"
    & $kb --db $dbPath project show $focus
    if ($LASTEXITCODE -eq 1) {
        # 1 is "no such project". Any other failure (an unreadable database, no
        # workspace) is not, and must not be reported as one.
        Write-Host "Project '$focus' not found. Available projects:"
        & $kb --db $dbPath project list
        exit 1
    } elseif ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    Write-Host "`n### Concepts`n"
    & $kb --db $dbPath project concepts $focus
    Write-Host "`n### Observations`n"
    & $kb --db $dbPath observation list --project $focus
    Write-Host "`n### Decision Records`n"
    & $kb --db $dbPath record list $focus
    Write-Host "`n### Documents`n"
    & $kb --db $dbPath document list --project $focus
}

Write-Host "Done."
