# Ozon in the marketplace connector, part 1 of 2: the Ozon key of two fields with its access levels and end date, the Ozon adapter that only reads, Ozon orders with several products and quantities in the one order list, the fake Ozon, and the shared parts Wildberries shaped made neutral

> **Plan 5** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 1 of the two Ozon plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md` («Ozon and Яндекс Маркет: keys, the order list, the same exchange with a fulfilment
> centre»).
> Written 2026-11-18 from `origin/main` at `63fac3b` (release 0.0.5); its section «Today» is checked again on the day it starts.
> Research: `.agents/research/ozon-official-seller-api-check-2026-11-18.md` (cited «OZC»: Ozon's own description file and Ozon's table
> of roles, both saved by the maintainer on 2026-11-18); the earlier research the high-level plan cites as «MC» (kept by the maintainer
> outside this repository).
>
> **The plan after it.** Plan 6 `.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md` adds
> the calls that write to Ozon and the exchange of Ozon orders with a fulfilment centre. **This plan writes nothing to Ozon.**
>
> **Branch** `feat/ozon-accounts-orders`. One plan = one branch = one squashed commit = one pull request. The commit is `feat:` —
> never `feat!:` and never a `BREAKING CHANGE:` footer: the release adds a marketplace, and Wildberries accounts and orders work as
> before. **One database migration.** Constitution: principles VI, IX, X and XI widened for a second marketplace, **MINOR**.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the engine**,
  **the journal**, **a mark**, **FBS**, **FBO** — as in Plans 2 and 3.
- **The Ozon key** — two values a seller copies from the Ozon cabinet: the **Client ID** (Ozon's number of the seller's account, sent
  in the header `Client-Id`; not a secret) and the **API key** (the secret, sent in the header `Api-Key`). Ozon shows the API key once,
  when it is made, and it lives 3 months (OZC §2).
- **An access level** (Ozon also says **role**) — what a key may call, chosen when it is made: `Posting FBS read-only`, `Product
  read-only` and others (OZC §3). Ozon's answer to `/v1/roles` lists the key's roles with their methods and the key's end date.
- **A posting** (Ozon's «отправление») — one Ozon FBS or FBO order as Ozon numbers it (`posting_number`, such as `89491381-0072-1`).
  Several postings of one buyer's purchase share an **order number** (`order_number`). In wyo an Ozon posting is one order.
- **SKU** — Ozon's number of a product; the **article** (`offer_id`) is the seller's own code of it.
- **rFBS** — Ozon's scheme where the seller itself delivers to the buyer; it needs the buyer's address.
- **An order line** — one product of an order with its quantity. A Wildberries order always has one line of quantity 1; an Ozon
  posting may have several lines with any quantity.
- **The fake Ozon** — the Go program this plan adds beside the fake Wildberries: it answers as Ozon's description file says, with
  made-up data.

## The maintainer's request and answers — nothing is open

- 2026-11-16, the high-level plan, Stage 2: «Ozon and Яндекс Маркет: keys, the order list, the same exchange with a fulfilment centre.
  Stock: …» (register item 1).
- 2026-11-18: «Ok, let's do the Ozon now? As usually, you can check similar implementation in [the maintainer's other repository].»
  Asked whether one plan or two: «Make it 2».
- 2026-11-18: Ozon refuses its description file to this machine (OZC §1). The maintainer saved it from a browser
  (`docs/marketplaces/swagger.json`), and Ozon's table of roles as a PDF. The file equals the GitHub copy of the same day but for 9
  example values (OZC §1, point 2), so later versions are read from that copy and compared with the committed file.
- Stated with the questions on 2026-11-18, and not objected to: stock is a plan of its own after Яндекс Маркет, because the neutral
  fulfilment-centre API has no stock part yet; FBO orders are read-only; rFBS orders are not read (they need the buyer's address); the
  wyo server never cancels at Ozon; Ozon is read every 5 minutes while «Внешний API» is filled (Plan 6); register items 22 and 28 go
  into the plans.
- 2026-11-17, still true: no marketplace key and no real account — everything is built and tested against the fakes; the checks on a
  real account are listed in «After the release». Russian only, with the languages capability; thorough instructions (section I).

## Today (to be checked again on the day the plan starts)

- After Plan 4 (release 0.0.5) the wyo server reads Wildberries and exchanges Wildberries FBS orders with a fulfilment centre. The
  adapter contract, the engine, the order list and the journal were built for every marketplace, but **twelve things in the shared code
  were shaped by Wildberries** (found by reading the code at `63fac3b`). This plan makes neutral the ones a reading marketplace needs:
  1. **A key is one text.** The secret store keeps one text per account, `Authorize` receives that text, and the key's last four
     characters are taken from it (`internal/sync/check.go:105-122`, `internal/accounts/accounts.go:75-83`).
  2. **An order is one product:** one article, one list of barcodes, no quantity, no lines (`internal/marketplace/model.go:63-82`;
     table `orders`); the deadline is always the order's creation time plus the account's hours.
  3. **The limit headers are Wildberries' own** (`X-Ratelimit-Remaining`, `X-Ratelimit-Retry`, `internal/sync/budget.go:12-20`).
  4. **One lock covers the readings of every account** (`internal/sync/engine.go:66`): a busy account of one marketplace delays the
     others.
  5. **The marketplaces' hosts are one hard-written list** in `internal/outbound/outbound.go:46-56`.
  6. **One fake marketplace on one address:** `WYO_MARKETPLACE_URL` and the flag `--wildberries` of `cmd/wyofakes`.
  7. **Shared texts worded for Wildberries:** `accounts.refusal.key_type_not_accepted`, `…category_missing`, `…key_refused`,
     `…seller_warehouse_off`, `accounts.note.test`, `accounts.note.missing.statistics` in `messages/ru.json`.
  8. **The colour mark table holds one mark** (`src/lib/components/marketplaces/MarketplaceMark.svelte:23`, `src/app.css:105`).
  9. **Shared helpers live inside the Wildberries adapter** (`value`, `number`, `cut`, `succeeded`, `eachHundred` in
     `internal/marketplace/wildberries/`), and the architecture test counts every folder under `internal/marketplace/` as an adapter
     (`internal/archtest/adapters.go:19-31`), so they cannot simply move beside it.
  10. **The architecture lists name Wildberries only:** the owners of generated packages, the «named only in» rule and the list of
      methods never called (`internal/archtest/rules_test.go:20-28, 96-111, 130-170`).
  11. **Tests of rules R3–R5 drive Wildberries only:** `TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader`,
      `TestAnHourAtFullLoadNeverExceedsWildberriesBudget`, the canary tests, the made-up day.
  The twelfth — the writing half of the contract, made of Wildberries' supplies — is Plan 6's.
- **Register item 28:** every reading writes every new or open order again even when nothing changed; this plan touches the engine,
  so it settles it (section C).
- The database of release 0.0.5 (migration `0003`) is not yet in `internal/store/testdata/` (it holds `0001.db`, `0002.db`):
  `docs/development.md` says the plan that adds the next migration writes it first.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Ozon's file is committed as Ozon's.** The file the maintainer saved, `docs/marketplaces/swagger.json`, is moved — unchanged — to
   `api/ozon/seller-api.json` (SHA-256 `57f27f1a…8be2d0f2e3336c084ca295d41d4969a0be8b2f`, OZC §1). `api/ozon/README.md` says where it
   came from, the day, the checksum, and how a later version is taken: download the GitHub copy of that day, read both as JSON,
   compare, read every difference before accepting it. **On the day the plan starts** the GitHub copy of that day is compared with the
   committed file this way; a difference in a method of OZC §5 stops the stage and asks.
3. **The generator reads Ozon's file.** oapi-codegen v2.8.2 (the version of `scripts/tools/versions.sh`) generates
   `internal/marketplace/ozon/ozonapi` from `api/ozon/seller-api.json` with `include-operation-ids` naming exactly this plan's seven
   methods (OZC §5, the rows marked «5»). Ozon's file holds types outside OpenAPI's list (`int`, `bool`, `timestamp`, `enum`, `array of strings`)
   and a server address without a scheme (OZC §1 and §5): **if the generator refuses the file, the stage stops and asks** — Ozon's
   file is never edited.
4. **How a seller makes the key is taken from Ozon's own text:** OZC §2 (section «Auth» of Ozon's file) and OZC §3 (the table of
   roles). Where the cabinet shows the Client ID is not in either (OZC §2): the maintainer saves, as for the description file, the page
   of Ozon's knowledge base that shows it, by a person in a browser; if it cannot be found, the README says only what OZC holds and the
   place goes to «After the release».
5. **The access levels are decided by the key's own list, not by names.** The table of roles is from 2024 and names none of the
   methods this plan calls (OZC §3). So the key check reads the key's methods from `/v1/roles` and asks whether each method the
   adapter needs is allowed: the key has the role `Admin`, or a role lists the method, or a role lists a beginning of its path that
   ends at a `/` (Ozon's example lists `/v1/posting` — OZC §2). The README names the roles whose rows list the earlier versions of the
   methods (section I); a key that lacks a method is refused or marked with the missing level's name.
6. **No sandbox:** Ozon names one environment, the seller's real shop (OZC §2). Nothing of this plan can be tried on Ozon before a
   real account; the checks are in «After the release».
7. **The database of release 0.0.5 is written first:** `go test ./internal/store -run TestWriteTheSnapshotOfTheVersionBeingReleased
   -snapshot=3` before migration `0004` is added (`docs/development.md`, «The database, the commands, the installer»).

## A. The shared parts made neutral

Each change keeps Wildberries working exactly as before; the Wildberries tests that exist stay as they are and must pass unchanged.

1. **A key of several fields.** The contract's `KeyField` already lists a form's boxes. ONE function in `internal/marketplace` packs a
   form's fields into the one text the secret store keeps and unpacks it; a text stored before this release (one field, not packed) is
   read as the first field's value, so every Wildberries account goes on working without touching the secret store. `Authorize`
   receives the unpacked fields. The key's last four characters are taken from the field marked secret (Ozon: the API key), never
   from the Client ID. A field that is not secret is shown in full on the account's card (Ozon: the Client ID — the number that tells
   two Ozon accounts apart). The key form already draws one box per field; a box of a field that is not secret is an ordinary text
   box, without the secret box's rules.
2. **The end of a key from the marketplace's answer.** `KeyFacts.ExpiresAt` may come from the check's answer (Ozon: `expires_at` of
   `/v1/roles`), not only from the key's own text. The states «Ключ истекает через N дней» (14 and 3 days before), «Ключ истёк» and
   their notices are reused unchanged.
3. **The adapter names its limit headers.** `Budget.Observe` reads the header names the adapter declares for a limit group: how many
   requests remain, and how many seconds to wait after `429 Too Many Requests` (Wildberries: `X-Ratelimit-Remaining`,
   `X-Ratelimit-Retry`; Ozon: `Ratelimit-Remaining`, `Retry-After` — OZC §4). The rest of the budget is unchanged.
4. **One lock per account.** The readings of each account are guarded by a lock of their own, so an account that waits for its
   budget no longer holds up the others; the in-place wait of at most 30 seconds and the «come back later» of a longer wait are
   unchanged.
5. **Each adapter names its hosts**; `internal/outbound` builds the one list of marketplace addresses from the registered adapters
   at start, in one place, and still refuses anything else (principle IX). The release-only test that compared one adapter's hosts
   with the hard-written list becomes one test over every adapter.
6. **One address for every fake marketplace.** `cmd/wyofakes` serves the fake Wildberries and the fake Ozon on one listener and tells
   them apart by the first part of the path, which is the marketplace's host — the form every adapter already uses in test builds
   (`marketplace.Origin`). The flag `--wildberries` becomes `--marketplaces` (the same default address); `scripts/dev.sh`,
   `e2e/support/fakes.ts` and `docs/development.md` follow.
7. **The shared texts made neutral.** The interface looks a refusal or a note up first in the adapter's own family
   (`marketplace.<code>.refusal.<code>`, `marketplace.<code>.note.<code>`) and only then in the shared one; the Wildberries-worded
   sentences of «Today», point 7, move to `marketplace.wildberries.*`, and the shared keys keep only neutral words. ONE function of
   `src/lib/marketplaces/refusals.ts` does the lookup, tested once.
8. **The colour mark of Ozon,** `azure`: one theme token (`--color-mark-azure`, with its dark variant) and one entry of the table of
   `MarketplaceMark`, under the contrast rules of `src/app.css`.
9. **Shared adapter helpers move to `internal/adapterkit`** (outside `internal/marketplace/`, so the architecture test does not take
   it for an adapter): the reading of optional values, numbers as text, the cut of a long refusal text, the success test of a status,
   the cutting of a list into requests of N. Both adapters import it; Wildberries' copies are deleted.
10. **The architecture lists become tables of every adapter:** the owner of each generated package (Ozon:
    `internal/marketplace/ozon/ozonapi`, owned by `internal/marketplace/ozon`), the folders allowed to name each marketplace (its
    adapter, its fake, `cmd/wyofakes`, `internal/server/adapters.go`), and the methods never called. The Ozon methods that cancel
    (`PostingAPI_CancelFbsPosting`, `PostingAPI_CancelFbsPostingProduct` — OZC §5) are kept out of the generator's list by a test
    that reads `api/codegen/ozon.cfg.yaml`.

## B. The Ozon adapter that only reads

`internal/marketplace/ozon`, the only package that knows Ozon; `ozonapi/` is generated (Stage 0, premise 3).

- **What it says about itself:** code `ozon`, mark `azure`, key fields `client_id` (not secret) and `api_key` (secret), the
  capabilities `key.check`, `warehouses.read`, `orders.new`, `orders.states`, `orders.feed`; its host `api-seller.ozon.ru`; one limit
  group of 50 requests a second per Client ID (OZC §4), with Ozon's two headers; an answer with an error costs one request (Ozon
  states no other cost).
- **Every request** carries `Client-Id` and `Api-Key` and goes through the one caller (principle XI).
- **Ozon's refusals,** turned into the account's state and a sentence by ONE function of the adapter, with a table-driven test.
  Ozon gives the HTTP status of only one of them (OZC §4), so the function reads Ozon's words first and the status second:

    | Ozon's words                                                    | The account                                                                                   |
    | --------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
    | `Invalid Api-Key, please check the key and try again`, or 401   | «Ключ не принят»; its readings stop                                                            |
    | `Api-key is deactivated, use another one or generate a new one` | «Ключ не принят», with Ozon's words: make a new key                                            |
    | `Api-Key is restricted to specific IP addresses`                | «Ключ не принят»: allow the wyo server's address among the key's networks in the cabinet       |
    | `403 Offer not signed`                                          | «Ключ не принят»: accept Ozon's offer in the cabinet (OZC §2)                                  |
    | `Api-Key is missing a required role for a method`               | the account works; the part that needs the method stops, and the card names the missing level |
    | `Circle is open`, `You have reached request rate limit…`, 429    | a limit met: the reading comes back later (the budget's rules)                                 |

- **`key.check`.** Without calling anybody: the Client ID must be digits only, the API key one line without spaces. Then:
    1. `/v1/roles` — the end date (`expires_at`; an ended key is «Ключ истёк») and the key's methods (Stage 0, premise 5). The methods
       of `orders.new`, `orders.states` and `warehouses.read` are required: a key without them is refused, with the missing level
       named («добавьте ключу уровень доступа Posting FBS read-only»). Without the methods of `orders.feed` (FBO orders) or of the
       products' barcodes the account is accepted and its card says in a sentence what will not be shown and which level to add.
    2. `/v1/seller/info` when the key allows it: the seller's name (`company.name`). When it does not, the card says the name is not
       shown.
    3. `/v2/warehouse/list`, every page.
   The account's profile number is the Client ID, so a key of another Client ID put into an account is refused («это ключ другого
   продавца — добавьте его как новый аккаунт»), as for Wildberries.
- **`warehouses.read`** — `/v2/warehouse/list`. A warehouse Ozon marks `is_rfbs` is stored with the mark «доставляет продавец», and
  no order of it is ever read (rFBS needs the buyer's address — «Not in this plan»); the account's card says how many such warehouses
  there are.
- **`orders.new`** — `/v4/posting/fbs/unfulfilled/list` with the status `awaiting_packaging`, the account's FBS warehouses, and the
  assembly-time filter (`cutoff`) from 180 days before to 180 days after now — the widest window inside Ozon's limit of one year, so no
  waiting posting is missed; pages by `cursor` while `has_next` (OZC §6). No `with` part is asked for.
- **`orders.states`** — `/v3/posting/fbs/get` for each open FBS order (Ozon has no list by posting numbers): the status, the
  cancellation and who made it, and the deadline, which Ozon asks sellers to read again regularly (OZC §6).
- **`orders.feed`** — the FBO orders: `/v3/posting/fbo/list` for the last 30 days, every page, at each reading. Ozon does not say
  whether `since` compares the creation or the last change (OZC §6), so the window is read again whole: a new FBO order is added, a
  known one gets its state; an FBO order older than 30 days keeps the last state read, and its details say so.
- **Barcodes** — `/v3/product/info/list` for the SKUs of new lines whose barcodes wyo does not know yet (up to 1 000 a request —
  OZC §4); a line of a SKU already read takes them from the earlier line. Without the method the lines have no barcode.
- **A posting becomes an order:** the posting number is the order's number, the order number its basket, the warehouse its
  warehouse, `in_process_at` its creation time, `shipment_date` its deadline, `parent_posting_number` the order it was split from;
  each product is a line (SKU, article, name, quantity, barcodes).
- **No buyer data is stored.** Ozon's lists carry the buyer's name, phone, e-mail, address and comment in every answer (OZC §6). The
  adapter drops them at its edge, and a test proves none of them reaches the database, the journal or a log.
- **Ozon's states on the connector's own** (the table; Ozon's words are shown beside):

    | State         | On the screen        | Ozon's FBS statuses (OZC §6)                                                                                                   |
    | ------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
    | `new`         | «Новый»              | `awaiting_packaging`, `awaiting_approve`, `awaiting_verification`                                                              |
    | `assembling`  | «На сборке»          | `awaiting_deliver`, `awaiting_registration`                                                                                    |
    | `in_delivery` | «В доставке»         | `acceptance_in_progress`, `driver_pickup`, `delivering`, `not_accepted`, `arbitration`, `client_arbitration`                    |
    | `received`    | «Получен покупателем»| `delivered`                                                                                                                    |
    | `cancelled`   | «Отменён» with whom  | `cancelled` (`seller` → the seller; `client`, `customer` → the buyer; `ozon`, `system`, `delivery` → the marketplace); `cancelled_from_split_pending` (Ozon's own words beside: «отменён из-за разделения отправления», and the parts' numbers in the details) |
    | `unknown`     | the received words   | any status the table does not hold                                                                                             |

   FBO statuses: `awaiting_packaging` → `new`, `awaiting_deliver` → `assembling`, `delivering` → `in_delivery`, `delivered` →
   `received`, `cancelled` → `cancelled`.

## C. The engine

- **Register item 28 — an order is written only when something of it changed.** The intake compares what it read with the stored
  order (its state, its deadline, its lines) and writes nothing when they are equal; an event of the order's history is written only on
  a change, as today. A test counts the writes of a reading that changed nothing: none.
- The periods of the order list stay those of Plan 3 for every marketplace (30 minutes; «Обновить» at most once in 5 minutes). The
  periods of the exchange are Plan 6's.
- A failure of one account never stops another's readings (section A, point 4); five failures in a row make that account alone «Нет
  связи».

## D. Orders with several products

- **Order lines:** the table `order_lines` holds every order's products; an order is shown and searched by its lines. Every existing
  order becomes one line in the migration (quantity 1), and the product columns of `orders` leave it — one place for a product of an
  order, never two.
- **The marketplace's own deadline:** `orders.ship_by` holds the deadline when the marketplace gives one (Ozon: `shipment_date`), and
  the deadline column uses it; when the marketplace gives none (Wildberries), the deadline is the creation time plus the account's
  hours, as today. The setting «Срок передачи заказа, часов» is shown only for an adapter that declares it gives no deadline.
- **The page «Заказы»:** the product cell shows the first line and «ещё N товаров»; a quantity above 1 is shown beside a line; the
  search finds an order by any line's article, barcode or SKU; the details beside the list show every line (`OrderLines`), the order
  a split order came from, or the parts it was split into. Wildberries' orders look as before (one line, no quantity shown).
- **What the exchange sends is unchanged for Wildberries:** the order sent to a fulfilment centre is built from the lines — one line
  of quantity 1, the same request as today, proven by the exchange's existing tests, which stay as they are. **Ozon orders are not
  sent in this release:** an account whose adapter claims none of the exchange's parts is shown in «Внешний API» with the sentence
  «Обмен с фулфилмент-центром для этого маркетплейса пока не поддерживается», and its orders' column «Обмен» says «не передаётся» with
  that reason.

## E. The fake Ozon

`internal/fakes/ozon`, built on `fakekit` like the fake Wildberries, from Ozon's description file only:

- **Exactly the adapter's routes** — a test ties every route of the adapter to a method of `api/ozon/seller-api.json` and to a route
  of the fake (the Wildberries form of `TestEveryRouteIsAMethodOfWildberriesOwnDescriptionFiles`, made one test over every adapter).
- **A state per key:** the pair Client ID and API key (`fakekit.Options.Key`); each key has its roles with their methods and its end
  date, set by a control call.
- **Ozon's answers:** the error body `{code, message, details}` with Ozon's words of OZC §4. The HTTP status of each word is the
  fake's choice, written in its code as «the fake's choice, not Ozon's» (Ozon does not give them): 401 for an invalid key, 403 for
  the other key words. The limit of 50 a second per Client ID on the injected clock, `Ratelimit-Remaining` on every answer,
  `Retry-After` on 429.
- **Postings** with several products and quantities, every status of OZC §6, a deadline, and **made-up buyer data in every answer**
  (name, phone, address, comment), so that the dropping of section B is proven against it; FBO postings; products with barcodes;
  warehouses with one rFBS among them.
- **Control calls:** setup (the seller, warehouses, a key's roles and end date), a new posting, a status change with who cancelled, a
  split made in the cabinet (the original `cancelled_from_split_pending`, the parts new with `parent_posting_number`), a new deadline,
  a new FBO posting.
- **Register item 22, its first half — a fault «refuse with this answer»:** the kit gains a fault kind that answers one route with a
  given status and body, a given number of times. The fake Ozon's tests use it, and it serves the fake Wildberries too. The fake
  fulfilment centre's transport boxes and the end of the exchange tests' own scripted refusals are Plan 6's (it touches the exchange).

## F. Rules R3, R4 and R5, proven for both marketplaces

- **R3, alone by default:** the made-up day of `TestAMadeUpDayReachesOnlyTheMarketplacesOfTheAccounts` runs with a Wildberries and
  an Ozon account: only their hosts appear, with «Внешний API» empty; with no Ozon account, Ozon's host never appears.
- **R4, the key stays:** the canary test is widened to an Ozon account — a distinctive API key is saved and every job of this plan
  runs against the fake Ozon with every kind of failure; the key must be in no log line, no journal row, no answer of the wyo server's
  own API, no error text, and the fake Ozon's own call log proves it went only into the `Api-Key` header.
- **R5, nothing hidden:** the header test and the test of an hour at full load become tables over every registered adapter with its
  fake: every request names the program; the budget is never exceeded at this plan's periods with 500 open Ozon orders.

## G. Data, code, tests

**Data — one migration, `0004_order_lines_ozon`:** `order_lines` (the order, the line's number, the marketplace's product number, the
article, the name, the barcodes, the kind, the brand, the size, the quantity of at least 1), filled with one line for every existing
order from its own columns, which are then dropped from `orders`; `orders` gains `ship_by` (the marketplace's deadline, empty when it
gives none) and `parent_number` (the order a marketplace split it from); `marketplace_warehouses` gains `seller_delivers`. The upgrade
test runs from `0003.db` (Stage 0, premise 7) with Wildberries orders and proves each became one line with the same product.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/marketplaces` gains each key field's «secret» mark and whether the adapter
gives deadlines; `GET /api/v1/orders` and `GET /api/v1/orders/{id}` give `lines[]`, `shipBy`, the split's parent and parts;
`GET /api/v1/accounts` gives the shown key fields and the notes of missing access. No new address.

