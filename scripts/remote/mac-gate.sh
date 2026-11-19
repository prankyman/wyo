#!/bin/bash
# Runs on the Mac mini M4, from the commit scripts/remote/gate.sh just checked out in ~/wyo-runs/wyo, under its run lock; never
# started by hand. Written for the Mac's /bin/bash 3.2.
#
#   mac-gate.sh <report dir> <tools dir>
#
# The final run, the same steps as the gate of GitHub Actions used to run, stopping at the first failure:
#   1. pnpm install --frozen-lockfile, then the pinned Go tools (`pnpm tools:install`)
#   2. the commit messages of the commits not on main (commitlint)
#   3. `pnpm precommit`: generate check, lint, type checks, the tests with coverage, the coverage floors, the build
#   4. `pnpm test:e2e`: the whole browser suite
#   5. `pnpm go:measure`: the wyo server's memory with three accounts and 10 000 orders
# Each step's output is in <report dir>/<step>.log; <report dir>/summary.md says what passed and how long each step took.
set -uo pipefail

report="$1"
top="$2"
mkdir -p "$report"
summary="$report/summary.md"

export NVM_DIR="$HOME/.nvm"
# shellcheck disable=SC1091
. "$NVM_DIR/nvm.sh" >/dev/null
nvm use -s "$(cat .nvmrc)" >/dev/null
# OpenSSL 3 first (pnpm remote:setup installs it): the release tests need the OpenSSL the pipeline signs with.
export PATH="/opt/homebrew/opt/openssl@3/bin:$top/bin:$top/go/bin:/opt/homebrew/bin:$PATH" GOTOOLCHAIN=local COREPACK_ENABLE_DOWNLOAD_PROMPT=0

say() { echo "$*" | tee -a "$summary"; }
now() { date +%s; }
started="$(now)"

# step <name> <command…>: runs one step into <name>.log; on a failure the summary gets the log's last lines and the run ends.
step() {
    local name="$1" begun
    shift
    begun="$(now)"
    if "$@" > "$report/$name.log" 2>&1; then
        say "- PASSED $name ($(( $(now) - begun )) s)"
        return 0
    fi
    say "- FAILED $name ($(( $(now) - begun )) s), the end of $name.log:"
    say '```'
    tail -60 "$report/$name.log" | tee -a "$summary"
    say '```'
    finish 1
}

finish() {
    [ -d playwright-report ] && cp -R playwright-report "$report/" 2>/dev/null
    [ -d test-results ] && cp -R test-results "$report/" 2>/dev/null
    if [ "$1" = 0 ]; then
        say "PASSED in $(( $(now) - started )) s"
    else
        say "FAILED after $(( $(now) - started )) s"
    fi
    exit "$1"
}

say "# The final run of $(git log -1 --format='%h %s')"
say ""
say "On $(hostname -s), $(sysctl -n hw.ncpu) cores; $(go version | cut -d' ' -f3), node $(node --version), pnpm $(pnpm --version), $(openssl version | cut -d' ' -f1-2)."
say ""

step install pnpm install --frozen-lockfile
step tools pnpm tools:install
if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
    step commit-messages pnpm exec commitlint --from origin/main --to HEAD --verbose
fi
step precommit pnpm precommit
step browser-tests pnpm test:e2e
step memory pnpm go:measure
finish 0
