# Яндекс Маркет in the marketplace connector, part 1 of 2: the Яндекс Маркет key and its access levels, the adapter that only reads (FBS, Express and FBY orders of every shop of one cabinet), Яндекс Маркет's notifications that make the wyo server read just the changed order, a safety read every 15 minutes so no order is ever missed, test orders read, marked and counted nowhere, the fake Яндекс Маркет, and the shared parts Ozon and Wildberries shaped made neutral

> **Plan 7** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 1 of the two Яндекс Маркет plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md` («Ozon and Яндекс Маркет: keys, the order list, the same exchange with a fulfilment
> centre»).
> Written 2026-11-19 from `origin/main` at `bfa9e24` (release 0.0.8, after Plan 6a). **It starts after the fixes of register item 38**
> (branch `fix/ozon-exchange-open-items-of-review`, not merged on 2026-11-19; it adds migration `0006_pack_sent_at.sql`) are merged, and
> its section «Today» is checked again on that day.
> Research: `.agents/research/yandex-market-official-partner-api-check-2026-11-19.md` (cited «YMC»: Яндекс Маркет's own documents and
> its own description file, read on 2026-11-19); the earlier research the high-level plan cites as «MC» (kept by the maintainer outside
> this repository).
>
> **The plan after it.** Plan 8 `.agents/plans/wyo-yandex-market-fbs-and-express-exchange-with-a-fulfilment-centre-codes-labels-daily-shipment-courier-code.md`
> adds the calls that write to Яндекс Маркет and the exchange of its orders with a fulfilment centre. **This plan writes nothing to
> Яндекс Маркет.**
>
> **Branch** `feat/yandex-market-accounts-orders`. One plan = one branch = one squashed commit = one pull request. The commit is `feat:`
> — never `feat!:` and never a `BREAKING CHANGE:` footer: the release adds a marketplace, and Wildberries and Ozon accounts, orders and
> exchanges work as before. **One database migration**, the next free number at the start (`0007`, since the fixes of item 38 take
> `0006`). Constitution: principles VI, X and XI widened, the policy test of the 1.6.0 rule built, **MINOR** (1.7.0 on 2026-11-19).

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused". **An issue
found on the way is fixed in this plan at once** (constitution 1.6.0, «Plan Implementation Workflow», step 2), and the report lists it
under «Found while implementing».

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the engine**,
  **the journal**, **a mark**, **an order line** — as in Plans 2, 3 and 5. **«Внешний API»** and **the exchange** — as in Plan 4.
- **The Яндекс Маркет key** — the «Api-Key-токен» a seller makes in the Яндекс Маркет cabinet, sent in the header `Api-Key`. It never
  expires and opens every shop of one cabinet (YMC §2). One field, secret.
- **A cabinet** (Яндекс Маркет's «кабинет», `businessId`) — the seller's whole Яндекс Маркет account. **A shop** (its «магазин»,
  `campaignId`) — one of the cabinet's shops; a cabinet may have several. **In wyo an account is a cabinet**, and its shops are listed on
  the account's card.
- **An access level** (Яндекс Маркет's «доступ», `authScopes`) — what a key may call, ticked when it is made: «Просмотр информации о
  заказах» (`INVENTORY_AND_ORDER_PROCESSING_READ_ONLY`), «Обработка заказов и учёт товаров» (`INVENTORY_AND_ORDER_PROCESSING`), «Просмотр
  товаров и карточек» (`OFFERS_AND_CARDS_MANAGEMENT_READ_ONLY`) and others (YMC §2).
- **The schemes:** **FBS** — the seller packs, Яндекс Маркет collects the day's parcels; **Express** («Экспресс») — the seller packs and
  a Яндекс Go courier takes one order within hours; **FBY** — the goods lie in Яндекс Маркет's warehouses and the seller only reads;
  **DBS** — the seller delivers to the buyer itself (YMC §6, §13).
- **A test order** — an order the seller makes in the cabinet to try an integration; Яндекс Маркет marks it `fake: true`, takes no
  money, and counts no mistake with it (YMC §9).
- **A notification** (Яндекс Маркет's «API-уведомление») — the `POST` Яндекс Маркет sends to an address the seller enters in the cabinet
  when an order appears or changes (YMC §8).
- **The safety read** — the reading of every order changed since the previous reading, made on a timer even while notifications work,
  so that a lost notification never loses an order.
- **The fake Яндекс Маркет** — the Go program this plan adds beside the fake Wildberries and the fake Ozon: it answers as Яндекс Маркет's
  description file says, with made-up data, and sends notifications.

## The maintainer's request and answers (2026-11-19) — nothing is open

- 2026-11-16, the high-level plan, Stage 2: «Ozon and Яндекс Маркет: keys, the order list, the same exchange with a fulfilment centre»
  (register item 1).
- «write the plan for Яндекс Маркет.» Asked whether one plan or two: «2». This is the first; Plan 8 is the second.
- **Notifications and reading:** «Let's have both? If possible - first suggest how, so that extra pulls did not take place, as the YM
  push notification is a great way to pass data only when it is needed. On the other hand - yes, the system must not miss any orders,
  so, probably, the pull must take place at least once in 15 minutes». The proposal — a notification makes the wyo server read just
  that order; a safety read every 15 minutes asks only for the orders changed since the previous one; without notifications the wyo
  server reads every 5 minutes — was answered «yes». Read here as: every 5 minutes while «Внешний API» is filled, as Ozon (the
  high-level plan, «How often it asks»); with it empty, the order list's 30 minutes of Plan 3. He asked to check that the fulfilment
  software can push too: its seller API already sends signed notifications, and the wyo server already receives them (Plan 4).
- **Test orders:** «Test orders are marked by YM as fake. You must pass them to [the maintainer's fulfilment software], so they could
  be tested, but both wyo and [the maintainer's fulfilment software] must not take them into account in terms of analytics and other
  aspects of software that may skew the results»; «The wyo and [the maintainer's fulfilment software] must both mark such orders as FAKE
  and make it visible in UI, so users could see it. Test orders ARE NOT PAID of course!» So: always read (no switch), marked in every
  place an order shows, counted in no total (section D); Plan 8 passes them on with the test mark.
- **Express:** «The plan 8 must handle the Express orders fully.» This plan reads and shows them.
- **The daily shipment, the label size, the courier's code** — Plan 8 (answered there). **The fulfilment software's side** — «Write a
  full [the maintainer's fulfilment software] plan»: its plan adds the label size, the courier-code orders and the test mark to its
  seller API; Plan 8 starts after it is released.
- Stated with the questions and not objected to: FBY orders are read-only; DBS orders are not read (they need the buyer's address);
  the wyo server never cancels an order and never removes goods from one; barcodes are shown only when the key has the level «Просмотр
  товаров и карточек».
- 2026-11-17, still true: no real account — everything is built and tested against the fakes; the checks on a real account are listed
  in «After the release». Russian only, with the languages capability; thorough instructions (section I).

## Today (to be checked again on the day the plan starts)

After Plan 6a (release 0.0.8) the wyo server reads Wildberries and Ozon and exchanges both with a fulfilment centre. **Ten things in the
shared code do not fit Яндекс Маркет** (found by reading the code at `bfa9e24` and the working tree of the item-38 fixes):

1. **A met limit is `429` only.** `Budget.Observe` stops a group only on `http.StatusTooManyRequests` (`internal/sync/budget.go:93`),
   `journal.OutcomeOf` calls only 429 «лимит» and every other 4xx «отказ» (`internal/journal/journal.go:73-77`), and the caller repeats
   only a «лимит» (`internal/sync/caller.go:192-198`). Яндекс Маркет answers a met limit with **`420 Enhance Your Calm`** (YMC §4).
2. **The wait is whole seconds:** `headerInt` (`budget.go:117-128`) reads a number; Яндекс Маркет gives a date
   (`X-RateLimit-Resource-Until`, RFC 822, YMC §4), which today becomes one second.
3. **No limit on requests at once.** The budget is a token bucket per account, group and host (`budget.go:16`); accounts run side by
   side (`internal/sync/engine.go:360-387`), and the lock per account covers readings only. Яндекс Маркет allows at most four requests
   at once (YMC §4).
4. **An error costs the same whatever its kind** (`LimitGroup.ErrorCost`, `internal/marketplace/contract.go:139-146`). Яндекс Маркет
   counts a client error and not a server error (YMC §4).
5. **Reading happens only on a timer:** the job `orders` every 30 minutes, and the exchange's jobs at the adapter's periods
   (`engine.go:32, 117, 293-316`). There is no way to read the orders a notification names, and no contract part for it.
6. **Only the fulfilment centre may notify the wyo server:** `POST /hooks/fulfilment/{text}` (`api/wyo.yaml:631`,
   `internal/httpapi/external.go:343-366`, `internal/exchange/notify.go:24-67`) checks a Standard Webhooks signature, which Яндекс Маркет
   does not send.
7. **No order is a test order:** no flag in `marketplace.Order` (`internal/marketplace/model.go:80-96`), in `orders` or in the
   wyo server's own API (`api/wyo.yaml:851`); the home page's numbers (`CountOrdersByState`, `CountOverdueOrders`,
   `internal/store/queries/orders.sql:105, 111`) would count one.
8. **An account is one profile number with warehouses;** Яндекс Маркет's account is a cabinet with several shops, and a warehouse
   belongs to shops through its models (YMC §6). `SchemeBadge` knows FBS and FBO only (`src/lib/components/orders/SchemeBadge.svelte`).
9. **The generator corrects descriptions only** (`internal/specfix/specfix.go:34-44`). Яндекс Маркет's description is a tree of 1 087
   files, on which oapi-codegen v2.8.2 stops with a stack overflow; joined into one file it generates (YMC §1, point 3).
10. **A marketplace's name in the program header:** every request names the program in `User-Agent` (principle XI); Яндекс Маркет asks
    for its own header `X-Market-Integration: <name>/<version>` too (YMC §3).

Also: the registry takes a code of letters only (`internal/marketplace/registry.go:28`, `^[a-z]+$`), so the code is `yandex`; the
wyo server gets its own certificate from Let's Encrypt (`internal/tlsauto`) and sees the caller's real address (`r.RemoteAddr`), which
the notifications need (section C). **Register item 33** (the fulfilment-centre client ignores the neutral API's `Retry-After`) sits in
the same function as points 1 and 2, so this plan closes it.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **The fixes of item 38 are merged and released**, and `main` holds migration `0006`; otherwise the stage waits.
3. **Яндекс Маркет's file is committed as Яндекс Маркет's.** The tree `openapi/` of `https://github.com/yandex-market/yandex-market-partner-api`
   at the commit of YMC §1 is copied unchanged to `api/yandex/` (licence file beside it, BSD-3-Clause). `api/yandex/README.md` gives the
   address, the commit, the day, the SHA-256 of `openapi.yaml`, and how a later version is taken: the repository's newest commit is
   compared with the committed tree (`git diff` of the two trees), and every difference in a path or schema the adapter uses is read
   before it is accepted. **On the day the plan starts** that comparison is made; a difference in a method of YMC §5 stops the stage
   and asks.
