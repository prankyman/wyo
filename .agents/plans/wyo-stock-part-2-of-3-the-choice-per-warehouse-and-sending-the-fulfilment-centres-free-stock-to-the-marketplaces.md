# Stock, part 2 of 3: the choice «Остатки этого склада» on each seller's warehouse at a marketplace — «Только показывать» by default, or «Передавать числа фулфилмент-центра»; the fulfilment centre's free number written exactly as it comes, 0 for a product it does not know; every write read first, never repeated blindly and kept within each marketplace's limits; the warning «Остатки этого склада меняет кто-то ещё»

> **Plan 10** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 2 of the three stock plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`, whose section «The exchange with a fulfilment centre», step 6, says «The fulfilment
> centre's free stock goes to the seller's Wildberries warehouses when it changes», and whose Stage 2 sends it «to every marketplace that
> sells from it, so a sale on one lowers what the others offer». Written 2026-11-21 from `origin/main` at `eed4607`. **It starts only
> after part 1** (Plan 9,
> `.agents/plans/wyo-stock-part-1-of-3-products-and-stock-read-from-the-marketplaces-and-the-fulfilment-centre-stock-page-wildberries-product-names.md`)
> **is merged and released**; its section «Today» is written against what part 1 builds and is checked again on that day. Part 3
> (`.agents/plans/wyo-stock-part-3-of-3-own-warehouses-and-own-count-without-a-fulfilment-centre-lowered-by-orders-written-to-the-marketplaces.md`)
> adds the third value of the choice.
> Research: `.agents/research/marketplaces-stock-official-documents-check-2026-11-21.md` (cited «STK»: part W Wildberries, part O Ozon,
> part Y Яндекс Маркет).
>
> **Branch** `feat/stock-sending-the-fulfilment-centres-numbers`. One plan = one branch = one squashed commit = one pull request. The
> commit is `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: every warehouse starts as «Только показывать», so nothing is
> written for anybody until the owner chooses it. **One database migration**, the next free number at the start. Constitution:
> principle XIII «Stock» widened, **MINOR**, the next free number at merge time.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused". **An issue
found on the way is fixed in this plan at once** (constitution, «Plan Implementation Workflow», step 2), and the report lists it under
«Found while implementing».

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the budget**,
  **the journal** — as in Plans 2 and 3. **A fulfilment centre**, **«Внешний API»**, **the matching of warehouses** — as in Plan 4.
  **A marketplace's own warehouse**, **a seller's warehouse at a marketplace**, **a marketplace product**, **the catalogue**, **the
  fulfilment centre's free number**, **a match** — as in part 1.
- **The choice** — «Остатки этого склада», one value for each seller's warehouse at a marketplace: who changes that warehouse's stock
  at the marketplace.
- **The writer** — the ONE function of the wyo server that writes stock to a marketplace (section D); part 3 uses it too.
- **The number to write** — for a product of a warehouse in the mode «Передавать числа фулфилмент-центра», the number section C gives.

## The maintainer's request and answers (2026-11-21) — nothing is open

- **With a fulfilment centre, its number as it comes:** «The fulfillment center's number if there is FO plugged in. if not - then it is
  wyo that takes care of it.» Nothing is subtracted for orders the fulfilment centre has not reserved yet, nothing added.
