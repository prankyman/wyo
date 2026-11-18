# Plan 5 — Ozon accounts and orders of several products, read from Ozon: report

**DONE.** Released as **0.0.6** on 2026-11-18 (`https://github.com/prankyman/wyo/releases/tag/v0.0.6`). Merged as `7e5ebcb` (pull
request #9, one commit `2975ccb`); the release commit is `663e6ea`.

- The plan: `.agents/plans/wyo-ozon-key-reading-adapter-orders-with-several-products-fake-ozon.md`.
- Pull request #9 (`https://github.com/prankyman/wyo/pull/9`), one commit `2975ccb`, merged as `7e5ebcb`.
- The check of rule R1 (every word of the list the maintainer keeps outside the repository, over every tracked and new file and the
  commit message) ran before every push and found nothing.

## The maintainer's decisions during the work

1. **Ozon's description file is refused by the generator** (oapi-codegen v2.8.2: `info.version` is the number `2.1`; 6 fields carry
   `required: true`; 14 field types are not OpenAPI's). The maintainer, 2026-11-18: «Yes, fix them in a temporary copy, use the generator.
   Fix whatever must be fixed.» `scripts/generate.sh` now runs `tools/specfix` (rules in `internal/specfix`), which writes a corrected
   copy into a temporary folder; the generator reads it; `api/ozon/seller-api.json` is committed unchanged. The 21 corrected places
   are listed in `api/ozon/README.md` and pinned by `TestOzonsFileGetsExactlyTheCorrectionsItsReadmeLists`.
2. **How often Ozon is read while «Внешний API» is filled.** The maintainer: «The wyo must check the orders as often as the external
   system says so.» Every account, Ozon's included, is read at the exchange's periods while the setting is filled (new orders every
   minute, states every 5 minutes); the plan's «30 minutes for every marketplace» (section C) was dropped. Plan 6 should take this as
   its premise.
3. **The form machinery's refusal texts.** A helper was refused permission to edit `src/lib/api/refusal-key.ts`,
   `src/lib/forms/super-form.ts`, `src/lib/forms/api-action.svelte.ts`; the maintainer answered (a) and the main agent made the change
   test-first: a form's family of refusal texts may be a lookup, so a marketplace's own sentence is found before the shared one.

## The gate

| Where                                         | What                                                                                              | Result |
| --------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------ |
| Development machine (limited: 4 cores, 15 GB) | Every Go package in both builds, the whole Go lint, ESLint over the tree, `generate:check`, every interface unit test, five browser specs, the smoke test | green; no whole-project type check (the machine's rule) |
| GitHub Actions, run 1 (`1c43ef4`)             | Secret scan, commit message, generate, lint, type checks, tests, build, browser tests              | svelte-check: one error — below |
| GitHub Actions, run 2 (`210e0f1`)             | The same                                                                                          | type checks clean, every test passed; one file under the per-file coverage floor — below |
| GitHub Actions, run 3 (`2975ccb`), the gate   | The same, 17 min 45 s on `ubuntu-24.04`                                                            | green |
| Gate — interface                              | Unit tests with coverage                                                                          | 1 533 tests in 217 files; 96.05 % of 8 333 statements, every file ≥ the floor |
| Gate — Go                                     | Tests with coverage, both builds                                                                  | 92.3 % of 10 557 statements in 178 files, every file ≥ 84 % |
| Gate — browser                                | Browser tests against the built wyo server and the fakes                                          | 105 tests, 51.2 s |
| Installer job, three clean machines           | `ubuntu-24.04`, `ubuntu-22.04`, `ubuntu-24.04-arm`                                                 | green in every run |
| Release run (merge `7e5ebcb`)                 | Gate, installer, release, released installer                                                      | green; 0.0.6 published |

**Run 1's failure:** `src/lib/marketplaces/exchange-support.test.ts` used the type `TMarketplace` without importing it (a type check
this machine may not run). The import was added. **Run 2's failure:** `internal/store/order_lines.go` was at 77.3 % (17 of 22
statements): a function nobody calls (`OrderLines`) was removed, and the refusal of lines for an order that does not exist got its test
(`TestTheLinesOfAnOrderThatDoesNotExistAreRefused`); the file reached about 90 %.

## What was built

- **Stage 0.** Ozon's file moved unchanged to `api/ozon/seller-api.json` (SHA-256 `57f27f1a…`); today's GitHub copy (commit
  `e22c5e9e`) compared as JSON: the same 9 example values differ, no method of the plan. The database of release 0.0.5 written to
  `internal/store/testdata/0003.db` before migration `0004`.
- **Migration `0004_order_lines_ozon`:** `order_lines` (every existing order became one line of quantity 1 with its own product, proven
  from `0003.db`), `orders.ship_by`, `orders.parent_number`, `marketplace_warehouses.seller_delivers`, `marketplace_accounts.key_fields`.
