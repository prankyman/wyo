# Яндекс Маркет in the marketplace connector, part 2 of 2: the whole FBS and Express exchange through the neutral fulfilment-centre API — each new order sent with its test mark and its way out, the boxes and their marking codes reported in one request with Яндекс Маркет's verdict, each box's label in the size the fulfilment centre names, «Готов к отгрузке» once packed, the daily shipment that Яндекс Маркет makes compared with the hand-over and confirmed with its act, the Express courier's code checked for the warehouse, cancellations passed on

> **Plan 8** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 2 of the two Яндекс Маркет plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`, whose section «The exchange with a fulfilment centre» says «Ozon and Яндекс Маркет follow
> the same six steps through their adapters; there the label exists only after packing is reported».
> Written 2026-11-19 from `origin/main` at `bfa9e24`. **It starts only after two releases:** Plan 7
> (`.agents/plans/wyo-yandex-market-key-reading-orders-notifications-safety-read-test-orders-fake-yandex-market.md`) merged and released,
> and the release of the maintainer's fulfilment software that adds three things to its seller API — the label size of each warehouse,
> orders a courier collects after a code check, and the test mark of an order (its plan is kept in the maintainer's other repository).
> Its section «Today» is checked again on that day.
> Research: `.agents/research/yandex-market-official-partner-api-check-2026-11-19.md` (cited «YMC»); `docs/fulfilment-centre-api.md` and
> `api/fulfilment-centre.yaml` (the neutral specification); Plan 6
> (`.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md`) and Plan 6a
> (`.agents/plans/wyo-plan-6a-ozon-exchange-review-fixes-endless-retries-shipment-recheck-split-original-docs.md`), whose exchange this
> plan widens.
>
> **Branch** `feat/yandex-market-exchange`. One plan = one branch = one squashed commit = one pull request. The commit is `feat:` — never
> `feat!:` and never a `BREAKING CHANGE:` footer: with «Внешний API» empty, which is the default, nothing changes for anybody, and the
> Wildberries and Ozon exchanges work as before. **One database migration**, the next free number at the start. Constitution: principle
> XII widened, **MINOR**, the next free number at merge time.

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
  **the journal** — as in Plans 2 and 3. **A fulfilment centre**, **«Внешний API»**, **the neutral API**, **an event**, **a held order**,
  **a hand-over**, **a marking code**, **the exchange** — as in Plan 4. **A parcel**, **reporting the packing**, **a batch** — as in
  Plan 6. **The Яндекс Маркет key**, **a cabinet**, **a shop**, **an access level**, **FBS**, **Express**, **a test order**, **a
  notification**, **the safety read** — as in Plan 7.
- **A box** (Яндекс Маркет's «коробка», `boxId`) — one of the fulfilment centre's parcels as Яндекс Маркет numbers it.
- **The box layout** (`setOrderBoxLayout`) — one request that tells Яндекс Маркет which units lie in which box, with each unit's marking
  code; a later one replaces it until the order is «Готов к отгрузке» (YMC §10).
- **«Готов к отгрузке»** (`PROCESSING` / `READY_TO_SHIP`) — the seller's statement that the order is packed and will be handed over; only
  such orders enter the act (YMC §10).
- **The daily shipment** (Яндекс Маркет's «отгрузка», methods `first-mile/shipments`) — the list Яндекс Маркет itself makes of one
  warehouse's orders for one day. Its kind is `WITHDRAW` (Яндекс Маркет's courier collects at the warehouse) or `IMPORT` (the warehouse
  brings the parcels to a sorting centre or a drop-off point). The seller **confirms** it after Яндекс Маркет's cutoff time and downloads
  its **act** («акт приёма-передачи»), which the receiver signs (YMC §11).
- **A courier-code hand-over** — how an Express order leaves: a Яндекс Go courier comes for it alone, names a code, the code is checked by
  Яндекс Маркет (`verifyOrderEac`), and only then the parcel is given (YMC §12). In the neutral API: an order sent with
  `handover: "courier_code"`, the event `order.courier_code_entered`, and the answer `PUT /api/v1/orders/{id}/courier-code-checks/{check_id}`.
- **The label size** — the width and height in millimetres the fulfilment centre names for each of its warehouses and marketplaces
  (`label_sizes` of `GET /api/v1/warehouses`).

## The maintainer's request and answers (2026-11-19) — nothing is open

- 2026-11-16, the high-level plan: Stage 2 holds «the same exchange with a fulfilment centre» for Яндекс Маркет; 2026-11-19: «write the
  plan for Яндекс Маркет», two plans («2»); this is the second.
- **The daily shipment.** Asked what the wyo server does when Яндекс Маркет's shipment holds orders the fulfilment centre does not hand
  over: «I am OK with your recommendation» — **the wyo server never confirms that shipment and never moves an order to another one**;
  the hand-over gets a problem naming those orders, and the owner moves or cancels them in the cabinet (Яндекс Маркет lowers the shop's
  quality index for either, YMC §11). «But who is courier? Why courier? What if the FO takes the packed order to YM drop-off center
  themselves?» — both kinds of shipment (`WITHDRAW`, `IMPORT`) have the same list, confirmation and act, so the walk is one.
- **The label size.** «FO may use different sizes. Isn't wyo, as a FO to marketplace connector not supposed to just pass the
  communication as is, without injecting any logic, but only hiding that it is the FO that is a backend for the FBS handling?» The
  proposal — the fulfilment centre names its size per warehouse and marketplace, and the wyo server asks each marketplace for exactly
  that, Wildberries included — was answered «yes».
- **Express.** «The plan 8 must handle the Express orders fully. Are you sure the [the maintainer's fulfilment software] Api is missing
  this functionality? Must it be added there?» It was missing; «Write a full [the maintainer's fulfilment software] plan» — the
  fulfilment software's plan adds the courier-code orders; this plan uses them.
- **Test orders.** «You must pass them to [the maintainer's fulfilment software], so they could be tested, but both wyo and [the
  maintainer's fulfilment software] must not take them into account in terms of analytics»; «The wyo and [the maintainer's fulfilment
  software] must both mark such orders as FAKE and make it visible in UI … Test orders ARE NOT PAID of course!» So a test order is sent
  with the test mark and walks the exchange like any order; the fulfilment centre packs it and never hands it over.
- Stated with the questions and not objected to: the wyo server never cancels an order, never removes goods from one (`allowRemove` is
  never sent) and never moves an order to another shipment.
- 2026-11-17, still true: no real account — the exchange is built and tested only against the fake Яндекс Маркет and the fake fulfilment
  centre; the checks on real ones are listed in «After the release».

## Today (to be checked again on the day the plan starts)

- After Plan 7 the wyo server reads Яндекс Маркет: accounts of a cabinet with its shops, FBS, Express and FBY orders with their lines and
  test marks, notifications and the safety read; Яндекс Маркет accounts are shown in «Внешний API» as «пока не поддерживается».
- **The writing half of the contract does not fit Яндекс Маркет** (read at `bfa9e24` and in Plan 6a; `internal/marketplace/writing.go`,
  `internal/exchange`, `internal/handover`):
  1. **The codes and the packing are two writes** (`codes.report`, then `orders.pack`, `internal/exchange/pack.go:33-42`). Яндекс Маркет
     takes the boxes and the codes in one request (YMC §10).
  2. **No write follows the labels.** Ozon is packed by its report and Wildberries by its confirmation; Яндекс Маркет needs «Готов к
     отгрузке» as its own write, and only after every code passed (YMC §10).
  3. **The wyo server makes or fills every batch** (`MakeBatch`, `FillBatch`; `internal/handover/steps.go:89-193`). Яндекс Маркет makes its
     daily shipment itself and nothing can be added to it (YMC §11); only comparing and confirming are the seller's. Plan 6a made ONE
     function that compares a batch with the hand-over both ways before «close» — this plan reuses it.
  4. **The label size is the wyo server's choice:** Wildberries' sticker is asked for at 58 × 40 mm (`internal/marketplace/wildberries/labels.go:22-25`);
     the fulfilment centre's warehouse list is read only for matching and not kept (`FcWarehouses`, `internal/exchange/setting.go:256`).
  5. **Every order leaves through a hand-over.** There is no hand-over of one order to a courier, and no courier's code.
  6. **No test mark is sent** with an order (`orderIn`, `internal/exchange/send.go:53-86`).
- **The neutral specification in this repository** (`api/fulfilment-centre.yaml`, `docs/fulfilment-centre-api.md`) does not yet have the
  three additions; Stage 0, premise 2 brings them in.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **The neutral specification gains the three additions exactly as the fulfilment software's released description has them:**
   `label_sizes` on each warehouse of `GET /api/v1/warehouses`; `test` and `handover` (`shipment` | `courier_code`) on an order; the event
   `order.courier_code_entered` with `data.details.check_id` and `data.details.code`; the call
   `PUT /api/v1/orders/{id}/courier-code-checks/{check_id}` with `{ result: accepted | refused | new_code_needed, attempts_left?, message? }`.
   The comparison is made outside this repository, as Plan 4's premise 3 and Plan 6's premise 5 (rule R1: nothing here names that
   software). `internal/fulfilment/fcapi` is regenerated; the fake fulfilment centre gains them.
3. **Яндекс Маркет's tree is compared again on the day** with the repository's newest commit (`api/yandex/README.md`); a difference in a
   method of YMC §5 marked for this plan stops the stage and asks.
4. **The generator reads the writing methods.** `api/codegen/yandex.cfg.yaml` gains `setOrderBoxLayout`, `getOrderIdentifiersStatus`,
   `generateOrderLabel`, `updateOrderStatus`, `verifyOrderEac`, `searchShipments`, `getShipment`, `confirmShipment`, `downloadShipmentAct`
   (YMC §5). `updateOrderItems`, `acceptOrderCancellation` and `transferOrdersFromShipment` stay out (Plan 7's test). `updateOrderStatus`
   can cancel, so it is reached only through ONE function that can send nothing but `PROCESSING` / `READY_TO_SHIP` (it takes no status
   argument), and an architecture test proves no other code calls the generated method;
   `allowRemove` is never set (a test on every box layout sent).
5. **The findings of Plan 7's checks with a real cabinet, if they exist by then,** replace the cautious choices of this plan (the
   warehouse of an order above all); otherwise the choices stand and stay in «After the release».
6. **A seller's key must be allowed to write:** `INVENTORY_AND_ORDER_PROCESSING` or `ALL_METHODS` (YMC §2, §5); otherwise the account
   keeps Plan 7's mark «ключ только для чтения» with the level to add («Обработка заказов и учёт товаров»).

## A. The contract widened

Each change keeps the Wildberries and Ozon exchanges working exactly as before; their tests stay as they are and must pass unchanged.

1. **Codes inside the packing report.** An adapter declares `CodesInPack`: its `orders.pack` takes each parcel's units with their marking
   codes, and it claims no separate `codes.report`; `codes.verdict` is read after the report. Яндекс Маркет declares it and «codes
   replace» (a later layout replaces the earlier, YMC §10). Ozon and Wildberries do not, and their flows are unchanged.
2. **A final write — the new part `orders.ready`:** sent when the fulfilment centre reports every parcel closed (`order.packed`), the
   codes were accepted and every parcel's label was attached. Яндекс Маркет: «Готов к отгрузке». Read before the write: an order already
   ready is not written again (principle XII).
3. **The label size is the fulfilment centre's.** The fulfilment centre's `label_sizes` are read when «Внешний API» is filled, every hour,
   and before a label is asked for when the last reading is older than an hour; they are kept per fulfilment-centre warehouse and
   marketplace (section J). `labels.fetch` receives the size of the order's warehouse; each adapter maps it to what its marketplace makes:
    - Яндекс Маркет — 58 × 40 → `A9_HORIZONTALLY`, 40 × 58 → `A9`, 75 × 120 → `A7`, 210 × 297 → `A4` (YMC §10); no size named →
      Яндекс Маркет's own default; any other size → a problem on the order: «Маркет не делает ярлыки размера W × H мм — выберите на складе
      фулфилмент-центра 58 × 40, 40 × 58, 75 × 120 или A4».
    - Wildberries — 58 × 40 and 40 × 30 (the two sizes of its sticker method); no size named → 58 × 40, as today; any other → the same
      problem in Wildberries' words. Wildberries' own tests of its 58 × 40 sticker stay unchanged.
    - Ozon — unchanged: its label type follows its warehouse's own setting (Plan 6).
    The rule is written once in principle XII: **the wyo server chooses nothing the fulfilment centre names.**
4. **A batch the marketplace makes alone.** An adapter declares that its batch is found, not made, and that it cannot be changed:
   Яндекс Маркет's steps are batch (find the day's shipment), move (only compare — Plan 6a's ONE comparison function, both ways), close
   (confirm) and documents. The runner calls no `MakeBatch` or `FillBatch` for it.
5. **A courier-code hand-over — the new part `orders.courier_code`:** checks a code for an order and answers accepted, refused with the
   attempts left, or «a new code is coming», with the marketplace's words. An adapter names which of its orders leave this way
   (Яндекс Маркет: the scheme `EXPRESS`). The engine sends such an order with `handover: "courier_code"`, puts it in no hand-over, and
   answers the fulfilment centre's `order.courier_code_entered` at once (section F).
6. **The test mark is sent:** `orderIn` passes the order's `test` (Plan 7) as the neutral `test`. Nothing else in the exchange treats a
   test order differently: Яндекс Маркет accepts the writes of a test order (YMC §9), and the fulfilment centre never hands it over.

## B. Step 1 — a new order is sent

- Every Яндекс Маркет FBS and Express order in the state «Новый» with `PROCESSING` / `STARTED` (YMC §7; `UNPAID` and `PENDING` wait) of a
  matched warehouse is sent with `POST /api/v1/orders`: the order number as the sender's own number, **every line** with its article
  (`offerId`) and barcodes (an item's `id` is its line's number in the order, not a product's number, so no marketplace product number is
  sent), the quantity, **Яндекс Маркет's shipment day and time** as `ship_by`, `test`, and `handover` (`courier_code` for Express,
  `shipment` otherwise). A changed shipment day is passed on with `PATCH /api/v1/orders/{id}`.
- **An order whose warehouse Plan 7 could not tell** («склад не определён») is held with that reason, and the owner sees it under
  «Требуют внимания»; the real-account check of Plan 7 decides the rule.
- **Accepted, held, goods not there** — as Plan 4, section D: the owner decides; the wyo server never cancels at Яндекс Маркет.
- **FBY orders** are never sent: Яндекс Маркет's own warehouses ship them.

## C. Step 2 — the boxes and their codes, after packing is finished

When the fulfilment centre reports packing finished (`order.packing_finished` — the contents are final, each unit's code attached):

1. **Before the report** the order is read (`orders.named`): cancelled — section G; already «Готов к отгрузке» — the layout is skipped and
   the walk goes to the labels (a layout cannot be changed after it, YMC §10).
2. **The layout:** `setOrderBoxLayout` with one box for each of the fulfilment centre's parcels; each box lists the order's items it
   holds by their `id` with `fullCount` (the units of that item in that box) and, for an item that takes codes
   (`requiredInstanceTypes` with `CIS` or `CIS_OPTIONAL`), the units' codes in `instances[].cis`, each written exactly as scanned with its
   separator once as `\u001d` — the ONE function of Plan 4 and its test that a double escape never happens. **Never** `allowRemove`,
   **never** `partialCount` (the neutral API has no unit split over boxes). The answer gives each box's `boxId`, stored on the parcel
   (`exchange_parcels`).
3. **The fulfilment centre's contents must equal Яндекс Маркет's items.** An order the fulfilment centre cut (`order.reduced`) is not
   reported: the owner removes the goods in the cabinet (the wyo server never removes them), and the layout goes on when Яндекс Маркет's
   items equal the parcels. A product split over several boxes, or a unit that needs `UIN`, `RNPT` or `GTD` (YMC §7): the neutral API
   carries none of them, so the order gets a problem that names them and says they must be entered in the cabinet (register item 24
   widened to Яндекс Маркет).
4. **The verdict:** `getOrderIdentifiersStatus` every 20 seconds for 10 minutes, then every 5 minutes (the one schedule function of
   Plans 4 and 6), until every code is final. Refused codes — a problem on the order at the fulfilment centre (`PUT …/problem`) with
   Яндекс Маркет's words; corrected codes in a new `order.packing_finished` send the whole layout again (it replaces the earlier). Яндекс
   Маркет needs a code and the fulfilment centre sent none — a problem («Маркет требует код маркировки»). No item takes codes — nothing to
   wait for.
5. **A parcel reopened** (`order.reopened`) before «Готов к отгрузке»: nothing is written; the next `order.packing_finished` sends the
   whole layout again. After it: the layout cannot change (YMC §10), so the order gets a problem that says so, for the owner and the
   fulfilment centre.

## D. Step 3 — the labels, then «Готов к отгрузке»

1. **The labels:** for each box, `generateOrderLabel` (`GET …/delivery/shipments/{shipmentId}/boxes/{boxId}/label`; the `shipmentId` is
   outdated and any number is taken, YMC §10) with `format` from section A, point 3 — a PDF from the API's own host, with the key, through
   the one caller, at most 5 MB, only a file that begins as a PDF — attached with `PUT /api/v1/orders/{id}/parcels/{n}/label` as
   `application/pdf`. **No scan text is sent:** which of the label's barcodes a scanner reads is not sourced.
2. **«Готов к отгрузке»** (`orders.ready`): at `order.packed`, when the codes are accepted and every label attached: the order is read;
   already ready — nothing; otherwise `updateOrderStatus` with `PROCESSING` / `READY_TO_SHIP`. Яндекс Маркет's refusal is a problem in its
   words with «Повторить».
3. **The order's exchange states for Яндекс Маркет,** in their order: «передан», «зарезервирован», «коды проверяются», «коды приняты»,
   «этикетка передана», «упакован», «готов к отгрузке», «передан в доставку» — the states of Plans 4 and 6 and one new,
   «готов к отгрузке».

## E. Step 4 — the hand-over through the daily shipment (FBS)

When the fulfilment centre asks for a hand-over's documents (`handover.requested`), the engine runs, for each Яндекс Маркет warehouse
among its orders, and stores each step:

1. **Batch — find the day's shipment.** `searchShipments` for the hand-over's day with the hand-over's order numbers, then `getShipment`.
   None yet — the hand-over waits with «Маркет ещё не сформировал отгрузку», and the step is tried again every 5 minutes while the request
   stands.
2. **Move — compare, both ways, with Plan 6a's ONE function.** Test orders are left out of the comparison (the fulfilment centre never
   hands them over; whether Яндекс Маркет puts them into a real shipment is not sourced — «After the release»).
    - **In the shipment, not in the hand-over:** a problem that names them — «Эти заказы есть в отгрузке Маркета, но фулфилмент-центр их не
      передаёт: перенесите их в следующую отгрузку или отмените в кабинете Маркета, затем нажмите «Повторить»». The wyo server never
      moves them (`transferOrdersFromShipment` is never generated) and never confirms such a shipment.
    - **In the hand-over, not in the shipment:** a problem that names them — they may belong to another day's shipment; the owner checks
      the cabinet.
3. **Close — confirm.** The shipment is read; confirmed already — nothing; `CONFIRM` among `availableActions` — `confirmShipment`.
   «Cutoff time for shipments has not been reached yet» — the hand-over waits with Яндекс Маркет's words and the time is tried again
   every 5 minutes; «Some orders have not been processed yet…» — a problem naming the orders not yet «Готов к отгрузке»; «Some orders
   are in the process of being excluded from shipment… wait up to 30 minutes» — the step waits and reads again (YMC §4, §11).
4. **Documents — the act.** When `DOWNLOAD_ACT` is offered, `downloadShipmentAct` (PDF, kind `act`), sent with
   `PUT /api/v1/handovers/{id}/documents`. **The waybill** of a `WITHDRAW` shipment is an `.xls` file, which the neutral API does not take
   (PDF, PNG or JPEG): it is not sent, and the hand-over's page says where the owner downloads it in the cabinet.

- **Both kinds of shipment** walk the same way; the hand-over's page names the kind in plain words: «Маркет забирает со склада» or
  «Склад везёт в сортировочный центр или пункт приёма».
- **Transport boxes** named by the fulfilment centre are not sent (pallets and their labels are Яндекс Маркет's other methods —
  «Not in this plan»); the page says so.
- **A request withdrawn** before the confirmation: nothing was written to Яндекс Маркет, so nothing is undone; after it nothing is undone,
  and the page says so.
- **«Передачи»** shows a Яндекс Маркет hand-over with its four steps, the shipment's number and kind, and the problem in Яндекс Маркет's
  words with «Повторить».

## F. Step 5 — the Express courier's code

1. The Express order walks sections C and D like an FBS one (codes, labels, «Готов к отгрузке»), and enters no hand-over.
2. **The fulfilment centre's event `order.courier_code_entered`** is applied at once, ahead of other work of the account, within the
   budget: `verifyOrderEac` with the code (YMC §12), then `PUT /api/v1/orders/{id}/courier-code-checks/{check_id}`:
    - `ACCEPTED` → `accepted`;
    - `REJECTED` → `refused` with `attempts_left` (Яндекс Маркет's `attemptsLeft`);
    - `NEED_UPDATE` → `new_code_needed` («Маркет отправит курьеру новый код»);
    - a refusal of the request (the order in a status that takes no code, a key without the level) → `refused` with Яндекс Маркет's words
      as the message, and a problem on the order for the owner.
3. **The code is never kept:** it is in the stored event only until the answer is sent, then blanked in the event row; it is in no log
   line, journal row, answer of the wyo server's own API or error text (rule R4's canary test is widened to it).
4. **Speed:** the courier is waiting, so the README asks to set the fulfilment centre's events to notifications («Внешний API», the events
   mode of Plan 4), where the event arrives at once; read every minute, it may wait up to a minute, and the page of «Внешний API» says so
   beside the Express warehouses.
5. **The code's kind:** Яндекс Маркет gives `delivery.eacType` «Если у магазина настроена работа с кодами подтверждения»
   (`COURIER_TO_MERCHANT` — the courier names the code). An Express order without it is still sent as a courier-code order; what Яндекс
   Маркет answers for it is not sourced — «After the release».
6. **Delivered:** the fulfilment centre's `order.handed_over` sets the exchange state «передан в доставку», as for a hand-over.

## G. Step 6 — cancellations

- **Яндекс Маркет cancelled** — learnt by a notification or the safety read (Plan 7) and passed on with `POST /api/v1/orders/{id}/cancel`;
  the fulfilment centre's answer is shown as it is (Plan 4, section G). After «Готов к отгрузке» the same, shown to the owner.
- **The warehouse cancelled** or a change waits for a person — shown to the owner behind the bell and under «Требуют внимания».
  **The wyo server never cancels at Яндекс Маркет** and never removes goods: the one status function (Stage 0, premise 4) sends only
  «Готов к отгрузке», and no box layout carries `allowRemove`.

## H. The owner's view and the instructions

- **«Внешний API»** accepts Яндекс Маркет accounts: «Склады» matches Яндекс Маркет's FBS and Express warehouses (a DBS one is never
  offered) and shows, beside each of the fulfilment centre's warehouses, the label size it names for each marketplace («58 × 40 мм — задаёт
  фулфилмент-центр»), or «размер маркетплейса» when it names none; «Маркетплейсы» shows a key that may only read with the level to add;
  the Express warehouses carry the note about the events mode (section F, point 4).
- **«Заказы»** — the details show the boxes with their labels (`ParcelList`), the lines (`OrderLines`), the codes' verdicts, the test mark
  (`TestOrderBadge`, Plan 7) and, for an Express order, its code checks (`CourierCodeChecks`: when, the answer, the attempts left; never
  the code). **«Передачи»** — the four steps of section E, a test order never among them. **«Журнал запросов»** — the label files as
  `{label file}`.
- **Notices** added to the bell: a problem on a Яндекс Маркет order (codes, an identifier to enter, a label size Маркет does not make,
  the layout or the status refused); a hand-over that waits for Яндекс Маркет (no shipment yet, the cutoff, orders to move); an Express
  check Яндекс Маркет refused.
- **`README.md`, «Работа с фулфилмент-центром»** gains Яндекс Маркет: the level «Обработка заказов и учёт товаров» the key needs now;
  **the plain list of what wyo then does at Яндекс Маркет by itself** — reports the boxes with their marking codes, fetches each box's
  label in the fulfilment centre's size, sets «Готов к отгрузке», compares the day's shipment with the hand-over, confirms it and passes
  its act, checks an Express courier's code — **and what it never does**: cancel an order, remove goods, move an order to another
  shipment, confirm a shipment with other orders, send DBS orders, send the key anywhere but Яндекс Маркет; **what the owner does in the
  cabinet** (register item 25 widened): moving or cancelling an order the fulfilment centre did not pack in time, the shipment schedule
  and its kind, confirmation codes for Express, pallets, the waybill of a collected shipment, the identifiers of section C, point 3;
  **test orders**: made in the cabinet, packed by the fulfilment centre, never handed over, cancelled by the owner after the try; a day
  with the exchange — where a Яндекс Маркет order that needs the owner shows and what to do.
- `docs/external-api.md` gains the Яндекс Маркет walk with each call on both sides; `docs/yandex.md` gains the writing methods and their
  periods; `docs/fulfilment-centre-api.md` gains the three additions (Stage 0, premise 2).
- **The constitution (MINOR):** principle XII — codes inside the packing report, the final «ready» write, a batch the marketplace makes
  alone (found, compared, confirmed, never changed), the courier-code hand-over and the code never kept, the label size the fulfilment
  centre's («the wyo server chooses nothing the fulfilment centre names»), the test mark passed on; the tests named.

## I. Rules R2, R3 and R4, proven here

- **R2, neutral:** `internal/exchange` and `internal/handover` hold no marketplace's name (the architecture test); the contract suite runs
  the writing parts for the three adapters; the specification's policy test of Plan 1 still passes with the three additions.
- **R3, alone by default:** with «Внешний API» empty, a made-up day with Яндекс Маркет, Ozon and Wildberries orders arriving sends nothing
  to the fake fulfilment centre, and nothing to Яндекс Маркет but readings.
- **R4, the keys stay:** the canary test of Plans 4 and 6 is widened to a Яндекс Маркет account through the whole walk, an Express order
  included: the Яндекс Маркет key, the fulfilment centre's key, the marking codes and the courier's code are searched for in every log
  line, journal row and answer, and — through the fakes' own call logs — in everything sent to the other side.

## J. Data, code, tests

**Data — one migration, `<next>_yandex_exchange_label_sizes_courier_codes`:** `fc_label_sizes` (the fulfilment centre's warehouse code,
the marketplace code, width and height in millimetres, when read); `exchange_orders` gains when «Готов к отгрузке» was sent and the
order's way out (`shipment` | `courier_code`); `exchange_courier_checks` (the order, the fulfilment centre's check number, the answer, the
attempts left, when it came and when it was answered — never the code). The upgrade test runs from the snapshot of Plan 7's release,
written first as Plans 5 and 7 did.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/orders/{id}` gives the boxes, the «ready» state and the code checks;
`GET /api/v1/handovers/{id}` gives Яндекс Маркет's steps and the shipment's kind; the matching of warehouses in «Внешний API» gives the
label sizes. No new address.

