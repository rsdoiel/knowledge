#!/bin/bash
# generated with CMTools 0.0.1 7534e43

#
# Release script for knowledge on GitHub using gh cli.
#
# Usage: release.bash [--dry-run] [--nested]
#   --dry-run  do everything up to the prompt (notes, checksums), say what
#              would be created, uploaded and tagged, and change nothing.
#   --nested   only the nested module: wait for the root tag to be public, require
#              it in semantic/go.mod, check, and tag semantic/vX.Y.Z. A normal run
#              does this itself after you finalize the draft; --nested resumes an
#              interrupted run.
#
# The release is made from the commit HEAD (workspace DR-0012). The script commits
# and pushes nothing for the root release: it stops unless the tree is committed and
# HEAD is pushed (make check-committed), and creates the draft with --target HEAD.
#
# An interrupted run is resumed by running the script again (workspace DR-0013).
# If the draft for the tag exists, targets HEAD and is still a draft, nothing is
# created again: each file in dist/ is skipped when the draft already holds it
# (same SHA-256), uploaded when missing, and uploaded again with --clobber when
# the draft holds a different or unfinished copy. A published release, or a draft
# for another commit, stops the script. Over SSH, run it under tmux or nohup so a
# dropped connection does not interrupt the upload.
#
# The nested module (semantic/) carries the same version as the root. The root tag
# is made by GitHub when the draft is finalized, so the nested tag has to wait for
# it: the script pushes nothing between creating the draft and your finalizing it.
DRY_RUN=0
NESTED_ONLY=0
for ARG in "$@"; do
    case "${ARG}" in
        -n|--dry-run) DRY_RUN=1 ;;
        --nested) NESTED_ONLY=1 ;;
        -h|--help) sed -n '2,6p;/^# Usage/,/^REPO_ID/p' "$0" | grep '^#'; exit 0 ;;
        *) echo "usage: release.bash [--dry-run] [--nested]" >&2; exit 2 ;;
    esac
done

# The notes file is removed however the script ends, even if the terminal is lost.
trap 'rm -f release_notes.tmp' EXIT

REPO_ID="${PWD##*/}"
GROUP_ID="$(git config --get remote.origin.url | sed -E 's#.*github\.com[:/]([^/]+)/.*#\1#')"
REPO_URL="https://github.com/${GROUP_ID}/${REPO_ID}"
echo "REPO_URL -> ${REPO_URL}"

#
# Generate a new draft release using jq and gh
#
RELEASE_TAG="v$(jq -r .version codemeta.json)"
if ! printf '%s' "${RELEASE_TAG}" | grep -qE '^v[0-9a-zA-Z._-]+$'; then
    echo "error: version contains unexpected characters: ${RELEASE_TAG}"
    exit 1
fi

#
# The nested module, semantic/, is tagged semantic/${RELEASE_TAG} once the root tag
# is public: its go.mod must require a root version the Go proxy can serve.
#
NESTED_TAG="semantic/${RELEASE_TAG}"
HAS_NESTED=0
[ -f semantic/go.mod ] && HAS_NESTED=1
ROOT_MODULE="$(sed -n 's/^module //p' go.mod | head -1)"
NESTED_MODULE="$(sed -n 's/^module //p' semantic/go.mod 2>/dev/null | head -1)"

# on_origin TAG: succeeds when TAG exists on origin.
on_origin() {
    git ls-remote --exit-code --tags origin "refs/tags/$1" >/dev/null 2>&1
}

# wait_for_publish: waits until the draft for ${RELEASE_TAG} is published, which is
# when GitHub makes the tag. Ctrl-C leaves everything as it is.
wait_for_publish() {
    echo "Waiting for you to finalize the draft: ${REPO_URL}/releases"
    echo "(Ctrl-C stops here; ./release.bash --nested carries on afterwards.)"
    while :; do
        DRAFT="$(gh release view "${RELEASE_TAG}" --json isDraft -q .isDraft 2>/dev/null)"
        if [ "${DRAFT}" = "false" ] && on_origin "${RELEASE_TAG}"; then
            break
        fi
        sleep "${POLL_SECONDS:-15}"
    done
    echo "${RELEASE_TAG} is published."
}

# wait_for_proxy MODULE VERSION: waits until the Go proxy serves MODULE@VERSION.
wait_for_proxy() {
    echo "Waiting for the Go proxy to serve $1@$2"
    TRIES="${PROXY_TRIES:-60}"
    while [ "${TRIES}" -gt 0 ]; do
        if (cd semantic && GOWORK=off go list -m "$1@$2" >/dev/null 2>&1); then
            echo "  served."
            return 0
        fi
        TRIES=$((TRIES - 1))
        sleep "${PROXY_SECONDS:-15}"
    done
    echo "error: the Go proxy does not serve $1@$2 yet; run ./release.bash --nested later"
    return 1
}

