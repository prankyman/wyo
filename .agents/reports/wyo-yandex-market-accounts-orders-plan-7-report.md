# Plan 7 report — Яндекс Маркет: the key, reading orders, notifications with a safety read, test orders, the fake Яндекс Маркет

Plan: `.agents/plans/wyo-yandex-market-key-reading-orders-notifications-safety-read-test-orders-fake-yandex-market.md`. Branch
`feat/yandex-market-accounts-orders`, one commit, one pull request.

## Status

Local focused runs passed (limited machine: 4 cores, 15 GB; no whole type check locally): the Go packages this plan changed, the
interface's changed test files, the browser specs `yandex`, `orders`, `marketplaces`, `overview`, `ozon`, `bell`, `screens-table`, and
the smoke test on the built wyo server with the fakes. The whole gate is the pull request's run in GitHub Actions.

## What was built

- **Stage 0.** `api/yandex/` holds Яндекс Маркет's description tree at commit `9be9f98e` (1 087 files and its BSD-3-Clause
  licence); compared on 2026-11-19 with the newest commit `4ca8b761`, which only adds five size-table methods. The rule «join» of
  `internal/specfix` (and `tools/specfix join`) joins the tree into one temporary file; oapi-codegen v2.8.2 generates
  `internal/marketplace/yandex/yandexapi` from it (`api/codegen/yandex.cfg.yaml`, five reading methods). `testdata/0006.db`, the
  database of release 0.0.10, was written before the migration.
- **Migration `0007_yandex_shops_notifications_test_orders`.** `orders.test`, `orders.scheme_name`, `orders.warehouse_text`;
  `marketplace_shops`; `marketplace_accounts.hook_text`, `notifications_seen_at`, `notifications_lost_at`;
  `marketplace_warehouses.express`; `marketplace_notifications`. Upgraded from `0006.db` with every order a real one.
- **Shared parts (section A).** A limit group's own limit status (Яндекс Маркет `420`) and wait as a date, read by one function
  (`waitOf`) for every marketplace and the fulfilment centre; a server error that costs nothing; at most N requests of an account
  in flight (one limiter in the caller); a marketplace's own program header (`X-Market-Integration: wyo/<version>`); the contract
  parts `orders.changes`, `orders.named`, `notifications.receive`; shops; the test mark; the account's profile handed to the
  adapter (`marketplace.Conn.Profile`). Register item 33: the fulfilment centre's group names `Retry-After`.
- **The adapter** `internal/marketplace/yandex`: key check by levels (orders required; products and changing orders noted),
  cabinet and shops, warehouses (DBS only = «доставляет продавец», Express), one refusals table, orders changed since a moment and
  named orders (real, then test ones), the states table, the warehouse by shop and scheme (with Яндекс Маркет's own text kept when
  several match), barcodes once per article, notifications read and answered in Яндекс Маркет's form.
- **The fake Яндекс Маркет** `internal/fakes/yandex` (helper agent), served by `cmd/wyofakes` on the marketplaces' address.
- **Notifications and the safety read.** `internal/notify`: the one store-then-kick (the fulfilment centre's hook uses it too) and
  the receiver of `POST /hooks/marketplace/{text}` (and `…/notification`): adapter's networks only (a test build also this
  machine), 256 KB, stored before the answer, `400` in the marketplace's form for an unreadable body, `404` for anything else. The
  engine reads notified orders a moment after they arrive; the safety read comes every 15 minutes while notifications arrive,
  otherwise 5 (with «Внешний API») or 30; an unnotified change older than 10 minutes makes them «не доходят» (bell line
  `account_notifications_lost`).
- **Test orders** are marked everywhere and counted in no total (`TestATestOrderIsCountedInNoTotal`).
- **The interface** (helper agent): `TestOrderBadge`, `NotificationAddressCard`, `ShopList`, `WarehouseCell`, the yellow mark,
  `SchemeBadge` Express/FBY, the test filter, the home page's note, every text of `marketplace.yandex.*`.
- **Documents** (helper agent): README section «Подключение Яндекс Маркета», `docs/yandex.md`, `docs/development.md`;
  constitution 1.7.0; the research file's section 15 (the notification's body); the register.
- **Measured** (`pnpm go:measure`, 2026-11-19, four accounts, 10 120 orders): peak 66 212 KB, at the end 41 192 KB, Go heap
  6 928 KB.

## Components

