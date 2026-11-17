#!/usr/bin/env bash
# `pnpm dev`: the fake Wildberries and the fake fulfilment centre, the wyo server in development mode, then Vite on its own port,
# passing /api on to the wyo server. Ctrl+C stops all three.
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

bin/wyofakes --wildberries 127.0.0.1:8791 --fulfilment 127.0.0.1:8792 &
pids+=("$!")
bin/wyo-dev --listen 127.0.0.1:8790 --mode development &
pids+=("$!")

pnpm exec vite dev --host 127.0.0.1 --port 5190 --strictPort
