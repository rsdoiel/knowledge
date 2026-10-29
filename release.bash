#!/bin/bash
# generated with CMTools 0.0.1 7534e43

#
# Release script for knowledge on GitHub using gh cli.
#
# Usage: release.bash [--dry-run] [--nested]
#   --dry-run  do everything up to the prompt (notes, checksums), say what
#              would be committed, pushed, uploaded and tagged, and change nothing.
#   --nested   only the nested module: wait for the root tag to be public, require
#              it in semantic/go.mod, check, and tag semantic/vX.Y.Z. A normal run
#              does this itself after you finalize the draft; --nested resumes an
#              interrupted run.
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

# Now we're ready to push things. This is the only place the release is
# committed and pushed (make release does not), and a draft that already
# exists for the tag stops the run so a second one is never created.
if gh release view "${RELEASE_TAG}" >/dev/null 2>&1; then
    echo "error: a release for ${RELEASE_TAG} already exists on GitHub; delete the draft first"
    rm -f release_notes.tmp
    exit 1
fi
if [ "${DRY_RUN}" = "1" ]; then
    echo "dry run: would commit (if the tree is dirty) and push, then create draft ${RELEASE_TAG} and upload:"
    git status --short
    ls dist/*.zip "dist/${CHECKSUM_FILE}"
    if [ "${HAS_NESTED}" = "1" ]; then
        print_nested_plan
    fi
    rm -f release_notes.tmp
    exit 0
fi
# shellcheck disable=SC2162
read -r -p "Push release to GitHub with gh? (y/N) " YES_NO
if [ "${YES_NO}" = "y" ]; then
	if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
		git commit -am "prep for ${RELEASE_TAG}" || exit 1
	fi
	git push origin "$(git rev-parse --abbrev-ref HEAD)" || exit 1
	# Now generate a draft release
	echo "Pushing release ${RELEASE_TAG} to GitHub"
	gh release create "${RELEASE_TAG}" \
		--draft \
		-F release_notes.tmp \
		--generate-notes
	echo "Uploading distribution files and checksums"
	echo "Starting upload: dist/${CHECKSUM_FILE}
	"
	gh release upload "${RELEASE_TAG}" "dist/${CHECKSUM_FILE}"
	echo "  Completed upload: dist/${CHECKSUM_FILE}"
	for FILE in dist/*.zip; do
		echo "Starting upload: ${FILE}
	"
		gh release upload "${RELEASE_TAG}" "${FILE}"
		echo "  Completed upload: ${FILE}"
	done

	rm release_notes.tmp
	if [ "${HAS_NESTED}" = "1" ]; then
		echo
		echo "Now finalize the draft; this script then tags the nested module (${NESTED_TAG})."
		echo
		echo "	${REPO_URL}/releases"
		echo
		wait_for_publish || exit 1
		release_nested
		exit $?
	fi
	cat <<EOT

Now goto repo release and finalize draft.

	${REPO_URL}/releases

EOT

fi
