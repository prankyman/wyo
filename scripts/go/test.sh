#!/usr/bin/env bash
# `pnpm go:test [flags] [package ...]`: the tests of the packages given (paths such as ./internal/config), or of every
# Go package. Every package is tested as the release build; then the packages whose code differs between the builds
# (TEST_TAG_PACKAGES of common.sh) are tested again with the tag wyotest. Every other package would run the very same
# code again, so it runs once. Flags go to `go test` before the packages: to run one test many times in a row,
# `pnpm go:test -run '^TestName$' -count=50 ./internal/exchange`.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

flags=()
packages=()
for arg in "$@"; do
    case "$arg" in
        ./*) packages+=("$arg") ;;
        *) flags+=("$arg") ;;
    esac
done
if [ "${#packages[@]}" -eq 0 ]; then
    packages=("${GO_PACKAGES[@]}")
fi

# ${flags[@]+…}: bash 3.2 (the Mac mini's) calls an empty array unbound under set -u.
go test -race -shuffle=on -count=1 ${flags[@]+"${flags[@]}"} "${packages[@]}"

tagged=()
while IFS= read -r line; do tagged+=("$line"); done < <(grep -F -x -f <(go list "${TEST_TAG_PACKAGES[@]}") <(go list "${packages[@]}") || true)
if [ "${#tagged[@]}" -eq 0 ]; then
    echo "The code of these packages is the same in the build with the wyotest tag: one run is enough."
    exit 0
fi
go test -race -shuffle=on -count=1 -tags wyotest ${flags[@]+"${flags[@]}"} "${tagged[@]}"