**Code — where things live:** `internal/marketplace` (the packed key, the declared headers and hosts), `internal/adapterkit` (the
shared helpers), `internal/marketplace/ozon` (the adapter; `ozonapi/` generated), `internal/fakes/ozon`, `internal/sync` (the headers,
the lock per account, the write only on a change), `internal/orders` (the lines), `internal/outbound` (the list built from the
adapters); `api/ozon/`, `api/codegen/ozon.cfg.yaml`, `scripts/generate.sh` (the `GENERATED` list); in the interface
`src/lib/components/orders/`, `src/lib/components/marketplaces/`, `src/lib/marketplaces/refusals.ts`.

**Components created** (each once, each with its test file in every state): `OrderLines` (an order's products with their quantities;
Plan 6 reuses it in the exchange's details), `ProductCell` (the first line and «ещё N товаров» — the list's product cell, used by the
order list and, in Plan 6, by «Передачи»). **Components reused:** `KeyForm`, `KeyHelpSteps`, `AccountCard`, `AccountStateBadge`,
`MarketplaceMark` (with its new mark), `OrderDetails`, `OrderStateBadge`, `SchemeBadge`, `DeadlineCell`, `DataList` with
`ColumnHeading` and `ColumnLegend`, `FilterSection`, `RowDetailsPanel`, `DisclosureForm`, `Field`, `TextBox`, `NoteBox`, `StatusBadge`
with the tones, `EmptyState`, `PageHeading`, `Stack`, the bell.

