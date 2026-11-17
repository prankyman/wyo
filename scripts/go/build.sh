#!/usr/bin/env bash
# `pnpm build`, Go side: bin/wyo with the version of package.json stamped in. Run after `vite build`, which writes the
# interface into internal/webui/dist/. CGO is off, so the file runs on any Linux of its processor.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

version="$(node -p "require('./package.json').version")"
if [ -z "$version" ]; then
    echo "package.json has no version" >&2
    exit 1
fi

mkdir -p bin
CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/prankyman/wyo/internal/version.Version=$version" \
    -o bin/wyo ./cmd/wyo
echo "built bin/wyo $version"