4. **The generator reads the joined copy.** `pnpm generate` joins the tree into one temporary file — a new rule of `internal/specfix`,
   «join»: every `$ref` to a file `components/<kind>/<Name>.yaml` becomes `#/components/<kind>/<Name>`, nothing else changes, and a
   reference that is neither stops the generation — and oapi-codegen v2.8.2 generates `internal/marketplace/yandex/yandexapi` from it
   with `include-operation-ids` naming exactly this plan's methods (YMC §5: `getAuthTokenInfo`, `getCampaigns`, `getBusinessOrders`,
   `getPartnerWarehouses`, `getOfferMappings`). Proven once on 2026-11-19 (YMC §1, point 3); if it fails on the day, the stage stops
   and asks — Яндекс Маркет's file is never edited.
5. **The database of the release before is written first:** `go test ./internal/store -run TestWriteTheSnapshotOfTheVersionBeingReleased
   -snapshot=6` before migration `0007` is added (`docs/development.md`, «The database, the commands, the installer»).
6. **How a seller makes the key and connects notifications is taken from Яндекс Маркет's own text** (YMC §2, §8). No real cabinet is
   available yet, and no cabinet page is ever committed: the repository is public.

## A. The shared parts made neutral

Each change keeps Wildberries and Ozon working exactly as before; their tests stay as they are and must pass unchanged.

