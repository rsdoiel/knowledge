$ErrorActionPreference = 'Stop'

$promptArgs = ($env:HARVEY_PROMPT ?? '').Trim() -split '\s+', 2
$dbPath = if ($promptArgs.Count -ge 1 -and $promptArgs[0]) { $promptArgs[0] } else { '' }
$action = if ($promptArgs.Count -ge 2 -and $promptArgs[1]) { $promptArgs[1] } else { '' }

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

# This skill needs kb >= 0.0.19: qualified record references (0.0.17), and accepting a
# record or promoting a summary being a person's act, enforced by kb (0.0.18, 0.0.19).
$kbMin = [version]'0.0.19'
$kbVer = ((& $kb -version 2>$null) -join ' ' -split '\s+')[1]
if (-not $kbVer -or [version]$kbVer -lt $kbMin) {
    Write-Error "this skill needs kb >= $kbMin; found $(if ($kbVer) { $kbVer } else { 'none' }). Upgrade: irm https://rsdoiel.github.io/knowledge/installer.ps1 | iex"
    exit 1
}

if (-not $action) {
    Write-Host "What would you like to add or change?"
    Write-Host "   1. project"
    Write-Host "   2. observation"
    Write-Host "   3. observation-update (correct one by superseding it)"
    Write-Host "   4. concept"
    Write-Host "   5. concept-suggest (read-only: candidate new concepts)"
    Write-Host "   6. source"
    Write-Host "   7. record (decision record)"
    Write-Host "   8. document (story, article, screenplay)"
    Write-Host "   9. document-draft"
    Write-Host "  10. document-promote"
    Write-Host "  11. document-tag (insert [[wikilinks]])"
    Write-Host "  12. project-rename"
    Write-Host "  13. concept-rename"
    Write-Host "  14. link-project-concept"
    Write-Host "  15. link-observation-concept"
    Write-Host "  16. link-observation-source"
    $action = Read-Host "Choice (name or number)"
    switch ($action) {
        '1'  { $action = 'project' }
        '2'  { $action = 'observation' }
        '3'  { $action = 'observation-update' }
        '4'  { $action = 'concept' }
        '5'  { $action = 'concept-suggest' }
        '6'  { $action = 'source' }
        '7'  { $action = 'record' }
        '8'  { $action = 'document' }
        '9'  { $action = 'document-draft' }
        '10' { $action = 'document-promote' }
        '11' { $action = 'document-tag' }
        '12' { $action = 'project-rename' }
        '13' { $action = 'concept-rename' }
        '14' { $action = 'link-project-concept' }
        '15' { $action = 'link-observation-concept' }
        '16' { $action = 'link-observation-source' }
    }
}

