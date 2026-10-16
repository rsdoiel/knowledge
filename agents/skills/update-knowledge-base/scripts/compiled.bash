#!/usr/bin/env bash
set -euo pipefail

read -ra _args <<< "${HARVEY_PROMPT:-}"
db_path="${_args[0]:-}"
action="${_args[1]:-}"

if [ -z "${db_path}" ]; then
    db_path="${DB_PATH:-agents/knowledge.db}"
fi

kb="kb"
if ! command -v kb >/dev/null 2>&1; then
    kb="${HOME}/Laboratory/knowledge/bin/kb"
    if [ ! -x "${kb}" ]; then
        echo "kb not found on PATH or at ${kb} — build it: cd ~/Laboratory/knowledge && make build" >&2
        exit 1
    fi
fi

# This skill needs kb >= 0.0.19: qualified record references (0.0.17), and accepting a
# record or promoting a summary being a person's act, enforced by kb (0.0.18, 0.0.19).
_kb_min="0.0.19"
_kb_ver="$("${kb}" -version 2>/dev/null | awk '{print $2}')"
if [ -z "${_kb_ver}" ] || [ "$(printf '%s\n%s\n' "${_kb_min}" "${_kb_ver}" | sort -V | head -n1)" != "${_kb_min}" ]; then
    echo "this skill needs kb >= ${_kb_min}; found ${_kb_ver:-none}. Upgrade: curl https://rsdoiel.github.io/knowledge/installer.sh | sh" >&2
    exit 1
fi

if [ -z "${action}" ]; then
    echo "What would you like to add or change?"
    echo "   1. project"
    echo "   2. observation"
    echo "   3. observation-update (correct one by superseding it)"
    echo "   4. concept"
    echo "   5. concept-suggest (read-only: candidate new concepts)"
    echo "   6. source"
    echo "   7. record (decision record)"
    echo "   8. document (story, article, screenplay)"
    echo "   9. document-draft"
    echo "  10. document-promote"
    echo "  11. document-tag (insert [[wikilinks]])"
    echo "  12. project-rename"
    echo "  13. concept-rename"
    echo "  14. link-project-concept"
    echo "  15. link-observation-concept"
    echo "  16. link-observation-source"
    read -rp "Choice (name or number): " action
    case "${action}" in
        1) action="project" ;;
        2) action="observation" ;;
        3) action="observation-update" ;;
        4) action="concept" ;;
        5) action="concept-suggest" ;;
        6) action="source" ;;
        7) action="record" ;;
        8) action="document" ;;
        9) action="document-draft" ;;
        10) action="document-promote" ;;
        11) action="document-tag" ;;
        12) action="project-rename" ;;
        13) action="concept-rename" ;;
        14) action="link-project-concept" ;;
        15) action="link-observation-concept" ;;
        16) action="link-observation-source" ;;
    esac
fi