**Tests** (written before the code they check, failing first):

- **Go:** the contract test suite run for the Ozon adapter against the fake Ozon
  (`TestTheAdapterKeepsTheContractAgainstTheFake` for `ozon`); the packed key (two fields; an old unpacked text read as the first
  field; the last four characters from the secret field only); every row of the refusals table; the check (each required method
  missing; FBO or barcodes missing as notes; an ended key; another Client ID refused; the seller's name absent); the roles' rule
  (`Admin`, an exact method, a beginning of a path at a `/`, a near miss such as `/v1/post` for `/v1/posting/…` refused); warehouses
  with an rFBS one; new postings (several lines; the same posting twice is one order; buyer data nowhere); every row of the states
  table and an unknown status; a split made in the cabinet; a new deadline; the FBO window; the barcodes read once per SKU; the
  declared headers in the budget (Ozon's `Retry-After`, Wildberries' unchanged); the lock per account (a waiting Ozon account does not
  delay a Wildberries one); a reading that changed nothing writes nothing; the hosts built from the adapters; the generator list
  without the cancel methods; the migration from `0003.db`; the tests of section F.
- **The fake Ozon's own tests** for each route and control call, and the kit's «refuse» fault.
- **Interface unit tests:** each new component once; `refusals.ts`' lookup; the key form with a box that is not secret; each page only
  for what it adds; `marketplace-name-policy`; the policy tests green; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes on one address, each worker with its own data): an Ozon account
  added with a made-up Client ID and key and its card (the Client ID in full, the key's last four characters only); each refusal's
  sentence; a key without the FBO level shows its note; an Ozon order of three lines appears after «Обновить» and its details show
  the lines; a split made in the fake shows the parts; a cancellation shows with whom; the Wildberries specs pass unchanged; every new
  or changed page swept at 390 × 844 and the wider widths.

## H. Measured

The memory measurement of Plan 3 (`pnpm go:measure`: three accounts, 10 000 orders, a made-up day) runs with two Wildberries
accounts and one Ozon account whose orders have three lines on average; the README's requirements are rewritten with the new number
and the date.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration, the lines and the stores. 3. The shared parts of section A (each with Wildberries' tests
   green after it). 4. The generated Ozon client and the fake Ozon. 5. The Ozon adapter: the key check, warehouses, the refusals.
   6. Orders: new, states, the FBO window, barcodes, the states table, the split. 7. The engine: the write only on a change, the lock.
   8. The interface: the key form, the card, the lines in «Заказы», the texts. 9. The tests of section F and the measurement. 10. The
   README, `docs/`, the help texts, the constitution. 11. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes on one address): add an Ozon account and a
  Wildberries account; see both cards (the Ozon Client ID in full, both keys by their last four characters only); make an Ozon
  posting of three lines and a Wildberries order in the fakes and press «Обновить»; open both orders' details; split the Ozon posting
  in the fake and read again; cancel one; filter by an article of the second line; open the journal; every page at a computer's width
  and at 390 × 844 with screenshots; the console, the raw-key check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release
  run watched to its end; `main` pulled; the report. At the merge the register's items are moved and added as «Not in this plan»
  and «After the release» say.

