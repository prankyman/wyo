#!/usr/bin/env bash
# `pnpm remote:gate`: the final run of the committed HEAD on the Mac mini M4 — every check and test the gate of GitHub Actions used
# to run (scripts/remote/mac-gate.sh). AGENTS.md, «The test flow of every plan».
#
# The commit travels as a git bundle over SSH: nothing is pushed for the run. Uncommitted changes of tracked files are refused (the
# run would not test them); untracked files are listed and never carried. On the Mac the run takes the run lock (two runs never share
# the clone), checks the commit out detached in ~/wyo-runs/wyo and runs in a tmux session of its own, so a dropped connection does
# not end it; it is stopped after 55 minutes. The report comes back into .remote-runs/<stamp>/ (ignored by git). Exit code 0 only
# when the run passed; GitHub Actions does not check it, so only a commit this run passed is pushed.
set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
# shellcheck source=scripts/remote/config.sh
source "$here/config.sh"
cd "$repo" || exit 1

untracked="$(git status --porcelain --untracked-files=normal | grep '^??' || true)"
[ -n "$untracked" ] && printf 'Not carried to the Mac (untracked):\n%s\n' "$untracked" >&2
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    echo "Uncommitted changes: the run carries a commit, so commit first (git commit --no-verify, later --amend)." >&2
    exit 2
fi

sha="$(git rev-parse HEAD)"
stamp="$(date +%Y%m%d-%H%M%S)"
report="$MAC_RUNS_DIR/$stamp"
session="wyo-run-$stamp"
git fetch -q origin main || { echo "git fetch origin main failed" >&2; exit 1; }

work="$(mktemp -d "${TMPDIR:-/tmp}/wyo-gate.XXXXXX")"
trap 'rm -rf "$work"' EXIT
# The commits the Mac's clone cannot fetch from GitHub: those of HEAD that main lacks. None when HEAD is on main already.
if [ -n "$(git rev-list -n 1 HEAD ^origin/main)" ]; then
    git bundle create -q "$work/commit.bundle" HEAD ^origin/main || { echo "git bundle failed" >&2; exit 1; }
else
    : > "$work/commit.bundle"
fi

cat > "$work/bootstrap.sh" <<'REMOTE'
set -uo pipefail
dir="$HOME/$1"; report="$HOME/$2"; sha="$3"
top="$(dirname "$dir")"
cd "$dir" || { echo "No run clone at $dir on the Mac: run \`pnpm remote:setup\` once." | tee "$report/summary.md"; exit 1; }
git fetch -q origin main || { echo "git fetch origin main failed" | tee "$report/summary.md"; exit 1; }
if [ -s "$report/commit.bundle" ]; then
    git fetch -q "$report/commit.bundle" HEAD || { echo "git fetch of the bundle failed" | tee "$report/summary.md"; exit 1; }
fi
# The clone is the runs' own: whatever a run or a hand left in it is dropped (-f, then git clean), so the commit is tested as
# committed.
git checkout -q -f --detach "$sha" || { echo "checkout of $sha failed" | tee "$report/summary.md"; exit 1; }
rm -f "$report/commit.bundle"
git clean -q -fdx -e node_modules -e .tools
# The run in a process group of its own (set -m), ended with everything it started after 55 minutes, so a run whose connection
# dropped never holds the lock for ever.
set -m
/bin/bash scripts/remote/mac-gate.sh "$report" "$top" &
run=$!
( sleep 3300; kill -TERM -- "-$run" 2>/dev/null; echo "Stopped after 55 minutes." >> "$report/summary.md" ) > /dev/null 2>&1 < /dev/null &
watchdog=$!
disown "$watchdog" 2>/dev/null
wait "$run"
code=$?
kill -TERM -- "-$watchdog" 2>/dev/null
exit $code
REMOTE

echo "Mac run $stamp of $(git log -1 --format='%h %s')"
ssh_mac "mkdir -p '$report'" || exit 1
ssh_mac "cat > '$report/commit.bundle'" < "$work/commit.bundle" || exit 1
ssh_mac "cat > '$report/bootstrap.sh'" < "$work/bootstrap.sh" || exit 1
printf '%s\n' "/usr/bin/lockf -t 1800 \$HOME/wyo-runs/.lock /bin/bash '$report/bootstrap.sh' '$MAC_DIR' '$report' '$sha' > '$report/run.log' 2>&1" \
    'echo $? > '"'$report/exit-code'" | ssh_mac "cat > '$report/launch.sh'" || exit 1
ssh_mac "/opt/homebrew/bin/tmux new-session -d -s '$session' '/bin/bash $report/launch.sh'" || { echo "the tmux session did not start" >&2; exit 1; }
echo "It runs in tmux session $session on the Mac; its report folder is ~/$report."

# The run does not depend on this connection: a poll that cannot reach the Mac is tried again.
waited=0
code=124
while [ "$waited" -lt 3700 ]; do
    sleep 20
    waited=$((waited + 20))
    answer="$(ssh_mac "cat '$report/exit-code' 2>/dev/null || echo running" 2>/dev/null)" || continue
    if [ "$answer" != running ]; then
        code="$answer"
        break
    fi
done
[ "$code" = 124 ] && echo "Still running after an hour: tmux session $session on the Mac was left alone." >&2

mkdir -p .remote-runs
if scp_from_mac "$report" ".remote-runs/"; then
    echo
    cat ".remote-runs/$stamp/summary.md" 2>/dev/null || cat ".remote-runs/$stamp/run.log"
    echo
    echo "Report: $repo/.remote-runs/$stamp/"
else
    echo "Could not copy the report back from ~/$report on the Mac." >&2
fi

if [ "$code" = 0 ]; then
    echo "PASSED: $sha. This very commit may be pushed."
fi
exit "$code"
