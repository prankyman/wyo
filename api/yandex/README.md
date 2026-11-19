# Яндекс Маркет's description files

This folder is the folder `openapi/` of Яндекс Маркет's own repository of its Partner API description, committed exactly as Яндекс
Маркет publishes it: `openapi.yaml` (OpenAPI 3.0.3, `info.title` «API Яндекс Маркета для продавцов»), `paths/*.yaml` (one file per
address) and `components/{schemas,parameters,responses}/*.yaml` — 1 087 files. `LICENSE` beside them is that repository's licence
(BSD-3-Clause), kept because the licence asks that a copy keeps its notice. The wyo server's Яндекс Маркет client
(`internal/marketplace/yandex/yandexapi/`) is generated from these files by `pnpm generate`.

## Where it came from

- **Repository:** `https://github.com/yandex-market/yandex-market-partner-api` (Яндекс Маркет's own; the documents at
  `https://yandex.ru/dev/market/partner-api/doc/ru/` point to it).
- **Commit:** `9be9f98e111133efdb31f7260bdb27358b378b29` of 2026-11-10 14:02:26 +0300, the commit the research read
  (`.agents/research/yandex-market-official-partner-api-check-2026-11-19.md`, section 1). Copied on 2026-11-19.
- **SHA-256 of `openapi.yaml`:** `16157bbf948b88cd1df7abcd2073502d473e733714de3d244f37b36ac61e15ae`.
- **Compared on 2026-11-19 with the repository's newest commit** `4ca8b761cd5a99d8bf36cacf626e4b768ac5a4cd` (2026-11-19 10:02:57
  +0300): it adds five methods of size tables (`/v2/businesses/{businessId}/size-tables…`) and their 30 schemas, and changes nothing
  else. None of them is a method the wyo server calls, so the commit of the research is kept.

## How a later version is taken

1. Clone the repository (or fetch it again) into a folder outside this repository.
2. List the difference between the committed tree and the newest commit's folder `openapi/`:
   `git diff --stat 9be9f98e111133efdb31f7260bdb27358b378b29 <newest> -- openapi` in the clone, then the full difference.
3. Read every difference in a path or a schema the wyo server uses (`api/codegen/yandex.cfg.yaml` lists the methods; their
   schemas are the ones the generated client holds) before accepting it.
4. Replace this folder's files with the newest `openapi/`, write the commit, the day and the new checksum here, run
   `pnpm generate`, and read the difference of the generated code.

## Why a joined copy is generated from

The generator (oapi-codegen v2.8.2) stops with «fatal error: stack overflow» on the tree as published (an endless recursion while it
follows the references from file to file). The files stay Яндекс Маркет's own, and `pnpm generate` joins them into one temporary
file: `scripts/generate.sh` runs `tools/specfix join` (the rule «join» of `internal/specfix`), which

- puts every path file where `openapi.yaml` names it;
- makes every component file an entry of `components/<kind>` under its file's name;
- writes every reference to a component file (`../components/<kind>/<Name>.yaml`, `./<Name>.yaml`, `../<kind>/<Name>.yaml`) as
  `#/components/<kind>/<Name>`.

Nothing else is changed, and a reference that is neither a component file nor, in the paths of `openapi.yaml`, a path file stops
the generation, so a new kind of reference is read by a person before a rule is added for it. The generator reads the joined copy,
and the copy is deleted. The tests are `TestATreeOfFilesIsJoinedIntoOneWithEveryFileReferenceMadeInternal` and
`TestAReferenceThatIsNeitherAComponentFileNorAPathFileStopsTheJoin` (`internal/specfix/join_test.go`) and
`TestYandexMarketsTreeIsJoinedIntoOneFileWithNoReferenceToAFile` (`tools/specfix/main_test.go`, the committed tree).

## The methods the wyo server never calls

The generated client holds only the methods `api/codegen/yandex.cfg.yaml` lists: `getAuthTokenInfo`, `getCampaigns`,
`getBusinessOrders`, `getPartnerWarehouses` and `getOfferMappings`, all of them reading. Three methods are never generated, because
each removes goods from an order, accepts a cancellation or moves orders out of a shipment, which lowers the shop's quality index or
cannot be undone (the research, section 5): `updateOrderItems`, `acceptOrderCancellation` and `transferOrdersFromShipment`. A test of
`internal/archtest` reads the configuration and fails when one of them is listed.