Created: `TestOrderBadge`, `NotificationAddressCard`, `ShopList`, `WarehouseCell`; in Go `internal/notify` (Kicker,
StoreThenKick, Marketplace), `memadapter.Changing`/`Notifying`, `fakekit.Paged`. Reused: `KeyForm`, `KeyHelpSteps`, `AccountCard`,
`MarketplaceMark`, `CopyButton`, `OrderDetails`, `SchemeBadge`, `FilterSection`, `DataList`, `NoteBox`, `StatusBadgeWithHelp`,
`Stack`, the one caller, budget, journal, contract suite.

## Found while implementing

1. **`orders.scheme` holds only `fbs` and `fbo` by a CHECK of migration 0002**, which SQLite cannot change without rebuilding the
   table (and its children's cascade). Fixed: the scheme stays who packs (`fbs`/`fbo`), and the marketplace's own name is a new
   column `orders.scheme_name` (`express`, `fby`); the API shows the name. Test:
   `TestATestOrderIsKeptMarkedFilteredAndLeftOutOfTheCountsAndASchemeIsFoundByItsOwnName`.
2. **Reading an account's cabinet asked Яндекс Маркет for the shops on every reading** (1 000 an hour). Fixed: the engine hands
   the adapter the account's profile (`marketplace.Conn.Profile`).
3. **The fake's made-up postcode `000000` was no canary** (it is inside the order number `1000000001`). Fixed: distinctive values.
4. **The two header tests of rule R5 were near copies.** Fixed: one table over every adapter
   (`TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader`).
5. **The two generator-list tests of Ozon were written inline**; Яндекс Маркет needed the same. Fixed: one `generator` helper in
   `internal/archtest` used by both.
6. **The plan keeps an order's own warehouse text when the warehouse cannot be told**; the first cut did not. Fixed:
   `orders.warehouse_text`, `Order.warehouseText`, shown beside «Склад не определён».
7. **Whether Яндекс Маркет adds `/notification` to the address is not sourced.** Both addresses are answered; register item 40.
8. **The route walk of `internal/httpapi` under the race detector takes longer than 2 minutes on a 4-core machine** (18 more
   routes); it is slow, not stuck. Run whole on the gate.
9. **A browser case of «Уведомления не доходят»** needs a change older than 10 minutes; it is proven by the Go tests
   (`TestAChangeThatNoNotificationNamedFoundByTheSafetyReadStopsTheNotificationsUntilTheNextOne`), not in a browser.

10. **The wyo server crashed on the first notified order** (found by the browser test): a released wyo server gives no waiter
    of its own (nil), and the new listener of notifications called it. Fixed: no waiter is a real timer, as for the caller and
    the scheduler (`internal/notify`, `TestAListenerGivenNoWaiterGathersOnARealTimer`); the browser test
    `e2e/specs/yandex.spec.ts` now passes.

11. **The card of a Яндекс Маркет account said nothing of its DBS warehouses** (found by the smoke test): the note of warehouses
    the seller delivers from had texts only in Ozon's family. Fixed: `marketplace.yandex.note.sellerDelivers.*`; the browser test
    checks the note.

12. **The marketplaces' notifications were never deleted** (found while writing the documents): the 30-day deletion had no caller.
    Fixed: the journal's nightly job deletes them with its own rows
    (`TestTheNightlyJobDeletesTheMarketplacesNotificationsOlderThanThirtyDays`, `internal/journal`).
13. **The journal showed the raw words of the two new readings.** Fixed: «Изменённые заказы», «Заказы из уведомлений».
14. **Left as the plan's table says, documented:** an Express order in a courier substatus (`COURIER_*`, not sourced) is
    «Неизвестное состояние»; a key with only the level `finance-and-accounting` (which the description lets read orders) is refused,
    as the plan lists the levels; whether a shop with its API off stops the cabinet's whole list is checked on the real cabinet
    (register item 40).

15. **The pull request's first run failed on time, not on behaviour** (run 44539962471): `internal/exchange` and
    `internal/httpapi` passed the 10-minute limit of `go test` (they had taken 393 s and 398 s on the last green run), a Wildberries
    sticker test saw two requests of 100 stickers each run past the caller's real 30-second deadline and repeated (99 s), and
    `synckit`'s count of the engine's jobs found the new safety-read job. Measured on this machine: migration 0007 adds about
    0.16 s to every fresh test database under the race detector (1.03 s → 1.18 s), so tests that open several databases are about
    10–15 % slower; the runner itself ran `internal/store` 96 % longer than on the last green run against 23 % here. Fixed: the
    job count (the new job is by design); the caller asks an adapter's description once per connection instead of at every request.
    Not changed: the tests' limits and deadlines.

## Rule R1

The check of rule R1 ran on 2026-11-19 before the push, over every tracked file of the branch and the commit message, with the
maintainer's list kept outside this repository: it found nothing.
