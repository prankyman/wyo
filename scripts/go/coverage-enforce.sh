#!/usr/bin/env bash
# `pnpm coverage:enforce`, Go side: the whole and every file must reach the floor written in tools/covercheck.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

go run ./tools/covercheck coverage/go.out coverage/go-wyotest.out
