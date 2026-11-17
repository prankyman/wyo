#!/usr/bin/env bash
# `pnpm format`, Go side: gofumpt rewrites the files.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

require_tool gofumpt
gofumpt -w "${GO_FOLDERS[@]}"