1. **The limit's own status and its wait.** `LimitGroup` gains the status by which its marketplace says a limit is met (`429` when not
   set; Яндекс Маркет `420`) and the form of its wait header: seconds (as today) or a date. ONE function reads the wait from either form
   (a date in the past or unreadable is one second, as today); `Budget.Observe`, `journal.OutcomeOf` and the caller's decision take the
   group's status, so a `420` of Яндекс Маркет is «лимит» in the journal and comes back later by the budget's rules. **Register item
   33:** the fulfilment centre's group names `Retry-After` (seconds), with its test.
2. **The cost of an error by its kind.** `LimitGroup` declares what a client error and a server error cost (Яндекс Маркет: 1 and 0 —
   YMC §4); not declared, both cost `ErrorCost`, as today.
3. **At most N requests at once per account.** `Description` gains the number of requests an account may have in flight together
   (Яндекс Маркет 4; not set — no limit, as today). ONE limiter per account, around the caller's send, holds the readings, the
   exchange and the hand-over alike; a request waits for a free place in the 30-second deadline and never goes over.
4. **A program header of the marketplace's own.** `Description` may name a header that carries `wyo/<version>`, ASCII only, at most 100
   characters (Яндекс Маркет: `X-Market-Integration`, YMC §3); the caller sets it on every request of that adapter, including a request
   without the key, beside `User-Agent`.
