#!/usr/bin/env bash
# `pnpm test:coverage`, Go side: runs every test with the race detector and writes two coverage profiles, one for the
# release build and one for the build with the wyotest tag. tools/covercheck reads both.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

mkdir -p coverage
go test -race -shuffle=on -count=1 -coverpkg=./... -coverprofile=coverage/go.out "${GO_PACKAGES[@]}"
go test -race -shuffle=on -count=1 -tags wyotest -coverpkg=./... -coverprofile=coverage/go-wyotest.out "${GO_PACKAGES[@]}"
