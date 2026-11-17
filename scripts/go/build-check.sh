#!/usr/bin/env bash
# The Go side of `pnpm check`: the wyo server compiles for both platforms it is released for, without C.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

for arch in amd64 arm64; do
    echo "go build linux/$arch"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -o /dev/null ./cmd/wyo
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -tags wyotest "${GO_PACKAGES[@]}"
done
