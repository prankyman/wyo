# Wildberries in the marketplace connector, part 2 of 3 of Stage 1: the marketplace-adapter contract, the Wildberries adapter that only reads, the seller's Wildberries accounts with their keys' health and expiry dates, the synchronisation engine with its request budget, one order list (FBS orders with their state, FBO orders read-only) and the journal of every request

> **Plan 3** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 2 of the three plans of **Stage 1** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`.
> Written 2026-11-17 from `origin/main` at `ab8838f`; **it starts only after Plan 2 is merged and released**
> (`.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`), and its
> section «Today» is checked again on that day.
> Research: `.agents/research/wildberries-official-description-files-check-2026-11-17.md` (cited «WBC»: Wildberries' own description
> files read on 2026-11-17); the pages saved in `docs/marketplaces/`; the earlier research the high-level plan cites as «WBA», «WBF»,
> «MC» (kept by the maintainer outside this repository).
>
> **The plan after it.** Plan 4
> `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md` adds the calls
> that write to Wildberries and the exchange with a fulfilment centre. **This plan writes nothing to Wildberries.**
>
> **Branch** `feat/wildberries-accounts-orders-journal`. One plan = one branch = one squashed commit = one pull request. The commit is
> `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: the release only adds pages and tables. **One database migration.**
> Constitution: the adapter contract and the synchronisation engine become principles with their tests, **MINOR**.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **the seller** — as in Plan 2.
- **A marketplace key** — the seller's own key or token of a marketplace. For Wildberries it is a **token**: a long text made in the
  seller's Wildberries cabinet, valid 180 days.
- **An account** — one seller profile on one marketplace, with its key. A seller may have several.
- **An adapter** — the one Go package that knows one marketplace. **The adapter contract** — the one list of things every adapter can
  be asked to do; an adapter says which of them it supports.
- **The synchronisation engine** (short: the engine) — the part of the wyo server that decides when to ask a marketplace, keeps
  within its limits, repeats what may be repeated and writes the journal.
- **A request budget** — how many requests may still be sent to one marketplace for one account right now.
- **A mark** — the stored point up to which a feed has been read, so the next reading goes on from there.
- **The journal** — one stored row for every request the wyo server sent: when, where, the method, the path, the answer's status,
  how long it took. Never a key, never a body.
- **FBS** — the order is packed outside the marketplace and handed to it; Wildberries calls such an order a «сборочное задание»
  (assembly order). **FBO** — the goods lie in the marketplace's own warehouse.
- **The fake Wildberries** — the Go program of Plan 1 that answers as Wildberries would.

## The maintainer's request and answers — nothing is open

- 2026-11-16, the high-level plan: «The website (software) itself upon the first log-in just asks for the marketplaces … seller API
  keys and then just works. It periodically, like once in half hour pulls the data for the software itself (or can do it on demand by
  button click … but allow it not too often)».
- The high-level plan's first version: «the keys of Wildberries with their health and expiry dates; one order list (FBS orders with
  their state; FBO orders read-only); … the journal of requests».
- 2026-11-17: no marketplace key exists yet — «Right now you'll have to do all work pretty much blindly». Everything is built and
  tested against the fake Wildberries; the checks on a real account are listed in «After the release».
- 2026-11-17: «Russian only. But do create the languages capability.»; «Be very thorough with instructions on how to install it and
  use it» — section J.

## Today (to be checked again on the day the plan starts)

- After Plan 2 the wyo server has a database, the owner's sign-in, the menu drawn from one list, the settings registry, the notice
  sources behind the bell, the secret store (never used yet), the scheduler, and the outbound gate that lists every address the wyo
  server may call (so far: the release address and the certificate authority).
- After Plan 1 the repository has Wildberries' description files under `api/wildberries/`, the generated client for the methods of
  WBC §2, and the fake Wildberries with Wildberries' own request arithmetic. Nothing calls them yet.