- **Three modes, mixed per marketplace:** «WYO only reads data from Marketplaces while the WMS for the same seller is integrated with
  marketplaces but not with WYO; WYO reads and writes the products because it itself takes care of product counts …; and WYO reads and
  writes the product counts but writes only via integrated WMS.» And: «while the WYO may not integrate with WMS, the WMS may integrate
  with marketplaces (at least for Yandex Market and Ozon, as WB for now cannot be taken care by WMS and hence WYO is neeeded as the WB
  connector from WB to WMS software. It means that WMS takes care of the available products count, and WYO should not do it, again -
  even if the integration from WYO to WMS is off.» — so the choice is per warehouse, and an empty «Внешний API» never means wyo may write.
- **The choice:** asked whether one value per warehouse — «Только показывать (их ведёт другая программа или кабинет)» by default,
  «Передавать числа фулфилмент-центра» only for a warehouse matched in «Внешний API», «Передавать свой счёт wyo» (part 3) — is the
  right reading of the three modes: «Yes». «Let's make a switch»: nothing is written by default.
- **The warning when someone else writes:** «Yes».
- **A product the fulfilment centre does not know:** «Writing 0».
- **Three plans:** «3 plans».
- 2026-11-17, still true: no real account — the writing is built and tested only against the fakes; the checks on real ones are in
  «After the release».

## Today (written against part 1; checked again on the day the plan starts)

- After part 1 the wyo server reads every marketplace's catalogue and the stock of every warehouse, and, with «Внешний API» filled, the
  fulfilment centre's products and stock every 2 minutes; each marketplace product has a match to the fulfilment centre's product or
  «не знает» / «неоднозначно» (`MatchProducts`, `product_matches`); «Остатки» shows everything and writes nothing.
- The contract names `stock.push` and no adapter claims it; no writing method of stock is generated (part 1, Stage 0 premise 4).
- Яндекс Маркет's levels: Plan 8's exchange needs «Обработка заказов и учёт товаров»; the stock is written only with «Управление
  товарами и карточками» or «Полное управление кабинетом» (STK Y§1.6) — a level the key check counts today only as «чтение товаров»
  (`docs/yandex.md`, «Как wyo читает уровни токена»).

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Part 1 is released** and its «After the release» checks, if made by then, are read: what they found about a product without a
   record (STK W§7 gap 5) and about the fulfilment centre's `marketplace_ids` replaces the cautious choices below.
3. **Each marketplace's description is compared again on the day** (as part 1, premise 3); a difference in a writing method of section D
   stops the stage and asks.
4. **The generators read the writing methods:** Wildberries `putV3StocksWarehouseId`; Ozon `ProductAPI_ProductsStocksV2`
   (`/v2/products/stocks`) and `ProductAPI_GetProductInfoStocksByWarehouseFbsV2` (the reading before a write); Яндекс Маркет
   `updateStocksOnPartnerWarehouses`. **Never generated** (an architecture test): Wildberries' `deleteV3StocksWarehouseId` (it removes
   the stock record for good, «Действие необратимо», STK W§1.2), Яндекс Маркет's `updateStocks` (cabinets with groups, part 1), any
   writing method of products. Ozon's file lists `quant_size` as required though Ozon removed it on 2025-06-26 (STK O§1.2): the
   generated request type is checked by a test never to send it.
5. **The generated request bodies are checked against the description files:** Wildberries answers `204` even to a misspelled field
   and then changes nothing (STK W§1.1), so a test compares each body's field names with the file's schema.

## A. The choice «Остатки этого склада»

1. **One value per seller's warehouse at a marketplace** (`StockModeChoice`, one component, its values given by what the warehouse
   allows; part 3 adds the third):
   - **«Только показывать (их ведёт другая программа или кабинет)»** — the default for every warehouse, after the update too. «?»:
     «Выберите, если остатки этого склада у маркетплейса меняет другая программа — например, система фулфилмент-центра, подключённая к
     маркетплейсу напрямую, — или вы сами в кабинете. wyo будет только показывать числа».
   - **«Передавать числа фулфилмент-центра»** — offered only when «Внешний API» is filled, the warehouse is matched there to one of the
     fulfilment centre's warehouses (so a DBS or rFBS warehouse, which is never matched, never gets it), the account's key may write
     stock (section B) and the marketplace does not mark the warehouse as closed for stock (Wildberries `isDeleting`, Яндекс Маркет's
     `apiAvailability` other than `AVAILABLE`, STK W§2, Y§3). «?»: «wyo будет сообщать маркетплейсу свободный остаток
     фулфилмент-центра для каждого товара этого склада — ровно то число, которое даёт фулфилмент-центр. Товар, которого
     фулфилмент-центр не знает, получит 0. Никакая другая программа не должна менять остатки этого склада». When it is not offered,
     it is shown dimmed with the reason and what to do («Сопоставьте склад в «Внешнем API»», the level to add).
2. **Switching on** opens `ConfirmationModal` with part 1's comparison of that warehouse: «Маркетплейс получит числа для N товаров:
   изменятся X; получат 0, потому что фулфилмент-центр их не знает, — Z; неоднозначны и тоже получат 0 — W. Убедитесь, что никакая
   другая программа и никто в кабинете не меняет остатки этого склада», and «Начать передачу». The choice is stored with its time.