5. **Every order changed since a time — a new contract part `orders.changes`.** One request gives new and changed orders together (Яндекс
   Маркет's list filtered by the time of the last change, YMC §7). An adapter that claims it is read with it in place of `orders.new`
   and `orders.states`; the engine keeps the time of the last good reading per account and asks from 10 minutes before it, because
   Яндекс Маркет warns that its times and the seller's may differ (YMC §8). The write only on a change (principle XI) makes the overlap
   cost nothing.
6. **The orders a notification names — a new contract part `orders.named`:** reads the given order numbers (Яндекс Маркет: up to 50 a
   request, YMC §7) and returns them as `orders.changes` does.
7. **A marketplace's notifications — a new contract part `notifications.receive`** (section C): the adapter turns a notification's body
   into the order numbers it names and builds the answer the marketplace wants; it declares the networks its notifications come from.
8. **The test mark of an order** (section D): `marketplace.Order.Test`, stored in `orders.test`, given by the wyo server's own API.
9. **Shops of an account.** `marketplace_shops` holds an account's shops (number, name, scheme, whether its API is off), read at the key
   check and with the warehouses; an account with one shop and no shops of its own (Wildberries, Ozon) has none, and its card shows
   nothing new.
10. **The architecture lists** gain the row of `yandex`: its generated package's owner (`internal/marketplace/yandex/yandexapi`), the
    folders allowed to name it (its adapter, its fake, `cmd/wyofakes`, `internal/server/adapters.go`), and the methods never generated:
    `updateOrderItems`, `acceptOrderCancellation`, `transferOrdersFromShipment` — read from `api/codegen/yandex.cfg.yaml` by a test — and
    the cancelling status of `updateOrderStatus`, refused by a test of Plan 8, which first generates that method.
11. **The colour mark of Яндекс Маркет,** `yellow`: one theme token (`--color-mark-yellow`, with its dark variant) and one entry of
    `MarketplaceMark`, under the contrast rules of `src/app.css`.

## B. The Яндекс Маркет adapter that only reads

`internal/marketplace/yandex`, the only package that knows Яндекс Маркет; `yandexapi/` is generated (Stage 0, premise 4).

- **What it says about itself:** code `yandex`, mark `yellow`, one key field `api_key` (secret), the capabilities `key.check`,
  `warehouses.read`, `orders.changes`, `orders.named`, `notifications.receive`; its host `api.partner.market.yandex.ru`; the program header
  `X-Market-Integration`; at most 4 requests at once; the limit status `420`, the headers `X-RateLimit-Resource-Remaining` (what is left)
  and `X-RateLimit-Resource-Until` (the date); a client error costs 1, a server error 0. **Limit groups, one per method, each of its
  own hourly amount** (YMC §5): `getAuthTokenInfo` 100 an hour, `getCampaigns` 1 000 an hour, `getBusinessOrders` 10 000 an hour,
  `getPartnerWarehouses` 100 an hour, `getOfferMappings` 100 a minute.
- **Every request** carries `Api-Key` and goes through the one caller (principle XI).
- **Яндекс Маркет's refusals,** turned into the account's state and a sentence by ONE function of the adapter, with a table-driven test.
  The body is `{ status: "ERROR", errors: [{ code, message }] }` (YMC §4):

    | Яндекс Маркет's answer                                                            | The account                                                                                         |
    | -------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
    | `401` «Api-Key token is invalid», «Api-Key token is revoked», any other `401`     | «Ключ не принят»; its readings stop                                                                  |
    | `403` «Token does not have any of the scopes to access the API method»            | the account works; the part that needs the method stops, and the card names the level to add         |
    | `403` «API for campaign … disabled»                                               | that shop is marked «API магазина выключен» on the card; the others are read                         |
    | `420`                                                                             | a limit met: the reading comes back after the date of `X-RateLimit-Resource-Until`                   |
    | `5xx`                                                                             | a server error: a reading is repeated after 1 and 3 seconds (principle XI); it costs nothing         |

- **`key.check`.** Without calling anybody: the key is one line without spaces. Then:
    1. `getAuthTokenInfo` (`POST /v2/auth/token`) — the key's levels. One of `INVENTORY_AND_ORDER_PROCESSING_READ_ONLY`,
       `INVENTORY_AND_ORDER_PROCESSING`, `ALL_METHODS_READ_ONLY`, `ALL_METHODS` is required: a key without them is refused with the level
       named («добавьте ключу доступ «Просмотр информации о заказах»»). Without `OFFERS_AND_CARDS_MANAGEMENT(_READ_ONLY)` or an
       `ALL_METHODS` level the account is accepted and its card says the barcodes will not be shown and which level to add. A key whose
       levels only read is marked «ключ только для чтения» — the mark Wildberries' and Ozon's read-only keys get — because Plan 8 writes.
    2. `getCampaigns` (`GET /v2/campaigns`, every page) — the shops and their cabinet (`business.id`). The account's profile number is
       the cabinet's number, so a key of another cabinet put into an account is refused («это ключ другого кабинета — добавьте его как
       новый аккаунт»), as for Wildberries and Ozon.
    3. `getPartnerWarehouses` (`POST /v3/businesses/{businessId}/warehouses`, every page).
- **`warehouses.read`** — `getPartnerWarehouses`: each warehouse's number, name and models (scheme `FBS`, `EXPRESS` or `DBS`, and the
  shop). A warehouse of DBS only is stored with the mark «доставляет продавец» (`marketplace_warehouses.seller_delivers`) and no order
  of it is read; an Express warehouse gets the mark «Экспресс». The card says how many of each there are.
- **`orders.changes`** — `getBusinessOrders` (`POST /v1/businesses/{businessId}/orders`) with `programTypes` `FBS`, `EXPRESS`, `FBY`, the
  last change from the time the engine gives, every page of 50 (`pageToken`). **Twice:** with `fake: false` and with `fake: true`,
  because the list gives test orders only when asked and asks for one kind at a time (YMC §7, §9). DBS and LaaS orders are not asked for.
- **`orders.named`** — the same method with `orderIds` (up to 50) and each `fake` value in turn: an order missing from the first answer
  is asked for in the second. Whether `orderIds` alone would return a test order is not sourced (YMC §14), so both are asked.
- **Barcodes** — `getOfferMappings` (`POST /v2/businesses/{businessId}/offer-mappings`, up to 100 articles a request, 100 a minute) for
  the articles of new lines whose barcodes wyo does not know yet; a line of a known article takes them from the earlier line. Without
  the level the lines have no barcode.
- **An order becomes a wyo order:** `orderId` is the order's number; `creationDate` its creation time; the shipment's day and time
  (`delivery.shipment.shipmentDate`, `shipmentTime`) its deadline — read as Moscow time, which is not sourced (YMC §14) and is checked on
  the real account; `programType` its scheme; `fake` its test mark; the shop its shop; each item a line (`id`, the article `offerId`, the
  name `offerName`, the quantity `count`, whether a marking code is required from `requiredInstanceTypes`). **The warehouse:** the
  warehouse whose model names the order's shop with the order's scheme; when several do, the order's own text `delivery.warehouseId`
  is kept in its details and the warehouse is left empty with the note «склад не определён» — which of Яндекс Маркет's numbers matches
  is not sourced (YMC §14, point 1).