## After the release — the checks with the first real Ozon account (register)

The format of `/v1/roles`' method list and which role opens each method this plan calls; the HTTP status of each key error; where the
cabinet shows the Client ID; what `since` and `to` of the FBO list compare (then the FBO window may become a mark); whether the
unprocessed list holds a posting the buyer cancelled before packing; how a split made in the cabinet shows; the real limit headers and
the budget's agreement with them (OZC §10, points 1–5). Only reading calls are made, so the check is safe on a working shop. What they
find becomes Plan 6's premises.

## I. The instructions

- **`README.md` gains «Подключение Ozon»**, step by step for a seller, written from OZC §2 and §3 (premise 4): what the Client ID and
  the API key are and why wyo needs both; in the Ozon cabinet «Настройки → Seller API» → «Сгенерировать ключ»; a name for the key
  («wyo»); the access levels to tick for this release — `Posting FBS read-only` (the FBS orders), `Posting FBO` (the FBO orders),
  `Product read-only` (the barcodes), `Warehouse` (the warehouses) — and that `Admin read only` also works; the purpose «для личного
  пользования», because the wyo server is the seller's own program on its own server and nobody else ever gets the key (rule R4; Ozon's
  text gives no rule for the choice, OZC §2); that Ozon shows the key only once; pasting both into «Маркетплейсы» → «Добавить аккаунт»;
  what the card then shows; that the key lives 3 months and how wyo warns; how to replace it; that a key limited to networks must
  allow the server's address; that Ozon's offer must be accepted. Then what «Заказы» shows of an Ozon order (the lines, the deadline
  Ozon sets, the split), that rFBS warehouses are not read and why, and what wyo sends to Ozon in this release: nothing but questions.
