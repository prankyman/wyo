# shellcheck shell=bash disable=SC2034 # sourced: the scripts that source it use these settings
# Settings of the runs on the Mac mini M4 (scripts/remote/; AGENTS.md, «The test flow of every plan»). Sourced, never run.
# The Mac's address is not kept in this public repository: scripts/remote/hosts.env (ignored by git) sets MAC_HOST and MAC_PORT;
# hosts.env.example shows its shape.
here_config="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$here_config/hosts.env" ]; then
    # shellcheck source=/dev/null
    . "$here_config/hosts.env"
fi
: "${MAC_HOST:?scripts/remote/hosts.env must set MAC_HOST (user@host of the Mac mini); see hosts.env.example}"
MAC_PORT="${MAC_PORT:-22}"
# Under the Mac's home: the run clone, the reports, Go.
MAC_DIR="wyo-runs/wyo"
MAC_RUNS_DIR="wyo-runs/reports"
MAC_GO_DIR="wyo-runs/go"
REPO_URL="https://github.com/prankyman/wyo.git"

ssh_mac() {
    ssh -p "$MAC_PORT" -o BatchMode=yes -o ConnectTimeout=15 "$MAC_HOST" "$@"
}
scp_from_mac() {
    scp -P "$MAC_PORT" -o BatchMode=yes -q -r "$MAC_HOST:$1" "$2"
}
