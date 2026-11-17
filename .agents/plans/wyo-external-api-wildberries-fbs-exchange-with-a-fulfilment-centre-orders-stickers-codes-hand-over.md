# «External API» in the marketplace connector, part 3 of 3 of Stage 1: the setting that points the connector at a fulfilment centre's system, and the whole Wildberries FBS exchange through the neutral fulfilment-centre API — each new order sent, the Wildberries sticker fetched and attached as the label once the goods are reserved, the marking codes reported to Wildberries with its verdict passed back, the hand-over's supply closed and its documents sent, cancellations passed on

> **Plan 4** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 3 of the three plans of **Stage 1** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`, whose section «The exchange with a fulfilment centre — Wildberries FBS, step by step»
> this plan implements.
> Written 2026-11-17 from `origin/main` at `ab8838f`; **it starts only after Plan 3 is merged and released**
> (`.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`), and its section «Today» is
> checked again on that day.
> Research: `.agents/research/wildberries-official-description-files-check-2026-11-17.md` (cited «WBC»); `docs/fulfilment-centre-api.md`
> and `api/fulfilment-centre.yaml` (the neutral specification, written by Plan 1); the earlier research the high-level plan cites as
> «WBF» and «FA» (kept by the maintainer outside this repository).
>
> **Branch** `feat/external-api-wildberries-fbs-exchange`. One plan = one branch = one squashed commit = one pull request. The commit
> is `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: with «External API» empty, which is the default, nothing changes
> for anybody. **One database migration.** Constitution: the exchange's rules become a principle with its tests, **MINOR**.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the adapter contract**, **the engine**,
  **the journal** — as in Plans 2 and 3.
- **A fulfilment centre** — an outside warehouse company that stores, packs and hands over the seller's goods. **Its system** — the
  program it runs. wyo knows it only as an address and a key; it never learns which program it is.
- **«External API»** — the wyo setting that holds that address and key. Empty by default.
- **The neutral fulfilment-centre API** (short: the neutral API) — the format of `docs/fulfilment-centre-api.md`.
- **An event** — a fact the fulfilment centre's system reports about an order or a hand-over («reserved», «packing finished»). **A
  notification** — one signed message with an event, sent to an address of the wyo server.
- **A held order** — an order the fulfilment centre's system received but could not take in yet (an unknown product, an unknown
  warehouse); it keeps it with the reason.
- **A hand-over** — the parcels of one day that leave the fulfilment centre together for the marketplace.
- **An assembly order** — Wildberries' name for one FBS order (one unit of goods). **A supply** — Wildberries' group of assembly
  orders that travel together; putting an order into a supply is what makes Wildberries issue its **sticker**; a supply is **closed**
  when it is handed to delivery, and only a closed supply has its **QR code**. **A transport box** — a box of a supply with its own
  sticker.
- **A marking code** — the code of the state marking system printed on one unit of marked goods; **Wildberries' verdict** — its
  answer on whether the code is accepted.
- **The exchange** — everything this plan makes the wyo server do between Wildberries and the fulfilment centre.

## The maintainer's request and answers — nothing is open