- The list component `DataList` and the date-range fields wait in the other repository (Plan 1, section E).
- **Wildberries' terms allow this use** (high-level plan, «What the research found»): a program the seller installs on a rented
  server is a local version that takes the **Personal token**; that token may not be handed to third parties or used in cloud
  services — which rule R4 excludes.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Wildberries' files are read again on the day.** The six files of WBC §1 are downloaded; a difference from the committed copies
   under `api/wildberries/` is read line by line; the methods of WBC §2 and the fields of WBC §6 must still be there. A change that
   touches this plan stops the stage and asks.
3. **How a seller makes the token is taken from Wildberries' own page**, read again on the day: the saved copy
   `docs/marketplaces/WB - Как создать, обновить или удалить токен WB API.pdf` and its live address. The steps of section J and of the
   interface's help are written from it, with the address and the date.
4. **The token's own fields** (WBC §4: type, end of life, seller profile, categories, read-only mark, test mark) are marked "earlier
   research, not re-read". They are read again in Wildberries' knowledge base on the day, by a person in a browser if a script is
   refused. If a field differs or cannot be confirmed, the check of section B relies on `/ping` and the seller's information only,
   and the constitution says so.
5. `sqlc` and the migration rules of Plan 2 are in place; this plan's migration is the second.

## A. The adapter contract — one, for every marketplace

`internal/marketplace` holds the contract; nothing marketplace-specific lives outside an adapter's own package (rule R6).

- **What an adapter says about itself:** its code (`wildberries`), the message keys of its name and help, its colour mark's name
  (Plan 1, section F), the fields of its key form (for Wildberries: one field, the token), and **its capabilities** — the list of
  contract parts it supports.
- **The contract's parts.** This plan defines every part Stage 1 needs and implements the reading ones:

    | Part                                    | What it does                                                                                              | Wildberries, this plan |
    | --------------------------------------- | --------------------------------------------------------------------------------------------------------- | ---------------------- |
    | `key.check`                             | Checks a key and says whose it is, what it may do and when it ends                                         | yes                    |
    | `warehouses.read`                       | The seller's warehouses on the marketplace                                                                 | yes                    |
    | `orders.new`                            | New orders of the seller-warehouse scheme                                                                  | yes                    |
    | `orders.states`                         | The present state of known orders                                                                          | yes                    |
    | `orders.feed`                           | Orders of every scheme, read on from a mark                                                                | yes                    |
    | `orders.confirm`, `labels.fetch`        | Confirms an order for packing; fetches its label                                                           | Plan 4                 |
    | `codes.report`, `codes.verdict`         | Reports marking codes; reads the marketplace's verdict                                                     | Plan 4                 |
    | `handover.group`, `handover.close`, `handover.documents` | Groups orders for hand-over, closes the group, fetches its documents                       | Plan 4                 |
    | `stock.push`, `stock.read`, `sales.read`, `money.read`, `products.read`, `supplies.*` | The parts of Stages 2–5 of the high-level plan              | named, not supported   |

- **The screens ask the adapter, never the marketplace's name.** `GET /api/v1/marketplaces` gives the interface each adapter's
  description and capabilities; the key form, the marks, the columns that exist only for some capability are drawn from it.
- **Enforced by tests:** an architecture test fails when a package outside `internal/marketplace/wildberries` imports the generated
  Wildberries client or holds the marketplace's name in a text; `marketplace-name-policy.test.ts` fails when a file under `src/`
  names a marketplace outside the message keys of the family `marketplace.<code>.*`.
- **One contract test suite**, `RunAdapterContract`, is written once and run for every adapter against its fake: an adapter that
  claims a capability must pass that capability's cases. The Wildberries adapter is its first caller; Ozon and Яндекс Маркет
  (Stage 2) will be the second and third without a new suite.

## B. Accounts and keys — the page «Маркетплейсы»

- **`/marketplaces`** lists the seller's accounts as cards: the marketplace's mark and name, the account's own name, its state, the
  key's last four characters, the day the key ends, the last successful reading, the last refusal in words. «Добавить аккаунт»
  opens the form (`DisclosureForm`) drawn from the adapter's description. Several accounts of one marketplace are allowed.
