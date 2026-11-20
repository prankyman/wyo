#!/usr/bin/env bash
# Sourced by every script in scripts/go/ and by scripts/generate.sh. Not run on its own.
#  - goes to the repository root;
#  - puts .tools/bin first on PATH, so the pinned Go tools win over any other copy;
#  - makes sure internal/webui/dist/index.html exists, so Go compiles on a fresh clone (dist/ is built, not committed).

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
export PATH="$root/.tools/bin:$PATH"

# The Go packages the checks run over.
GO_PACKAGES=(./cmd/... ./internal/... ./tools/... ./messages/...)
# The packages whose tests run different code in the build with the test tag `wyotest` than in the release build
# (scripts/go/test-tag-packages.txt, kept exact by internal/archtest). Only they run a second time, with the tag.
# A read loop, not mapfile: the Mac mini's bash is 3.2 (scripts/remote/).
TEST_TAG_PACKAGES=()
while IFS= read -r line; do TEST_TAG_PACKAGES+=("$line"); done < <(grep -v -e '^#' -e '^$' "$root/scripts/go/test-tag-packages.txt")
# The same places as folders, for tools that take paths.
GO_FOLDERS=(cmd internal tools messages)

ensure_interface_placeholder() {
    local index="$root/internal/webui/dist/index.html"
    if [ ! -f "$index" ]; then
        mkdir -p "$root/internal/webui/dist"
        printf '%s\n' '<!doctype html><title>wyo</title><p>The interface was not built. Run pnpm build.</p>' >"$index"
    fi
}

require_tool() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "$1 is not installed. Run: pnpm tools:install" >&2
        exit 1
    fi
}

ensure_interface_placeholder
