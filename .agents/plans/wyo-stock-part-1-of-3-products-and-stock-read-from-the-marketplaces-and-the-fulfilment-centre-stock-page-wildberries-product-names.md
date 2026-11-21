# Stock, part 1 of 3: the products and the stock of every warehouse read from Wildberries, Ozon and Яндекс Маркет, and from the fulfilment centre when «Внешний API» is filled; each marketplace product matched to the fulfilment centre's product by one rule; the new page «Остатки», which only shows and writes nothing; Wildberries product names in order lines

> **Plan 9** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 1 of the three stock plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`, whose Stage 2 holds «Stock: what lies in each marketplace's warehouses, and the
> fulfilment centre's free stock sent to every marketplace that sells from it, so a sale on one lowers what the others offer», and
> whose «How often it asks» says «every 30 minutes — … stock in the marketplaces' warehouses». Part 2
> (`.agents/plans/wyo-stock-part-2-of-3-the-choice-per-warehouse-and-sending-the-fulfilment-centres-free-stock-to-the-marketplaces.md`)
> sends the fulfilment centre's numbers; part 3
> (`.agents/plans/wyo-stock-part-3-of-3-own-warehouses-and-own-count-without-a-fulfilment-centre-lowered-by-orders-written-to-the-marketplaces.md`)
> keeps wyo's own count. Written 2026-11-21 from `origin/main` at `eed4607` (release 0.0.13). Its section «Today» is checked again on
> the day the plan starts.
> Research: `.agents/research/marketplaces-stock-official-documents-check-2026-11-21.md` (cited «STK»: part W Wildberries, part O Ozon,
> part Y Яндекс Маркет); `docs/fulfilment-centre-api.md` and `api/fulfilment-centre.yaml` (the neutral specification).
>
> **Branch** `feat/stock-reading-products-and-the-stock-page`. One plan = one branch = one squashed commit = one pull request. The
> commit is `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: the plan only reads, and nothing a seller had changes.
> **One database migration**, the next free number at the start (0009 on the day of writing). Constitution: a new principle XIII
> «Stock», **MINOR**, the next free number at merge time.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused". **An issue
found on the way is fixed in this plan at once** (constitution, «Plan Implementation Workflow», step 2), and the report lists it under
«Found while implementing».

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the engine**,
  **the journal**, **the budget** — as in Plans 2 and 3. **A fulfilment centre**, **«Внешний API»**, **the neutral API**, **the
  matching of warehouses** (`fc_warehouse_map`: which of the fulfilment centre's warehouses serves a marketplace warehouse) — as in
  Plan 4.
- **A marketplace's own warehouse** — a warehouse the marketplace itself runs and ships from: Wildberries' warehouses, Ozon's FBO
  warehouses (Ozon stores and ships), Яндекс Маркет's FBY warehouses. The seller cannot set its stock; wyo only shows it.
- **A seller's warehouse at a marketplace** — the marketplace's record of a place the seller ships from (Wildberries' «склад
  продавца», Ozon's FBS and rFBS warehouses, Яндекс Маркет's FBS, Express and DBS warehouses). The seller tells the marketplace how
  many units lie there; the marketplace sells that many. This plan only reads the number; parts 2 and 3 write it.
- **A marketplace product** — the thing a marketplace keeps one stock number for: at Wildberries **one size of a card** (its number
  `chrtId`, the card's number `nmID`, the seller's article `vendorCode`, the size, the barcodes `skus`, STK W§3); at Ozon **one
  product** (`product_id`, `sku`, the article `offer_id`, the barcodes, STK O§4); at Яндекс Маркет **one offer** (the seller's SKU
  `offerId`, the card's number `marketSku`, the barcodes, STK Y§4).
- **The catalogue** — the marketplace products of an account, as wyo last read them.
- **The fulfilment centre's free number** — the field `free` of a row of `GET /api/v1/stock`: «How many units orders may still take, a
  whole number. It is exactly the number the website sends to marketplaces and shops» (the fulfilment software's released description).
- **A match** — which of the fulfilment centre's products a marketplace product is, found by the one rule of section F.

## The maintainer's request and answers (2026-11-21) — nothing is open

- 2026-11-17, register item 1, and 2026-11-18, register item 29: stock is the last open part of Stage 2. 2026-11-21: «Go ahead and
  create a plan»; asked how many plans: «2 plans», then, offered three: «3 plans».
- **What is written, and from where:** «The fulfillment center's number if there is FO plugged in. if not - then it is wyo that takes
  care of it … this software may not have the FO plugged in and function purely stand-alone so the business logic must take it into
  consideration.» — so this plan reads both sides, and parts 2 and 3 write.
- **Three modes:** «WYO only reads data from Marketplaces while the WMS for the same seller is integrated with marketplaces but not with
  WYO; WYO reads and writes the products because it itself takes care of product counts …; and WYO reads and writes the product counts
  but writes only via integrated WMS.» The choice per warehouse with «only show» as the default is part 2's; this plan is the reading
  all three modes share.
- **A switch, nothing written by default:** «Let's make a switch» — this plan writes no stock at all.
- **Product names:** «Yes, include the names» — Wildberries order lines get the product's name (register item 20).
- **A product the fulfilment centre does not know:** «Writing 0» (part 2) — so this plan shows, before any choice, which products the
  fulfilment centre does not know.
- 2026-11-17, still true: no real account — everything is built and tested against the fakes; the checks on real ones are listed in
  «After the release».

## Today (to be checked again on the day the plan starts)

- **The contract names the stock parts and drives none of them:** `products.read`, `stock.read` and `stock.push` are named
  (`internal/marketplace/contract.go:49-53`), and `supportedNow` does not list them, so an adapter that claimed one would fail the
  contract suite.
- **Order lines:** an Ozon line's name and barcodes come from `/v3/product/info/list` read for the order
  (`internal/marketplace/ozon/orders.go:418`), a Яндекс Маркет line's from `getOfferMappings`; a Wildberries line carries the kind,
  the brand and the size the orders feed gives, and **no name** (register item 20).
- **Warehouses:** `marketplace_warehouses` keeps a seller's warehouse's number and name (migration 0002); the marketplaces' own
  warehouses are kept nowhere.
- **The neutral specification has no products and no stock** (`docs/fulfilment-centre-api.md`, «What is not here yet»).
- **The menu** (`src/lib/nav/nav-items-data.ts`, `NAV_ITEM_DATA`): the phone's strip holds «Обзор», «Заказы», «Маркетплейсы»,
  «Журнал запросов» and «Настройки» (phone tabs 1 to 5) and «Ещё»; register item 21 (long labels cut on the strip) is open.
- **Generated clients:** Wildberries `api/codegen/wildberries-items.cfg.yaml` lists only `getV3Warehouses` and `getV3Offices`; no
  analytics configuration exists; Ozon's and Яндекс Маркет's list no stock method (STK O, «What the wyo server has today»; STK Y§6).

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **The neutral specification gains the products and the stock exactly as the fulfilment software's released description has
   them:** `GET /api/v1/products` (`updated_since`, `article`, `barcode`, `archived`, `limit` up to 200, `cursor`; each product's `id`,
   `article`, `name`, `barcodes[]` with `kind` `factory` | `internal`, `marketplace_ids[]` of `{ marketplace, id }`, `archived`,
   `has_stock`, `updated_at`), `GET /api/v1/products/{id}`, `GET /api/v1/stock` (`warehouse`, `article`, `changed_since`, `limit` up to
   200, `cursor`; each row's `product_id`, `article`, `warehouse`, `free`, `on_hand`, `reserved`, `quarantine`, `damaged`,
   `processing`, `lots[]`; «With `changed_since` the products whose stock changed after that moment and is now zero come too»; stock
   cannot be written), and the event `stock.changed` (`details.changed[]` of `{ product_id, article, warehouse, free }`, at most 500
   an event). Only the reading calls are copied — the product writes stay out (wyo writes no product). The comparison is made outside
   this repository, as Plan 4's premise 3 (rule R1: nothing here names that software). `internal/fulfilment/fcapi` is regenerated; the
   specification's policy test of Plan 1 still passes.
3. **Each marketplace's description is compared again on the day:** Wildberries' files `02-items.yaml` and `11-analytics.yaml` with
   the live ones (STK W§6), Ozon's file with the newest copy (STK O§7), Яндекс Маркет's tree with the newest commit (STK Y§6). A
   difference in a method of sections B to D stops the stage and asks.
4. **The generators read the methods of sections B to D**, each named by the architecture test of `internal/archtest/rules_test.go`:
   Wildberries `postV2GetCardsList`, `postV3StocksWarehouseId` (the reading `POST` only; the `PUT` waits for part 2, the `DELETE` is
   never generated), and, in a new `api/codegen/wildberries-analytics.cfg.yaml`, `postV1StocksReportWbWarehouses`; Ozon
   `ProductAPI_GetProductList`, `ProductInfoWarehouseStocks`, `GetProductInfoStocksByWarehouseFbo` and the list of Ozon's warehouses
   `/v1/warehouse/ozon/list`; Яндекс Маркет `getStocksOnPartnerWarehouses`, `getStocks`, `getFulfillmentWarehouses`. **No writing
   method of stock or products is generated in this plan** (an architecture test).
5. **Methods Ozon marks as ending are not used:** the field `total` of `/v3/product/list` (switched off 2026-11-23), the methods
   `/v1/product/info/stocks-by-warehouse/fbs` and `/v1/warehouse/list` (switched off 2026-04-07), `/v1/analytics/manage/stocks`
   (STK O§7a).

## A. The contract: two parts made real

1. **`products.read`** gives an account's marketplace products: each one's number (the one its stock is kept by: Wildberries
   `chrtId`, Ozon `product_id`, Яндекс Маркет `offerId`), its other numbers (Wildberries `nmID`; Ozon `sku`; Яндекс Маркет
   `marketSku`), the article, the name, the size (Wildberries' `techSize`), the barcodes, and whether it is archived. An adapter
   declares whether it can read **only the changed products** after a mark (Wildberries, by `updatedAt`, STK W§3) or only the whole
   list (Ozon, Яндекс Маркет). Both `products.read` and `stock.read` go into `supportedNow`.
2. **`stock.read`** gives, for one warehouse, a row per product: **«Доступно»** (what buyers may order now), **«В резерве»**,
   **«Всего»**, **«Едет к покупателям»**, **«Едет обратно»** — each only when the marketplace gives it, never worked out from the
   others — and the marketplace's own words for any further kind (Яндекс Маркет's `QUARANTINE`, `DEFECT`, `EXPIRED`,
   `UTILIZATION`). An adapter declares which of its warehouses are its own and which are the seller's; a reading of a seller's
   warehouse is asked for the catalogue's numbers where the marketplace wants them named (Wildberries).
3. **A product absent from the marketplace's answer has no number** («—» on the page), never 0: whether a marketplace omits a product
   without a record or answers 0 is not documented (STK W§7 gap 5, O§8 gap 10).
4. The one contract suite (`RunAdapterContract`) gains the two parts' cases, run for the three adapters.

## B. Wildberries

- **The catalogue:** `POST content-api.wildberries.ru/content/v2/get/cards/list`, 100 cards a page, sorted by `updatedAt` ascending,
  the reply's cursor kept as the mark for the next reading of changes (STK W§3); a card in the trash is not returned, so a whole
  reading once a day drops the products no longer listed. Limit 80 requests a minute, 750 ms apart (STK W§3).
- **A seller's warehouse:** `POST marketplace-api.wildberries.ru/api/v3/stocks/{warehouseId}` with up to 1000 `chrtIds` of the
  catalogue a request; the answer's `amount` is «Доступно» (STK W§1.3, §5: «Заказы поступают на каждый склад в соответствии с
  остатками, которые вы указали для него»). It shares the 300-a-minute pool of the category «Маркетплейс» with the orders, an answer
  with an error counted as 10 (STK W§1.1).
- **Wildberries' own warehouses:** `POST seller-analytics-api.wildberries.ru/api/analytics/v1/stocks-report/wb-warehouses`, up to
  250 000 rows a request, 3 requests a minute, refreshed by Wildberries every 30 minutes (STK W§4.1): `quantity` is «Доступно»,
  `inWayToClient` «Едет к покупателям», `inWayFromClient` «Едет обратно»; the warehouse's number and name come in each row and are kept.
- **The token:** the category «Контент» opens the catalogue and the category «Аналитика» Wildberries' own warehouses; Plan 3's token
  check reads the categories from the token itself. A token without «Контент» is still accepted: the account's card says «Без категории
  «Контент» wyo не видит товары и остатки ваших складов на Wildberries — добавьте её токену» (the stock of a seller's warehouse can be
  read only by the catalogue's numbers); without «Аналитика»: «Без категории «Аналитика» wyo не видит остатки на складах Wildberries».
- **The hosts** `content-api.wildberries.ru` and `seller-analytics-api.wildberries.ru` join the adapter's declared hosts, so the one
  caller lets them through (principle IX).

## C. Ozon

- **The catalogue:** `/v3/product/list` (up to 1000 a page, `last_id`) for the numbers, then `/v3/product/info/list` (up to 1000
  numbers a request) for the name, the barcodes, `sku` and `is_archived` (STK O§4) — **the same adapter function** that names an
  order's lines today, so there is one reading of Ozon's product details.
- **A seller's warehouse (FBS, rFBS):** `/v1/product/info/warehouse/stocks`, one warehouse, up to 1000 rows a page, by cursor (STK
  O§2.2): `free_stock` is «Доступно», `reserved` «В резерве», `present` «Всего».
- **Ozon's own warehouses (FBO):** `/v1/product/info/stocks-by-warehouse/fbo` by up to 1000 `sku` of the catalogue (STK O§5.2):
  `present` is «Всего», `reserved` «В резерве»; «Доступно» stays empty, because Ozon does not give it there (present minus reserved
  is not stated by Ozon). The warehouses' names from `/v1/warehouse/ozon/list` (STK O§3.3).
- **The key:** Ozon's method list of the key (`/v1/roles`, Plan 5's rule «Метод разрешён, если…») decides each reading; a method
  the key lacks is a note on the account's card naming what will not be shown and Ozon's own refusal words, never a refusal of the
  key. Which role opens each of these methods is not documented (STK O§8 gap 1).
- No method states a limit of its own; Ozon's 50 requests a second for one Client ID applies (STK O, «General limits»).

## D. Яндекс Маркет

- **The catalogue:** `getOfferMappings` (`POST /v2/businesses/{businessId}/offer-mappings`), 100 a page, 100 requests a minute
  (STK Y§4) — the method Plan 7 already reads for an order's lines, through the same adapter function.
- **A seller's warehouse (FBS, Express, DBS) in a cabinet without groups of warehouses:** `getStocksOnPartnerWarehouses`
  (`POST /v3/businesses/{businessId}/offers/stocks`, `partnerWarehouseId`, 100 a page, 500 requests a minute; STK Y§2.1): `AVAILABLE`
  is «Доступно», `FREEZE` «В резерве», `FIT` «Всего» (STK Y§2.2), the other kinds in their own words in the row's details.
- **Яндекс Маркет's own warehouses (FBY):** `getStocks` (`POST /v2/campaigns/{campaignId}/offers/stocks`) of each FBY shop, counted
  by the products returned (100 000 a minute), with each warehouse's number; the names from `getFulfillmentWarehouses`
  (`GET /v2/warehouses`, 100 requests an hour), read once a day (STK Y§2.3, §3).
- **The key:** the level «Просмотр товаров и карточек» or a higher one opens the catalogue and both stock readings (STK Y§2.1, §4) —
  the level Plan 7 already notes for barcodes; a key without it gets the same note, widened to the stock.
- **A cabinet with groups of warehouses** stays as Plan 7 left it (`docs/yandex.md`; register item 40): its warehouses are not read
  for stock, and the account's card says so.

## E. The fulfilment centre's products and stock — only while «Внешний API» is filled

1. **Read through the one fulfilment-centre client** (`internal/fulfilment`), within its limit of 300 requests a minute:
   - **the stock** every 2 minutes with `changed_since` set 5 minutes before the start of the last reading (an overlap: a row read
     twice changes nothing), the whole stock once a day and at once when «Внешний API» is filled; a product whose stock fell to zero
     comes with `free` 0 (premise 2);
   - **the products** every hour with `updated_since`, the whole list once a day.
2. **Kept exactly as they come:** `free`, `on_hand`, `reserved`, `quarantine`, `damaged`, `processing` of each product and
   warehouse; nothing is added, subtracted or rounded (the maintainer: «The fulfillment center's number … as it comes»).
3. **The event `stock.changed` is not asked for:** the reading every 2 minutes is as soon as the event (the fulfilment software makes
   it every 2 minutes) and works in both events modes of Plan 4. A `stock.changed` that arrives by notification is answered `2xx`,
   stored like any event and applies nothing; `docs/external-api.md` says so.
4. **Emptying «Внешний API»** stops every reading of it at once (principle XII) and removes the fulfilment centre's part from
   «Остатки»; its stored numbers are deleted, so nothing stale is ever shown.

## F. Matching a marketplace product to the fulfilment centre's product — one function

ONE function, `MatchProducts`, used here for the fulfilment centre and by part 3 for wyo's own products. For a marketplace product of
one account, the fulfilment centre's products are looked through in three steps, stopping at the first that finds any:

1. **By barcode:** an FC product that has any of the marketplace product's barcodes (either `kind`).
2. **By the marketplace's own number:** an FC product whose `marketplace_ids` holds this marketplace (its code in the fulfilment
   centre's list, `fc_marketplace_map`) with an `id` equal to one of the marketplace product's numbers (Wildberries `chrtId` or
   `nmID`; Ozon `product_id` or `sku`; Яндекс Маркет `offerId` or `marketSku`).
3. **By article:** an FC product whose article equals the marketplace product's article exactly.

The step's result is a **match** only when it finds exactly one FC product **and** no other marketplace product of the same account
finds that FC product at the same step. Otherwise the product is **«неоднозначно»**, with the candidates named — for example two
sizes of one Wildberries card share one article, and neither gets the article's number. Nothing found at all: **«фулфилмент-центр не
знает этот товар»**. The match and how it was found («по штрихкоду», «по номеру маркетплейса», «по артикулу») are kept and made again
after every reading of either side. Part 2 sends 0 for a product without a match (the maintainer: «Writing 0»); this plan only shows it.

## G. How often, and the budget

| What | How often | Source of the period |
| --- | --- | --- |
| The marketplace's catalogue | every hour (Wildberries by its changes, the whole list once a day; Ozon and Яндекс Маркет whole) | this plan |
| Stock in the marketplaces' own warehouses | every 30 minutes | the high-level plan; Wildberries refreshes it every 30 minutes (STK W§4.1) |
| Stock in the seller's warehouses at the marketplaces | every 30 minutes (parts 2 and 3 read the written ones more often) | the high-level plan |
| The fulfilment centre's stock / products | every 2 minutes by changes / every hour by changes; each whole once a day | section E |
| «Обновить» on «Остатки» | at most once in 5 minutes, then the answer says when it may be pressed again | the high-level plan |

Every timer is spread by a few random seconds (rule R5). The hour tests of rule R5 are widened: `TestAnHourAtFullLoadNeverExceedsWildberriesBudget`
with 10 000 sizes on three warehouses beside its orders (300 a minute, an error counted as 10; the catalogue's 80 a minute; the
analytics 3 a minute), `TestAnHourOfReadingsOf500OzonPostingsNeverExceedsOzonsLimit` with 10 000 products, and
`TestAnHourAtFullLoadOf500YandexMarketOrdersNeverExceedsItsLimits` with 10 000 offers (100 and 500 a minute, `getFulfillmentWarehouses`
100 an hour). A reading that changed nothing writes nothing but its record (register item 28's rule).

## H. Product names in order lines (register item 20)

- **Wildberries:** a line is named from the catalogue by its `chrtId` (the card's `title`); a line whose product the catalogue does not
  have yet is named after the next reading of the catalogue.
- **One store function** names every line with an empty name from the catalogue after each reading, for the three marketplaces; the
  Ozon and Яндекс Маркет readings for an order go through the same adapter functions as the catalogue (sections C and D), so no second
  copy of either exists.
- `OrderLines` and `ProductCell` show the name as they already do for Ozon; the Wildberries order list's kind, brand and size stay.
- Without the token's category «Контент» the names stay empty, and the account's card says why (section B).

## I. The owner's view and the instructions

- **«Остатки»** — a new menu item after «Заказы», in «Ещё» on the phone (register item 21: no new tab on the strip). Its intro: «Здесь
  видно, сколько товара лежит на складах маркетплейсов и на ваших складах у маркетплейсов, а если заполнен «Внешний API», то и у
  фулфилмент-центра. wyo здесь только показывает числа и ничего не меняет у маркетплейсов».
- **The list of places** (`StockPlaceList`), in three groups, each card naming the account, the place, how many products have a
  number, the units «Доступно» in all and when it was read (`ReadTime`):
  - «Склады маркетплейсов» — Wildberries', Ozon's FBO and Яндекс Маркет's FBY warehouses, with «?»: «Этим складом управляет сам
    маркетплейс: он сам считает остатки, wyo их только показывает»;
  - «Ваши склады у маркетплейсов» — with «?»: «Здесь маркетплейс продаёт столько, сколько ему сообщили. Сейчас wyo ничего сюда не
    пишет»; a warehouse matched in «Внешний API» names the fulfilment centre's warehouse beside it; a DBS or rFBS warehouse is marked
    «доставляет продавец», as Plans 5 and 7 mark it;
  - «Фулфилмент-центр» (only with «Внешний API» filled) — its warehouses.
- **A place's products** (`StockTable`, one component for every kind of place, its columns given by what the place has): the product
  (`ProductCell`: article, name, size, barcodes), «Доступно», «В резерве», «Всего», «Едет к покупателям», «Едет обратно» — a column
  shown only when the place has it, each with «?» in the marketplace's own words (STK W§4.1, O§2.2, Y§2.2); a fulfilment centre's
  warehouse shows `free` as «Свободно», then «На полках», «В резерве», «Карантин», «Брак», «В обработке». Search by article, name or
  barcode; `Pagination`; a row opens `RowDetailsPanel` with every number the marketplace gave in its own words.
- **A seller's warehouse matched in «Внешний API»** gets two more columns: «У фулфилмент-центра» (its free number) and the match
  (`MatchBadge`: «по штрихкоду», «по номеру маркетплейса», «по артикулу», «неоднозначно» with the candidates in the details,
  «фулфилмент-центр не знает этот товар»), the filters «Расходятся с фулфилмент-центром», «Неизвестны фулфилмент-центру»,
  «Неоднозначны», and a summary line above the table: «N товаров: совпадают X, расходятся Y, неизвестны фулфилмент-центру Z,
  неоднозначны W». Its «?»: «Так вы заранее видите, что получит маркетплейс, если включить передачу остатков: товар, которого
  фулфилмент-центр не знает, получит 0» — the sentence part 2 makes true.
- **Empty states that teach:** no account — «Добавьте аккаунт маркетплейса в разделе «Маркетплейсы»»; a token without «Контент» — the
  card's sentence of section B; a place not read yet — «Ещё не прочитано: wyo читает остатки раз в 30 минут, или нажмите «Обновить»».
- **«Обновить»** (`RefreshButton`, Plan 3) reads every place again at once, at most once in 5 minutes.
- **«Маркетплейсы»** — each account's card gains a line «Товары и остатки» with the notes of sections B to D.
- **`README.md`** gains a section «Остатки»: what each kind of place is; what each number means in each marketplace's words; that wyo
  only shows until the owner chooses otherwise (parts 2 and 3); what the token or key needs for it (Wildberries «Контент» and
  «Аналитика», Ozon's roles as the key's own list shows them, Яндекс Маркет «Просмотр товаров и карточек»); how often each number is
  read; the match and its three ways. Section 12 «Работа с фулфилмент-центром» gains what wyo reads of the fulfilment centre's stock.
  `docs/wildberries.md`, `docs/ozon.md`, `docs/yandex.md` gain each method with its period and limit; `docs/external-api.md` the
  fulfilment centre's two readings and `stock.changed`; `docs/fulfilment-centre-api.md` the products and the stock (premise 2), and
  «What is not here yet» loses them.
- **The constitution (MINOR):** a new principle **XIII «Stock»**: products and stock are read from every marketplace and, while
  «Внешний API» is filled, from the fulfilment centre; the fulfilment centre's numbers are kept as they come; a number the marketplace
  does not give is shown empty and never worked out; one matching function with its three steps and the one-to-one rule; nothing is
  written to a marketplace's stock unless the owner chose it for that warehouse (parts 2 and 3); the tests named.

## J. Rules R2, R3, R4 and R5, proven here

- **R2, neutral:** the specification's policy test passes with the products and the stock; `internal/fulfilment` alone reads them; the
  matching function lives outside every adapter and names no marketplace (the architecture test).
- **R3, alone by default:** with «Внешний API» empty a made-up day of readings sends nothing to the fake fulfilment centre and shows no
  fulfilment-centre part on «Остатки»; nothing is written to any marketplace (`TestNoStockIsWrittenToAnyMarketplaceInThisPlan`: the
  fakes' call logs hold no writing method of stock or products).
- **R4, the keys stay:** the canary test is widened to every new reading — the marketplaces' keys and the fulfilment centre's key in no
  log line, journal row or answer.
- **R5, the limits:** section G's hour tests.

## K. Data, code, tests

**Data — one migration, `<next>_products_stock_reading`:**
- `marketplace_products` (the account, the product's number, its other numbers, article, name, size, barcodes, archived, when read);
- `marketplace_own_warehouses` (the account, the marketplace's number and name of its own warehouse, when read) — kept apart from
  `marketplace_warehouses`, so the reading of the seller's warehouses never touches them;
- `marketplace_stock` (the account, whose warehouse it is — the seller's or the marketplace's —, the warehouse's number, the product's
  number, «Доступно», «В резерве», «Всего», «Едет к покупателям», «Едет обратно», each empty when not given, the other kinds as
  JSON, when read);
- `fc_products` (the fulfilment centre's `id`, article, name, barcodes with their kind, `marketplace_ids`, archived, `updated_at`,
  when read) and `fc_stock` (the product, the fulfilment centre's warehouse code, `free`, `on_hand`, `reserved`, `quarantine`,
  `damaged`, `processing`, when read);
- `product_matches` (the account, the product's number, the side — `fc` here, `own` reserved for part 3 —, the matched product or
  none, how: `barcode` | `marketplace_id` | `article` | `none` | `ambiguous`, the candidates, when made);
- the readings' marks (Wildberries' cursor, the fulfilment centre's last `changed_since` and `updated_since`) in `sync_marks`.
The upgrade test runs from the database snapshot of release 0.0.13, written first, as Plans 5 and 7 did.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/stock/places` (the places with their totals and read times),
`GET /api/v1/stock/places/{place}/products` (search, the filters of section I, cursor), `POST /api/v1/stock/refresh` (the answer
names when it may be asked again).

**Code — where things live:** `internal/marketplace` (the two parts), `internal/marketplace/wildberries`, `…/ozon`, `…/yandex` (each
its readings), `internal/stock` (new: the readings' jobs, the matching function, the store's queries; names no marketplace),
`internal/fulfilment` (the two readings), `internal/fakes/wildberries`, `…/ozon`, `…/yandex` (the catalogue and stock routes),
`internal/fakes/fulfilment` (products and stock, with control calls that set a product's numbers), the interface's
`src/lib/components/stock/` and the route `src/routes/(app)/stock/`.

**Components created** (each once, with its test file in every state): `StockPlaceList` (the three groups of places),
`StockPlaceCard` (one place: account, totals, read time), `StockTable` (a place's products, columns from what the place gives),
`StockNumbers` (one product's numbers with their «?» in the marketplace's words), `MatchBadge` (the five states of a match),
`MatchSummary` (the summary line). **Components reused:** `ProductCell`, `OrderLines`, `WarehouseCell`, `RefreshButton`, `DataList`,
`ColumnHeading`, `ColumnLegend`, `FilterSection`, `Pagination`, `RowDetailsPanel`, `PageHeading`, `HeadingWithHelp`, `HelpTooltip`,
`StatusBadge` with the tones, `NoteBox`, `EmptyState`, `FactList`, `MarketplacesSection`; in Go the one caller, the one budget, the one
journal, the one fulfilment-centre client, the one contract suite, the one reading of a wait header.

**Tests** (written before the code they check, failing first):

- **Go, each adapter against its fake:** the catalogue read whole and by changes, every page, a card in the trash dropped by the whole
  reading; each stock reading of each kind of warehouse, every page; a product absent from an answer kept without a number; each
  missing category, role or level giving its note and nothing else; each documented refusal (STK W§1.3, O§2, Y§2.1) in the
  marketplace's words; the contract suite's new cases for the three adapters.
- **Go, the fulfilment centre:** the stock read by changes with the overlap and whole, a product sold out coming with 0, the products by
  changes; `stock.changed` arriving and applying nothing; «Внешний API» emptied — the readings stop and the numbers are deleted.
- **Go, the matching function,** a table of every case: each step; one-to-one; two FC products at one step; two sizes sharing one
  article; a barcode found at step 1 hiding an article at step 3; none; a match made again after either side changes.
- **Go, names:** a Wildberries line named after the catalogue reading; Ozon's and Яндекс Маркет's lines unchanged; the architecture test
  that the product details of Ozon and Яндекс Маркет are read by one function each.
- **The tests of section J; the hour tests of section G; the upgrade test.**
- **Interface unit tests:** each new component once in every state; the page only for what it adds; the policy tests green, the
  marketplace-name policy among them; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes, each worker with its own data): «Остатки» with a place of each
  kind of each marketplace; a matched warehouse with the five match states and its filters; a token without «Контент» and its notes;
  «Обновить» pressed twice; a Wildberries order's line with its name; every new state swept at 390 × 844 and the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises (the neutral specification's products and stock among them). 2. The migration. 3. The contract's two parts
   and the contract suite. 4. The generated readings and the fakes' routes. 5. The Wildberries, Ozon and Яндекс Маркет readings.
   6. The fulfilment centre's readings. 7. The matching function. 8. The names of order lines. 9. The wyo server's own API. 10. The page
   «Остатки» and the cards. 11. The tests of section J and the hour tests. 12. The README, `docs/`, the help texts, the constitution.
   13. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes): one account of each marketplace; «Остатки»
  with every kind of place; fill «Внешний API», match a Wildberries and an Ozon warehouse, and see the fulfilment centre's numbers and
  the five match states; change a number in the fake fulfilment centre and see it within 2 minutes; a Wildberries order's line named;
  empty «Внешний API» and see its part gone; every page at a computer's width and at 390 × 844 with screenshots; the console, the
  raw-key check and the outside-request check after every screen; the fakes' call logs hold no writing method of stock.
- **Gate:** the test flow of `AGENTS.md` (`pnpm remote:gate` on the Mac mini); the check of rule R1; squash; push; the pull request;
  the merge by the agent; the release run watched to its end; `main` pulled; the report, with «Found while implementing». At the merge
  the register is updated as «Not in this plan» and «After the release» say.

## After the release — the checks with the first real accounts (register)

Only reading calls, so safe on a working shop: whether Wildberries' stock reading omits a size without a record or answers 0 (STK W§7
gap 5); which token types reach `wb-warehouses` (gap 11); which Ozon role opens each new method (STK O§8 gap 1) and whether a row exists
for a product never stocked (gap 10); which of Яндекс Маркет's seven kinds an FBS warehouse returns (STK Y§7 gap 4); the real limit
headers against the budget; with a real fulfilment centre, how its `marketplace_ids` name each marketplace's products (which number of
section F, step 2, it holds). What they find becomes part 2's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Writing any stock** — part 2 (the fulfilment centre's numbers, the choice per warehouse) and part 3 (wyo's own count).
- **A view of one product across all places** («где лежит этот товар») — not asked; later, with Stage 4's planning.
- **The fulfilment centre's products that no marketplace product matches** — not asked; the owner sees them in the fulfilment centre.
- **A match set by hand** — not asked; a wrong or missing match is fixed by the barcode or the article in either system.
- **Prices of orders and orders older than 90 days** (register item 20's other parts) — Stage 3 and not asked.
- **The phone strip's cut labels** (register item 21) — a decision of the maintainer's; «Остатки» takes no tab.
- **Яндекс Маркет cabinets with groups of warehouses** — register item 40 first.
- **A new register item, «The checks of Plan 9 on the first real accounts and the first real fulfilment centre»,** holds «After the
  release».

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — the existing tests stay as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key and no buyer datum in a log, a journal row, an answer or an error.
5. Everything is a component and is reused: one caller, one budget, one journal, one fulfilment-centre client, one contract suite, one
   matching function, one reading of each marketplace's product details, one table component for every place; search before writing;
   the components section K names are created once. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **Nothing is written to a marketplace's stock or products in this plan**, and the fulfilment centre's numbers are kept as they come.
   With «Внешний API» empty nothing is sent anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about a marketplace is
   written only from its official documents (STK), with the address and the date; a gap is marked "not sourced", gets the cautious
   choice, and goes to «After the release». The marketplaces' description files are never edited.
9. An issue found on the way is fixed in this plan at once, or goes to the register with its reason when it cannot be; the report lists
   every finding under «Found while implementing».
10. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's words are shown as it gives them
    beside wyo's sentence; no sentence in Go. Name the program every time. Code is written mobile-first (390 × 844 first).
11. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage; the
    final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

Two contract parts in three adapters, ten generated reading methods, two fulfilment-centre readings, six tables, three addresses of the
wyo server's own API, one page with six components. The matching function's table test and the three widened hour tests are the
largest test work, and the parts that protect a seller later: part 2 writes by the match, so a wrong match there would sell goods that
are not there.