- **What happens when the owner saves a Wildberries token** (the part `key.check`):
    1. The token's own fields are read without calling anybody (WBC §4): its type, its end of life, the seller profile, the
       categories it opens, the read-only mark.
    2. **A Personal token is accepted.** A test token is accepted and the account is marked «тестовый»: it talks only to
       Wildberries' sandbox hosts (WBC §2, column S), which is how the first check without real orders is made. The orders feed
       has no sandbox host, so a test account shows FBS orders only, and its card says so. A token of the type
       «Базовый» or «Сервисный» is refused with the reason and the remedy: the first has limits too low for this work (WBC §3: the
       orders feed once in 3 hours), the second belongs to a cloud service of Wildberries' catalogue.
    3. The category «Маркетплейс» is required. Without the category «Статистика» the account is accepted and the card says in a
       sentence that FBO orders and product names will not be shown, and how to add the category.
    4. `GET /ping` on each needed host, then `GET /api/v1/seller-info`: the seller's name is shown, and the profile id must equal the
       token's own.
    5. The token is stored only in the secret store (Plan 2, section B); the page ever after shows its last four characters and the
       day it ends, and the box empties after the save.
- **Replacing a key:** the same form on the account's card. A token of another seller profile is refused («это ключ другого
  продавца — добавьте его как новый аккаунт»), so an account's orders never change owner.
- **States of an account,** each a tone of the status badge with a «?» that says what it means and what to do: «Работает»; «Ключ
  истекает через N дней» (14 days before the end and again 3 days before); «Ключ истёк»; «Ключ не принят» (Wildberries answered
  401: every reading of this account stops until a check passes or the key is replaced); «Нет связи» (five failures in a row);
  «Отключён» (by the owner: nothing is asked, the orders stay).
- **Removing:** an account from which no order ever came may be deleted; any other is only switched off.
- **Per account settings:** «Срок передачи заказа, часов» (48 by default: the limit after which Wildberries fines — high-level plan,
  the exchange's step 1), used for the deadline column of FBS orders.
- **Notice sources** added to the bell (Plan 2, section D): a key that ends soon or has ended; a key not accepted; an account without
  connection.
- **What Wildberries' answers look like** (401 with «token expired», 403 with «scope is not allowed for this resource», the 403 of a
  new cabinet whose seller-warehouse sales are not switched on) is turned into the account's state and a sentence by ONE function of
  the adapter, with a table-driven test.

## C. The synchronisation engine

`internal/sync`, built on the scheduler of Plan 2 — one engine for every marketplace and, in Plan 4, for the fulfilment centre.

- **One caller.** Every request to a marketplace goes through `sync.Caller`: it takes the key from the secret store only for the
  moment of the call, names the program (`User-Agent: wyo/<version> (+https://github.com/prankyman/wyo)`, rule R5), keeps the
  budget, the deadline and the repeats, and writes the journal row. An adapter never makes a request itself (an architecture test).
  The marketplaces' hosts join the list of `internal/outbound`.
- **The budget.** For each account and each of the marketplace's limit groups, a count of requests that may be sent now, started
  from the limits the adapter declares (for Wildberries' seller-warehouse methods: 300 a minute, 200 milliseconds apart, 20 at once;
  the feed: 1 a minute — WBC §2) and corrected by every answer's `X-Ratelimit-Remaining`. An answer with a client error costs 10
  (WBC §5). The engine never sends a request the budget does not allow; on 429 it waits as `X-Ratelimit-Retry` says — up to 30
  seconds in place, longer by coming back later — and doubles the wait when 429 repeats.
- **Repeats.** A reading that failed for the network or with a server error is repeated after 1 and 3 seconds (each shifted at
  random by a fifth); a client error is never repeated; a writing call (Plan 4) is never repeated by the caller. A redirect is
  refused. The deadline of one call is 30 seconds.
