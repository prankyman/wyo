#!/usr/bin/env bash
# `pnpm remote:setup`: prepares the Mac mini M4 for the final runs (scripts/remote/gate.sh). Safe to run again.
#
#   1. ~/wyo-runs/ with the run clone ~/wyo-runs/wyo (from GitHub over HTTPS: the repository is public, no key on the Mac) and the
#      report folder ~/wyo-runs/reports.
#   2. Go at the version of go.mod's `toolchain` line, the official archive from go.dev with its SHA-256 checked against go.dev's
#      own list, in ~/wyo-runs/go.
#   3. pnpm at package.json's `packageManager` version through corepack, its links in ~/wyo-runs/bin; Node is the Mac's nvm Node of
#      .nvmrc.
#   4. OpenSSL 3 (Homebrew's openssl@3, installed beside the Mac's own and never linked over it): internal/release tests that a
#      signature OpenSSL 3 makes — the release pipeline's and the installer's — is accepted; the runs put it first on their PATH.
#   5. The packages and Chromium for the browser tests. The pinned Go tools are installed by each run (`pnpm tools:install` of the
#      commit it tests, which skips what is installed already).
# Nothing outside ~/wyo-runs is changed. The Mac's own bash is 3.2: the script it runs avoids what 3.2 lacks.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/remote/config.sh
source "$here/config.sh"

toolchain="$(awk '$1 == "toolchain" { print $2 }' "$here/../../go.mod")"
[ -n "$toolchain" ] || { echo "go.mod has no toolchain line" >&2; exit 1; }

ssh_mac /bin/bash -s -- "$MAC_DIR" "$MAC_RUNS_DIR" "$MAC_GO_DIR" "$REPO_URL" "$toolchain" <<'REMOTE'
set -euo pipefail
dir="$HOME/$1"; runs="$HOME/$2"; godir="$HOME/$3"; url="$4"; toolchain="$5"
top="$(dirname "$dir")"
mkdir -p "$runs" "$top/bin"

if [ "$("$godir/bin/go" env GOVERSION 2>/dev/null || true)" != "$toolchain" ]; then
    file="$toolchain.darwin-arm64.tar.gz"
    tmp="$(mktemp -d)"
    fetch() { curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --speed-limit 10000 --speed-time 30 "$@"; }
    fetch -o "$tmp/$file" "https://go.dev/dl/$file"
    want="$(fetch 'https://go.dev/dl/?mode=json&include=all' | /usr/bin/python3 -c '
import json, sys
name = sys.argv[1]
for release in json.load(sys.stdin):
    for f in release["files"]:
        if f["filename"] == name:
            print(f["sha256"])
' "$file")"
    [ -n "$want" ] || { echo "go.dev lists no checksum for $file" >&2; exit 1; }
    echo "$want  $tmp/$file" | shasum -a 256 -c - >/dev/null
    rm -rf "$godir"
    tar -xzf "$tmp/$file" -C "$tmp"
    mv "$tmp/go" "$godir"
    rm -rf "$tmp"
fi

[ -x /opt/homebrew/opt/openssl@3/bin/openssl ] || /opt/homebrew/bin/brew install --quiet openssl@3

export NVM_DIR="$HOME/.nvm"
# shellcheck disable=SC1091
. "$NVM_DIR/nvm.sh" >/dev/null
[ -d "$dir/.git" ] || git clone -q "$url" "$dir"
cd "$dir"
git fetch -q origin main && git checkout -q --detach origin/main
nvm use -s "$(cat .nvmrc)" >/dev/null
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
corepack enable --install-directory "$top/bin" pnpm
export PATH="$top/bin:$godir/bin:/opt/homebrew/bin:$PATH" GOTOOLCHAIN=local
pnpm install --frozen-lockfile > /dev/null
pnpm exec playwright install chromium > /dev/null
echo "Run clone ready: $dir at $(git log -1 --format='%h %s')"
go version
printf 'node %s, pnpm %s, %s\n' "$(node --version)" "$(pnpm --version)" "$(git --version)"
REMOTE