3. **Switching off** to «Только показывать» stops the writing at once; nothing written is undone — the marketplace keeps the last
   numbers, and the confirmation says so.
4. **The choice falls back to «Только показывать» by itself** when «Внешний API» is emptied, the warehouse is unmatched, the account is
   removed or its key can no longer write: writing stops at once (principle XII: emptying the setting stops every call), and a notice
   behind the bell says «Передача остатков склада … остановлена: …» with the reason. Filling «Внешний API» again does not switch it on:
   the owner chooses again.
5. **One writer per warehouse.** wyo writes only a warehouse the owner chose; it cannot see another program, so the choice's «?», the
   confirmation, the README and section F's warning are how the owner keeps one writer.
6. **«Внешний API» → «Склады»** (the matching of warehouses) gains, above the table: «wyo передаёт фулфилмент-центру заказы только
   сопоставленных складов. Если система фулфилмент-центра сама подключена к маркетплейсу и получает его заказы, не сопоставляйте этот
   склад здесь — иначе один заказ придёт к фулфилмент-центру дважды», and beside each matched warehouse its choice with a link to
   «Остатки».

## B. The key must allow writing stock

- **Wildberries:** a token not marked read-only (bit 30 of `s`, Plan 3's token check) with the category «Маркетплейс». The `PUT` is
  marked `x-readonly-method: false`; that a read-only token is refused is not documented (STK W§7 gap 1), so such a token is never
  offered the writing, and a `403` on a write is the account's problem as Plan 3 shows it.
- **Ozon:** `/v2/products/stocks` among the key's methods (`/v1/roles`, Plan 5's rule; Ozon's 2024 table puts it in the role `Product`,
  STK O§1.6).
- **Яндекс Маркет:** «Управление товарами и карточками» or «Полное управление кабинетом» (STK Y§1.6). «Обработка заказов и учёт
  товаров», which the exchange of orders needs, does not open it; the account's card says «Чтобы передавать остатки, добавьте ключу
  уровень «Управление товарами и карточками»». The key check of Plan 7 learns this level as a writing level.
- Each account's card on «Маркетплейсы» gains the line «Передача остатков: разрешена» or the level to add.

## C. The number to write — the fulfilment centre's, exactly as it comes

For each product of the warehouse's account in the catalogue that is not archived:

1. **A match:** the matched fulfilment-centre product's `free` at the fulfilment centre's warehouse matched to this one — **as it
   comes, nothing subtracted, added or rounded**. The fulfilment centre has no row for it at that warehouse: 0 (nothing lies there).
2. **«Фулфилмент-центр не знает этот товар» or «неоднозначно»:** 0 (the maintainer: «Writing 0»): the fulfilment centre gives no
   number, so the marketplace must not sell it from this warehouse.
3. **Two numbers no marketplace takes:** a `free` below 0 is written as 0 — no marketplace takes a number below 0 (Яндекс Маркет:
   `count` from 0, STK Y§1.2), and 0 says what it means, nothing orders may take; a number above the marketplace's ceiling
   (Wildberries 100 000, STK W§1.1; Ozon «меньше миллиона», O§1.3; Яндекс Маркет 2 000 000 000, Y§1.2) is not written, and the
   product gets the problem «Фулфилмент-центр даёт N, а маркетплейс принимает не больше M».
4. **An archived product is not written** (Ozon refuses it, `PRODUCT_IS_ARCHIVED`, STK O§1.5; Wildberries' trash is not in the
   catalogue).

## D. The writer — read first, only what differs, within the limits

1. **When.** After each reading of the fulfilment centre's stock (every 2 minutes, part 1), the products whose number to write changed
   are queued for every warehouse in this mode. **Every 30 minutes**, and at once after switching on, a full comparison: the
   warehouse's numbers are read from the marketplace and every product whose number there differs from the number to write is queued
   — this catches what orders and the cabinet changed, and is where section F looks.