- **What is asked and how often** (the high-level plan, «How often it asks», for an installation without «External API»):

    | Job, per account         | Contract part       | Period                                         |
    | ------------------------ | ------------------- | ---------------------------------------------- |
    | New orders               | `orders.new`        | 30 minutes                                     |
    | States of open orders    | `orders.states`     | 30 minutes (up to 1 000 orders a request)      |
    | The orders feed          | `orders.feed`       | 30 minutes (Wildberries renews it as often)    |
    | Warehouses               | `warehouses.read`   | once a day and on a key check                  |
    | The key check            | `key.check`         | once a day                                     |

    Each timer is shifted by a few random seconds, only so that many installations do not ask in the same second (rule R5).
- **«Обновить»** on the order list starts the three order jobs of every working account at once — and is answered at most once in
  5 minutes: until then the button is drawn blocked with the sentence «Обновить можно через N мин.» (`BlockedButton`).
- **Marks** are stored per account and feed; a job that failed leaves its mark where it was.
- **A failure never passes silently:** five in a row make the account «Нет связи»; an unaccepted key stops the account's jobs; each
  shows behind the bell and on the account's card.

## D. Orders — one list for every marketplace and scheme

- **One table `orders`**, one row for one order of one account: the marketplace's own number, the scheme, the buyer's basket number,
  the seller's warehouse on the marketplace, the article, the barcodes, the marketplace's own product numbers, the product's kind,
  brand and size when known, the times, the deadline, the connector's state, the marketplace's state as it was received. And
  `order_events`: every change of state with its time and where it came from.
- **No buyer data is stored.** A Wildberries assembly order may carry the buyer's address and the buyer's comment (the earlier
  research «WBF» §2.3); the feed carries the buyer's region (WBC §6). The adapter drops all three at its edge, and a test proves
  that none of them reaches the database, the journal or a log.
- **FBS orders** come from `orders.new` (Wildberries: `GET /api/v3/orders/new`) and are kept current by `orders.states`
  (`POST /api/v3/orders/status`). Wildberries gives one unit of goods per assembly order; the quantity is always 1.
- **FBO orders and the products' names** come from `orders.feed` (`GET /api/v1/supplier/orders`, read on from its mark — WBC §6).
  A feed row whose order id equals a known FBS order's fills that order's kind, brand and size; a row of a Wildberries warehouse
  («Склад WB») that matches none becomes an FBO order, **read-only**; a row of the seller's warehouse that matches none is left
  for the FBS reading to bring.
- **The connector's own states,** the same for every marketplace; the adapter maps the marketplace's states onto them, and the
  marketplace's own words are shown beside:

    | State             | On the screen        | Wildberries' pair of statuses (seller's / Wildberries') — «WBF» §3                              |
    | ----------------- | -------------------- | ------------------------------------------------------------------------------------------------ |
    | `new`             | «Новый»              | `new` / `waiting`                                                                                |
    | `assembling`      | «На сборке»          | `confirm` / `waiting`                                                                            |
    | `in_delivery`     | «В доставке»         | `complete` / `waiting`, `sorted`, `ready_for_pickup`                                             |
    | `received`        | «Получен покупателем»| `complete` / `sold`                                                                              |
    | `cancelled`       | «Отменён» with whom  | `cancel`; or Wildberries' `canceled`, `declined_by_client`, `canceled_by_client`, `defect`       |
    | `unknown`         | the received words   | any pair the table does not hold — shown as received and counted on «Обзор»                       |

- **The deadline** of an FBS order is its creation time plus the account's hours; the column turns to the attention tone 6 hours
  before it and to the stop tone after it.
- **The page «Заказы»** (`/orders`) — the list component `DataList`, copied now with `ColumnHeading` and `ColumnLegend` and their
  tests: a table on a computer, cards on a phone. Columns: the marketplace's mark and the account; the scheme; the number; the
  product (kind, brand, size, article, barcode); the warehouse; created; the deadline; the state. Filters in the address, so a
  filtered list can be reopened (`FilterSection`): account, scheme, state, period, and a search by number, article or barcode.
  Pages of 50. A row opens the order's details beside the list (`RowDetailsPanel`): every field, the marketplace's own state in its
  words, the history of states. Every empty state teaches: no account yet — a sentence and a link to «Маркетплейсы»; nothing found
  by the filter — what was filtered.
- **«Обзор»** gains its cards: the accounts and their keys; new orders, orders on assembly and orders past their deadline; the last
  successful reading. The strip of steps moves on.

