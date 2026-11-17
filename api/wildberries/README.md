# Wildberries' description files

These six files are Wildberries' own OpenAPI descriptions of its seller API, downloaded unchanged on **2026-11-17** (the day they
were read). They are committed so that the Go code in `internal/marketplace/wildberries/wbapi/` is generated from a fixed text, and
so that a new version of a file shows up as a difference that is read before it is accepted.

| File                | Address it was downloaded from                                       | Lines  |
| ------------------- | -------------------------------------------------------------------- | ------ |
| `01-general.yaml`   | `https://dev.wildberries.ru/api/swagger/yaml/ru/01-general.yaml`     | 2 347  |
| `02-items.yaml`     | `https://dev.wildberries.ru/api/swagger/yaml/ru/02-items.yaml`       | 10 141 |
| `03-orders-fbs.yaml`| `https://dev.wildberries.ru/api/swagger/yaml/ru/03-orders-fbs.yaml`  | 7 150  |
| `07-orders-fbw.yaml`| `https://dev.wildberries.ru/api/swagger/yaml/ru/07-orders-fbw.yaml`  | 2 606  |
| `11-analytics.yaml` | `https://dev.wildberries.ru/api/swagger/yaml/ru/11-analytics.yaml`   | 7 075  |
| `12-reports.yaml`   | `https://dev.wildberries.ru/api/swagger/yaml/ru/12-reports.yaml`     | 3 760  |

The English copies are at the same addresses with `/en/` in place of `/ru/`.

## What is generated from them

Only the methods the connector uses are generated (the list is section 2 of
`.agents/research/wildberries-official-description-files-check-2026-11-17.md`), by `oapi-codegen`, with the settings in
`api/codegen/wildberries-*.cfg.yaml`:

| File                 | Go package                                                    |
| -------------------- | ------------------------------------------------------------- |
| `01-general.yaml`    | `internal/marketplace/wildberries/wbapi/general`              |
| `02-items.yaml`      | `internal/marketplace/wildberries/wbapi/items`                |
| `03-orders-fbs.yaml` | `internal/marketplace/wildberries/wbapi/orders`               |
| `12-reports.yaml`    | `internal/marketplace/wildberries/wbapi/reports`              |

`07-orders-fbw.yaml` and `11-analytics.yaml` are kept for the stages that will use them; nothing is generated from them yet.

Four packages, not one, because the files define types with the same names (`Next`, for one), which cannot live in one Go package.

## Updating

Download the file again from the address above, read the difference (`git diff`), run `pnpm generate` and read the difference of
the generated code. A file is never edited by hand.
