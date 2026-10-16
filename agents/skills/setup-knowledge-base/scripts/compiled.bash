#!/usr/bin/env bash
set -euo pipefail

read -ra _args <<< "${HARVEY_PROMPT:-}"
db_path="${_args[0]:-}"

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

"${kb}" --db "${db_path}" project list

echo "Database ready: ${db_path}"
