#!/usr/bin/env bash
# `pnpm go:test [package]`: the tests of one package (a path such as ./internal/config), or of every Go package.
# Each package is tested twice, as a release (no tag) and as a development and test build (tag wyotest), because
# some behaviour exists in only one of them.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

if [ "$#" -gt 0 ]; then
    packages=("$@")
else
    packages=("${GO_PACKAGES[@]}")
fi

go test -race -shuffle=on -count=1 "${packages[@]}"
go test -race -shuffle=on -count=1 -tags wyotest "${packages[@]}"