switch ($action) {
    'project' {
        $projName = Read-Host "Project name (filesystem-safe, no spaces)"
        if (-not $projName) { Write-Error "Name is required."; exit 1 }
        $projDesc = Read-Host "Description (optional)"
        $projStatus = Read-Host "Status [concept/active/paused/concluded] (default: active)"
        if (-not $projStatus) { $projStatus = 'active' }
        & $kb --db $dbPath project add --status $projStatus $projName $projDesc
    }

    'observation' {
        Write-Host "Available projects:"
        & $kb --db $dbPath project list
        $projName = Read-Host "Project name"
        if (-not $projName) { Write-Error "Project name is required."; exit 1 }
        $obsKind = Read-Host "Kind [note/finding/decision/question/hypothesis] (default: note)"
        if (-not $obsKind) { $obsKind = 'note' }
        $obsBody = Read-Host "Observation body"
        if (-not $obsBody) { Write-Error "Body is required."; exit 1 }
        & $kb --db $dbPath observation add --project $projName $obsKind $obsBody
    }

    'observation-update' {
        # An observation is never edited in place (DR-0023): this inserts a
        # NEW observation and links it back with a supersedes edge.
        $projName = Read-Host "Project name (to list its observations)"
        if ($projName) { & $kb --db $dbPath observation list --project $projName }
        $obsId = Read-Host "Observation ID being corrected"
        if (-not $obsId) { Write-Error "Observation ID is required."; exit 1 }
        $obsBody = Read-Host "Corrected body"
        if (-not $obsBody) { Write-Error "Body is required."; exit 1 }
        & $kb --db $dbPath observation update $obsId $obsBody
    }

    'concept' {
        $conceptName = Read-Host "Concept name"
        if (-not $conceptName) { Write-Error "Name is required."; exit 1 }
        $conceptDesc = Read-Host "Description (optional)"
        & $kb --db $dbPath concept add $conceptName $conceptDesc
    }

    'concept-suggest' {
        # Read-only. Never writes; a candidate becomes a concept only when a
        # human runs concept add. Expect roughly half the list to be noise.
        $projName = Read-Host "Scope to one project (blank for the whole corpus)"
        $sugLimit = Read-Host "How many to show (default 20)"
        if (-not $sugLimit) { $sugLimit = '20' }
        if ($projName) {
            & $kb --db $dbPath concept suggest --project $projName --limit $sugLimit
        } else {
            & $kb --db $dbPath concept suggest --limit $sugLimit
        }
        Write-Host ""
        Write-Host "These are candidates, not concepts. Add one with:"
        Write-Host "  $kb --db $dbPath concept add NAME `"DESCRIPTION`""
    }

    'source' {
        $srcTitle = Read-Host "Source title"
        if (-not $srcTitle) { Write-Error "Title is required."; exit 1 }
        $srcDoi = Read-Host "DOI (optional)"
        if ($srcDoi) {
            & $kb --db $dbPath source add $srcTitle --doi $srcDoi
        } else {
            & $kb --db $dbPath source add $srcTitle
        }
    }

    'record' {
        $recTitle = Read-Host "Record title"
        if (-not $recTitle) { Write-Error "Title is required."; exit 1 }
        $recTrigger = Read-Host "Trigger [design/plan-review/implementation/live-test/release-review/request]"
        if (-not $recTrigger) { Write-Error "Trigger is required."; exit 1 }
        $projName = Read-Host "Project name (blank for a workspace-tier record)"
        $recKind = Read-Host "Kind [decision/correction/refinement] (default: decision)"
        if (-not $recKind) { $recKind = 'decision' }
        if ($projName) {
            & $kb --db $dbPath record new --title $recTitle --trigger $recTrigger --project $projName --kind $recKind
        } else {
            & $kb --db $dbPath record new --title $recTitle --trigger $recTrigger --workspace --kind $recKind
        }
        Write-Host ""
        Write-Host "Fill in the body sections, then index it with:"
        Write-Host "  $kb --db $dbPath ingest PATH_TO_DECISIONS_DIR"
        Write-Host "It is written 'proposed'. Only its author promotes it — never do that on their behalf."
    }

    'document' {
        Write-Host "Available projects:"
        & $kb --db $dbPath project list
        $projName = Read-Host "Project name"
        if (-not $projName) { Write-Error "Project name is required."; exit 1 }
        $docPath = Read-Host "Path to the document (.md, .fountain/.spmd, .txt)"
        if (-not $docPath) { Write-Error "Path is required."; exit 1 }
        if (-not (Test-Path $docPath)) { Write-Error "No such file: $docPath"; exit 1 }
        & $kb --db $dbPath document ingest $docPath --project $projName --dry-run
        $confirm = Read-Host "Proceed with the ingest? [y/N]"
        if ($confirm -ne 'y' -and $confirm -ne 'Y') {
            Write-Host "Cancelled; nothing written."
            exit 0
        }
        & $kb --db $dbPath document ingest $docPath --project $projName
        Write-Host "Sections land 'unsummarized' — invisible to search until drafted and promoted."
    }

    'document-draft' {
        $projName = Read-Host "Project name (blank for every project's queue)"
        if ($projName) {
            & $kb --db $dbPath document review list --project $projName
        } else {
            & $kb --db $dbPath document review list
        }
        $sectionId = Read-Host "Section ID"
        if (-not $sectionId) { Write-Error "Section ID is required."; exit 1 }
        $summaryBody = Read-Host "Summary body"
        if (-not $summaryBody) { Write-Error "Summary body is required."; exit 1 }
        $draftedBy = Read-Host "Written by (human, or a model identifier)"
        if (-not $draftedBy) { $draftedBy = 'human' }
        $confidence = Read-Host "Confidence 0-1 (optional)"
        if ($confidence) {
            & $kb --db $dbPath document draft $sectionId $summaryBody --by $draftedBy --confidence $confidence
        } else {
            & $kb --db $dbPath document draft $sectionId $summaryBody --by $draftedBy
        }
        Write-Host "Drafted, not searchable — promotion is a separate, human action."
    }

    'document-promote' {
        $projName = Read-Host "Project name (blank for every project's queue)"
        if ($projName) {
            & $kb --db $dbPath document review list --project $projName --status drafted
        } else {
            & $kb --db $dbPath document review list --status drafted
        }
        $sectionId = Read-Host "Section ID to promote"
        if (-not $sectionId) { Write-Error "Section ID is required."; exit 1 }
        & $kb --db $dbPath document review promote $sectionId
    }

    'document-tag' {
        # A pure file operation: edits the project's document files, never
        # the database. Always preview first.
        $projName = Read-Host "Project name"
        if (-not $projName) { Write-Error "Project name is required."; exit 1 }
        $concepts = Read-Host "Concepts, comma-separated (blank = density threshold)"
        if ($concepts) {
            & $kb --db $dbPath document tag --project $projName --concept $concepts --dry-run
        } else {
            & $kb --db $dbPath document tag --project $projName --dry-run
        }
        $confirm = Read-Host "Write these changes to the files? [y/N]"
        if ($confirm -ne 'y' -and $confirm -ne 'Y') {
            Write-Host "Cancelled; no file written."
            exit 0
        }
        if ($concepts) {
            & $kb --db $dbPath document tag --project $projName --concept $concepts
        } else {
            & $kb --db $dbPath document tag --project $projName
        }
    }

    'project-rename' {
        # Flags precede the names: project rename [--root PATH] [--dry-run] OLD NEW
        Write-Host "Projects:"
        & $kb --db $dbPath project list
        $oldName = Read-Host "Current name"
        $newName = Read-Host "New name"
        if (-not $oldName -or -not $newName) { Write-Error "Both names are required."; exit 1 }
        & $kb --db $dbPath project rename --dry-run $oldName $newName
        $confirm = Read-Host "Proceed? This rewrites the project: frontmatter of every owned record. [y/N]"
        if ($confirm -ne 'y' -and $confirm -ne 'Y') {
            Write-Host "Cancelled; nothing renamed and no file written."
            exit 0
        }
        & $kb --db $dbPath project rename $oldName $newName
    }

    'concept-rename' {
        Write-Host "Concepts:"
        & $kb --db $dbPath concept list
        $oldName = Read-Host "Current name"
        $newName = Read-Host "New name"
        if (-not $oldName -or -not $newName) { Write-Error "Both names are required."; exit 1 }
        & $kb --db $dbPath concept rename $oldName $newName
    }

    'link-project-concept' {
        Write-Host "Projects:"
        & $kb --db $dbPath project list
        Write-Host "Concepts:"
        & $kb --db $dbPath concept list
        $projName = Read-Host "Project name"
        $conceptName = Read-Host "Concept name"
        if (-not $projName -or -not $conceptName) { Write-Error "Both names are required."; exit 1 }
        & $kb --db $dbPath link project $projName $conceptName
    }

    'link-observation-concept' {
        $projName = Read-Host "Project name (to list its observations)"
        if ($projName) { & $kb --db $dbPath observation list --project $projName }
        Write-Host "Concepts:"
        & $kb --db $dbPath concept list
        $obsId = Read-Host "Observation ID"
        $conceptName = Read-Host "Concept name"
        if (-not $obsId -or -not $conceptName) { Write-Error "Observation ID and concept name are required."; exit 1 }
        & $kb --db $dbPath link observation $obsId $conceptName
    }

    'link-observation-source' {
        $obsId = Read-Host "Observation ID"
        $srcId = Read-Host "Source ID"
        $relationship = Read-Host "Relationship (default: cited)"
        if (-not $relationship) { $relationship = 'cited' }
        if (-not $obsId -or -not $srcId) { Write-Error "Observation ID and Source ID are required."; exit 1 }
        & $kb --db $dbPath source link $obsId $srcId --relationship $relationship
    }

    default {
        Write-Error "Unknown action: $action`nValid actions: project, observation, observation-update, concept, concept-suggest, source, record, document, document-draft, document-promote, document-tag, project-rename, concept-rename, link-project-concept, link-observation-concept, link-observation-source"
        exit 1
    }
}

Write-Host "Done."
