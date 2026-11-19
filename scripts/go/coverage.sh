#!/usr/bin/env bash
# `pnpm test:coverage`, Go side: runs every test with the race detector as the release build, then again with the
# wyotest tag only the packages whose code differs between the two builds (TEST_TAG_PACKAGES of common.sh); every other
# package would run the very same code again. Two coverage profiles, one per build; tools/covercheck reads both.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

mkdir -p coverage
go test -race -shuffle=on -count=1 -coverpkg=./... -coverprofile=coverage/go.out "${GO_PACKAGES[@]}"
go test -race -shuffle=on -count=1 -tags wyotest -coverpkg=./... -coverprofile=coverage/go-wyotest.out "${TEST_TAG_PACKAGES[@]}"