- 2026-11-16, the high-level plan: the connector «serves as a channel between the marketplaces … and [a fulfilment centre's
  system]»; «The connector must not communicate with [a fulfilment centre's system] unless another setting is selected inside it -
  'external API', which the user of the connector must be instructed to point to [that system's] API with the key created inside
  [it].»
- The connector names no fulfilment product, «but must say in its description what it does, and mention, among other things, that
  it can work with external fulfillment centers»; its documentation «must have that neutral 'fulfillment center API'».
- The first version holds «the whole Wildberries FBS exchange with a fulfilment centre»; stock follows in Stage 2.
- 2026-11-17: no marketplace key and no real account yet — the exchange is built and tested only against the fake Wildberries and
  the fake fulfilment centre; the checks on real ones are listed in «After the release».
- 2026-11-17: «Russian only. But do create the languages capability.»; «Be very thorough with instructions on how to install it and
  use it» — section K.

## Today (to be checked again on the day the plan starts)

- After Plan 3 the wyo server reads Wildberries and writes nothing to it: the adapter supports `key.check`, `warehouses.read`,
  `orders.new`, `orders.states`, `orders.feed`; the contract names the writing parts and marks them unsupported. The engine has one
  caller with a budget and a journal; orders, their states and their history are stored; the pages «Маркетплейсы», «Заказы»,
  «Журнал запросов» exist.
- After Plan 1 the repository has the neutral specification, the generated client `internal/fulfilment/fcapi`, the fake fulfilment
  centre that sends signed notifications, and the fake Wildberries with supplies, stickers, marking-code verdicts, transport boxes,
  shipping settings and every documented refusal of closing a supply.
- The outbound gate (Plan 2, section H) lists no fulfilment centre: today the wyo server cannot call one.
- **Not sourced on Wildberries' side** (WBC §7), which shapes four cautious choices of this plan: whether sending marking codes
  replaces the earlier ones (so the earlier ones are removed first — section E); which text a sticker gives to a scanner (so no scan
  text is sent with the label — section D); whether a sticker survives its order's move between two open supplies (so the stickers
  are read again after the move and compared — section F); whether the list of new orders still holds an order the buyer cancelled
  (so an order's state is read before it is confirmed — section D).

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Wildberries' files are read again on the day** (as Plan 3, premise 2): every writing method of WBC §2, the refusal codes of
   closing a supply (`SupplyHasZeroOrders`, `MetaValidationFail`, `FulfillmentSupplyBoxesRequired`, `SupplyShippingRequired`), the
   sticker sizes («580x400 px при `width=58&height=40`»), the marking-code rules. A change that touches this plan stops the stage
   and asks.
3. **The neutral specification still equals the format the maintainer provides:** the one-time comparison of Plan 1, section I, is
   made again outside this repository. A difference is brought into the specification, the generated code and the fake first.
4. **The findings of Plan 3's checks with a real account, if they exist by then,** replace the cautious choices named in «Today»;
   otherwise the cautious choices stand and stay in «After the release».
5. **Wildberries' sandbox has no shipping-point, shipping-method or transfer method** (WBC §2, column S), so those steps can be
   proven only against the fake and, later, a real account. The plan says so wherever it matters.
6. **The signature scheme** of notifications is the Standard Webhooks specification, read again on the day; its published test
   vector is already a unit test (Plan 1).
7. A seller's token must be allowed to write: the read-only mark of the token (Plan 3, section B) is what the wyo server checks,
   because Wildberries has no harmless writing call to try.

## A. «External API» — the page «Внешний API»

Menu item «Внешний API» (`/external-api`). Until the setting is filled the page is an introduction — what a fulfilment centre is,
what wyo will do once the setting is filled, what it will never do — and the first section. The sections are folded forms, each
with its own «Сохранить» (`SectionForm`):

1. **«Подключение».** The address of the fulfilment centre's API (only `https://`) and its key. «Проверить и сохранить» calls
   `GET /api/v1/account` there and shows what came back: the name the fulfilment centre gives this seller, whether the key may
   write, the marketplaces it accepts orders of. The key goes to the secret store; afterwards the page shows its last four
   characters. A key that may only read is refused with the reason. «Отключить» empties the setting: from that moment nothing is
   sent anywhere but to the marketplaces.
2. **«Склады».** Each of the seller's warehouses on the marketplace (Plan 3 reads them) beside a choice of the fulfilment centre's
   warehouses (`GET /api/v1/warehouses`): «the fulfilment warehouse that stands for the seller's Wildberries warehouse» of the
   high-level plan. **Only orders of a matched warehouse are sent;** the others stay as they are, and their row says why.
3. **«Маркетплейсы».** For each account, the marketplace's code in the fulfilment centre's own list (proposed when a code equals the
   adapter's), and a warning with its remedy when the account's key may not write.
4. **«События»** — how the wyo server learns what the fulfilment centre did. «Запрашивать раз в минуту» is the default and works
   on any server. «Принимать уведомления» shows the wyo server's own address to copy into the fulfilment centre's system and a box
   for the signing secret that system shows; a test notification proves the pair. With notifications on, the wyo server still asks
   every 10 minutes, so that a lost notification is only late, never missed.
5. **The marketplace's own hand-over settings**, drawn from what the adapter declares (`AdapterSettingsForm`), never written into a
   page. For Wildberries: how the supply is shipped (by the seller itself, or by a transport company), the city, the kinds of
   receiving point, and the point itself (section F).

**The exchange works only while «External API» is filled and its last check passed** (high-level plan, «The fulfilment-centre
client»). The fulfilment centre's host joins the list of `internal/outbound` only then, and leaves it when the setting is emptied.

## B. The fulfilment-centre client — one, and neutral

- `internal/fulfilment` is the only package that knows the neutral API (an architecture test); it uses the generated client and
  sends every request through the engine's one caller (Plan 3, section C): the budget of the specification (300 requests a minute,
  the wait the answer names), the repeats' rules, and a journal row for every request, marked as sent to the fulfilment centre.
- **What never goes to a fulfilment centre** (rule R4 and the high-level plan's step 1), each proven by a test that searches
  everything the fake fulfilment centre received: a marketplace key; a buyer's address, comment or region. And the fulfilment
  centre's key never goes to a marketplace.
- A refusal of the fulfilment centre is turned into a state and a sentence by ONE function, table-driven: the key is not accepted;
  the API is switched off there; this marketplace is not enabled there; the order number was used with other lines; the order is
  already handed over; too many requests; the system is not available.

## C. The exchange engine

`internal/exchange` moves every order through the steps below. It knows no marketplace: for each step it asks the order's adapter
for a contract part (Plan 3, section A), and the adapter says for each order which **group** it belongs to (for Wildberries: the
seller's warehouse, the size class of the goods, cross-border or not, business buyer or not — one supply holds one group, «WBF»
§4.1) as a text the engine only compares.

- **Events are applied once and in order.** Every event is stored first (`fc_events`: its id, its number in the fulfilment
  centre's sequence, its kind); an event already stored is ignored; events are applied in the order of their numbers; the number of
  the last applied event is the mark from which `GET /api/v1/events` is asked.
- **A notification** arrives at `/hooks/fulfilment/<a random text made when notifications were switched on>`. The wyo server
  accepts it only with a right signature and a time no more than five minutes away, stores it, answers at once and applies it
  afterwards. The address is open to the internet by necessity; it accepts at most 256 KB and nothing unsigned.
- **A step that writes to a marketplace is never repeated blindly.** Before each write the engine reads the state it is about to
  change and skips a write that has already happened; every step's outcome is stored before the next begins, so a wyo server
  stopped in the middle goes on from the last finished step.
- **How often it asks while «External API» is filled** (the high-level plan, «How often it asks»): Wildberries' new orders every
  minute and open orders' states every 5 minutes, inside the budget of 300 requests a minute; the fulfilment centre's events every
  minute. Each timer has its few random seconds.
- **The exchange's own state of an order,** shown in the order list beside the marketplace's state: «не передаётся» (with the
  reason: the warehouse is not matched, «External API» is empty, the order was already on assembly when the exchange began),
  «передан», «ждёт у фулфилмент-центра» (held, with its reasons in words), «зарезервирован», «этикетка передана», «упакован»,
  «коды проверяются», «коды приняты», «передан в доставку», «отменён», and «проблема» with its sentence.

## D. Steps 1 and 2 — a new order, then its sticker

1. **A new order.** Every FBS order in the state «Новый» of a matched warehouse is sent with `POST /api/v1/orders`: the
   marketplace's order number as the sender's own number, the marketplace's code, the buyer's basket number, the matched warehouse,
   one line with the barcode, the article and the marketplace's own size number, the quantity 1, and the hand-over deadline (the
   order's creation time plus the account's hours, 48 by default). The same order sent twice is one order there (the specification
   recognises a repeat by the sender's number), so a request whose answer was lost is simply sent again by the next run.
    - **Accepted** — the order waits for its reservation.
    - **Held** — the order list shows «ждёт у фулфилмент-центра» with each reason in words («товар с артикулом … не найден у
      фулфилмент-центра», «склад не найден»); the fulfilment centre's system tries again by itself, and its event `order.accepted`
      moves the order on.
    - Orders that were already on assembly or in delivery when the exchange was switched on are never sent.
2. **The sticker.** When the fulfilment centre reports the goods reserved (`order.reserved`), the engine:
    - reads the order's state at Wildberries; an order no longer «Новый» is not confirmed (a cancelled one goes to section G);
    - asks the adapter for `orders.confirm`: the order is put into the account's open supply of its group — made when there is none,
      named «wyo — сборка <date>» — which moves it to «На сборке» and makes Wildberries issue the sticker;
    - asks for `labels.fetch`: the sticker as a picture of 58 × 40 mm, and attaches it with
      `PUT /api/v1/orders/{id}/parcels/1/label`. No scan text is sent with it (see «Today»).
   **It waits for the reservation on purpose:** putting an order into a supply ends the buyer's free hour of cancellation, and a
   later cancellation by the seller is fined (high-level plan, step 2).
3. **Goods that are not there.** An order that is held, or accepted and still not reserved when a quarter of its deadline's hours
   has passed (12 hours for the default 48), appears behind the bell and under the list's filter «Требуют внимания». **The owner
   decides** — wait, bring the goods, or cancel the order in the Wildberries cabinet; the wyo server never cancels at Wildberries
   (section G).
4. A refusal of Wildberries at these steps (it could not put the order into the supply) becomes a problem on the order at the
   fulfilment centre, `PUT /api/v1/orders/{id}/problem`, **in Wildberries' own words**, and a notice for the owner.

## E. Step 3 — the marking codes

- When the fulfilment centre reports packing finished (`order.packing_finished`, which carries each unit's code), and the order is
  one Wildberries takes codes for (it said so when the order was new; the adapter stores that with the order), the engine asks
  the adapter for `codes.report`. Codes sent earlier for the same order are removed first, because whether a second sending
  replaces the first is not sourced.
- **A code is kept and sent exactly as it was scanned,** with its separator character; ONE function writes it into a request, and a
  test proves the separator is escaped once, never twice.
- Then `codes.verdict` is asked until Wildberries' answer is final: every 20 seconds for 10 minutes, then every 5 minutes.
    - **Accepted** (or «проверка не требуется») — the order's exchange state becomes «коды приняты».
    - **Not finished in time** (Wildberries' own «проверка не завершена и будет продолжена») — shown as such; it does not stop the
      hand-over, as Wildberries says.
    - **Refused** — a problem on the order at the fulfilment centre with the verdict in Wildberries' own words (wrong format, no
      separator, not found, not in circulation, already sold), and a notice. When corrected codes pass, the problem is cleared with
      `DELETE …/problem`.
- **Wildberries wants codes and the fulfilment centre sent none** — a problem on the order («Wildberries требует код маркировки»).
  **The fulfilment centre sent codes and Wildberries takes none for this order** — nothing is sent, and the order's history says so.
- When the fulfilment centre reports the parcel reopened (`order.reopened`), the codes are removed at Wildberries and the engine
  waits for the next `order.packing_finished`.
- An order for which Wildberries requires another identifier than a marking code (a jewellery number, a phone's number, a customs
  declaration) gets a problem that names it and says it must be entered in the Wildberries cabinet («Not in this plan»).

## F. Step 4 — the hand-over

When the fulfilment centre asks for a hand-over's documents (`handover.requested`: the hand-over's number, its day, its parcels,
the number of transport boxes), the engine runs these steps for each group among the hand-over's orders — usually one; one supply
per group, as Wildberries requires — and stores each step's outcome:

1. **A new supply** named after the hand-over (`handover.group`).
2. **Exactly the hand-over's orders are moved into it** from the open supply they were confirmed into. Then their stickers are read
   again and compared with the ones attached in section D: **a sticker that changed stops the hand-over** with a problem that names
   the orders, because a parcel with an old sticker would not be accepted.
3. **Transport boxes** are added when the hand-over names a number.
4. **The shipping point, type and date** — Wildberries requires them before a supply is closed («WBF» §4.2, step 7). The type and
   the point come from the account's hand-over settings, the date from the hand-over's day. Wildberries gives the list of points
   only for a supply that already holds orders, so the point is chosen there: on the hand-over's page the owner picks from
   Wildberries' list for this supply, and the choice is remembered for the next hand-overs. A remembered point Wildberries no longer
   offers, or a first hand-over with no point yet, makes the hand-over wait with a notice — it is never guessed.
5. **The supply is closed** (`handover.close`).
6. **The documents** (`handover.documents`): the supply's QR code and each transport box's sticker, sent with
   `PUT /api/v1/handovers/{id}/documents` — the QR code as the kind `handover_code` with the supply's number as its scan text, the
   boxes' stickers as `box_label`, and the supply's number as the sender's own number of the hand-over.

- **What Wildberries refuses becomes a problem on the hand-over** (`PUT /api/v1/handovers/{id}/problem`) in Wildberries' own
  words — marking codes missing or not accepted, shipping settings missing, a transport box required, no orders — and a notice.
  After the cause is removed the owner presses «Повторить» on the hand-over's page, or the fulfilment centre asks again; the engine
  goes on from the step that failed.
- A request withdrawn (`handover.request_withdrawn`) before the supply is closed stops the steps and deletes the still empty
  supply; after it is closed nothing can be undone, and the page says so.
- **The page «Передачи»** (`/handovers`): the list of hand-overs (the number, the day, the state, the supplies' numbers, the
  problem) and, for one hand-over, the strip of its six steps with «Сейчас: …», the problem in Wildberries' words, the choice of
  the shipping point, «Повторить».

## G. Step 5 — cancellations

- **The marketplace cancelled.** The engine reads the state of every open order every 5 minutes; a cancellation is passed on with
  `POST /api/v1/orders/{id}/cancel`. The fulfilment centre's answer is shown as it is: cancelled; will be cancelled when the picker
  is back; a person at the warehouse must do it; already handed over.
- **The warehouse cancelled** (`order.cancelled` whose author is the warehouse), or a change waits for a person there
  (`order.change_waits`): shown to the owner behind the bell and under «Требуют внимания», with what he can do. **The wyo server
  never cancels an order at Wildberries by itself** — it costs the seller a fine (high-level plan, step 5). The adapter does not
  even hold that call: an architecture test fails if the generated cancel method is used.

## H. The owner's view

- **«Заказы»** gains the column «Обмен» (the exchange's state as a badge with a «?»), the filter «Требуют внимания», and in the
  details beside the list: the exchange's history, the held reasons, the problem, the marking codes' verdicts, and «Передать
  заново» for a step that failed for a reason now removed.
- **«Передачи»** — section F. **«Журнал запросов»** shows the requests to the fulfilment centre too, with a filter by where a
  request went.
- **Notice sources** added to the bell: the fulfilment centre does not answer or does not accept the key; held orders; an order
  not reserved in time; a marking code refused; a hand-over that waits or has a problem; an order cancelled by the warehouse; an
  account whose key may not write.
- **«Обзор»**: the card «Внешний API» (its state, the last event's time, how many orders need attention); the strip's fourth step.
- **Help in the interface, by the constitution's help-text rules:** every section of «Внешний API» begins with what it is for and
  what changes when it is filled; every exchange state, problem and button has its «?»; the page's first lines say plainly what the
  wyo server will write to the marketplace by itself once the exchange works — put orders into supplies, send marking codes, create
  and close supplies — and what it never does.

## I. Rules R2, R3 and R4, proven here

- **R2, neutral:** `internal/exchange` and `internal/fulfilment` hold no marketplace's name (an architecture test); the
  specification's policy test of Plan 1 still passes; «External API» accepts any `https://` address and any key.
- **R3, alone by default:** with «External API» empty, a made-up day of the scheduler with orders arriving sends nothing to the
  fake fulfilment centre — its own call log is empty — and the outbound gate holds no such host. Emptying the setting in the middle
  of an exchange stops every call to it at once.
- **R4, the keys stay:** the canary test is widened to the whole exchange: the marketplace token and the fulfilment centre's key
  are searched for in every log line, journal row, answer and — through the two fakes' own call logs — in everything sent to the
  other side.

## J. Data, code, tests

**Data — one migration, `0003_external_api_exchange`:** `fc_connection` (one row: the address, the secret's name, the key's last
four characters, the state, the last check, how events are received, the notifications' address text and secret name, the mark of
the last applied event), `fc_warehouse_map`, `fc_marketplace_map`, `fc_events`, `exchange_orders` (the order, its number at the
fulfilment centre, the exchange's state, the held reasons, the problem, the label's fingerprint, the supply it stands in),
`order_codes` (each unit's code, when it was sent, the verdict), `marketplace_supplies` (the account, the supply's number, its
group, whether it is the open assembly supply or a hand-over's), `handovers`, `handover_steps`; `orders` gains the group text and
the identifiers the marketplace takes for the order; `marketplace_accounts` gains the adapter's hand-over settings. Marking codes
are deleted 90 days after their order's hand-over by a nightly job.

**The wyo server's own API** (`api/wyo.yaml`): `GET`, `PUT` and `DELETE /api/v1/external-api`, `POST /api/v1/external-api/check`,
`GET` and `PUT /api/v1/external-api/warehouses`, `…/marketplaces`, `…/events`; `GET /api/v1/handovers`, `GET /api/v1/handovers/{id}`,
`POST …/retry`, `GET` and `PUT …/shipping-point`; `POST /api/v1/orders/{id}/exchange/retry`; and `POST /hooks/fulfilment/{text}` —
the one address of this plan that is open without sign-in and accepts only a signed notification; it carries no cookie, so the
rule against writes from another site (Plan 2, section C) does not apply to it, and the signature stands in its place. Each is a
row of the access table.

**The adapter contract** gains the cases of its writing parts in the one contract test suite; the Wildberries adapter now supports
`orders.confirm`, `labels.fetch`, `codes.report`, `codes.verdict`, `handover.group`, `handover.close`, `handover.documents`, and
declares its hand-over settings.

**Code — where things live:** `internal/fulfilment` (the client, the refusals' function, the notifications' receiver),
`internal/exchange` (the steps, the events, the stored outcomes), `internal/marketplace/wildberries` (the writing parts),
`internal/handover`; in the interface `src/routes/(app)/external-api`, `handovers`, and `src/lib/components/external-api/`,
`exchange/`, `handovers/`.

**Components created** (each once, with its test file in every state): `ConnectionCheckResult`, `WarehouseMapTable`,
`MarketplaceCodeMap`, `EventsModeForm` (with the address to copy and the secret box), `AdapterSettingsForm` (drawn from the adapter's
declared settings), `ExchangeStateBadge`, `HeldReasons`, `ProblemLine` (a problem in the marketplace's or the fulfilment centre's
words, with its «?»), `CodeVerdictList`, `HandoverSteps`, `ShippingPointPicker`. **Components reused:** `SectionForm`,
`DisclosureForm`, `Field`, `TextBox`, `SelectBox`, `CopyButton`, the secret-box rules, `DataList`, `FilterSection`,
`RowDetailsPanel`, `StepStrip`, `StatusBadge` with the tones, `NoteBox`, `EmptyState`, `PageHeading`, `Stack`, `Button`,
`BlockedButton`, `ModalWindow` with `WindowButtons`, `InstallationStateCard`, `MarketplaceMark`, `OrderDetails`, the bell.

**Tests** (written before the code they check, failing first):

- **Go, with both fakes in one test process:** the whole walk — an order made in the fake Wildberries, sent, reserved, confirmed
  into a supply, its sticker attached, packed with codes, the codes accepted, the hand-over asked, the new supply made, the orders
  moved, the boxes added, the shipping set, the supply closed, the documents received by the fake fulfilment centre. Then each
  branch: a held order and its release; an order cancelled before its confirmation; each refusal of Wildberries at each step to its
  problem in Wildberries' words; each verdict of a code; codes removed after a reopening; a sticker that changed after the move;
  a shipping point no longer offered; a request withdrawn before and after the closing; each answer of the fulfilment centre to a
  cancellation; a cancellation by the warehouse and the cancel method never called.
- **Stopped in the middle:** the hand-over's and the order's steps are interrupted after every single step in turn; after the
  restart the walk ends right, and the fake Wildberries' call log shows no write made twice.
- **Events:** applied in order; a repeat ignored; a notification with a wrong signature or an old time refused; asking and
  notifications together lose and double nothing.
- **Never sent:** the tests of sections B and I; the separator of a marking code; the budget never exceeded during a made-up hour
  at this plan's periods with 500 open orders.
- **The fakes gain** what this plan needs (a sticker that changes on command, a point that disappears); each addition has its test.
- **Interface unit tests:** each new component once; each page only for what it adds; the policy tests green, the
  marketplace-name policy among them; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with both fakes, each worker with its own data): «Внешний API» filled,
  checked and emptied; the warehouses matched; the whole walk driven through the fakes' control calls and seen in «Заказы» and
  «Передачи»; a held order with its reason; a refused code as a problem; the shipping point chosen; a notification received; no
  request to any address but the wyo server's own from the browser; every new page swept at 390 × 844 and the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration and the stores. 3. The fulfilment-centre client, the refusals, the outbound gate's
   rule. 4. «Внешний API»: the calls and the page's five sections. 5. Events: the asking, the notifications' receiver, the order of
   applying. 6. Steps 1 and 2: orders out, held orders, the confirmation, the sticker. 7. Step 3: the marking codes and verdicts.
   8. Step 4: the hand-over, the page «Передачи», the shipping point. 9. Step 5: cancellations. 10. The owner's view: the column, the
   filter, the notices, «Обзор». 11. The tests of section I and the interrupted walks. 12. The README, `docs/`, the help texts, the
   constitution. 13. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with both fakes): fill «Внешний API»; match a
  warehouse; make an order in the fake Wildberries and walk it to the hand-over's documents through the fakes' control calls,
  looking at «Заказы», the details, «Передачи», the bell and the journal at each step; a held order; a refused code; empty the
  setting and see that nothing more is sent. Every page at a computer's width and at 390 × 844 with screenshots; the console, the
  raw-key check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release
  run watched to its end; `main` pulled; the report. **With this release the high-level plan's Stage 1 is complete.**

## After the release — the checks with the first real account and the first real fulfilment centre (register)

A real Wildberries order printed at 58 × 40 mm from the sticker the wyo server attached, and the text its code gives to a scanner
(then the scan text can be sent with the label); whether a sticker changes when its order moves between two open supplies; whether
a second sending of codes replaces the first; how long a verdict stays unfinished; which supplies Wildberries refuses without a
transport box; the shipping points' list and the closing of a supply on a real account (the sandbox has neither); the time between a
hand-over's request and its documents; notifications received at a real address behind a real certificate. What they find becomes
the next plan's premises.

## K. The instructions

- **`README.md` gains «Работа с фулфилмент-центром»**, step by step for a seller: what «Внешний API» is and that wyo talks to nobody
  but the marketplaces until it is filled; what to ask the fulfilment centre for (the address of its API and a key made in its
  system for this seller; that its system must speak the fulfilment-centre API, with the link to the specification); filling the
  five sections with an example of each; the token's right to write and how to make such a token; **the plain list of what wyo then
  does at Wildberries by itself and what it never does**; a day with the exchange — where an order that needs the owner shows and
  what to do in each case; how to stop the exchange.
- **`docs/external-api.md`:** the exchange step by step with each call on both sides, for a seller's engineer and for a fulfilment
  centre's developer. **`docs/fulfilment-centre-api.md`** gains the section the high-level plan's rule R4 asks for: a fulfilment
  centre's staff never install the connector for a seller and never hold a seller's marketplace key, with the reason in
  Wildberries' own words.
- The README's first paragraph says what the program does and that it «может работать с внешними фулфилмент-центрами»; nothing
  anywhere names a fulfilment product.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Stock** (the fulfilment centre's free stock sent to the marketplaces — the high-level plan's step 6), **Ozon and Яндекс
  Маркет** — Stage 2. **Supplies to marketplace warehouses** — Stage 5.
- **Identifiers other than marking codes** (a jewellery number, a phone's number, an expiry date, a customs declaration): the order
  gets a problem that names the identifier; a later plan.
- **Cancelling at Wildberries from wyo; moving an order back from a closed supply; orders Wildberries wants shipped again; passes
  to Wildberries' warehouses; days off:** the owner does these in the Wildberries cabinet; each is named in the README.
- **Packing without a fulfilment centre** (printing stickers from wyo itself): high-level plan, «Not in this plan».
- **The scan text of a sticker sent with the label:** after the check on a real sticker.
- **Several fulfilment centres at once:** one «External API»; the high-level plan names one setting.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, marking code or buyer datum in a log, a journal row, an answer or an
   error.
5. Everything is a component and is reused: one fulfilment-centre client, one exchange engine, one caller, one journal, one
   contract test suite; search before writing; the components section J names are created once; a page writes nothing a component
   already draws. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/wildberries` and the message keys `marketplace.wildberries.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **No write to a marketplace is repeated blindly, and no order is ever cancelled at a marketplace by the wyo server.** With
   «External API» empty nothing is sent anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about Wildberries
   is written only from its official documents, with the address and the date; a gap is marked "not sourced", gets the cautious
   choice, and goes to «After the release».
9. Every visible text is a key of `messages/ru.json`, written by the help-text rules; a marketplace's refusal is shown in the
   marketplace's own words beside wyo's sentence; no sentence in Go. Name the program every time.
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

Nine tables, about fifteen calls of the wyo server's own API, two pages and a grown order list, eleven components, seven contract
parts in the Wildberries adapter, and the exchange engine with its stored steps. The interrupted-walk tests are the largest test
work and the part that protects a seller from a supply closed twice or a code sent to the wrong order.