## E. The journal — the page «Журнал запросов»

- **One row for every request** the wyo server sent to a marketplace (and, in Plan 4, to a fulfilment centre): the time, the
  account, the method, the path with placeholders in place of every number («/api/v3/supplies/{supplyId}/deliver»), the status,
  the outcome in one of six words (done, limit met, refused, server error, no connection, key not accepted), the error's code and
  its text of at most 500 characters with anything secret blanked, the duration, the job that sent it. **Never a key, never a
  query, never a body** — high-level plan, «The synchronisation engine».
- The table takes inserts only (a database trigger refuses a change of a row); a nightly job deletes rows older than 30 days.
- **`/journal`**: the list (`DataList`), newest first, with filters by account, outcome and period (`DateRangeFields`, copied now);
  an introduction that says what the journal is for — to see what wyo asked a marketplace and what it was answered — and that it
  holds no key.
- A journal row that cannot be written never changes the outcome of the request.

## F. Rules R3, R4 and R5, proven here

- **R3, alone by default:** a test runs the scheduler through a made-up day with accounts present and «External API» empty, behind
  an outbound gate that records every address: only the marketplaces' hosts, the release address and the certificate authority
  appear. With no account, no marketplace host appears either.
- **R4, the key stays:** the canary test of Plan 2 is widened — a distinctive token is saved, every job of this plan is run against
  the fake Wildberries with every kind of failure, and the token must be in no log line, no journal row, no answer of the wyo
  server's own API, no error text. The fake Wildberries' own call log proves the token went only into the `Authorization` header.
- **R5, nothing hidden:** every request carries the program's name and version; the budget is never exceeded in a test that drives
  the engine against the fake Wildberries' real arithmetic for a made-up hour at full load; no proxy setting exists.

## G. Data, code, tests

**Data — one migration, `0002_accounts_orders_journal`:** `marketplace_accounts` (the marketplace's code, the account's name, the
state, the seller's name and profile id, the secret's name, the key's last four characters, type and end of life, the categories,
the hours of the deadline, the last check and its outcome, the failures in a row), `marketplace_warehouses`, `sync_marks`, `orders`,
`order_events`, `request_journal` with its trigger. The numbers a marketplace gives are stored as text; nothing sorts them as
numbers.

**The wyo server's own API** (`api/wyo.yaml`): `GET /api/v1/marketplaces`; `GET` and `POST /api/v1/accounts`, `GET`, `PATCH` and
`DELETE /api/v1/accounts/{id}`, `PUT /api/v1/accounts/{id}/key`, `POST /api/v1/accounts/{id}/check`, `POST …/disable`,
`POST …/enable`; `GET /api/v1/orders`, `GET /api/v1/orders/{id}`; `POST /api/v1/sync/refresh`; `GET /api/v1/journal`. All for the
signed-in owner; each is a row of the access table.

**Code — where things live:** `internal/marketplace` (the contract, the registry, the contract test suite),
`internal/marketplace/wildberries` (the adapter; `wbapi/` is generated), `internal/sync` (the caller, the budget, the jobs, the
marks), `internal/journal`, `internal/orders`, `internal/accounts`; in the interface `src/routes/(app)/marketplaces`, `orders`,
`journal`, and `src/lib/components/marketplaces/`, `orders/`, `journal/`.

