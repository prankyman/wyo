#!/usr/bin/env bash
# `pnpm generate` makes every generated file; `pnpm generate:check` (--check) makes them in a temporary folder and fails
# when a committed generated file differs from a fresh one.
#
# Generated:
#   internal/httpapi/gen/                              from api/wyo.yaml (oapi-codegen)
#   internal/fulfilment/fcapi/                         from api/fulfilment-centre.yaml (oapi-codegen)
#   internal/marketplace/wildberries/wbapi/<package>/  from api/wildberries/*.yaml (oapi-codegen)
#   internal/marketplace/ozon/ozonapi/                 from api/ozon/seller-api.json (oapi-codegen), through a corrected
#                                                      copy written by tools/specfix into a temporary folder: Ozon's file
#                                                      departs from OpenAPI where the generator refuses it, and the
#                                                      committed file is never edited (api/ozon/README.md)
#   src/lib/api/wyo-api.d.ts                           from api/wyo.yaml (openapi-typescript)
#   internal/store/db/                                 from internal/store/*.sql, migrations/ and queries/ (sqlc)
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
require_tool sqlc

# The generated files, relative to the root.
GENERATED=(
    internal/httpapi/gen/server.gen.go
    internal/fulfilment/fcapi/fcapi.gen.go
    internal/marketplace/wildberries/wbapi/general/general.gen.go
    internal/marketplace/wildberries/wbapi/items/items.gen.go
    internal/marketplace/wildberries/wbapi/orders/orders.gen.go
    internal/marketplace/wildberries/wbapi/reports/reports.gen.go
    internal/marketplace/ozon/ozonapi/ozonapi.gen.go
    src/lib/api/wyo-api.d.ts
    internal/store/db/accounts.sql.go
    internal/store/db/db.go
    internal/store/db/exchange.sql.go
    internal/store/db/journal.sql.go
    internal/store/db/models.go
    internal/store/db/migrations.sql.go
    internal/store/db/orders.sql.go
    internal/store/db/people.sql.go
    internal/store/db/secrets.sql.go
    internal/store/db/sessions.sql.go
    internal/store/db/settings.sql.go
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

    # The corrected copy of Ozon's file lives only for this run.
    local ozon
    ozon="$(mktemp -d)"
    go run ./tools/specfix "$root/api/ozon/seller-api.json" "$ozon/seller-api.json" >/dev/null

    (
        cd "$out"
        oapi-codegen -config "$root/api/codegen/wyo.cfg.yaml" "$root/api/wyo.yaml"
        oapi-codegen -config "$root/api/codegen/fulfilment-centre.cfg.yaml" "$root/api/fulfilment-centre.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-general.cfg.yaml" "$root/api/wildberries/01-general.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-items.cfg.yaml" "$root/api/wildberries/02-items.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-orders.cfg.yaml" "$root/api/wildberries/03-orders-fbs.yaml"
        oapi-codegen -config "$root/api/codegen/wildberries-reports.cfg.yaml" "$root/api/wildberries/12-reports.yaml"
        oapi-codegen -config "$root/api/codegen/ozon.cfg.yaml" "$ozon/seller-api.json"
    )
    rm -rf "$ozon"

    # sqlc resolves every path of its configuration against the folder the configuration is in, so when the output goes
    # elsewhere the inputs go with it.
    if [ "$out" != "$root" ]; then
        mkdir -p "$out/internal/store"
        cp "$root/sqlc.yaml" "$out/sqlc.yaml"
        cp -R "$root/internal/store/schema_migrations.sql" "$root/internal/store/migrations" "$root/internal/store/queries" "$out/internal/store/"
    fi
    (cd "$out" && sqlc generate)

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

# A file in the generated folder that a fresh run does not make (a query file was deleted) is out of date too.
for extra in "$root"/internal/store/db/*.go; do
    if [ ! -f "$tmp/internal/store/db/$(basename "$extra")" ]; then
        echo "no longer generated (delete it): ${extra#"$root"/}" >&2
        status=1
    fi
done

if [ "$status" -eq 0 ]; then
    echo "generated files are up to date"
fi
exit "$status"
