#!/usr/bin/env bash
# `pnpm generate` makes every generated file; `pnpm generate:check` (--check) makes them in a temporary folder and fails
# when a committed generated file differs from a fresh one.
#
# Generated:
#   internal/httpapi/gen/                              from api/wyo.yaml (oapi-codegen)
#   internal/fulfilment/fcapi/                         from api/fulfilment-centre.yaml (oapi-codegen)
#   internal/marketplace/wildberries/wbapi/<package>/  from api/wildberries/*.yaml (oapi-codegen)
#   src/lib/api/wyo-api.d.ts                           from api/wyo.yaml (openapi-typescript)
set -euo pipefail
# shellcheck source=scripts/go/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/go/common.sh"

check=false
case "${1:-}" in
"") ;;
--check) check=true ;;
*)
    echo "usage: scripts/generate.sh [--check]" >&2
    exit 2
    ;;
esac

require_tool oapi-codegen

# The generated files, relative to the root.
GENERATED=(
    internal/httpapi/gen/server.gen.go
    internal/fulfilment/fcapi/fcapi.gen.go
    internal/marketplace/wildberries/wbapi/general/general.gen.go
    internal/marketplace/wildberries/wbapi/items/items.gen.go
    internal/marketplace/wildberries/wbapi/orders/orders.gen.go
    internal/marketplace/wildberries/wbapi/reports/reports.gen.go
    src/lib/api/wyo-api.d.ts
)

# generate_into <folder>: writes every generated file below <folder>, at the same relative paths.
generate_into() {
    local out="$1" file
    for file in "${GENERATED[@]}"; do
        mkdir -p "$out/$(dirname "$file")"
    done

    # oapi-codegen writes the path of `output:` relative to where it runs, and wants to find go.mod above that place.
    if [ "$out" != "$root" ]; then
        cp "$root/go.mod" "$out/go.mod"
    fi

    (
        cd "$out"
        oapi-codegen -config "$root/api/codegen/wyo.cfg.yaml" "$root/api/wyo.yaml"
        oapi-codegen -config "$root/api/codegen/fulfilment-centre.cfg.yaml" "$root/api/fulfilment-centre.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-general.cfg.yaml" "$root/api/wildberries/01-general.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-items.cfg.yaml" "$root/api/wildberries/02-items.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-orders.cfg.yaml" "$root/api/wildberries/03-orders-fbs.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-reports.cfg.yaml" "$root/api/wildberries/12-reports.yaml"
    )

    if [ -f "$root/node_modules/.bin/openapi-typescript" ]; then
        pnpm exec openapi-typescript "$root/api/wyo.yaml" -o "$out/src/lib/api/wyo-api.d.ts"
    else
        echo "openapi-typescript is not installed (run: pnpm install); the interface's types were not generated" >&2
        return 1
    fi
}

if [ "$check" = false ]; then
    generate_into "$root"
    echo "generated ${#GENERATED[@]} files"
    exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
generate_into "$tmp"

status=0
for file in "${GENERATED[@]}"; do
    if [ ! -f "$root/$file" ]; then
        echo "missing (run: pnpm generate): $file" >&2
        status=1
    elif ! diff -q "$root/$file" "$tmp/$file" >/dev/null; then
        echo "out of date (run: pnpm generate): $file" >&2
        diff -u "$root/$file" "$tmp/$file" | head -20 >&2 || true
        status=1
    fi
done

if [ "$status" -eq 0 ]; then
    echo "generated files are up to date"
fi
exit "$status"
