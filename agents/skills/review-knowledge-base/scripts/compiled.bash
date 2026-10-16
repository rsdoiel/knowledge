#!/usr/bin/env bash
set -euo pipefail

read -ra _args <<< "${HARVEY_PROMPT:-}"
db_path="${_args[0]:-}"
focus="${_args[1]:-}"

if [ -z "${db_path}" ]; then
    db_path="${DB_PATH:-agents/knowledge.db}"
fi
if [ -z "${focus}" ]; then
    focus="${FOCUS:-all}"
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

# kb's exit status follows the workspace convention (workspace DR-0003; kb v0.0.14
# and later): 0 success; 1 a normal negative answer (no such project,
# a search with no results, a stale index); 2 the command line was wrong; 65 and
# above the data or the environment failed. Earlier kb exited 1 for every failure,
# which is why 1 is never treated as an error here and anything above 1 always is.

if ! command -v sqlite3 >/dev/null 2>&1; then
    echo "sqlite3 not found on PATH — install SQLite3 and retry." >&2
    exit 1
fi

# The root walked by the index check. Derived from the database path so the
# check covers the workspace the report is about, not whichever directory
# this happened to be invoked from.
index_root="$(cd "$(dirname "${db_path}")/.." 2>/dev/null && pwd || echo "${HOME}/Laboratory")"

today="$(date '+%Y-%m-%d')"
printf "## Knowledge Base Report  (%s)\n\n" "${today}"

case "${focus}" in
    all)
        printf "### Projects\n\n"
        "${kb}" --db "${db_path}" project list
        printf "\n### Recent Observations  (last 20)\n\n"
        # LEFT JOIN observation_relations so a corrected observation is
        # labelled rather than reported as current (kb v0.0.8, DR-0023).
        sqlite3 -header -column "${db_path}" \
            "SELECT o.id, p.name AS project, o.kind, o.created_at, CASE WHEN sup.to_id IS NULL THEN '' ELSE 'superseded by ' || sup.from_id END AS status, substr(o.body, 1, 80) AS body_preview FROM observations o JOIN projects p ON p.id = o.project_id LEFT JOIN observation_relations sup ON sup.to_id = o.id AND sup.relationship = 'supersedes' ORDER BY o.created_at DESC LIMIT 20;"
        printf "\n### Concepts\n\n"
        "${kb}" --db "${db_path}" concept list
        printf "\n### Decision Records\n\n"
        "${kb}" --db "${db_path}" record list --all
        printf "\n### Documents\n\n"
        "${kb}" --db "${db_path}" document list
        printf "\n### Review Queue\n\n"
        "${kb}" --db "${db_path}" document review list
        printf "\n### Index Check\n\n"
        # --check never writes. Without it, --all rewrites every index.md it
        # finds, which is not a review action. Exit 1 here means stale, which
        # is the finding, so it must not abort the report. Above 1 the check
        # itself failed (a malformed record, an unreadable file): that is not
        # a finding, so say so, and carry on with the rest of the report.
        # index never takes --db: it walks record files, not the database, and
        # since kb v0.0.13 it refuses one (exit 2), which `|| true` used to hide.
        rc=0
        "${kb}" index "${index_root}" --all --check || rc=$?
        if [ "${rc}" -gt 1 ]; then
            echo "(the index check failed: kb exited ${rc}, see above)" >&2
        fi
        printf "\n### Candidate Concepts  (suggestions, not findings)\n\n"
        rc=0
        "${kb}" --db "${db_path}" concept suggest --limit 10 || rc=$?
        if [ "${rc}" -ne 0 ]; then
            echo "(candidate concepts unavailable: kb exited ${rc}, see above)" >&2
        fi
        ;;

    projects)
        printf "### Projects\n\n"
        "${kb}" --db "${db_path}" project list
        ;;

    observations)
        printf "### Recent Observations  (last 20)\n\n"
        sqlite3 -header -column "${db_path}" \
            "SELECT o.id, p.name AS project, o.kind, o.created_at, CASE WHEN sup.to_id IS NULL THEN '' ELSE 'superseded by ' || sup.from_id END AS status, substr(o.body, 1, 80) AS body_preview FROM observations o JOIN projects p ON p.id = o.project_id LEFT JOIN observation_relations sup ON sup.to_id = o.id AND sup.relationship = 'supersedes' ORDER BY o.created_at DESC LIMIT 20;"
        ;;

    concepts)
        printf "### Concepts\n\n"
        "${kb}" --db "${db_path}" concept list
        ;;

    concept-candidates)
        printf "### Candidate Concepts  (suggestions, not findings)\n\n"
        "${kb}" --db "${db_path}" concept suggest --limit 20
        ;;

    records)
        printf "### Decision Records\n\n"
        "${kb}" --db "${db_path}" record list --all
        ;;

    documents)
        printf "### Documents\n\n"
        "${kb}" --db "${db_path}" document list
        ;;

    review-queue)
        printf "### Review Queue\n\n"
        "${kb}" --db "${db_path}" document review list
        ;;

    index-check)
        printf "### Index Check  (root: %s)\n\n" "${index_root}"
        "${kb}" index "${index_root}" --all --check
        ;;

    search:*)
        term="${focus#search:}"
        printf "### Search results for \"%s\"\n\n" "${term}"
        # kb search exits 1 when it finds nothing: "no results", a normal
        # answer, not a failure. 2 or more is a failure (a bad term, a broken
        # database, a missing workspace) and is passed on, not reported as
        # "no results".
        rc=0
        "${kb}" --db "${db_path}" search "${term}" || rc=$?
        if [ "${rc}" -gt 1 ]; then
            exit "${rc}"
        fi
        ;;

    *)
        printf "## Project: %s\n\n" "${focus}"
        rc=0
        "${kb}" --db "${db_path}" project show "${focus}" || rc=$?
        if [ "${rc}" -eq 1 ]; then
            # 1 is "no such project". Any other failure (an unreadable database,
            # no workspace) is not, and must not be reported as one.
            echo "Project '${focus}' not found. Available projects:"
            "${kb}" --db "${db_path}" project list
            exit 1
        elif [ "${rc}" -ne 0 ]; then
            exit "${rc}"
        fi
        printf "\n### Concepts\n\n"
        "${kb}" --db "${db_path}" project concepts "${focus}"
        printf "\n### Observations\n\n"
        "${kb}" --db "${db_path}" observation list --project "${focus}"
        printf "\n### Decision Records\n\n"
        "${kb}" --db "${db_path}" record list "${focus}"
        printf "\n### Documents\n\n"
        "${kb}" --db "${db_path}" document list --project "${focus}"
        ;;
esac

echo "Done."
