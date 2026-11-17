#!/usr/bin/env bash
# The Go side of `pnpm lint`: gofumpt prints nothing, golangci-lint passes, go vet passes.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require_tool gofumpt
require_tool golangci-lint

unformatted="$(gofumpt -l "${GO_FOLDERS[@]}")"
if [ -n "$unformatted" ]; then
    echo "These files are not formatted (run: pnpm format):" >&2
    echo "$unformatted" >&2
    exit 1
fi

golangci-lint run "${GO_PACKAGES[@]}"
# The files that exist only in a build made with the wyotest tag.
golangci-lint run --build-tags wyotest "${GO_PACKAGES[@]}"
go vet "${GO_PACKAGES[@]}"
# The same code with the development and test build tag.
go vet -tags wyotest "${GO_PACKAGES[@]}"