2. **Read first, write only what differs** (principle XII, «never a blind write»): before each batch the products' current numbers are
   read — Wildberries `POST /api/v3/stocks/{warehouseId}` by `chrtIds` (`amount`); Ozon `/v2/product/info/stocks-by-warehouse/fbs` by
   `sku` (`free_stock`; Ozon: «Переданный остаток — количество товара в наличии без учёта зарезервированных», STK O§6); Яндекс Маркет
   `getStocksOnPartnerWarehouses` by up to 500 `offerIds` (`AVAILABLE`). A product whose number already equals the number to write is
   not written.
3. **The batches and the limits,** all through the one caller and the one budget:
   - **Wildberries** `PUT /api/v3/stocks/{warehouseId}` with `{"stocks":[{"chrtId","amount"}]}`, up to 1000 items, inside the
     300-a-minute pool of the category «Маркетплейс» that the orders share, an answer with an error counted as 10 (STK W§1.1);
   - **Ozon** `POST /v2/products/stocks` with `{"stocks":[{"product_id","warehouse_id","stock"}]}` (only `product_id`, never with
     `offer_id`, as Ozon advises; never `quant_size`), up to 100 pairs, 80 requests a minute, and **one pair at most once in 30
     seconds**: a newer number for a pair written less than 30 seconds ago waits, and only the latest is written (STK O§1.3);
   - **Яндекс Маркет** `POST /v3/businesses/{businessId}/offers/stocks/update` with `skuItems` of `{sku, partnerWarehouseId, count}`,
     up to 2000 items, 50 requests a minute, at most 4 at once, a body under 512 KB; `updatedAt` is not sent, so Яндекс Маркет takes its
     own time (STK Y§1.2, §1.4) and no clock difference can refuse it.
4. **Яндекс Маркет once a day:** every product of a written warehouse is sent once a day even when equal — Яндекс Маркет's own advice,
   «Передавать остатки нужно как можно чаще — хотя бы раз в день» (STK Y§1.4).
5. **Each outcome is stored before the next batch** (`stock_writes`: the number, when, the answer), so a wyo server stopped in the middle
   goes on from the last stored write, and a write made is not repeated.
6. **One function, one caller for each method:** the adapter's stock-writing method is called only by the writer (an architecture test),
   and the writer names no marketplace: what differs — batch size, the per-pair wait, the ceiling, whether an order lowers the number by
   itself — the adapter declares in the contract's part `stock.push`.

## E. The marketplace's answers

- **Wildberries:** `204` — written. `409` lists the refused items with their codes (`NotFound`, `CargoWarehouseRestriction…`,
  `DeliveryTypeRestriction`, `SubjectRestriction`, `TnvedRequired`, `UploadDataLimit`; STK W§1.1): each listed product gets the problem in
  Wildberries' words and is not written again until the catalogue or the warehouse changes; whether the request's other items were
  written is not documented (STK W§7 gap 4), so it is sent again without the listed items, once. `406 StocksUpdateBlock` — the
  warehouse's writing waits with that problem; `406 WarehouseStocksUpdateBlock` and `409 StoreIsProcessing` — Wildberries is updating the
  warehouse: tried again in 5 minutes, and while the warehouse's `isProcessing` (read every hour) is set the warehouse waits.
- **Ozon:** `200` with `result[]` per pair. `updated: true` — written. `TOO_MANY_REQUESTS` or `Stock is updated too frequently` — tried
  again after 30 seconds. Any other code of STK O§1.5 (`PRODUCT_IS_ARCHIVED`, `offer_id_not_found`, `NON_KGT_ON_KGT_WAREHOUSE`,
  `OVER_MAX_OVH_KGT`, …) — the product's problem in Ozon's words, not written again until the catalogue or the warehouse
  changes. `WAREHOUSE_NOT_FOUND` for every pair — the warehouse's problem.
- **Яндекс Маркет:** `200` (an empty answer) — written. `400` with a message of STK Y§1.5 — the request's problem in Яндекс Маркет's
  words; whether part of the request was applied is not documented (STK Y§7 gap 7), so the next full comparison reads and sends what
  differs. `420` — waited out as its headers say. `403 API for campaign … disabled` or `423` — the warehouse's problem.
- A problem is shown on the product and on the warehouse's place; one that stops a whole warehouse also goes behind the bell. A refused
  product of a warehouse in this mode is never left silent.

## F. The warning «Остатки этого склада меняет кто-то ещё»

