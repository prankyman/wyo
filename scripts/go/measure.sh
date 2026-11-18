#!/usr/bin/env bash
# `pnpm go:measure`: the memory of the wyo server with three accounts and 10 000 orders in its database, reading the fake
# Wildberries and the fake Ozon at the synchronisation's periods for a made-up day (plans 3 and 5, section H; the test
# TestTheMemoryOfADayWithThreeAccountsAndTenThousandOrders of internal/server, built only with the tag wyomeasure).
# It prints the line that starts with "wyo-memory:" and, in GitHub Actions, adds it to the run's summary. No race
# detector: it would change what is measured.
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

output=$(mktemp)
trap 'rm -f "$output"' EXIT

go test -count=1 -v -tags 'wyotest wyomeasure' -run '^TestTheMemoryOfADayWithThreeAccountsAndTenThousandOrders$' ./internal/server >"$output" 2>&1 || {
    cat "$output"
    exit 1
}

line=$(grep -o 'wyo-memory: .*' "$output")
echo "$line"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '### Memory of the wyo server for a made-up day\n\n%s\n' "$line" >>"$GITHUB_STEP_SUMMARY"
fi
