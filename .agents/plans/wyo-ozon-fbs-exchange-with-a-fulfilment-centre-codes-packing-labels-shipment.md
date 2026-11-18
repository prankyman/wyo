# Ozon in the marketplace connector, part 2 of 2: the whole Ozon FBS exchange through the neutral fulfilment-centre API — each new posting sent, the marking codes reported to Ozon with its verdict, the packing reported with the fulfilment centre's parcels (splitting the posting when there are several), each parcel's label fetched after packing, the day's shipment made with exactly the hand-over's postings and its documents sent, cancellations passed on — and the writing half of the adapter contract made neutral

> **Plan 6** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 2 of the two Ozon plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`, whose section «The exchange with a fulfilment centre» says «Ozon and Яндекс Маркет
> follow the same six steps through their adapters; there the label exists only after packing is reported».
> Written 2026-11-18 from `origin/main` at `63fac3b`; **it starts only after Plan 5 is merged and released**
> (`.agents/plans/wyo-ozon-key-reading-adapter-orders-with-several-products-fake-ozon.md`), and its section «Today» is checked again on
> that day.
> Research: `.agents/research/ozon-official-seller-api-check-2026-11-18.md` (cited «OZC»); `docs/fulfilment-centre-api.md` and
> `api/fulfilment-centre.yaml` (the neutral specification); Plan 4
> (`.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`), whose
> exchange this plan widens.
>
> **Branch** `feat/ozon-fbs-exchange`. One plan = one branch = one squashed commit = one pull request. The commit is `feat:` — never
> `feat!:` and never a `BREAKING CHANGE:` footer: with «Внешний API» empty, which is the default, nothing changes for anybody, and the
> Wildberries exchange works as before. **One database migration.** Constitution: principle XII widened to a marketplace whose label
> follows the packing, **MINOR**.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the engine**,
  **the journal** — as in Plans 2 and 3. **A fulfilment centre**, **«External API»** («Внешний API»), **the neutral API**, **an
  event**, **a held order**, **a hand-over**, **a marking code**, **the exchange** — as in Plan 4. **The Ozon key**, **an access
  level**, **a posting**, **SKU**, **an order line** — as in Plan 5.
- **A parcel** — one closed box of an order at the fulfilment centre (`parcels[]` of the neutral API); an order may have several.
- **Reporting the packing** (Ozon's «сборка», the method `/v4/posting/fbs/ship`) — telling Ozon that a posting is packed and how its
  products lie in parcels. **Several parcels split the posting:** Ozon makes one posting of each parcel's contents (OZC §7), each
  with its own number and its own label.
- **An exemplar** — Ozon's record of one unit of a product in a posting, to which its marking code is attached (OZC §7).
- **A shipment** (Ozon's «отгрузка», also «перевозка»; methods `/v1/carriage/*`) — Ozon's list of the postings of one day and one
  warehouse that leave together. When it is approved, Ozon gives its documents — the **shipping list and the waybill** («лист
  отгрузки и транспортная накладная», one PDF) and the **hand-over barcode**, which the receiving point scans (OZC §9).
- **A batch** — the adapter contract's neutral word for a marketplace's group of orders handed over together: Wildberries' supply,
  Ozon's shipment.

## The maintainer's request and answers — nothing is open

- 2026-11-16, the high-level plan: the connector «serves as a channel between the marketplaces … and [a fulfilment centre's system]»;
  Stage 2 holds «the same exchange with a fulfilment centre» for Ozon.
- 2026-11-18: «Ok, let's do the Ozon now?»; two plans («Make it 2»); this is the second.
- Stated with the questions on 2026-11-18, and not objected to: an order packed in several parcels is split at Ozon by the packing
  report itself, and the split is checked on a real account later; the wyo server never cancels at Ozon; Ozon is read every 5 minutes
  while «Внешний API» is filled (the high-level plan, «How often it asks»); register item 22 goes into the plans; identifiers other
  than marking codes become a problem on the order (register item 24).
- 2026-11-17, still true: no real account — the exchange is built and tested only against the fake Ozon and the fake fulfilment
  centre; the checks on real ones are listed in «After the release».

## Today (to be checked again on the day the plan starts)

- After Plan 5 the wyo server reads Ozon: accounts with the two-field key, FBS and FBO postings with their lines in the one order list,
  the journal; Ozon accounts are shown in «Внешний API» as «пока не поддерживается».
- **The writing half of the adapter contract is Wildberries' supply** (found by reading the code at `63fac3b`;
  `internal/marketplace/writing.go`, `internal/exchange`, `internal/handover`):
  1. **Confirming is putting into a batch:** `orders.confirm` is `NewBatch`, `Batch`, `PutInBatch`, run at the fulfilment centre's
     `order.reserved`, and **the label is fetched straight after** (`internal/exchange/confirm.go:169`). Ozon has no confirmation, and
     its label exists only after the packing is reported (OZC §8).
  2. **One label per order** (`exchange_orders.label_print`, parcel 1 only), and **the contract's text promises a picture of
     58 × 40 mm** (`internal/marketplace/writing.go:120`). **The contract suite demands a PNG and a scan text** for every label and
     document (`internal/marketplace/contracttest/writing.go:144-145, 303-307`) — the neutral API takes PDF, PNG or JPEG and the scan
     text is optional (`docs/fulfilment-centre-api.md`, «The label that comes afterwards»).
  3. **The hand-over runner needs all five writing parts, stored shipping settings and a chosen point,** and waits for ever on
     `settings_missing` for an adapter without them (`internal/handover/steps.go:23-44, 80, 254-261`). Its six steps are fixed.
  4. **A new batch is empty,** so the step «move» only adds orders; Ozon's new shipment takes in **every** ready posting (OZC §9).
  5. **The exchange's periods are one set for every marketplace** (`internal/exchange/exchange.go:41-47`: new orders every minute).
  6. **The format of a batch's name is written twice** (`internal/exchange/confirm.go:13`, `internal/handover/handover.go:58`) — a
     copy, which this plan, touching both, turns into one.
- **Register item 22, its second half:** the fake fulfilment centre never names transport boxes, and the exchange's tests wrap the
  fake Wildberries with scripted refusals (`internal/exchange/rig_test.go`, `refuse`); Plan 5 gave the fakes the fault «refuse with
  this answer».

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Ozon's file is compared again on the day** with the GitHub copy of that day (`api/ozon/README.md`); a difference in a method of
   OZC §5 marked «Plan 6» stops the stage and asks.
3. **The generator reads the writing methods.** `api/codegen/ozon.cfg.yaml` gains the seventeen methods of OZC §5 (the rows marked «6»).
   Among them `/v2/posting/fbs/act/get-postings`, whose request field `id` has the type `int`, outside OpenAPI's list (OZC §5): if the
   generator refuses it, the stage stops and asks — Ozon's file is never edited. The cancel methods stay out (Plan 5's test).
4. **The findings of Plan 5's checks with a real account, if they exist by then,** replace the cautious choices of this plan;
   otherwise the choices stand and stay in «After the release».
5. **The neutral specification still equals the format the maintainer provides** — the one-time comparison of Plan 1, section I,
   made again outside this repository, as Plan 4's premise 3.
6. **A seller's key must be allowed to write:** the key's own methods from `/v1/roles` (Plan 5) must allow `/v4/posting/fbs/ship`,
   the exemplar methods, the label methods and the shipment methods of OZC §5; otherwise the account is marked «ключ только для
   чтения» with the access level to add (`Posting FBS`, OZC §3) — the same mark Wildberries' read-only token gets.

## A. The writing half of the contract made neutral

Each change keeps the Wildberries exchange working exactly as before; its tests stay as they are and must pass unchanged.

1. **A new part, `orders.pack` — reporting the packing.** It takes the order's number and its parcels' contents (each parcel: the
   lines' marketplace product numbers and quantities) and answers with the marketplace's number of each parcel; a second call reads
   whether the report took effect. Wildberries does not claim it. **The engine chooses the flow by the parts the adapter claims:**
   with `orders.confirm`, the order is confirmed at `order.reserved` and the label follows (Wildberries, as today); with
   `orders.pack`, nothing is written at the reservation, and the codes, the packing report and then the labels follow
   `order.packing_finished` (Ozon). The exchange's parts a marketplace needs are `labels.fetch`, `codes.report`, `codes.verdict`, the
   three `handover.*` parts, and `orders.confirm` or `orders.pack`; an adapter without them is shown as «пока не поддерживается»
   (Plan 5).
2. **Labels for each parcel, made when the marketplace is ready.** `labels.fetch` gains the form a marketplace that makes labels
   later needs: ask for the labels of some numbers and get a ticket; fetch by the ticket until they are ready. A marketplace that
   answers at once (Wildberries) returns them on the first call. The label is stored per parcel (`exchange_parcels`): its
   marketplace number, the ticket, its fingerprint, when it was sent. The contract's text drops «58 × 40 mm»: a label is a file of
   the type the neutral API takes (PDF, PNG or JPEG) with an optional scan text. **The contract suite's label and document cases are
   corrected to that rule:** they demanded a PNG and a scan text of every adapter, which the contract's own types never required — a
   Wildberries-only fact written into the shared suite. Wildberries' own label tests keep checking its PNG of 58 × 40 mm and its scan
   text, unchanged.
3. **Codes that replace.** An adapter declares whether a second sending of codes replaces the first. Ozon does («Всегда передавайте
   полный набор данных по экземплярам и продуктам», OZC §7): the engine then sends the full set again instead of removing first.
   Wildberries does not say so, and its cautious removal stays.
4. **A batch of exactly the hand-over's orders.** `orders.confirm`'s `PutInBatch` is joined by one call that makes a batch hold exactly
   the given orders: Wildberries adds the missing ones (its new supply is empty, as today); Ozon rewrites the shipment's list
   (`/v1/carriage/set-postings`, OZC §9).
5. **The hand-over's steps are the adapter's.** An adapter declares which of the six steps (batch, move, boxes, shipping, close,
   documents) it needs: Wildberries all six, Ozon four — batch (the shipment is found or made), move (exactly the hand-over's
   postings), close (approved), documents. The runner skips the steps an adapter does not declare and asks for stored shipping
   settings only when it declares «shipping». The step names stay the table's; the strip on «Передачи» draws the declared steps.
6. **Documents with their kinds.** `handover.documents` returns a list, each with its kind of the neutral API: Wildberries the
   supply's QR code (`handover_code`, the supply's number as its scan text) and the boxes' stickers (`box_label`), as today; Ozon the
   hand-over barcode (`handover_code`, with Ozon's text of it as the scan text) and the shipping list with the waybill (`act`).
7. **The exchange's periods are the adapter's:** Wildberries new orders every minute and states every 5 minutes (as today), Ozon
   both every 5 minutes (the high-level plan, «How often it asks»). The fulfilment centre's events stay every minute.
8. **One format of a batch's name** for the exchange and the hand-over («wyo — сборка <date>», «wyo — передача <number> <date>»).

## B. Step 1 — a new posting is sent

- Every Ozon FBS posting in the state «Новый» of a matched warehouse is sent with `POST /api/v1/orders` (Plan 4, section D): the
  posting number as the sender's own number, the order number as the basket, **every line** with its article, barcodes, the SKU as
  the marketplace's product number and its quantity, and **Ozon's own deadline** (`shipment_date`) as `ship_by`. A deadline Ozon
  changes later is passed on with `PATCH /api/v1/orders/{id}`.
- **Accepted, held, goods not there** — as Plan 4, section D: the owner decides; the wyo server never cancels at Ozon.
- **Nothing is written to Ozon at `order.reserved`:** the posting stays «awaiting_packaging» at Ozon until the packing is reported.
- **A posting split in the Ozon cabinet** (Plan 5): the original's cancellation is passed on as a marketplace's cancellation (section
  G), and its parts are new postings, sent as new orders.
- **A warehouse with Ozon's automatic assembly** (`is_auto_assembly`, OZC §6): what Ozon does with it is not sourced, so the warehouse's
  row in «Внешний API» → «Склады» warns, and a posting that Ozon shows packed before the wyo server reported it is never reported
  again (section D).

## C. Step 2 — the marking codes, after packing is finished

When the fulfilment centre reports packing finished (`order.packing_finished` — the contents are final and each unit's code is
attached, `docs/fulfilment-centre-api.md`):

1. `/v6/fbs/posting/product/exemplar/create-or-get` gives the posting's exemplars and, per product, whether a marking code is needed
   or possible and whether another identifier is needed (OZC §7).
2. **What is sent:** for each product whose code is needed or possible, the units' codes the fulfilment centre scanned, each on its
   own exemplar; **never** a code for a product that takes none (Ozon's error `MANDATORY_MARK_REDUNDANT`); the whole set every time
   (section A, point 3). A code is sent exactly as scanned, with its separator written once as `\u001d` (OZC §7) — the ONE function of
   Plan 4, reused, with its test that a double escape never happens.
3. `/v6/fbs/posting/product/exemplar/set`, then `/v5/fbs/posting/product/exemplar/status` until Ozon's answer is final — every 20
   seconds for 10 minutes, then every 5 minutes (the same schedule function as Wildberries' verdicts). A 200 of `set` means only that
   Ozon took the task (OZC §7).
    - `ship_available` — the codes are accepted: the order's exchange state becomes «коды приняты», and the packing is reported
      (section D).
    - `validation_in_process`, a mark `processing` — still checking.
    - A mark `failed` («система не успела обработать запрос», OZC §7) — the full set is sent once more; a second `failed` is a problem.
    - A mark with `error_codes` — a problem on the order at the fulfilment centre with Ozon's codes in Ozon's words (`PUT …/problem`),
      and a notice; corrected codes in a new `order.packing_finished` clear it.
4. **Ozon needs a code and the fulfilment centre sent none** — a problem («Ozon требует код маркировки»). **No product needs a code**
   — nothing is sent and the packing is reported at once.
5. **Ozon needs another identifier** (a customs declaration number, a batch registration number, a jewellery number, an IMEI, a weight,
   the country of origin — OZC §6, §7) or a count of boxes for a multi-box product (`is_multibox`): the neutral API carries none of
   them, so the order gets a problem that names it and says it must be entered in the Ozon cabinet (register item 24). When the owner
   has entered it, «Передать заново» goes on.
6. **A parcel reopened before the packing is reported** (`order.reopened`): nothing is written to Ozon; the next
   `order.packing_finished` sends the full set again. **After it is reported:** changing codes at Ozon is not sourced (OZC §7), so the
   order gets a problem that says so, for the owner and the fulfilment centre.

## D. Step 3 — the packing reported, then the labels

1. **Before the report** the posting is read (`/v3/posting/fbs/get`): cancelled — section G; already packed — the report is skipped
   (never a write twice).
2. **The report:** `/v4/posting/fbs/ship` with one package for each parcel of the fulfilment centre, holding that parcel's units by
   SKU and quantity, and `with.additional_data`. **One parcel — one package; several parcels split the posting** (OZC §7). The
   fulfilment centre's contents must equal Ozon's lines: a posting whose lines the fulfilment centre cut (`order.reduced`) is not
   reported — the owner removes the products in the Ozon cabinet (the wyo server never cancels at Ozon), and the report goes on when
   Ozon's lines equal the parcels.
3. **A 200 is not the end** («Ответ с кодом 200 не гарантирует успешную сборку заказа», OZC §7): the posting is read again until it is
   `awaiting_deliver` (packed) — every 20 seconds for 10 minutes, then every 5 minutes. The substatus `ship_failed` makes the report
   again, as Ozon says, at most three times; then a problem in Ozon's words. Ozon's refusals of the report (`HAS_INCORRECT_PRODUCT_QUANTITY`,
   `EXEMPLAR_INFO_NOT_FILLED_COMPLETELY`, `INCORRECT_OVH_FOR_POSTING` and the others of OZC §7) are problems in Ozon's words;
   `POSTING_ALREADY_SHIPPED` means it is done.
4. **The parts:** the answer names each part's posting number and products (`additional_data`). Each parcel takes the part with the
   same contents; two parcels with the same contents may take either, since their labels are alike. The parts are stored on the
   parcels (`exchange_parcels`) and shown in the order's details; a part never enters the order list as a new order to send.
5. **The labels:** 60 seconds after the posting is packed («Рекомендуем запрашивать этикетки через 45–60 секунд после сборки заказа»,
   OZC §8), one label task per part (`/v3/posting/fbs/package-label/create`), the task of the warehouse's label type (its
   `carriage_label_type`: `BIG` — the big label, `SMALL` — the small one, `UNSPECIFIED` — the ordinary one, `big_label`); then
   `/v2/posting/fbs/package-label/get` every 10 seconds for 5 minutes, then every 5 minutes, until `completed`. A part among
   `unprinted_postings`, or `error`, is a problem in Ozon's words.
6. **The label file** is fetched from Ozon's link: `https` only, a host under `ozone.ru` (the domain of Ozon's own example link,
   OZC §8), **without the key** (sending the key to a file link is not sourced, and a key that is not needed is never sent), through
   the one caller — the journal row names the path `{label file}`, never the link — at most 5 MB (the neutral API's limit), and only
   a file that begins as a PDF. It is attached with `PUT /api/v1/orders/{id}/parcels/{n}/label` as `application/pdf`. **No scan text
   is sent:** which of the posting's two barcodes a scanner reads is not sourced (OZC §8).
7. The order's exchange states for Ozon, in their order: «передан», «зарезервирован», «коды проверяются», «коды приняты», «этикетка
   передана», «упакован», «передан в доставку» — the states of Plan 4, in another order; the packing report shows in the history.

## E. Step 4 — the hand-over through Ozon's shipment

When the fulfilment centre asks for a hand-over's documents (`handover.requested`), the engine runs, for each Ozon warehouse among its
orders (one shipment per warehouse and day — `delivery_method_id` is the warehouse's number for FBS, OZC §9), and stores each step:

1. **Batch — the shipment.** `/v2/carriage/delivery/list` for the warehouse and the hand-over's day. A shipment of status `new`
   already there is reused; otherwise, when Ozon lists `create`, `/v1/carriage/create`. When Ozon cannot make one yet, or answers
   `POSTING_NOT_FOUND`, the hand-over waits with Ozon's words and the step is tried again every 5 minutes while the request stands —
   each try reads the list first, so a shipment is never made twice.
2. **Move — exactly the hand-over's postings.** A new shipment takes every posting ready for shipment (OZC §9), so its list is
   rewritten with `/v1/carriage/set-postings` to exactly the parts of the hand-over's parcels. A posting Ozon refuses in its answer is a
   problem naming it in Ozon's words. Then the shipment's list is read back (`/v2/posting/fbs/act/get-postings`) and compared both
   ways: a difference stops the hand-over with a problem that names the postings. Ozon says the method is not available to sellers
   outside Russia (OZC §9); its refusal is a problem, and the README says so.
3. **Close — approved:** `/v1/carriage/approve` (without `containers_count`, which is for entrusted acceptance only — OZC §9). Before
   it, the shipment is read; one already approved is not approved again.
4. **Documents:** `/v2/posting/fbs/act/check-status` every 10 seconds for 5 minutes, then every 5 minutes, until `ready`; a posting of
   the hand-over among `removed_from_act` is a problem that names it; `error` and `cancelled` ask again, as Ozon says. Then
   `/v2/posting/fbs/act/get-pdf` (the shipping list and the waybill, kind `act`), `/v2/posting/fbs/act/get-barcode` (the picture,
   kind `handover_code`) and `/v2/posting/fbs/act/get-barcode/text` (its scan text), sent with `PUT /api/v1/handovers/{id}/documents`.
   Whether Ozon answers with the file itself or with an object holding it is not sourced (OZC §9): both are read, by one function.

- **Transport boxes** named by the fulfilment centre are not sent to Ozon (Ozon has no boxes outside entrusted acceptance); the
  hand-over's page says so.
- **A request withdrawn** (`handover.request_withdrawn`) before the approval cancels the shipment wyo made (`/v1/carriage/cancel`); after
  it nothing is undone, and the page says so.
- **What Ozon refuses becomes a problem on the hand-over** in Ozon's words, with «Повторить» on the hand-over's page; the engine goes
  on from the step that failed.
- **«Передачи»** shows an Ozon hand-over with its four steps, the shipment's number, the problem in Ozon's words.

## F. Register item 22, its second half — the fakes do what the tests need

- **The fake fulfilment centre names transport boxes** in `handover.requested` on command, so the step «Добавление коробов» of
  Wildberries is proven through the fake's own event.
- **The exchange's tests drop their scripted refusals** and use the kit's fault «refuse with this answer» (Plan 5) on the fake
  Wildberries and the fake Ozon — the same refusals in each marketplace's own error shape, now made by the fakes.

## G. Step 5 — cancellations

- **Ozon cancelled** — the engine reads the state of every open Ozon order every 5 minutes; a cancellation is passed on with
  `POST /api/v1/orders/{id}/cancel`, and the fulfilment centre's answer is shown as it is (Plan 4, section G). A cancellation after the
  packing (`cancelled_after_ship`, OZC §6) is passed on the same way and shown to the owner.
- **The warehouse cancelled** or a change waits for a person — shown to the owner behind the bell and under «Требуют внимания».
  **The wyo server never cancels at Ozon** and never removes a product from a posting: the generated client does not hold
  `/v2/posting/fbs/cancel` or `/v2/posting/fbs/product/cancel` (Plan 5's test of the generator's list).

## H. The owner's view and the instructions

- **«Внешний API»** accepts Ozon accounts: «Склады» matches Ozon's FBS warehouses (an rFBS one is never offered; one with automatic
  assembly warns), «Маркетплейсы» shows an Ozon key that may only read with the level to add. Ozon declares no hand-over settings, so
  its section 5 is empty and says so.
- **«Заказы»** — the details show the parcels with their Ozon postings and labels (`ParcelList`), the lines (`OrderLines`, Plan 5),
  the codes' verdicts. **«Передачи»** — the declared steps. **«Журнал запросов»** — the label files as `{label file}`.
- **Notices** added to the bell: an Ozon problem on an order (codes, an identifier to enter, the packing refused, a label not made);
  a hand-over that waits for Ozon.
- **`README.md`, «Работа с фулфилмент-центром»** gains Ozon: the access level `Posting FBS` the key needs now; **the plain list of what
  wyo then does at Ozon by itself** — sends the marking codes, reports the packing with the fulfilment centre's parcels (and so splits
  a posting packed in several), fetches each parcel's label, makes the day's shipment with exactly the hand-over's postings, approves
  it, fetches its documents — **and what it never does**: cancel a posting or remove a product from it, approve a shipment with other
  postings, send the key anywhere but Ozon; **what the owner does in the Ozon cabinet** (register item 25 widened): cancelling,
  removing products, the identifiers of section C, point 5, a multi-box product's boxes, automatic assembly, entrusted acceptance,
  the warehouse's drop-off settings; a day with the exchange — where an Ozon order that needs the owner shows and what to do.
- `docs/external-api.md` gains the Ozon walk with each call on both sides; `docs/ozon.md` gains the writing methods and their periods.
- **The constitution, 1.5.0 (MINOR):** principle XII — the flow chosen by the parts an adapter claims, the codes that replace, the
  batch of exactly the hand-over's orders, the declared steps, the label files fetched without the key from the adapter's declared
  domain; principle IX — that domain in the one list.

## I. Rules R2, R3 and R4, proven here

- **R2, neutral:** `internal/exchange` and `internal/handover` hold no marketplace's name (the architecture test); the contract suite
  runs the writing parts for both adapters; the specification's policy test of Plan 1 still passes.
- **R3, alone by default:** with «Внешний API» empty, a made-up day with Ozon and Wildberries orders arriving sends nothing to the fake
  fulfilment centre, and nothing to Ozon but readings.
- **R4, the keys stay:** the canary test of Plan 4 is widened to an Ozon account through the whole walk: the Ozon API key, the
  fulfilment centre's key and the marking codes are searched for in every log line, journal row and answer, and — through the fakes'
  own call logs — in everything sent to the other side; the label file's request carries no key.

## J. Data, code, tests

**Data — one migration, `0005_parcels_ozon_exchange`:** `exchange_parcels` (the order, the parcel's number, the marketplace's number of
the parcel, the label ticket, the label's fingerprint, when it was sent), filled from `exchange_orders`' one label of every existing
Wildberries order (parcel 1, the order's own number), whose label columns are then dropped; `exchange_orders` gains when the packing
was reported and how many times it was tried; `marketplace_warehouses` gains the label type and the automatic-assembly mark. The
upgrade test runs from `0004.db` (written first, as Plan 5 did) with a Wildberries order on each exchange state.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/orders/{id}` gives the parcels with their marketplace numbers and labels;
`GET /api/v1/handovers/{id}` gives the declared steps; `GET /api/v1/marketplaces` gives each adapter's exchange flow and steps. No new
address.

**The adapter contract** gains `orders.pack`, the asynchronous form of `labels.fetch`, the «codes replace» mark, the batch of exactly
these orders, the declared steps, documents with kinds and the exchange's periods, each with its cases in the one contract suite; the
Ozon adapter now supports `orders.pack`, `labels.fetch`, `codes.report`, `codes.verdict`, `handover.group`, `handover.close`,
`handover.documents`.

**Code — where things live:** `internal/marketplace` (the contract's writing half), `internal/marketplace/ozon` (the writing parts),
`internal/exchange` (the flow by parts, the packing, the labels per parcel), `internal/handover` (the declared steps),
`internal/fakes/ozon` (the writing routes), `internal/fakes/fulfilment` (transport boxes); in the interface
`src/lib/components/exchange/`, `src/lib/components/handovers/`.

**Components created** (each once, with its test file in every state): `ParcelList` (an order's parcels: the marketplace's number of
each, its label's state). **Components reused:** `OrderLines`, `ProductCell` (Plan 5), `HandoverSteps` (now drawn from the declared
steps — its test widened for a list of four), `ExchangeStateBadge`, `HeldReasons`, `ProblemLine`, `CodeVerdictList`,
`WarehouseMatchTable`, `MarketplacesSection`, `AdapterSettingsForm` (its empty state), `OrderExchangeDetails`, `HandoverDetails`,
`StatusBadge` with the tones, `NoteBox`, `EmptyState`, `DataList`, `RowDetailsPanel`, `StepStrip`, the bell.

**Tests** (written before the code they check, failing first):

- **Go, with the fake Ozon and the fake fulfilment centre in one test process:** the whole walk — a posting of two lines made in the
  fake Ozon, sent, reserved, packed in one parcel with codes, the codes accepted, the packing reported, the label fetched and
  attached, the hand-over asked, the shipment made with exactly its postings, approved, the documents received by the fake
  fulfilment centre. Then each branch: packed in two parcels (the split; each parcel's label from its own part); no code needed; a
  code refused with Ozon's codes; a mark `failed` once and twice; an identifier other than a code; a multi-box product; `ship_failed`
  once and three times; each refusal of the report; a cut by the fulfilment centre; a label not made; the shipment that took other
  postings (rewritten), a posting refused by `set-postings`, a difference after the rewrite, a posting removed from the act; a
  shipment already there; a request withdrawn before and after the approval; a cancellation before and after the packing; a split
  made in the cabinet; a posting packed by automatic assembly; a deadline changed.
- **Stopped in the middle:** the order's and the hand-over's Ozon steps are interrupted after every single step in turn; after the
  restart the walk ends right, and the fake Ozon's call log shows no write made twice.
- **The Wildberries walk and its branches pass unchanged**, now with the fakes' own refusals in place of the scripted ones (section F)
  — the same refusals, made by the fakes.
- **Never sent:** the tests of section I; the separator of a marking code; no key on the label file's request; a label link to a host
  outside the declared domain refused before it leaves; the budget never exceeded during a made-up hour at this plan's periods with
  500 open Ozon orders.
- **The fakes gain** the writing routes, a label made after N readings, a shipment that takes every ready posting, `ship_failed` on
  command, transport boxes named by the fake fulfilment centre; each with its test.
- **Interface unit tests:** `ParcelList` once; `HandoverSteps` with four steps; each page only for what it adds; the policy tests
  green, the marketplace-name policy among them; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes, each worker with its own data): an Ozon warehouse matched; the
  whole walk of an Ozon posting packed in two parcels, driven through the fakes' control calls and seen in «Заказы» (the parcels and
  their postings) and «Передачи» (four steps); a refused code as a problem; an Ozon key that may only read shows its level to add;
  every new state of a page swept at 390 × 844 and the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration and the parcels. 3. The contract's writing half of section A, under Wildberries' tests.
   4. The generated writing client and the fake Ozon's writing routes; the fake fulfilment centre's boxes; the scripted refusals
   replaced. 5. Step 1: postings out, the deadline. 6. Step 2: the codes and the verdict. 7. Step 3: the packing report, the parts,
   the labels and their files. 8. Step 4: the shipment, the documents. 9. Step 5: cancellations. 10. The owner's view: «Внешний API»,
   «Заказы», «Передачи», the notices. 11. The tests of section I and the interrupted walks. 12. The README, `docs/`, the help texts,
   the constitution. 13. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes): fill «Внешний API»; match an Ozon
  warehouse; make an Ozon posting of two lines in the fake and walk it, packed in two parcels, to the hand-over's documents through the
  fakes' control calls, looking at «Заказы», the details, «Передачи», the bell and the journal at each step; a refused code; a
  Wildberries order walked the same way beside it; empty the setting and see that nothing more is sent. Every page at a computer's
  width and at 390 × 844 with screenshots; the console, the raw-key check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release
  run watched to its end; `main` pulled; the report. At the merge the register is updated as «Not in this plan» and «After the
  release» say.

## After the release — the checks with the first real Ozon account and the first real fulfilment centre (register)

A real posting packed in one parcel and in two: the numbers of the parts and how the original shows; the words of `exemplar/status`'
`error_codes`; whether codes can be changed after packing; a label printed from the file the wyo server attached — its size, the host
of its link, whether the link works without the key, which barcode the fulfilment centre's scanner reads (then the scan text can be
sent with the label); when Ozon lets a shipment be made, and a second shipment on the same day; the body of `act/get-pdf` and
`act/get-barcode`; what automatic assembly does (OZC §10, points 5–9). What they find becomes the next plan's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Stock** — a plan of its own after Яндекс Маркет (register item 1); **Яндекс Маркет** — the plans after this one.
- **Identifiers other than marking codes, and a multi-box product's boxes:** a problem that names them, entered in the Ozon cabinet
  (register item 24 widened to Ozon), until the neutral API carries them.
- **Cancelling or cutting a posting at Ozon from wyo; entrusted acceptance with cargo places; the drop-off point's settings; passes:**
  the owner does these in the Ozon cabinet; each is named in the README (register item 25 widened to Ozon).
- **The scan text of an Ozon label** sent with it: after the check on a real label.
- **Splitting a posting before packing** (`/v1/posting/fbs/split`, `/v4/posting/fbs/ship/package`): the packing report splits by the
  fulfilment centre's parcels; the other methods are left unless the real check shows the report cannot.
- **A new register item, «The checks of Plan 6 on the first real Ozon account and the first real fulfilment centre»,** holds «After
  the release»; register item 22 closes at the merge.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — the only corrected test cases are the contract suite's label and document cases of section A, point 2, whose reason is
   written there; Wildberries' own tests stay as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, marking code or buyer datum in a log, a journal row, an answer or an
   error.
5. Everything is a component and is reused: one fulfilment-centre client, one exchange engine, one caller, one journal, one contract
   test suite, one schedule of asking again, one writing of a marking code, one reading of a returned file, one format of a batch's
   name; search before writing; the components section J names are created once. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **No write to a marketplace is repeated blindly, and no order is ever cancelled at a marketplace by the wyo server.** With
   «External API» empty nothing is sent anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about Ozon is written
   only from its official documents (OZC), with the address and the date; a gap is marked "not sourced", gets the cautious choice, and
   goes to «After the release». Ozon's file is never edited.
9. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in its own words
   beside wyo's sentence; no sentence in Go. Name the program every time. Code is written mobile-first (390 × 844 first).
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

One new table and four new columns, no new address of the wyo server's own API, one component, seven contract parts in the Ozon
adapter and the contract's writing half made neutral under Wildberries' unchanged tests. The interrupted walks and the split into
parcels are the largest test work, and the parts that protect a seller from a shipment approved with the wrong postings or a code
sent to the wrong unit.