1. **Explained or not.** After a write of N for a product, a later reading of the marketplace's number is **explained** when it is not
   above N and not below N minus the units of this warehouse's orders of that product that arrived since the write; a cancellation
   brings the number back up within the same range. An order lowers the number by itself at Wildberries (STK W§5: «остатки уменьшатся
   автоматически») and at Яндекс Маркет (STK Y§5.1); for Ozon it is not documented (STK O§6), so the same rule is used and checked after
   the release. The adapter declares it (`StockLoweredByOrders`); the warning names no marketplace.
2. **Twice in a row.** A product unexplained in two readings in a row (one reading may lag an order not read yet) marks the warehouse:
   its place and its choice show «Остатки этого склада меняет кто-то ещё: у N товаров число у маркетплейса не совпадает с тем, что
   передал wyo, и заказы этого не объясняют», the products listed, and «Проверьте, не подключена ли к этому складу другая программа и
   не меняют ли остатки в кабинете. Если их ведёт другая программа, выберите «Только показывать»». A notice goes behind the bell once.
3. **wyo goes on writing** — which program writes is the owner's choice; the warning clears itself after two explained readings in a
   row of every marked product.

## G. The owner's view and the instructions

- **«Остатки»:** the choice on each seller's warehouse; the warehouse's writing state (`StockWriteSummary`: «Передача включена 21.11 в
  12:30 · передано сегодня N раз · отказов M»); each product's last write in `StockTable` (`StockWriteState`: «передано 12 в 12:31»,
  «отказ: <the marketplace's words>», «ждёт: Ozon разрешает менять число не чаще раза в 30 секунд»); the warning
  (`ForeignWriterWarning`); the filters «С отказом» and «Ждут передачи».
