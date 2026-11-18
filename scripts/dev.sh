#!/usr/bin/env bash
# `pnpm dev`: the fake marketplaces (the fake Wildberries and the fake Ozon, on one address) and the fake fulfilment centre, the
# wyo server in development mode, then Vite on its own port, passing /api on to the wyo server. Ctrl+C stops all three.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/go/common.sh"

pids=()
stop_all() {
    for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
    wait 2>/dev/null || true
}
trap stop_all EXIT INT TERM

mkdir -p bin
go build -tags wyotest -o bin/wyofakes ./cmd/wyofakes
go build -tags wyotest -o bin/wyo-dev ./cmd/wyo

# The development data folder (not committed). The first start makes it, with the owner `owner` and the password
# `development-password`; later starts keep what is there.
data=.dev-data
if [ ! -f "$data/wyo.db" ]; then
    bin/wyo-dev init --data "$data"
    bin/wyo-dev owner create --data "$data" --login owner --password-file <(printf '%s\n' development-password)
fi

bin/wyofakes --marketplaces 127.0.0.1:8791 --fulfilment 127.0.0.1:8792 &
pids+=("$!")
# The wyo server asks the fake marketplaces (the fake Wildberries and the fake Ozon, on one address) instead of the real ones
# (rule R3).
WYO_MARKETPLACE_URL=http://127.0.0.1:8791 bin/wyo-dev --listen 127.0.0.1:8790 --mode development --data "$data" &
pids+=("$!")

pnpm exec vite dev --host 127.0.0.1 --port 5190 --strictPort
