# Plan 4 — «Внешний API» and the whole Wildberries FBS exchange with a fulfilment centre: report

**DONE.** Released as **0.0.5** on 2026-11-18 (`https://github.com/prankyman/wyo/releases/tag/v0.0.5`). Merged as `74c6053` (pull
request #8, one commit `89f8be7`); the release commit is `5f80439`.

- The plan: `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`.
- Pull request #8 (`https://github.com/prankyman/wyo/pull/8`), one commit `89f8be7`, merged as `74c6053`.

With this release Stage 1 of the high-level plan (`.agents/plans/Marketplace_connector_plan.md`) is complete.

## The gate

| Where                                         | What                                                                                          | Result |
| --------------------------------------------- | --------------------------------------------------------------------------------------------- | ------ |
| Development machine (limited: 4 cores, 15 GB) | Every changed Go package in both builds with the race detector (54 package runs), the whole lint, `pnpm generate:check`, each changed interface test file, the three new browser specs alone (also 3 times each) and the screens table | green; no whole-project type check (the machine's rule) |
| GitHub Actions, run 1 (`8021ffd`)             | Secret scan, commit message, generate, lint, type checks, tests, build, browser tests          | the type checks passed (0 errors, 1 warning); `TestTheRunningServerMakesTheDailyCopy` failed — below |
| GitHub Actions, run 2 (`34b2b88`)             | The same                                                                                       | every test passed, type checks clean; three files of `internal/httpapi` under the per-file coverage floor — below |
| GitHub Actions, run 3 (`89f8be7`), the gate   | The same, 10 min 49 s on `ubuntu-24.04`                                                         | green |
| Gate — interface                              | Unit tests with coverage                                                                       | 1 480 tests in 214 files; 96.00 % of 8 200 statements, every file ≥ the floor |
| Gate — Go                                     | Tests with coverage, both builds                                                               | 91.9 % of 9 414 statements in 156 files, every file ≥ 84 % |
| Gate — browser                                | Browser tests against the built wyo server and both fakes                                     | 96 tests, 48.0 s |
| Installer job, three clean machines           | `ubuntu-24.04`, `ubuntu-22.04`, `ubuntu-24.04-arm`                                             | green in both runs |

**Run 1's failure, a real defect, fixed:** the made-up day of `TestTheRunningServerMakesTheDailyCopy` (`internal/server/app_test.go`)
wakes for every scheduled job. The exchange's jobs woke every 20 seconds (verdicts) and every minute (orders, events) even with
«Внешний API» empty — about 7 500 wake-ups in the day instead of about 100 — so under the pipeline's coverage the day reached only 03:25
before the test's 20-second limit. Locally it passed. The jobs do nothing while the setting is empty, so the waking was waste in the
program: now each exchange job wakes every 10 minutes while «Внешний API» is empty and at its own period while it is filled, and filling
the setting reads and sends the new orders at once (`whileFilled` in `internal/exchange/exchange.go`; test
`TestTheExchangesJobsWakeRarelyWhileTheSettingIsEmpty`, written first and seen failing). The test itself was not changed; it now takes
about 1 second. **Run 2's failure:** the new API handler files `handovers.go` (67 %), `external.go` (79 %) and `exchange.go` (82.5 %) had no
tests for a database that fails, for a hand-over with its batches, orders and problem, for an order's verdicts and history, for «Передать
заново» of a refused sending and for a notification over 256 KB; those tests were added (`internal/httpapi/exchange_test.go`) and the
files reached 84.8–95 %. The one warning of run 1's type check (`EventsModeForm.svelte`, a form made once from the first value, as intended) is
marked the way the project marks it elsewhere.

## Premises (Stage 0), all held

1. Limited machine → focused local runs; the pull request's run is the first whole gate.
2. Wildberries' description files downloaded again on 2026-11-18 (`01-general.yaml`, `03-orders-fbs.yaml`) are byte for byte the
   committed copies; the writing methods, the refusals of closing a supply and the sticker sizes are as the plan read them.
3. The neutral specification still equals the format the maintainer provides (compared outside this repository): every one of the 21
   operations, their parameters and the schemas equal; the one difference is the deliberate one (`delivery`, left out until the stage
   that uses it).
4. No finding of a real account exists yet (register items 5 and 18): the plan's cautious choices stand and are listed in register
   item 23.
5. The sandbox has no shipping-point, shipping-method or transfer method: a test token is not capable of the hand-over's parts.
6. The Standard Webhooks specification read again on 2026-11-18; the receiver is tested with the reference libraries' published vector.
7. A key that may only read is refused for the exchange (the token's own read-only mark, Plan 3).

## What was built

- **«Внешний API»** (`src/routes/(app)/external-api`): an introduction (what a fulfilment centre is, what wyo writes to the marketplace
  by itself, what it never does), and five folded sections — «Подключение» (address and key checked with `GET /api/v1/account` before
  they are saved; a key that may only read refused; «Проверить снова», «Отключить»), «Склады», «Маркетплейсы», «События» (asking every
  minute, or notifications with the address to copy and the signing secret), «Передача заказов маркетплейсу» (the adapter's declared
  hand-over settings).
- **The one fulfilment-centre client** (`internal/fulfilment`): the only package that imports the generated neutral API (architecture
  test); every request through the one caller (`sync.Endpoint`, `Target.Client`), the budget of 300 a minute, journal rows marked
  `fulfilment`; the refusals' table (`VerdictOf`); the signature check. The outbound gate lists the fulfilment centre's address only
  while the setting is filled (`SetFulfilment`).
- **The exchange engine** (`internal/exchange`): orders sent (held ones with their reasons), the order confirmed into the account's open
  supply of its group only after the reservation, its sticker attached at 58 × 40 mm without a scan text, marking codes reported exactly
  as scanned (the earlier ones removed first) and the verdict asked every 20 s for 10 minutes and then every 5 minutes, cancellations
  passed on, events stored and applied once and in order (up to the last number read in order, so asking and notifications lose and
  double nothing), problems in the marketplace's own words reported to the fulfilment centre, «Передать заново».
- **The hand-over** (`internal/handover`): the six steps for each group, each stored before the next; a sticker that changed after the
  move stops it; a shipping point never guessed; a withdrawn request undone before the closing.
- **The Wildberries adapter's writing parts** and their cases in the one contract test suite; the fake Wildberries changes a sticker on
  command and takes a comma list of point kinds as its description file says.
- **«Заказы»**: the column «Обмен», the filter «Требуют внимания», the exchange in the details; **«Передачи»**; the journal's filter
  «Куда»; the card «Внешний API» on «Обзор» and the strip's fourth step; the bell's notices of source `external_api`.
- **Migration 0003** (the exchange's tables) and `internal/store/testdata/0002.db`, the database of 0.0.4, written before the migration
  was added; the upgrade test reads it.
- **Documents:** README section 10 «Работа с фулфилмент-центром» (the sections renumbered from 10 on), `docs/external-api.md`, the
  neutral API's section «Who installs the sender, and who holds the marketplace's key» with the reason in the marketplace's own words,
  `docs/wildberries.md` (the writing methods), `docs/development.md` (driving the exchange through the fakes); constitution 1.3.0,
  principle XII; the register's items 22 to 28.

**Components created** (each once, with its own test file): `ConnectionCheckResult`, `WarehouseMatchTable` (the plan's
`WarehouseMapTable`), `MarketplaceCodeMap`, `EventsModeForm`, `AdapterSettingsForm`, `ExchangeStateBadge`, `HeldReasons`, `ProblemLine`,
`CodeVerdictList`, `OrderExchangeDetails`, `HandoverSteps`, `HandoverStateBadge`, `HandoverDetails`, `ShippingPointPicker`, and the
sections of the page. **Reused:** `SectionForm`, `CollapsibleSection`, `Field`, `TextBox`, `SelectBox`, `CopyButton`, `FactList`,
`DataList`, `FilterSection`, `RowDetailsPanel`, `StepStrip`, `StatusBadgeWithHelp`, `NoteBox`, `EmptyState`, `PageHeading`,
`HeadingWithHelp`, `Stack`, `Button`, `ButtonWithHelp`, `ConfirmationModal`, `InstallationStateCard` (extended), `MarketplaceMark`,
`OrderDetails` (extended, not copied). In Go: one key-tail function for every key (`accounts.LastFour`), one label fingerprint
(`marketplace.Label.Print`), one in-process client for tests (`synckit.HandlerDoer`).

## Tests

- **Go:** the whole walk with both fakes in one process; every branch of the plan's list (held, cancelled before the reservation, each
  answer to a cancellation, a cancellation by the warehouse with Wildberries' cancel never called, every refusal at every step in
  Wildberries' words, every verdict, codes missing or not taken, other identifiers, codes removed after a reopening, a changed sticker,
  a point no longer offered, a request withdrawn before and after the closing, boxes, completion); every write of the walk lost once in
  turn and the walk ending right with no write made twice; the events (order, repeats, signatures, times, a notification before its
  predecessor); rule R3 (nothing sent while empty, «Отключить» stops at once); rule R4 (`TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide`
  in `internal/server` and the exchange's own); the budget of a made-up hour; the store's round trips; the API's calls and the access
  walk with the new `signed` access of `POST /hooks/fulfilment/{text}`; the architecture tests (cancel never used, only
  `internal/fulfilment` imports the neutral client, no marketplace named in the exchange's folders).
- **Interface:** each new component once in every state; the pages for what they add; the policy tests.
- **Browser:** `e2e/specs/external-api.spec.ts` (8), `exchange.spec.ts` (4), `handovers.spec.ts` (3), each page swept at 390 × 844 and
  the wider widths, no request outside the wyo server.

**Defects the tests found, all fixed in this plan:** a lost call at confirming, the label or the codes left the order stuck (the event
is now applied again by the next run); a re-applied packing event doubled the history; «Передать заново» after a refused removal of
codes sent the void codes again (a codes state `removing`); a stale problem stayed on an order the warehouse cancelled; a 503 of the
fulfilment centre after the caller's repeats was read as «Нет связи», not «Система недоступна»; the «Подключение» form refused the test
build's plain `http://` address; the exchange's jobs woke every 20 seconds with the setting empty (run 1, above).

**Test expectations changed, with the reason:** the exact answers of `GET /api/v1/marketplaces` and `GET /api/v1/orders/{id}` gained the
new fields; the list of tables of a new database gained the new tables; the made-up days count the exchange's jobs; a Wildberries route
test now requires `Write` to equal the description file's own `x-readonly-method` mark (stronger); `AppNav`'s «Ещё» case uses a menu of
five items, and a new case checks where the two new items sit; the key tail of «Внешний API» is the last four characters like every
other key (the tests had asserted an ellipsis before it, a choice of mine that the browser tests showed differed from the accounts').

## Smoke test (locally, the built wyo server in test mode with both fakes, one browser)

«Внешний API» empty (the fake fulfilment centre received nothing), filled through the page, the five sections; a marked order and a held
order sent; the marked one reserved, confirmed, its label attached, packed with a code, the code accepted — seen in «Заказы», under
«Требуют внимания» and in the details; the hand-over asked, «Передачи» waiting for the point, the point picked, all six steps done and the
documents at the fake fulfilment centre; the journal filtered by «Фулфилмент-центр»; «Обзор»; the bell; «Отключить», after which the fake
fulfilment centre received nothing more. Every screen at 1280 × 800 and 390 × 844 with screenshots; after each: no console error, no raw
text key, no sideways scroll, no request outside the wyo server, no key and no marking code in the page. No finding.

## Rule R1 and the secret scan

The search of rule R1 ran over every tracked and staged file and over the commit message before each push: nothing found. The secret
scan flagged seven made-up test values (and one of them again as decoded from base64); each is an exception by its exact value in
`.gitleaks.toml`, with its reason.

## Where the plan was not followed to the letter

- **A tenth table** `exchange_history` (an order's exchange history) and `handover_orders` besides the plan's nine; the plan's
  `order_codes` keeps each unit's code.
- **The hook's access:** `POST /hooks/fulfilment/{text}` is a row of the access table with a third access, `signed`, so the walk of every
  path checks it.
- **A test-build-only address** `POST /__test/exchange/run` runs the exchange's jobs at once for the browser tests; a released wyo server
  has no such address (test `TestTheExchangeRunsOnRequestOnlyInATestBuild`).
- **The jobs wake every 10 minutes while «Внешний API» is empty** (run 1); filling it starts at once.
- **Words on the screens:** the interface calls a marketplace's group of orders «партия» (Wildberries' «поставка» is named only in its
  own texts and the README).
- **Tests first:** the engine, the fulfilment client and the store were written before most of their tests (the whole-walk test came
  right after the engine); the branch, interruption and refusal tests then found the five defects above, each seen failing before its fix.
  The interface and the Wildberries parts were written test-first.
- **Not proven through the fake's own events:** transport boxes (the fake fulfilment centre names none) and some Wildberries refusals
  (the fake cannot refuse them on command) are proven through the hand-over's runner and a scripted refusal (register item 22).
- **The budget test** runs 500 orders for the six busiest minutes and 50 orders for a whole hour (register item 28 says why).

## Not done — waits for the maintainer or a real account

The plan's «After the release» checks (register item 23): a printed sticker, a scanner's text, a sticker after a move, a second sending
of codes, the verdict's time, boxes, shipping points and closing on a real account, a real notification address. The confirmation
sentence of a picked shipping point is never seen, because the picker leaves as the strip moves on (the strip is the feedback).

## Release

**0.0.5**, released 2026-11-18 (`https://github.com/prankyman/wyo/releases/tag/v0.0.5`, release commit `5f80439`). The merge's run
(44307525116) skipped the gate and the installer job (the merge commit holds byte for byte the files of the pull request's last passed
run); the release job and the released installer passed. Checked by hand after download: the signature of `checksums.txt` is accepted by
the committed public key (`internal/release/keys/2026-1.pem`), every file matches its checksum, and `wyo version` prints 0.0.5.

An installed wyo server updates itself to 0.0.5 at its next daily look (01:00–03:00 Moscow time) or by «Обновить сейчас»; migration
0003 is applied where the database lies. Nothing changes for a seller until «Внешний API» is filled.