# print_nested_plan: says what the nested release does, for --dry-run.
print_nested_plan() {
    echo "dry run: after you finalize the draft it would wait for the Go proxy, require"
    echo "         ${ROOT_MODULE}@${RELEASE_TAG} in semantic/go.mod, go mod tidy with GOWORK=off,"
    echo "         commit and push that, run make check-go-version and make check-semantic-tag,"
    echo "         then ask before tagging ${NESTED_TAG} and pushing that one tag."
}

# release_nested: require the published root version in semantic/go.mod, check it
# from a clean clone, then tag ${NESTED_TAG} and push that one tag.
release_nested() {
    on_origin "${RELEASE_TAG}" || {
        echo "error: ${RELEASE_TAG} is not on origin: finalize the draft first"
        return 1
    }
    if on_origin "${NESTED_TAG}"; then
        echo "error: ${NESTED_TAG} is already on origin"
        return 1
    fi
    if [ "${DRY_RUN}" = "1" ]; then
        print_nested_plan
        return 0
    fi
    if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
        echo "error: tracked files have changes; commit or discard them first"
        return 1
    fi
    BRANCH="$(git rev-parse --abbrev-ref HEAD)"
    git pull --ff-only origin "${BRANCH}" || return 1
    wait_for_proxy "${ROOT_MODULE}" "${RELEASE_TAG}" || return 1
    (cd semantic && GOWORK=off go get "${ROOT_MODULE}@${RELEASE_TAG}" && GOWORK=off go mod tidy) || return 1
    if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
        git add semantic/go.mod semantic/go.sum || return 1
        git commit -m "semantic: require ${ROOT_MODULE} ${RELEASE_TAG}" || return 1
        git push origin "${BRANCH}" || return 1
    fi
    make check-go-version || return 1
    make check-semantic-tag || return 1
    # shellcheck disable=SC2162
    read -r -p "Tag ${NESTED_TAG} and push it? (y/N) " YES_NO
    if [ "${YES_NO}" != "y" ]; then
        echo "Not tagged. ./release.bash --nested does this later."
        return 0
    fi
    git tag "${NESTED_TAG}" || return 1
    git push origin "${NESTED_TAG}" || return 1
    wait_for_proxy "${NESTED_MODULE}" "${RELEASE_TAG}" || return 1
    echo "Installable: go install ${NESTED_MODULE}/cmd/kb@${RELEASE_TAG}"
}

if [ "${NESTED_ONLY}" = "1" ]; then
    if [ "${HAS_NESTED}" != "1" ]; then
        echo "error: there is no semantic/ module here"
        exit 1
    fi
    release_nested
    exit $?
fi

echo "tag: ${RELEASE_TAG}, notes:"
jq -r .releaseNotes codemeta.json >release_notes.tmp
cat release_notes.tmp