**Components created** (each once, each with its test file in every state): `MarketplaceMark` (the coloured dot and the name, from
the adapter's description), `AccountCard`, `KeyForm` (drawn from the adapter's key fields), `KeyHelpSteps` (how to make the key, a
`StepStrip` with the steps' texts), `AccountStateBadge`, `OrderStateBadge`, `SchemeBadge`, `DeadlineCell`, `OrderDetails`,
`RefreshButton` (the 5-minute rule), `JournalOutcomeBadge`. **Components reused:** `DataList` with `ColumnHeading` and
`ColumnLegend`, `FilterSection`, `RemovableChip`, `DateRangeFields`, `Pagination`, `RowDetailsPanel`, `DisclosureForm`,
`SectionForm`, `Field`, `TextBox`, `NumberField`, `SelectBox`, `Button`, `BlockedButton`, `ShowOnceSecret`'s rules for a secret box,
`StatusBadge` with the tones, `NoteBox`, `EmptyState`, `PageHeading`, `Stack`, `InstallationStateCard`, the bell.

**Tests** (written before the code they check, failing first):

- **Go:** the contract test suite and the Wildberries adapter under it; the token's fields (a Personal token; a test token goes to
  the sandbox hosts; «Базовый» and «Сервисный» refused; an ended token; a text that is not a token; a missing category; the
  read-only mark); the check (each of Wildberries' refusals to its state and sentence; a profile id that differs); the intake of new
  orders (the same order twice is one row); every pair of the states table and an unknown pair; a cancellation noticed; the feed read
  on from its mark, joined to FBS orders, turned into FBO rows; no buyer data anywhere; the deadline; the budget with the injected
  clock (the limits, an error counted as 10, the header's correction, the wait after 429 and its doubling); the repeats (a server
  error twice, a client error never); five failures; an unaccepted key stops the jobs; the 5-minute rule; the random shift's bounds;
  one journal row per request and what it never holds; the journal's trigger and its 30 days; the tests of section F; the access
  table.
- **The fake Wildberries gains** the orders feed and the control calls this plan needs; each addition has its own test.
- **Interface unit tests:** each new component once; each page only for what it adds; `marketplace-name-policy`; the policy tests
  green; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fake Wildberries, each worker with its own data): an account added
  with a made-up token and its card; each refusal's sentence; an order made in the fake appears after «Обновить», and the button's
  5 minutes; a cancellation shows; the filters and the address; the details beside the list; a key that ends in two days shows
  behind the bell; the journal with its filters; no request to any address but the wyo server's; every new page swept at 390 × 844
  and the wider widths.

## H. Measured

The memory of the wyo server with three accounts and 10 000 orders in its database, reading the fake Wildberries at this plan's
periods for a made-up day, is measured in the pipeline and written into the README's requirements with the date (Plan 2, section J).

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration and the stores. 3. The adapter contract, the registry, the contract test suite.
4. The caller, the budget, the journal. 5. The Wildberries adapter: the key check, warehouses. 6. Accounts: the calls, the page
   «Маркетплейсы», the notices. 7. Orders: new, states, the feed, the states table. 8. The list component copied; the page
   «Заказы»; «Обновить». 9. The page «Журнал запросов». 10. «Обзор», the tests of section F, the measurement. 11. The README, the
   help texts, the constitution. 12. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fake Wildberries): add an account; see its
  card and the key's last four characters only; make three orders in the fake and press «Обновить»; cancel one in the fake; filter;
  open the details; open the journal; every page at a computer's width and at 390 × 844 with screenshots; the console, the raw-key
  check and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the merge by the agent; the release
  run watched to its end; `main` pulled; the report.

## I. After the release — the checks with the first real account (register)

With a test token first (the sandbox), then with a Personal token: that the token's fields are as WBC §4 says; the two hosts'
`/ping`; the real limit headers and the budget's agreement with them; whether a new cabinet answers 403 until seller-warehouse sales
are switched on; whether the list of new orders still holds an order the buyer cancelled within the hour (WBC §7, point 1); which
status a buyer's later cancellation shows (point 3); how soon a feed row follows its order. What they find becomes Plan 4's
premises.

## J. The instructions

- **`README.md` gains «Подключение Wildberries»**, step by step for a seller: what a token is and why wyo needs the Personal one;
  where in the Wildberries cabinet it is made, press by press (premise 3), which categories to tick («Маркетплейс», «Статистика»)
  and that «только чтение» is enough for this release; pasting it into «Маркетплейсы» → «Добавить аккаунт»; what the card then
  shows; that the token lives 180 days and how wyo warns; how to replace it. Then «Заказы» (what each column and state means, how
  often wyo asks, the «Обновить» rule) and «Журнал запросов» (what it shows and what it never holds), and what wyo sends to
  Wildberries in this release: nothing but questions.