case "${action}" in
    project)
        read -rp "Project name (filesystem-safe, no spaces): " proj_name
        if [ -z "${proj_name}" ]; then echo "Name is required." >&2; exit 1; fi
        read -rp "Description (optional): " proj_desc
        read -rp "Status [concept/active/paused/concluded] (default: active): " proj_status
        proj_status="${proj_status:-active}"
        "${kb}" --db "${db_path}" project add --status "${proj_status}" "${proj_name}" "${proj_desc}"
        ;;

    observation)
        echo "Available projects:"
        "${kb}" --db "${db_path}" project list
        read -rp "Project name: " proj_name
        if [ -z "${proj_name}" ]; then echo "Project name is required." >&2; exit 1; fi
        read -rp "Kind [note/finding/decision/question/hypothesis] (default: note): " obs_kind
        obs_kind="${obs_kind:-note}"
        read -rp "Observation body: " obs_body
        if [ -z "${obs_body}" ]; then echo "Body is required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" observation add --project "${proj_name}" "${obs_kind}" "${obs_body}"
        ;;

    observation-update)
        # An observation is never edited in place (DR-0023): this inserts a
        # NEW observation and links it back with a supersedes edge.
        read -rp "Project name (to list its observations): " proj_name
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" observation list --project "${proj_name}"
        fi
        read -rp "Observation ID being corrected: " obs_id
        if [ -z "${obs_id}" ]; then echo "Observation ID is required." >&2; exit 1; fi
        read -rp "Corrected body: " obs_body
        if [ -z "${obs_body}" ]; then echo "Body is required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" observation update "${obs_id}" "${obs_body}"
        ;;

    concept)
        read -rp "Concept name: " concept_name
        if [ -z "${concept_name}" ]; then echo "Name is required." >&2; exit 1; fi
        read -rp "Description (optional): " concept_desc
        "${kb}" --db "${db_path}" concept add "${concept_name}" "${concept_desc}"
        ;;

    concept-suggest)
        # Read-only. Never writes; a candidate becomes a concept only when a
        # human runs concept add. Expect roughly half the list to be noise.
        read -rp "Scope to one project (blank for the whole corpus): " proj_name
        read -rp "How many to show (default 20): " sug_limit
        sug_limit="${sug_limit:-20}"
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" concept suggest --project "${proj_name}" --limit "${sug_limit}"
        else
            "${kb}" --db "${db_path}" concept suggest --limit "${sug_limit}"
        fi
        echo
        echo "These are candidates, not concepts. Add one with:"
        echo "  ${kb} --db ${db_path} concept add NAME \"DESCRIPTION\""
        ;;

    source)
        read -rp "Source title: " src_title
        if [ -z "${src_title}" ]; then echo "Title is required." >&2; exit 1; fi
        read -rp "DOI (optional): " src_doi
        if [ -n "${src_doi}" ]; then
            "${kb}" --db "${db_path}" source add "${src_title}" --doi "${src_doi}"
        else
            "${kb}" --db "${db_path}" source add "${src_title}"
        fi
        ;;

    record)
        read -rp "Record title: " rec_title
        if [ -z "${rec_title}" ]; then echo "Title is required." >&2; exit 1; fi
        read -rp "Trigger [design/plan-review/implementation/live-test/release-review/request]: " rec_trigger
        if [ -z "${rec_trigger}" ]; then echo "Trigger is required." >&2; exit 1; fi
        read -rp "Project name (blank for a workspace-tier record): " proj_name
        read -rp "Kind [decision/correction/refinement] (default: decision): " rec_kind
        rec_kind="${rec_kind:-decision}"
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" record new --title "${rec_title}" --trigger "${rec_trigger}" --project "${proj_name}" --kind "${rec_kind}"
        else
            "${kb}" --db "${db_path}" record new --title "${rec_title}" --trigger "${rec_trigger}" --workspace --kind "${rec_kind}"
        fi
        echo
        echo "Fill in the body sections, then index it with:"
        echo "  ${kb} --db ${db_path} ingest PATH_TO_DECISIONS_DIR"
        echo "It is written 'proposed'. Only its author promotes it — never do that on their behalf."
        ;;

    document)
        echo "Available projects:"
        "${kb}" --db "${db_path}" project list
        read -rp "Project name: " proj_name
        if [ -z "${proj_name}" ]; then echo "Project name is required." >&2; exit 1; fi
        read -rp "Path to the document (.md, .fountain/.spmd, .txt): " doc_path
        if [ -z "${doc_path}" ]; then echo "Path is required." >&2; exit 1; fi
        if [ ! -f "${doc_path}" ]; then echo "No such file: ${doc_path}" >&2; exit 1; fi
        "${kb}" --db "${db_path}" document ingest "${doc_path}" --project "${proj_name}" --dry-run
        read -rp "Proceed with the ingest? [y/N]: " confirm
        if [ "${confirm}" != "y" ] && [ "${confirm}" != "Y" ]; then
            echo "Cancelled; nothing written."
            exit 0
        fi
        "${kb}" --db "${db_path}" document ingest "${doc_path}" --project "${proj_name}"
        echo "Sections land 'unsummarized' — invisible to search until drafted and promoted."
        ;;

    document-draft)
        read -rp "Project name (blank for every project's queue): " proj_name
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" document review list --project "${proj_name}"
        else
            "${kb}" --db "${db_path}" document review list
        fi
        read -rp "Section ID: " section_id
        if [ -z "${section_id}" ]; then echo "Section ID is required." >&2; exit 1; fi
        read -rp "Summary body: " summary_body
        if [ -z "${summary_body}" ]; then echo "Summary body is required." >&2; exit 1; fi
        read -rp "Written by (human, or a model identifier): " drafted_by
        drafted_by="${drafted_by:-human}"
        read -rp "Confidence 0-1 (optional): " confidence
        if [ -n "${confidence}" ]; then
            "${kb}" --db "${db_path}" document draft "${section_id}" "${summary_body}" --by "${drafted_by}" --confidence "${confidence}"
        else
            "${kb}" --db "${db_path}" document draft "${section_id}" "${summary_body}" --by "${drafted_by}"
        fi
        echo "Drafted, not searchable — promotion is a separate, human action."
        ;;

    document-promote)
        read -rp "Project name (blank for every project's queue): " proj_name
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" document review list --project "${proj_name}" --status drafted
        else
            "${kb}" --db "${db_path}" document review list --status drafted
        fi
        read -rp "Section ID to promote: " section_id
        if [ -z "${section_id}" ]; then echo "Section ID is required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" document review promote "${section_id}"
        ;;

    document-tag)
        # A pure file operation: edits the project's document files, never
        # the database. Always preview first.
        read -rp "Project name: " proj_name
        if [ -z "${proj_name}" ]; then echo "Project name is required." >&2; exit 1; fi
        read -rp "Concepts, comma-separated (blank = density threshold): " concepts
        if [ -n "${concepts}" ]; then
            "${kb}" --db "${db_path}" document tag --project "${proj_name}" --concept "${concepts}" --dry-run
        else
            "${kb}" --db "${db_path}" document tag --project "${proj_name}" --dry-run
        fi
        read -rp "Write these changes to the files? [y/N]: " confirm
        if [ "${confirm}" != "y" ] && [ "${confirm}" != "Y" ]; then
            echo "Cancelled; no file written."
            exit 0
        fi
        if [ -n "${concepts}" ]; then
            "${kb}" --db "${db_path}" document tag --project "${proj_name}" --concept "${concepts}"
        else
            "${kb}" --db "${db_path}" document tag --project "${proj_name}"
        fi
        ;;

    project-rename)
        # Flags precede the names: project rename [--root PATH] [--dry-run] OLD NEW
        echo "Projects:"
        "${kb}" --db "${db_path}" project list
        read -rp "Current name: " old_name
        read -rp "New name: " new_name
        if [ -z "${old_name}" ] || [ -z "${new_name}" ]; then echo "Both names are required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" project rename --dry-run "${old_name}" "${new_name}"
        read -rp "Proceed? This rewrites the project: frontmatter of every owned record. [y/N]: " confirm
        if [ "${confirm}" != "y" ] && [ "${confirm}" != "Y" ]; then
            echo "Cancelled; nothing renamed and no file written."
            exit 0
        fi
        "${kb}" --db "${db_path}" project rename "${old_name}" "${new_name}"
        ;;

    concept-rename)
        echo "Concepts:"
        "${kb}" --db "${db_path}" concept list
        read -rp "Current name: " old_name
        read -rp "New name: " new_name
        if [ -z "${old_name}" ] || [ -z "${new_name}" ]; then echo "Both names are required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" concept rename "${old_name}" "${new_name}"
        ;;

    link-project-concept)
        echo "Projects:"
        "${kb}" --db "${db_path}" project list
        echo "Concepts:"
        "${kb}" --db "${db_path}" concept list
        read -rp "Project name: " proj_name
        read -rp "Concept name: " concept_name
        if [ -z "${proj_name}" ] || [ -z "${concept_name}" ]; then echo "Both names are required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" link project "${proj_name}" "${concept_name}"
        ;;

    link-observation-concept)
        read -rp "Project name (to list its observations): " proj_name
        if [ -n "${proj_name}" ]; then
            "${kb}" --db "${db_path}" observation list --project "${proj_name}"
        fi
        echo "Concepts:"
        "${kb}" --db "${db_path}" concept list
        read -rp "Observation ID: " obs_id
        read -rp "Concept name: " concept_name
        if [ -z "${obs_id}" ] || [ -z "${concept_name}" ]; then echo "Observation ID and concept name are required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" link observation "${obs_id}" "${concept_name}"
        ;;

    link-observation-source)
        read -rp "Observation ID: " obs_id
        read -rp "Source ID: " src_id
        read -rp "Relationship (default: cited): " relationship
        relationship="${relationship:-cited}"
        if [ -z "${obs_id}" ] || [ -z "${src_id}" ]; then echo "Observation ID and Source ID are required." >&2; exit 1; fi
        "${kb}" --db "${db_path}" source link "${obs_id}" "${src_id}" --relationship "${relationship}"
        ;;

    *)
        printf "Unknown action: %s\nValid actions: project, observation, observation-update, concept, concept-suggest, source, record, document, document-draft, document-promote, document-tag, project-rename, concept-rename, link-project-concept, link-observation-concept, link-observation-source\n" "${action}" >&2
        exit 1
        ;;
esac

echo "Done."