- **The same in the interface:** `KeyHelpSteps` with the steps `marketplace.ozon.step.<n>`; a «?» on the new column contents and
  notes — by the constitution's help-text rules.
- `docs/ozon.md`: the methods wyo calls and how often, for a seller's engineer (the form of `docs/wildberries.md`).
- `docs/development.md`: the fakes on one address, the fake Ozon's control calls, how Ozon's file is updated (`api/ozon/README.md`).
- **The constitution, 1.4.0 (MINOR):** principle VI names Ozon's file and the comparison of its later versions; principle IX — the
  hosts are declared by the adapters and listed in one place; principle X — keys of several fields, order lines, the adapter's
  declared headers and hosts, its own refusals and notes; principle XI — the lock per account, the write only on a change.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Every call that writes to Ozon, and the exchange of Ozon orders with a fulfilment centre** — Plan 6.
- **Stock** (register item 1's second half): a plan of its own after Яндекс Маркет, with the stock part of the neutral API first.
- **Яндекс Маркет** — the plans after Plan 6.
- **rFBS orders** (Ozon's orders the seller delivers itself): they need the buyer's address, which wyo does not store — register
  item 19 widened to Ozon.
- **Ozon's notifications to a public address** (its «пуш-уведомления»): Ozon itself says the notice of a new order may come late and
  the list must be read regularly (`#tag/push_types` of Ozon's file); wyo reads the lists.
- **Ozon's product cards and prices:** the lines show the name and barcodes; cards are the contract part `products.read` of a later
  stage — register item 20 widened to Ozon.
- **FBO orders older than 30 days** keep their last state (section B); revisited after the real check of `since`.
- **A new register item, «The checks of Plan 5 on the first real Ozon account»,** holds «After the release».

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — Wildberries' existing tests stay exactly as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, no buyer's name, phone, address or comment in a log, a journal row, an
   answer or an error.
5. Everything is a component and is reused: one adapter contract, one contract test suite, one caller, one budget, one journal, one
   order list, one packing of a key, one lookup of a refusal's text; shared adapter helpers in `internal/adapterkit`, never copied; the
   components section G names are created once. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **This plan sends Ozon only reading requests.** No test, no development run and no pipeline run calls the real Ozon; a fact about
   Ozon is written only from its official documents (OZC), with the address and the date; a gap is marked "not sourced", gets the
   cautious choice, and goes to «After the release». Ozon's file is never edited.
8. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in its own words
   beside wyo's sentence; no sentence in Go. Name the program every time.
9. Rules R1–R7 of the high-level plan bind every file. Code is written mobile-first (390 × 844 first).
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

One new table and three new columns, no new address of the wyo server's own API, two small components, one adapter with five reading
parts, one fake, and eleven shared places made neutral. The shared places are most of the risk: each is changed under Wildberries'
own tests, which must stay green unchanged. A comparable earlier piece of work grew several times beyond its first estimate once its
tests, texts and documents were counted; the stages above are cut so that each ends green.
