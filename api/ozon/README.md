# Ozon's description file

`seller-api.json` is Ozon's own description of its Seller API (OpenAPI 3.0.0, `info.title` «Документация Ozon Seller API»,
`info.version` 2.1, 497 methods), committed exactly as Ozon publishes it. The wyo server's Ozon client
(`internal/marketplace/ozon/ozonapi/`) is generated from it by `pnpm generate`.

## Where it came from

- **Address:** `https://docs.ozon.ru/api/seller/swagger.json`.
- **Saved on 2026-11-18 by the maintainer from a browser.** Ozon refuses the file to the machine the research ran on (an answer
  `307 Temporary Redirect`, then `403` with the header `ozon-antibot: 1`), so it could not be downloaded by a program.
- **SHA-256:** `57f27f1a08d09f75473601183b8ee2d0f2e3336c084ca295d41d4969a0be8b2f`, 4 378 503 bytes.
- **The research that reads it:** `.agents/research/ozon-official-seller-api-check-2026-11-18.md`, section 1.

## How a later version is taken

A copy of the same file is kept on GitHub by a third party, not by Ozon, and renewed about daily:
`https://github.com/vispar-tech/ozon-seller-api-schema`, file `schemas/ozon-seller-api-openapi.json`. On 2026-11-18 its copy of that
day (commit `e22c5e9ee2a0`) and the file here, both read as JSON so that spacing and the order of keys do not count, differed only in
9 example values inside 5 methods (the image server `ir.ozone.ru` against `ir-20.ozone.ru`). To take a later version:

1. Download the GitHub copy of the day, or ask the maintainer to save Ozon's own file from a browser.
2. Read both files as JSON and list every difference.
3. Read every difference in a method the wyo server calls (`api/codegen/ozon.cfg.yaml` lists them) before accepting it.
4. Replace `seller-api.json`, write the new checksum and day here, run `pnpm generate`, and read the difference of the generated code.

## The corrections made for the generator

The generator (oapi-codegen v2.8.2) refuses Ozon's file as it is, because the file departs from OpenAPI 3.0 in 21 places. The
maintainer decided on 2026-11-18 that the file stays Ozon's own and the generation corrects a temporary copy: `scripts/generate.sh`
runs `tools/specfix` (the rules are in `internal/specfix`), which writes the corrected copy into a temporary folder; the generator reads
that copy, and the copy is deleted. Only descriptions of fields are corrected, never examples:

| What Ozon's file holds                                 | Places | What the copy holds                                     |
| ------------------------------------------------------ | ------ | ------------------------------------------------------- |
| `info.version` written as the number `2.1`              | 1      | the text `"2.1"`                                        |
| `required: true` on a single field                      | 6      | nothing (OpenAPI wants a list of field names there)      |
| the type `int`                                          | 4      | `integer`                                               |
| the type `bool`                                         | 1      | `boolean`                                               |
| the type `timestamp`                                    | 3      | `string`                                                |
| the type `enum`                                         | 2      | `string`                                                |
| the type `array of strings`                             | 3      | `array` (with items of the type `string` when it has none) |
| the type `file`                                         | 1      | `string` with the format `binary`                       |

The test `TestOzonsFileGetsExactlyTheCorrectionsItsReadmeLists` (`tools/specfix/main_test.go`) lists the 21 places. A new version of
the file that adds or removes one fails it, and a type no rule knows stops `pnpm generate`, so every new departure is read by a person
before a rule is added for it.

## The methods the wyo server never calls

`/v2/posting/fbs/cancel` (`PostingAPI_CancelFbsPosting`) and `/v2/posting/fbs/product/cancel` (`PostingAPI_CancelFbsPostingProduct`)
cancel a posting at Ozon. The wyo server never cancels an order at a marketplace (the constitution, principle XII), so they are never
in the generator's list; a test reads `api/codegen/ozon.cfg.yaml` to keep them out.