- **The same in the interface:** `KeyHelpSteps` on the form; a «?» on every column, state and setting; introductions and empty
  states — by the constitution's help-text rules.
- `docs/wildberries.md`: the methods wyo calls and how often, for a seller's engineer.

## K. Updates with a short stop, and the installer run again (the maintainer, 2026-11-18)

The maintainer, 2026-11-18, starting this plan: «Make sure the installation script not only installs the app, but also checks for
updates and updates it with minimal downtime.» His answers the same day: the work is part of this plan's one commit; «(a) Make sure
it cleans up after the upgrade»; the installer run again goes through the updater; the daily look «between 1 am MSK and 3 am MSK».

- **The copy of the database is made while the wyo server still runs** (SQLite's write-ahead mode lets a second process read a
  consistent picture), and only then: stop, swap the program, start, wait for `/readyz`. The wyo server is down only while the new
  version starts, whatever the database's size. A failed copy stops the update before the wyo server is stopped. The price: a
  put-back restores the copy, so changes made in the second or two between the copy and the stop are lost.
- **Cleaning up after a successful update:** the downloads, a leftover `/usr/local/bin/wyo.new`, the work folders of interrupted
  runs and the request file are deleted; the copies made before updates are trimmed to the newest three at once (the one pruning
  function of `internal/backup`); one `wyo.previous` stays so the earlier version can be put back. A failed clean-up never turns a
  successful update into a failure.
- **The installer run again** on a server whose wyo server runs: it refreshes the units and `wyo.env` and, when the release is newer,
  asks the updater (the request file `install`, then `wyo-update.service`) instead of replacing the program itself — the same copy,
  put-back and short stop. It restarts the wyo server only when the program, `wyo.env` or `wyo.service` changed.
- **The daily look** runs between 01:00 and 03:00 Moscow time whatever the server's own time zone: `OnCalendar=` at 01:00
  `Europe/Moscow` with a random delay of up to two hours (rule R5: installations do not ask in the same second).
- **Tests:** the updater's order of steps (copy before stop; a failed copy never stops the wyo server), the clean-up, the put-back;
  the installer's timer text; the installer job on clean machines: a second run with a newer release goes through the updater.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Every call that writes to Wildberries, «External API» and the exchange** — Plan 4.
- **Ozon, Яндекс Маркет** — Stage 2 of the high-level plan; **Мегамаркет, Авито, Лемана ПРО** — Stage 6.
- **Product cards** (photographs, full names): the list shows the kind, the brand and the size the orders feed gives; reading cards
  is the contract part `products.read` of a later stage.
- **Stock, sales, money, advertising** — Stages 2 and 3. **Prices of orders:** read with the money, Stage 3.
- **Orders the seller delivers to the buyer itself** (Wildberries' other seller-warehouse schemes): they need the buyer's address,
  which wyo does not store.
- **Tokens of the types «Базовый» and «Сервисный»:** refused (section B).
- **Orders older than 90 days and Wildberries' archive:** the feed keeps 90 days; history before the installation is not read.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key, no buyer's address, comment or region in a log, a journal row, an answer
   or an error.
5. Everything is a component and is reused: one adapter contract, one contract test suite, one caller, one budget, one journal, one
   order list; search before writing; a list is `DataList`, a status is a badge with a tone, a secret box follows the secret rules.
   The components section G names are created once. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/wildberries` and the message keys `marketplace.wildberries.*`.
7. **This plan sends Wildberries only reading requests.** No test, no development run and no pipeline run calls the real
   Wildberries; a fact about Wildberries is written only from its official documents, with the address and the date, and a gap is
   marked "not sourced" and goes to section I.
8. Every visible text is a key of `messages/ru.json`, written by the help-text rules; no sentence in Go.
9. Rules R1–R7 of the high-level plan bind every file; the check of R1 before every push. Name the program every time.
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

Six tables, about fourteen calls of the wyo server's own API, three pages, eleven small components, one adapter with five contract
parts, and the engine. The engine's budget and the journal are the parts every later marketplace and Plan 4 stand on, so they get
the most tests.
