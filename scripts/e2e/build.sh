#!/usr/bin/env bash
# The build step of `pnpm test:e2e` (Playwright's global setup runs it once before the first test): the wyo interface, the wyo
# server and the fakes, as the browser tests need them.
#  - the interface is built with `vite build` into internal/webui/dist/ UNLESS a real build is there and no source file is newer
#    than it (the placeholder that scripts/go/common.sh writes on a fresh clone is not a build);
#  - bin/wyo-test is the wyo server built with the wyotest tag (the showcase page and the test mode exist only in such a build);
#    it is built after the interface because it carries the interface's files inside itself. Its version reads 0.0.0-e2e, so no
#    page can show it by accident;
#  - bin/wyofakes is the fake Wildberries and the fake fulfilment centre in one program.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/../go/common.sh"

index="internal/webui/dist/index.html"

interface_is_fresh() {
    [ -f "$index" ] || return 1
    ! grep -q 'The interface was not built' "$index" || return 1
    # Any source of the interface newer than the build makes it stale. Test files are not part of the build.
    local newer
    newer="$(find src static messages svelte.config.js vite.config.ts package.json pnpm-lock.yaml api/wyo.yaml \
        -type f -newer "$index" ! -name '*.test.ts' 2>/dev/null | head -1)"
    [ -z "$newer" ]
}

if interface_is_fresh; then
    echo "e2e build: the interface in internal/webui/dist is up to date"
else
    echo "e2e build: building the interface"
    pnpm exec vite build
fi

mkdir -p bin
CGO_ENABLED=0 go build -trimpath -tags wyotest \
    -ldflags "-X github.com/prankyman/wyo/internal/version.Version=0.0.0-e2e" \
    -o bin/wyo-test ./cmd/wyo
CGO_ENABLED=0 go build -trimpath -tags wyotest -o bin/wyofakes ./cmd/wyofakes
echo "e2e build: bin/wyo-test and bin/wyofakes are ready"