**The adapter contract** gains `CodesInPack`, `orders.ready`, the batch found and never changed, `orders.courier_code`, the size given
to `labels.fetch`, each with its cases in the one contract suite; the Яндекс Маркет adapter now supports `orders.pack`, `orders.ready`,
`labels.fetch`, `codes.verdict`, `orders.courier_code`, `handover.group` (find), `handover.close`, `handover.documents`.

**Code — where things live:** `internal/marketplace` (the contract's new parts), `internal/marketplace/yandex` (the writing parts, the one
status function), `internal/marketplace/wildberries` (the size from the fulfilment centre), `internal/exchange` (codes in the pack, the
ready write, the courier's code, the label sizes read and kept), `internal/handover` (the found batch), `internal/fulfilment` (the three
additions), `internal/fakes/yandex` (the writing routes, the daily shipment, the code check), `internal/fakes/fulfilment` (the label
sizes, the courier-code order and its event and answer, a test order never handed over); in the interface
`src/lib/components/exchange/`, `src/lib/components/handovers/`, `src/lib/components/external-api/`.

**Components created** (each once, with its test file in every state): `CourierCodeChecks` (an order's code checks without the code).
**Components reused:** `ParcelList`, `OrderLines`, `ProductCell`, `TestOrderBadge` (Plan 7), `HandoverSteps` (four steps),
`ExchangeStateBadge` (its new state), `HeldReasons`, `ProblemLine`, `CodeVerdictList`, `WarehouseMatchTable` (its new column),
`MarketplacesSection`, `OrderExchangeDetails`, `HandoverDetails`, `StatusBadge` with the tones, `NoteBox`, `EmptyState`, `DataList`,
`RowDetailsPanel`, `StepStrip`, the bell.

**Tests** (written before the code they check, failing first):

- **Go, with the fake Яндекс Маркет and the fake fulfilment centre in one test process:** the whole FBS walk — an order of two lines made
  in the fake, sent, reserved, packed in one parcel with codes, the layout reported, the codes accepted, the label fetched in the
  fulfilment centre's size and attached, the parcel closed, «Готов к отгрузке» sent, the hand-over asked, the day's shipment found,
  compared, confirmed after its cutoff, the act received by the fake fulfilment centre. **The whole Express walk** — sent as a courier-code
  order, packed, ready, a code refused with attempts, then accepted, handed over. **A test order** — sent with its mark, packed, ready,
  never in a hand-over and left out of the shipment's comparison. Then each branch: two parcels (two boxes, two labels); no code needed;
  a code refused and corrected; a code needed and missing; `UIN` needed; a cut by the fulfilment centre; a reopened parcel before and
  after «ready»; each label size of Яндекс Маркет and one it does not make; Wildberries' 58 × 40 and 40 × 30 from the fulfilment centre and
  its default without one; no shipment yet; the cutoff not reached; the shipment with an order the hand-over lacks, and the reverse; a
  shipment already confirmed; orders being excluded; the waybill not sent; a request withdrawn before and after the confirmation;
  `NEED_UPDATE`; a refused `verifyOrderEac`; a cancellation before and after «ready»; a changed shipment day.
- **Stopped in the middle:** the order's and the hand-over's Яндекс Маркет steps are interrupted after every single step in turn; after
  the restart the walk ends right, and the fake's call log shows no write made twice.
- **The Wildberries and Ozon walks and their branches pass unchanged.**
- **Never sent:** the tests of section I; no box layout with `allowRemove` or `partialCount`; no status but `READY_TO_SHIP` through the one
  status function and no other caller of the generated method; `transferOrdersFromShipment` never generated; the courier's code blanked
  after the answer; the budget never exceeded during a made-up hour at this plan's periods with 500 open Яндекс Маркет orders, the
  shipment methods' 200 an hour among them.
- **The fakes gain** the writing routes, the daily shipment of both kinds with its cutoff and `availableActions`, the act, the code check
  with its three answers; the fake fulfilment centre gains the label sizes, the courier-code order with its event and answer, and a test
  order it never hands over; each with its test.
- **Interface unit tests:** `CourierCodeChecks` once; `HandoverSteps` with Яндекс Маркет's four; the new state of `ExchangeStateBadge`;
  `WarehouseMatchTable`'s size column; each page only for what it adds; the policy tests green, the marketplace-name policy among them;
  `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes, each worker with its own data): a Яндекс Маркет warehouse matched
  with its label size shown; the whole FBS walk of a two-parcel order driven through the fakes' control calls and seen in «Заказы» and
  «Передачи»; an Express order's code refused and accepted, seen in its details; a test order marked and absent from «Передачи»; a
  shipment holding an order the hand-over lacks shown as a problem with its sentence; every new state of a page swept at 390 × 844 and
  the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises (the neutral specification's three additions among them). 2. The migration. 3. The contract's new parts of
   section A, under Wildberries' and Ozon's tests. 4. The generated writing client, the fakes' writing routes and the fake fulfilment
   centre's additions. 5. Step 1: orders out with the mark and the way out. 6. Step 2: the layout and the verdict. 7. Step 3: the labels
   in the fulfilment centre's size, «Готов к отгрузке». 8. Step 4: the daily shipment and the act. 9. Step 5: the courier's code. 10. Step
   6: cancellations. 11. The owner's view: «Внешний API», «Заказы», «Передачи», the notices. 12. The tests of section I and the interrupted
   walks. 13. The README, `docs/`, the help texts, the constitution. 14. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes): fill «Внешний API»; match a Яндекс Маркет FBS
  and an Express warehouse and see the label sizes; walk an FBS order of two parcels to the act, an Express order through a refused and an
  accepted code, and a test order to «Готов к отгрузке», looking at «Заказы», the details, «Передачи», the bell and the journal at each
  step; a shipment with an order the hand-over lacks; an Ozon and a Wildberries order walked beside them; empty the setting and see that
  nothing more is sent. Every page at a computer's width and at 390 × 844 with screenshots; the console, the raw-key check and the
  outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release run
  watched to its end; `main` pulled; the report, with «Found while implementing». At the merge the register is updated as «Not in this
  plan» and «After the release» say.

## After the release — the checks with the first real Яндекс Маркет cabinet and the first real fulfilment centre (register)

Test orders made in the cabinet first (they change nothing for the shop, YMC §9): the layout with codes and its verdict; a label printed
from the file at the fulfilment centre's size, and which barcode its scanner reads (then the scan text can be sent); «Готов к отгрузке»;
whether a test order enters a day's shipment; then a real FBS order: when the shipment appears and its cutoff, the confirmation, the act
of each kind of shipment; a real Express order: the courier's code as it really comes, how long the answer takes, an Express order
without `eacType`; whether Яндекс Маркет moves an unready order out of a shipment by itself (YMC §14, point 3). What they find becomes the
next plan's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **DBS orders** — register item 19 (Plan 7). **Stock** — register item 29.
- **Moving an order to the next shipment, cancelling or cutting an order at Яндекс Маркет:** each lowers the shop's quality index (YMC
  §10, §11); the owner does them in the cabinet — register item 25 widened to Яндекс Маркет.
- **Pallets and their labels, and the waybill of a collected shipment** (`setShipmentPalletsCount`, `downloadShipmentPalletLabels`,
  `…/transportation-waybill`): the neutral API carries no pallets and no `.xls`; the owner uses the cabinet; later, if asked.
- **Identifiers other than marking codes, and a product split over several boxes:** a problem that names them, entered in the cabinet
  (register item 24 widened), until the neutral API carries them.
- **The scan text of a Яндекс Маркет label** sent with it: after the check on a real label.
- **A new register item, «The checks of Plan 8 on the first real Яндекс Маркет cabinet and the first real fulfilment centre»,** holds
  «After the release».

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — Wildberries' and Ozon's own tests stay as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, marking code, courier's code or buyer datum in a log, a journal row, an
   answer or an error.
5. Everything is a component and is reused: one fulfilment-centre client, one exchange engine, one caller, one journal, one contract
   test suite, one schedule of asking again, one writing of a marking code, one comparison of a batch with a hand-over (Plan 6a's), one
   status function, one reading of a returned file; search before writing; the components section J names are created once. Every
   subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **No write to a marketplace is repeated blindly, no order is ever cancelled, cut or moved at a marketplace by the wyo server, and the
   wyo server chooses nothing the fulfilment centre names.** With «Внешний API» empty nothing is sent anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about Яндекс Маркет is
   written only from its official documents (YMC), with the address and the date; a gap is marked "not sourced", gets the cautious
   choice, and goes to «After the release». Яндекс Маркет's file is never edited.
9. An issue found on the way is fixed in this plan at once, or goes to the register with its reason when it cannot be; the report lists
   every finding under «Found while implementing».
10. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in its own words
    beside wyo's sentence; no sentence in Go. Name the program every time. Code is written mobile-first (390 × 844 first).
11. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

One new table for label sizes, one for code checks, two new columns, no new address of the wyo server's own API, one component, eight
contract parts in the Яндекс Маркет adapter, four contract changes made under Wildberries' and Ozon's unchanged tests, and the neutral
specification's three additions. The interrupted walks, the daily shipment's comparison and the Express code check are the largest test
work, and the parts that protect a seller from a shipment confirmed with the wrong orders or a parcel given to the wrong courier.