- **No buyer data is stored.** The list carries the buyer's name, phone, address and comment (`buyer`, `delivery.address`, `notes`). The
  adapter drops them at its edge, and a test proves none of them reaches the database, the journal or a log.
- **Яндекс Маркет's states on the connector's own** (the table; Яндекс Маркет's words are shown beside, from its description file):

    | State         | On the screen        | Яндекс Маркет's status and substatus (YMC §7)                                                                    |
    | ------------- | -------------------- | ------------------------------------------------------------------------------------------------------------- |
    | `new`         | «Новый»              | `PROCESSING` + `STARTED` («заказ подтвержден, его можно начать обрабатывать»); `UNPAID`, `PENDING`              |
    | `assembling`  | «На сборке»          | `PROCESSING` + `READY_TO_SHIP` («заказ собран и готов к отправке»)                                               |
    | `in_delivery` | «В доставке»         | `PROCESSING` + `SHIPPED`; `DELIVERY`; `PICKUP`                                                                  |
    | `received`    | «Получен покупателем»| `DELIVERED`                                                                                                     |
    | `cancelled`   | «Отменён» with whom  | `CANCELLED`: `SHOP_FAILED` → the seller; `USER_…`, `RESERVATION_EXPIRED` → the buyer; any other reason → the marketplace |
    | `unknown`     | the received words   | any status the table does not hold (`PARTIALLY_RETURNED`, `RETURNED`, `UNKNOWN` among them)                       |

## C. Notifications and the safety read

1. **The address.** Each Яндекс Маркет account gets, when it is added, its own address of the wyo server:
   `https://<the wyo server's domain>/hooks/marketplace/<text>`, where `<text>` is a random text made and kept as the fulfilment
   centre's hook text is (`rand.Text()`, `internal/exchange/setting.go:240-247`) and compared in constant time
   (`internal/exchange/notify.go:26-27`). The card shows it with `CopyButton` and the steps of
   section I. Its access entry is `AccessMarketplaceHook` (`internal/httpapi/access.go`), and `api/wyo.yaml` lists the address.
2. **Who may call it.** Only a caller whose address is inside the networks the adapter declares (Яндекс Маркет: `5.45.207.0/25`,
   `141.8.142.0/25`, `5.255.253.0/25`, YMC §8); anything else, a wrong text, or a body over 256 KB is answered `404`, so the address tells
   a stranger nothing. A test build lets the fake call from `127.0.0.1`; the release build's test proves it does not.
3. **The answer at once.** The body is read and the notification **stored before anything else** (`marketplace_notifications`: the
   account, the time, the kind, the order numbers; no body is kept), then answered `200` with the answer the adapter builds (Яндекс
   Маркет: `{ "version": "<wyo version>", "name": "wyo", "time": "<now>" }`), well inside the 1 second of a `PING` and the 10 seconds of the
   others. A body the adapter cannot read is answered `400` with its reason, as Яндекс Маркет asks; nothing is answered `500` for a
   later failure, because Яндекс Маркет then holds back every other notification until that one is answered (YMC §8). The reading is
   done afterwards (the store-then-kick of the fulfilment centre's notifications).
4. **What is read.** `ORDER_CREATED`, `ORDER_UPDATED`, `ORDER_STATUS_UPDATED`, `ORDER_CANCELLED` → `orders.named` for their numbers,
   at once, within the budget; several notifications of one order within a few seconds make one reading. `PING` marks the account's
   notifications «подключены». Every other kind (returns, cancellation requests of DBS, reviews, chats) is answered `200` and ignored.
   A notification of an FBY order (a notification set up for the whole cabinet sends them too, YMC §8) is read like any other.
5. **The safety read** (`orders.changes`): **every 15 minutes while notifications work; otherwise every 5 minutes while «Внешний API»
   is filled and every 30 minutes while it is empty** (Plan 3's period). Notifications **work** from the first `PING` or notification
   received for the account; they **stop working** when a safety read finds a change older than 10 minutes that no notification named —
   then the card says «Уведомления не доходят» with what to check in the cabinet, the bell shows a yellow line, and the 5- or 30-minute
   reading returns until the next notification arrives. «Обновить» stays at most once in 5 minutes.
6. **The budget.** At the busiest — 500 open orders, a notification for each change and the safety reads — the readings stay far inside
   `getBusinessOrders`' 10 000 an hour; a test of a made-up hour proves it.

## D. Test orders — marked everywhere, counted nowhere

- **Always read**, from the lists and from notifications, with no switch (the maintainer's answer). Stored with `orders.test = 1`.
- **Marked everywhere an order shows:** a new component `TestOrderBadge` — «Тестовый заказ» with its «?» («Этот заказ создан продавцом в
  кабинете маркетплейса для проверки. Покупателя нет, деньги не списываются, маркетплейс не учитывает ошибки по нему. wyo показывает и
  передаёт его, но не включает ни в одно число на главной странице.») — in the order list's row beside `SchemeBadge`, in the details'
  heading, and (Plan 8) in «Передачи» and the bell's lines.
- **Counted in no total:** the home page's numbers by state and overdue (`CountOrdersByState`, `CountOverdueOrders`) and every later
  report or summary leave test orders out, and the home page says so under its numbers when the account has any. **They stay where the
  owner acts on an order:** the order list (with the filter «Только тестовые» / «Без тестовых» of `FilterSection`) and the list «Требуют
  внимания» — a test order that needs the owner is shown there, marked, because trying the whole walk is what it is for.
- **The rule is written once:** principle X gains «a test order is counted in no total», with its policy test
  `TestATestOrderIsCountedInNoTotal` (`internal/store`: every query of `internal/store/queries/` whose name begins with `Count` or
  `Sum` over `orders` filters `test = 0`, or is on the test's allow-list with its reason — the list «Требуют внимания» among them).

## E. The fake Яндекс Маркет

`internal/fakes/yandex`, built on `fakekit` like the other fakes, from Яндекс Маркет's description file only:

- **Exactly the adapter's routes** — the test that ties every route of an adapter to a method of its committed description and to a
  route of its fake (Plan 5) gains the row of `yandex`.
- **A state per key:** each key has its levels, its cabinet, shops (one with API off on command), warehouses (FBS, Express, DBS), and
  orders.
- **Яндекс Маркет's answers:** the error body of YMC §4 with Яндекс Маркет's words; the hourly amounts of section B on the injected
  clock; `X-RateLimit-Resource-Remaining` on every answer; `420` with `X-RateLimit-Resource-Limit`, `X-RateLimit-Resource-Until` (a date)
  and `X-RateLimit-Resource-Remaining`; `420` also for a fifth request at once; a `5xx` on command, which uses up nothing.
- **Orders** of FBS, Express and FBY, with several lines and quantities, every status and substatus of section B, a shipment day and
  time, some test orders (`fake: true`, returned only when asked for), and **made-up buyer data in every answer**, so the dropping of
  section B is proven against it.
- **Notifications:** the fake sends `POST` to the address set by a control call, from `127.0.0.1`, for each new order and each change
  (one `PING` when the address is set); on command it loses the next N notifications (to prove the safety read finds what they named),
  sends one twice, or sends a kind wyo ignores.
- **Control calls:** setup (the cabinet, shops, warehouses, a key's levels), a new order (of a scheme, test or not), a status change, a
  cancellation with its reason, the notification address and its faults, a shop's API off. **The kit's fault «refuse with this answer»**
  (Plan 5) serves it too.

## F. Rules R3, R4 and R5, proven for three marketplaces

- **R3, alone by default:** the made-up day of `TestAMadeUpDayReachesOnlyTheMarketplacesOfTheAccounts` runs with a Wildberries, an Ozon
  and a Яндекс Маркет account: only their hosts appear, with «Внешний API» empty; with no Яндекс Маркет account, its host never appears;
  a notification is the only call that comes in, and it starts no call anywhere but to Яндекс Маркет.
- **R4, the key stays:** the canary test is widened to a Яндекс Маркет account — a distinctive key is saved and every job of this plan
  runs against the fake with every kind of failure; the key must be in no log line, no journal row, no answer of the wyo server's own
  API (the notification answer among them), no error text, and the fake's own call log proves it went only into the `Api-Key` header.
  The notification address's text is shown only to the signed-in owner and is in no log line or journal row.
- **R5, nothing hidden:** the header test becomes a table over every adapter with its fake: every request names the program in
  `User-Agent`, and every Яндекс Маркет request also in `X-Market-Integration: wyo/<version>`; the test of an hour at full load runs
  with 500 open Яндекс Маркет orders and the budget is never exceeded.

## G. Data, code, tests

**Data — one migration, `0007_yandex_shops_notifications_test_orders`** (the number is the next free one at the start): `orders` gains
`test` (0 or 1, default 0); `marketplace_shops` (the account, the shop's number, name, scheme, API off); `marketplace_accounts` gains
the notification address's text (kept as the fulfilment centre's hook text is), when notifications were first seen and when they were
last found not arriving; `marketplace_warehouses`
gains `express`; `marketplace_notifications` (the account, the time, the kind, the order numbers, when read; kept 30 days, like the
journal). The upgrade test runs from `0006.db` (Stage 0, premise 5) and proves every existing order has `test = 0`.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/marketplaces` gains whether the adapter receives notifications and its
program header; `GET /api/v1/accounts` gives the shops, the notification address (to the owner only) and its state;
`GET /api/v1/orders` and `GET /api/v1/orders/{id}` give `test` and the scheme `express`/`fby`, and take the filter `test`; the new
address `POST /hooks/marketplace/{text}`.

**Code — where things live:** `internal/marketplace` (the new parts, the declared limit status, wait form, error costs, requests at once,
program header, notification networks, the test mark), `internal/marketplace/yandex` (the adapter; `yandexapi/` generated),
`internal/fakes/yandex`, `internal/sync` (the limiter at once, the wait from a date, the changed-since reading, the named reading, the
safety read's periods), `internal/journal` (the outcome by the group's status), `internal/httpapi` and `internal/notify` (new: the
marketplace's notifications stored and kicked; ONE store-then-kick function shared with the fulfilment centre's hook, moved out of
`internal/exchange/notify.go`), `internal/specfix` and `tools/specfix` (the rule «join»), `internal/store` (the queries with `test = 0`);
`api/yandex/`, `api/codegen/yandex.cfg.yaml`, `scripts/generate.sh` (the `GENERATED` list); in the interface
`src/lib/components/orders/`, `src/lib/components/marketplaces/`.

**Components created** (each once, each with its test file in every state): `TestOrderBadge` (the test mark with its «?»; Plan 8 reuses
it in «Передачи» and the bell's lines), `NotificationAddressCard` (an account's notification address with `CopyButton`, its state —
«не подключены», «подключены», «не доходят» — and the steps to connect them), `ShopList` (an account's shops with the scheme and «API
магазина выключен»). **Components reused:** `KeyForm`, `KeyHelpSteps`, `AccountCard`, `AccountStateBadge`, `MarketplaceMark` (its new
mark), `CopyButton`, `OrderDetails`, `OrderLines`, `ProductCell`, `OrderStateBadge`, `SchemeBadge` (gains Express and FBY), `DeadlineCell`,
`DataList` with `ColumnHeading` and `ColumnLegend`, `FilterSection`, `RowDetailsPanel`, `NoteBox`, `StatusBadge` with the tones,
`EmptyState`, `PageHeading`, `Stack`, `RefreshButton`, the bell.

**Tests** (written before the code they check, failing first):

- **Go:** the contract test suite run for the Яндекс Маркет adapter against its fake (`TestTheAdapterKeepsTheContractAgainstTheFake`
  for `yandex`), with the cases of the new parts `orders.changes`, `orders.named` and `notifications.receive`; every row of the refusals
  table; the check (each required level missing; barcodes missing as a note; a read-only key marked; another cabinet refused; a shop
  with API off); warehouses with DBS and Express ones; new and changed orders (several lines; the same order twice is one order; test
  orders asked separately; buyer data nowhere); every row of the states table and an unknown status; the warehouse found by the shop and
  left empty when several match; barcodes read once per article; a `420` with a date waits until it, Wildberries' and Ozon's waits
  unchanged, the fulfilment centre's `Retry-After` read (item 33); a fifth request at once waits for a place; a server error costs
  nothing; the program header on every request; the notification address (a wrong text, an outside network, a large body: `404`; a
  `PING` answered with the version inside 1 second; an unreadable body `400`; an order notification read at once and its order changed;
  two notifications of one order make one reading; a kind wyo ignores answered `200`); the safety read's periods with notifications
  working and not, and the switch to «не доходят» when a lost notification's order is found; the join rule of `internal/specfix`
  (a reference to a file that is not a component stops it); the generator list without the never-called methods; the migration from
  `0006.db`; `TestATestOrderIsCountedInNoTotal`; `TestEveryPlanReportListsWhatWasFoundWhileImplementing` (constitution 1.6.0: every
  report of `.agents/reports/` written from this plan on has the section «Found while implementing»; the reports before it are the
  allow-list, each with the reason «written before the rule»); the tests of section F.
- **The fake Яндекс Маркет's own tests** for each route, each control call and each notification fault.
- **Interface unit tests:** each new component once; `SchemeBadge` with its new values; each page only for what it adds;
  `marketplace-name-policy`; the policy tests green; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes on one address, each worker with its own data): a Яндекс Маркет
  account added with a made-up key and its card (the key's last four characters only, the shops, the notification address with its
  copy button); each refusal's sentence; a key without the products level shows its note; an FBS order of three lines appears after a
  notification **without «Обновить»**, and its details show the lines; a test order appears marked in the row and the details and is
  missing from the home page's numbers; an Express and an FBY order show their schemes; lost notifications make the card say «Уведомления
  не доходят» after the safety read; a cancellation shows with whom; the Wildberries and Ozon specs pass unchanged; every new or changed
  page swept at 390 × 844 and the wider widths.

## H. Measured

The memory measurement of Plan 3 (`pnpm go:measure`) runs with a Wildberries, an Ozon and a Яндекс Маркет account whose orders have
three lines on average, with notifications for every change; the README's requirements are rewritten with the new number and the date.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration and the stores. 3. The shared parts of section A (each with Wildberries' and Ozon's tests
   green after it). 4. The join rule, the generated client and the fake Яндекс Маркет. 5. The adapter: the key check, shops, warehouses,
   the refusals. 6. Orders: changes, named, the states table, barcodes, the warehouse. 7. Notifications and the safety read. 8. Test
   orders: the mark, the counts, the policy test. 9. The interface: the key form, the card, the address, «Заказы», the home page, the
   texts. 10. The tests of section F, the measurement, the report policy test. 11. The README, `docs/`, the help texts, the
   constitution. 12. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes on one address): add a Яндекс Маркет account
  beside an Ozon and a Wildberries one; see the cards (keys by their last four characters only, the shops, the address); set the fake's
  notification address to the one shown; make an FBS order of three lines, an Express order and a test order in the fake and see them
  arrive without «Обновить»; open their details; lose the next notifications and see the safety read bring the order and the card say
  «Уведомления не доходят»; cancel one; filter «Только тестовые»; open the home page and the journal; every page at a computer's width
  and at 390 × 844 with screenshots; the console, the raw-key check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release run
  watched to its end; `main` pulled; the report, with «Found while implementing». At the merge the register's items are moved and
  added as «Not in this plan» and «After the release» say.

## After the release — the checks with the first real Яндекс Маркет cabinet (register)

Only reading calls are made, and test orders made in the cabinet, so the check is safe on a working shop: which of Яндекс Маркет's
numbers ties an order to its warehouse (YMC §14, point 1); the time zone of the shipment's day and time; whether Яндекс Маркет accepts
the wyo server's Let's Encrypt certificate for notifications (YMC §14, point 2) and calls from the three networks only; the
`PING` and the order notifications as they really come; whether `orderIds` alone returns a test order; the real limit headers and the
budget's agreement with them; how a shop with API off answers. What they find becomes Plan 8's premises.

## I. The instructions

- **`README.md` gains «Подключение Яндекс Маркета»**, step by step for a seller, written from YMC §2 and §8 (premise 6): what the key
  is and why wyo needs it (one key opens every shop of one cabinet and never expires); in the cabinet: account icon → «Настройки» → «API
  и модули» → «Токены авторизации» → «Создать новый токен»; only the cabinet's owner or manager can make one; the levels to tick for this
  release — «Просмотр информации о заказах» (the orders) and «Просмотр товаров и карточек» (the barcodes), and for Plan 8 «Обработка
  заказов и учёт товаров»; pasting it into «Маркетплейсы» → «Добавить аккаунт»; what the card then shows. **Then the notifications:**
  why they matter (an order reaches wyo within seconds instead of up to 5 minutes, and wyo asks Яндекс Маркет less); copying the address
  from the card; in the cabinet: «API и модули» → «API-уведомления» → «Подключить уведомления»; a name; «кабинет» (all shops); the types
  «создание заказа», «изменение заказа», «изменение статуса заказа», «отмена заказа»; pasting the address; «Подключить»; that they start
  within 2 minutes; that the card then says «подключены»; that wyo still reads every 15 minutes, and what «Уведомления не доходят» means
  and what to check. **Then test orders:** «API и модули» → «Тестовый заказ»; that wyo shows them marked and counts them nowhere; that
  they are deleted after 10 days. Then what «Заказы» shows of a Яндекс Маркет order (the lines, the schemes, the deadline it sets), that
  DBS orders are not read and why, and what wyo sends to Яндекс Маркет in this release: nothing but questions.
- **The same in the interface:** `KeyHelpSteps` with the steps `marketplace.yandex.step.<n>`, the steps of `NotificationAddressCard`, a
  «?» on every new column content, mark and note — by the constitution's help-text rules.
- `docs/yandex.md`: the methods wyo calls and how often, the notifications and the safety read, for a seller's engineer (the form of
  `docs/ozon.md`).
- `docs/development.md`: the fake Яндекс Маркет's control calls and notifications, how Яндекс Маркет's file is updated
  (`api/yandex/README.md`), the join rule.
- **The constitution, 1.7.0 (MINOR):** principle VI names Яндекс Маркет's tree, its comparison by commit, and the join rule of the
  temporary copy; principle X — the parts `orders.changes`, `orders.named`, `notifications.receive`, the declared limit status, wait
  form, error costs, requests at once and program header, shops, and «a test order is counted in no total»
  (`TestATestOrderIsCountedInNoTotal`); principle XI — the limiter at once, the safety read and its periods; «Where code lives» gains
  `internal/marketplace/yandex`, `internal/fakes/yandex`, `internal/notify`, `api/yandex/`; the 1.6.0 rule names its now-built test.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Every call that writes to Яндекс Маркет, and the exchange of its orders with a fulfilment centre (FBS and Express)** — Plan 8.
- **DBS orders** (the seller delivers to the buyer itself): they need the buyer's address, which wyo does not store — register item 19
  widened to Яндекс Маркет. **LaaS orders** — not asked for; the same item.
- **Notifications about returns, cancellation requests, reviews and chats** — answered and ignored; returns belong to a later stage.
- **Ozon's notifications** (register item 32) stay open: Ozon says its notice may come late; the shared part `notifications.receive`
  of this plan is ready for them if the reading proves too slow.
- **Product cards and prices of Яндекс Маркет** — register item 20 widened. **Stock** — register item 29.
- **A new register item, «The checks of Plan 7 on the first real Яндекс Маркет cabinet»,** holds «After the release». Register item 33
  closes at the merge.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — Wildberries' and Ozon's existing tests stay exactly as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, no notification address's text, no buyer's name, phone, address or comment
   in a log, a journal row, an answer or an error.
5. Everything is a component and is reused: one adapter contract, one contract test suite, one caller, one budget, one limiter at once,
   one journal, one order list, one reading of a wait header, one store-then-kick of a notification, one lookup of a refusal's text,
   one test mark; shared adapter helpers in `internal/adapterkit`, never copied; the components section G names are created once.
   Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **This plan sends Яндекс Маркет only reading requests.** No test, no development run and no pipeline run calls the real Яндекс Маркет;
   a fact about it is written only from its official documents (YMC), with the address and the date; a gap is marked "not sourced",
   gets the cautious choice, and goes to «After the release». Яндекс Маркет's file is never edited.
8. An issue found on the way is fixed in this plan at once, test first, or goes to the register with its reason when it cannot be; the
   report lists every finding under «Found while implementing» (constitution 1.6.0).
9. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in its own words
   beside wyo's sentence; no sentence in Go. Name the program every time. Code is written mobile-first (390 × 844 first).
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

One new table for shops, one for notifications, five new columns, one new address of the wyo server, three components, one adapter
with five parts, one fake that also sends notifications, and five shared places made neutral. The limiter at once and the wait from a
date sit in the one caller every marketplace and the fulfilment centre use, so they carry most of the risk; each is changed under the
other adapters' unchanged tests. Plans 5 and 6 each grew several times beyond their first estimate once tests, texts and documents were
counted; the stages above are cut so that each ends green.