- **Shared parts made neutral:** a key of several fields packed by one function (`marketplace.PackKey`/`UnpackKey`; a key of one field
  is kept as its value, so Wildberries' stored keys and tests are unchanged); `Authorize` gets the fields; the last four characters come
  from the secret field; each adapter names its limit headers and its hosts, and the outbound gate lists the registered adapters' hosts;
  one lock per account (proven failing against a one-account-at-a-time engine); a reading writes an order only when its state, deadline
  or lines changed (register item 28, closed); shared helpers in `internal/adapterkit`; architecture tables of every adapter; Ozon's
  two cancel methods are never generated (a test reads `api/codegen/ozon.cfg.yaml`); an ended key given by the marketplace's answer is
  refused when added.
- **The Ozon adapter** (`internal/marketplace/ozon`): `/v1/roles` (end date, methods: `Admin`, an exact method, or a beginning of a
  path at a `/`), the seller's name, warehouses (rFBS marked and never read), new FBS postings with lines, states and deadlines one by
  one, FBO postings of 30 days, barcodes once per product; Ozon's refusals in one table-tested function; buyer data dropped. It passes
  `RunAdapterContract` against the fake Ozon and sends Ozon only reading requests.
- **The fake Ozon** (`internal/fakes/ozon`), both fake marketplaces on one address (`--marketplaces`, one router `fakekit.HostRouter`),
  the kit's `refuse` fault (register item 22, first half), the kit's shared bucket and control-body helpers.
- **Rules R3, R4, R5 for Ozon:** the made-up day with a Wildberries and an Ozon account (Ozon's host only with its account), the Ozon
  key canary (the key only in `Api-Key`; the made-up buyer in no log, journal row, answer or database file), the header test and the
  hour of 500 open postings at Ozon's 50 a second.
- **The wyo interface:** `ProductCell`, `OrderLines`; the order details with the split's parent and parts; the key form's ordinary box
  for a field that is not secret; the card's Client ID in full, notes of missing access levels, the rFBS count, no «Права ключа» and no
  deadline hours for Ozon; the mark `azure`; «не передаётся» for Ozon orders in «Заказы» and in «Внешний API»; Wildberries' own refusal
  sentences moved to `marketplace.wildberries.*`; Ozon's texts and key steps.
- **Documents:** README «Подключение Ozon» (sections renumbered), «Заказы», the exchange's section, the measurement; `docs/ozon.md`;
  `docs/development.md`; constitution 1.4.0; register items 29–33 added, 19, 20, 22 widened, 28 closed.
- **Measured** (`pnpm go:measure`, 2026-11-18, the development machine): two Wildberries accounts and one Ozon account whose orders hold
  three products on average, 10 090 orders: peak 66.3 MB for the whole test process.
- **Test-build address** `POST /__test/orders/run` reads the orders at once (without «Обновить»'s 5 minutes), for browser tests.

## Smoke test

The built wyo server in test mode with the fakes on one address, one browser: an Ozon and a Wildberries account added through the form
(the Client ID in full, both keys by their last four characters, the rFBS note); an Ozon posting of three products and a Wildberries
order read with «Обновить»; the details' three lines; a split made in the fake and read again (the parts with their parent, the original
cancelled naming its parts); a part cancelled by the seller; a search by the second line's article; the journal; «Маркетплейсы»,
«Заказы», «Журнал запросов», «Внешний API» at 390 × 844 with no sideways scroll. 35 checks passed, no console error, no request but the
wyo server's own; screenshots read. The phone's menu strip still cuts long labels (register item 21).

## Changes of existing tests (shape only, the same expectations)

The contract's new types made these mechanical changes: an order's product fields are read from its one line in the tests of the
Wildberries adapter, the engine, the store and the API handlers; `Authorize` and `ReadKey` take and give `marketplace.Key`; the
Wildberries refusal sentences are expected under their new keys in `e2e/specs/marketplaces.spec.ts`, which now chooses Wildberries in
the form (the form asks for the marketplace once two exist); the API's exact answers gained the new required fields. The contract suite
now requires every new order to name its products, and allows key-field names of lower-case words joined by underscores (`client_id`).
One unit test of «Заказы» asserted that an Ozon order carrying an exchange state shows that state; the browser test against the real
wyo server showed the wyo server always sends one, and the plan says the column reads «не передаётся» — the test was corrected and the
page fixed. Not every new test was seen failing first: the specfix command, the Ozon adapter's check, refusal and order tests and the
outbound release test were written beside their code; the engine's lock test was proven by running it against a one-at-a-time engine.

## Not done, or left open (each in the register)

- The sentence in an FBO order's details that its state is no longer read after 30 days (item 31).
- Ozon's checks on a real account (item 30), notifications (32), stock (29), the fulfilment client's `Retry-After` (33).
- `Admin read only` is accepted only when its method list names the methods; the README says it is not yet tried on a real key.

## Cleanup

The plan's branch is deleted here and on GitHub. The smoke test's wyo server and fakes, started by this session, were stopped, and their
data folder and password file deleted. The untracked `docs/qr-code.png` is not this plan's and was left untouched.