- **«Внешний API» → «Склады»:** the sentence and the choices of section A, point 6.
- **«Маркетплейсы»:** the line «Передача остатков» of section B.
- **Notices behind the bell:** the writing stopped (with why), a warehouse's refusal, the warning.
- **`README.md`, «Остатки»,** gains the choice and when to take each value: the fulfilment centre's system connected to the
  marketplace directly (Ozon and Яндекс Маркет can be; Wildberries cannot) → «Только показывать», and the warehouse left unmatched in
  «Внешний API» if that system also takes the orders; wyo passes the fulfilment centre's numbers → «Передавать числа
  фулфилмент-центра»; no fulfilment centre → part 3. **One program per warehouse**, with Wildberries' own warning that a second tool may
  zero the stock and that orders not delivered are fined (STK W§5). What 0 does, and why a product the fulfilment centre does not know
  gets it. **Section 12** gains the stock in the plain list of what wyo does at each marketplace by itself, and what it never does:
  remove a stock record (Wildberries' `DELETE` is never generated), write a warehouse the owner did not choose, write a number other
  than the fulfilment centre's — except 0 for a product it does not know or gives below 0. Sections 7, 8 and 9 gain each key's level
  for the stock.
- `docs/wildberries.md`, `docs/ozon.md`, `docs/yandex.md` gain the writing methods with their limits and answers; `docs/external-api.md`
  the stock's walk from the fulfilment centre's reading to the marketplace.
- **The constitution (MINOR):** principle XIII widened — the choice per warehouse, «Только показывать» by default and after any loss of
  its conditions; the fulfilment centre's number as it comes, 0 for no number or below 0, nothing above the ceiling; the one writer, read
  first, only what differs, each outcome stored, no stock record ever removed; the warning; the tests named.

## H. Rules R2, R3, R4 and R5, proven here

- **R3, alone by default:** with «Внешний API» empty no warehouse can take «Передавать числа фулфилмент-центра»; a made-up day with
  every warehouse at its default writes nothing (`TestNoStockIsWrittenToAWarehouseTheOwnerDidNotChoose`: the fakes' call logs hold no
  stock write); emptying «Внешний API» stops every write at once.
- **R4, the keys stay:** the canary test widened to the writes — the marketplace's key and the fulfilment centre's key in no log line,
  journal row, answer or request to the other side.
- **R5, the limits:** a made-up hour with 10 000 products on three warehouses of each marketplace and the fulfilment centre changing 500
  numbers every 2 minutes, beside the orders' exchange: Wildberries' pool never over 300 a minute (an error counted as 10); Ozon never
  over 80 writes a minute and never a pair twice in 30 seconds; Яндекс Маркет never over 50 a minute or 4 at once.
- **R2, neutral:** the writer and the warning live in `internal/stock` and name no marketplace (the architecture test).

## I. Data, code, tests

**Data — one migration, `<next>_stock_sending`:** `stock_modes` (the account, the warehouse's number, the value `show` | `fc` | `own` —
`own` is written only by part 3 —, when chosen, why it fell back); `stock_writes` (the account, the warehouse, the product: the last
number written, when, the answer, the problem's code and words, the next try, the unexplained readings in a row);
`marketplace_warehouses` gains the marketplace's mark that stock cannot be written now (Wildberries' `isProcessing` / `isDeleting`, Яндекс
Маркет's `apiAvailability`). The upgrade test runs from the snapshot of part 1's release, written first.

**The wyo server's own API** (`api/wyo.yaml`): `PUT /api/v1/stock/places/{place}/mode` (the choice; the answer names the fall-back
reasons when refused), `GET /api/v1/stock/places/{place}/mode-preview` (the confirmation's numbers); `GET /api/v1/stock/places/…` of part
1 gains each product's write state and the warehouse's warning.

**The adapter contract** gains `stock.push` (the batch, the per-pair wait, the ceiling, `StockLoweredByOrders`, the per-item answers),
with its cases in the one contract suite, claimed by the three adapters.

**Code — where things live:** `internal/marketplace` (the part), `internal/marketplace/wildberries`, `…/ozon`, `…/yandex` (each its write
and its reading before it), `internal/stock` (the choice, the number to write, the writer, the warning), `internal/fakes/wildberries`,
`…/ozon`, `…/yandex` (the writing routes with their refusals and limits, and an order lowering the stock where the marketplace does it),
the interface's `src/lib/components/stock/` and `src/lib/components/external-api/`.

**Components created** (each once, with its test file in every state): `StockModeChoice` (the values, dimmed ones with their reason),
`StockModeConfirmation` (the preview inside `ConfirmationModal`), `StockWriteState` (a product's last write), `StockWriteSummary` (a
warehouse's writing), `ForeignWriterWarning`. **Components reused:** `StockPlaceList`, `StockPlaceCard`, `StockTable`, `MatchBadge`,
`MatchSummary` (part 1), `ConfirmationModal`, `WarningBanner`, `StatusBadge` with the tones, `NoteBox`, `HelpTooltip`, `FilterSection`,
`WarehouseMatchTable`, `MarketplacesSection`, the bell; in Go the one caller, the one budget, the one journal, the one contract suite, the
matching function, the reading of each place (part 1).

**Tests** (written before the code they check, failing first):

- **Go, with the three fake marketplaces and the fake fulfilment centre in one test process:** switching on → the full comparison → the
  writes; a number changed at the fake fulfilment centre reaching each marketplace within its reading and the marketplace's limit; a
  product unknown and one ambiguous written 0; a `free` below 0 written 0; one above the ceiling refused before a call with its problem;
  an archived product not written; Ozon's 30 seconds with three changes inside them (only the last written); every answer of section E,
  each in the marketplace's words; Яндекс Маркет's daily sending; the fall-back on each of its four causes, and no write after it.
- **Stopped in the middle:** the writer interrupted after every single batch in turn; after the restart the walk ends right, and the
  fakes' call logs show no write made twice.
- **The warning:** a reading explained by orders; unexplained once (no warning); twice (the warning, one notice); cleared after two
  explained readings; a cancellation raising the number within the range.
- **Never sent:** the tests of section H; Wildberries' `DELETE` and Яндекс Маркет's `updateStocks` never generated; no Ozon body with
  `quant_size` or with both `offer_id` and `product_id`; every Wildberries body's field names equal to the description file's; the
  writing method of each adapter called only by the writer.
- **The fakes gain** the writing routes, their refusals on command (the fault kind «refuse with this answer»), their limits, and an order
  lowering the stock where the marketplace documents it; each with its test.
- **Interface unit tests:** each new component once in every state; the pages only for what they add; the policy tests green, the
  marketplace-name policy among them; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes, each worker with its own data): a matched Wildberries warehouse
  switched on through the confirmation and its products written; a refusal shown; the warning appearing after the fake's stock is
  changed twice by hand; «Внешний API» emptied and the fall-back notice; a warehouse not offered the value, with its reason; every new
  state swept at 390 × 844 and the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration. 3. The contract's part `stock.push` and its suite cases. 4. The generated writing methods and
   the fakes' writing routes. 5. The choice and its conditions. 6. The number to write. 7. The writer: read first, batches, limits,
   stored outcomes. 8. The answers. 9. The warning. 10. The owner's view: «Остатки», «Внешний API», «Маркетплейсы», the notices. 11. The
   tests of section H and the interrupted walks. 12. The README, `docs/`, the help texts, the constitution. 13. The smoke test, then the
   gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes): fill «Внешний API»; match a Wildberries, an
  Ozon and a Яндекс Маркет warehouse; switch each to «Передавать числа фулфилмент-центра» through the confirmation and see its numbers
  reach the fake; change a number at the fake fulfilment centre and see it reach all three; a product the fulfilment centre does not
  know written 0; a refusal; the warning after changing a fake's stock by hand twice; switch one back to «Только показывать» and see no
  more writes; empty «Внешний API» and see the fall-back notice and no more writes; every page at a computer's width and at 390 × 844
  with screenshots; the console, the raw-key check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md` (`pnpm remote:gate` on the Mac mini); the check of rule R1; squash; push; the pull request; the
  merge by the agent; the release run watched to its end; `main` pulled; the report, with «Found while implementing». At the merge the
  register is updated as «Not in this plan» and «After the release» say.

## After the release — the checks with the first real accounts and the first real fulfilment centre (register)

On a warehouse whose goods are hidden first where the marketplace allows it (Яндекс Маркет's own advice, STK Y§5.6): whether
Wildberries takes `amount` 0 and what it does to the card (STK W§7 gap 3), and whether one refused item stops a whole request (gap 4);
what Ozon's `free_stock` reads after a write of N and how soon (STK O§8 gap 8), and which of its two words the 30-second rule returns
(gap 5); whether Яндекс Маркет applies a request in part (STK Y§7 gap 7) and whether its own help means the number sent must exclude
reserved orders (gap 1 — the maintainer's rule stands either way: the fulfilment centre's number as it comes); whether an Ozon order lowers
the number by itself (section F); how long a write takes to reach the buyer's page. What they find becomes part 3's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **wyo's own count** — part 3.
- **DBS and rFBS warehouses** — wyo reads none of their orders (register item 19), so neither the fulfilment centre nor wyo can count
  them.
- **Яндекс Маркет cabinets with groups of warehouses** (`updateStocks`) — register item 40 first.
- **Stopping the writing by itself on the warning** — not asked; the owner chooses which program writes.
- **Several fulfilment centres** — register item 27.
- **A new register item, «The checks of Plan 10 on the first real accounts and the first real fulfilment centre»,** holds «After the
  release».

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — the existing tests stay as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key and no buyer datum in a log, a journal row, an answer or an error.
5. Everything is a component and is reused: one writer, one caller, one budget, one journal, one contract suite, one matching function,
   one choice component, one table of a place's products; search before writing; the components section I names are created once.
   Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **No stock is written to a warehouse the owner did not choose; the fulfilment centre's number is written as it comes (0 only for no
   number or below 0); no write is repeated blindly; no stock record is ever removed.** With «Внешний API» empty nothing is sent
   anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about a marketplace is
   written only from its official documents (STK), with the address and the date; a gap is marked "not sourced", gets the cautious
   choice, and goes to «After the release». The marketplaces' description files are never edited.
9. An issue found on the way is fixed in this plan at once, or goes to the register with its reason when it cannot be; the report lists
   every finding under «Found while implementing».
10. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in its own words
    beside wyo's sentence; no sentence in Go. Name the program every time. Code is written mobile-first (390 × 844 first).
11. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage; the
    final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

One contract part in three adapters, four generated methods, two tables and one column, two addresses of the wyo server's own API, five
components. The interrupted walks, the per-pair wait of Ozon and the warning's arithmetic are the largest test work, and the parts that
protect a seller from two programs fighting over one warehouse and from goods sold that are not there.