#
# Generate checksums for distribution zip files
#
CHECKSUM_FILE="${REPO_ID}-${RELEASE_TAG}-checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum dist/*.zip | sed 's|dist/||' > "dist/${CHECKSUM_FILE}"
else
    shasum -a 256 dist/*.zip | sed 's|dist/||' > "dist/${CHECKSUM_FILE}"
fi
echo "Checksums written to dist/${CHECKSUM_FILE}"

# The release is built from one known commit. Stop unless it is committed and
# pushed.
if ! make -s check-committed; then
    exit 1
fi
RELEASE_COMMIT="$(git rev-parse HEAD)"

# digest prints the SHA-256 of a file.
digest() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d' ' -f1
    else
        shasum -a 256 "$1" | cut -d' ' -f1
    fi
}

# A release that already exists is resumed only if it is a draft for this commit
# (DR-0013); anything else stops the run, so a second release is never created
# and a published one is never touched.
RESUME=0
VIEW=""
if VIEW="$(gh release view "${RELEASE_TAG}" --json isDraft,targetCommitish,assets 2>/dev/null)"; then
    if [ "$(printf '%s' "${VIEW}" | jq -r .isDraft)" != "true" ]; then
        echo "error: ${RELEASE_TAG} is not a draft on GitHub; it is published and is not touched"
        if [ "${HAS_NESTED}" = "1" ]; then
            echo "       (if ${NESTED_TAG} is not tagged yet, run ./release.bash --nested)"
        fi
        exit 1
    fi
    DRAFT_TARGET="$(printf '%s' "${VIEW}" | jq -r .targetCommitish)"
    if [ "${DRAFT_TARGET}" != "${RELEASE_COMMIT}" ]; then
        echo "error: the draft for ${RELEASE_TAG} targets ${DRAFT_TARGET}, not HEAD ${RELEASE_COMMIT}; delete the draft first"
        exit 1
    fi
    RESUME=1
fi

# plan_for FILE prints what to do with one file: upload, replace or skip.
plan_for() {
    if [ "${RESUME}" != "1" ]; then
        echo upload
        return
    fi
    printf '%s' "${VIEW}" | jq -r --arg n "$(basename "$1")" --arg d "sha256:$(digest "$1")" --argjson s "$(wc -c <"$1" | tr -d ' ')" '
        ([.assets[]? | select(.name == $n)] | .[0]) as $a
        | if $a == null then "upload"
          elif $a.state != "uploaded" then "replace"
          elif ($a.digest // "") != "" then (if $a.digest == $d then "skip" else "replace" end)
          elif $a.size == $s then "skip"
          else "replace" end'
}

# The checksums file first, then the zips.
PLAN=()
TODO_COUNT=0
FILE_COUNT=0
for FILE in "dist/${CHECKSUM_FILE}" dist/*.zip; do
    ACTION="$(plan_for "${FILE}")"
    PLAN+=("${ACTION} ${FILE}")
    FILE_COUNT=$((FILE_COUNT + 1))
    [ "${ACTION}" = "skip" ] || TODO_COUNT=$((TODO_COUNT + 1))
done

if [ "${RESUME}" = "1" ]; then
    echo "Resume: the draft ${RELEASE_TAG} exists and targets ${RELEASE_COMMIT}; ${TODO_COUNT} of ${FILE_COUNT} files to upload"
else
    echo "New release: draft ${RELEASE_TAG} at ${RELEASE_COMMIT}; ${FILE_COUNT} files to upload"
fi
for LINE in "${PLAN[@]}"; do
    echo "  ${LINE%% *}: ${LINE#* }"
done

# finish says what comes after the upload: finalize the draft and, with a nested
# module, wait for it to be published and tag the nested module.
finish() {
    rm -f release_notes.tmp
    if [ "${HAS_NESTED}" = "1" ]; then
        echo
        echo "Now finalize the draft; this script then tags the nested module (${NESTED_TAG})."
        echo
        echo "	${REPO_URL}/releases"
        echo
        wait_for_publish || return 1
        release_nested
        return $?
    fi
    cat <<EOT

Now goto repo release and finalize draft.

	${REPO_URL}/releases

EOT
}

if [ "${DRY_RUN}" = "1" ]; then
    if [ "${HAS_NESTED}" = "1" ]; then
        print_nested_plan
    fi
    echo "dry run: nothing changed"
    exit 0
fi
if [ "${RESUME}" = "1" ] && [ "${TODO_COUNT}" = "0" ]; then
    echo "Nothing to upload: the draft holds every file."
    finish
    exit $?
fi
if [ "${RESUME}" = "1" ]; then
    PROMPT="Resume the draft release on GitHub with gh? (y/N) "
else
    PROMPT="Create the draft release on GitHub with gh? (y/N) "
fi
# shellcheck disable=SC2162
read -r -p "${PROMPT}" YES_NO
if [ "${YES_NO}" = "y" ]; then
    if [ "${RESUME}" != "1" ]; then
        echo "Pushing release ${RELEASE_TAG} to GitHub"
        gh release create "${RELEASE_TAG}" \
            --draft \
            --target "${RELEASE_COMMIT}" \
            -F release_notes.tmp \
            --generate-notes || exit 1
    fi
    echo "Uploading distribution files and checksums"
    for LINE in "${PLAN[@]}"; do
        ACTION="${LINE%% *}"
        FILE="${LINE#* }"
        case "${ACTION}" in
            skip)
                echo "  Skipped (already uploaded): ${FILE}"
                continue
                ;;
            replace) CLOBBER="--clobber" ;;
            *) CLOBBER="" ;;
        esac
        echo "  Starting upload: ${FILE}"
        # shellcheck disable=SC2086
        if ! gh release upload "${RELEASE_TAG}" "${FILE}" ${CLOBBER}; then
            echo "error: upload of ${FILE} failed; run release.bash again to resume"
            exit 1
        fi
        echo "  Completed upload: ${FILE}"
    done
    finish
    exit $?
fi
