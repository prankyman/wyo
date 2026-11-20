# Plan 8 report — Яндекс Маркет's FBS and Express exchange through the fulfilment centre: boxes and codes, labels in the fulfilment centre's size, «Готов к отгрузке», the daily shipment, the courier's code

Plan: `.agents/plans/wyo-yandex-market-fbs-and-express-exchange-with-a-fulfilment-centre-codes-labels-daily-shipment-courier-code.md`.
Branch `feat/yandex-market-exchange`, one commit, one pull request.

## Status

Built and tested locally (limited machine: 4 cores, 15 GB): every Go package the plan changed (with and without the `wyotest` tag),
the interface's changed test files and the policy tests, the browser specs `exchange-yandex` (new) and `yandex`, and the smoke test
(`wyo-yandex-market-fbs-and-express-exchange-with-a-fulfilment-centre-codes-labels-daily-shipment-courier-code-smoke-test-report.md`).
The whole gate runs on the Mac mini (`pnpm remote:gate`).

## What was built

- **Stage 0.** The neutral specification (`api/fulfilment-centre.yaml`, `docs/fulfilment-centre-api.md`) gained the three additions
  exactly as the fulfilment software's released description (2026-11-20) has them: `label_sizes` of each warehouse, `test` and
  `handover` (`shipment` | `courier_code`) on an order, the event `order.courier_code_entered` (`details.check_id`, `details.code`)
  and `PUT /orders/{id}/courier-code-checks/{check_id}`. Яндекс Маркет's tree was compared with its newest commit `b71cd43`
  (2026-11-20): one order substatus removed, no method of the plan changed, the commit kept. The generator reads the nine writing
  methods. `testdata/0007.db` (release 0.0.12, now with an exchange order and a parcel) was written before the migration.
- **Migration `0008_yandex_exchange_label_sizes_courier_codes`:** `fc_label_sizes`, `fc_connection.label_sizes_read_at`,
  `exchange_orders` made again for the state `ready` with `ready_at` and `handover`, `exchange_parcels.box`, `marketplace_supplies.kind`,
  `exchange_courier_checks` (never the code). Upgraded from `0007.db`.
- **The contract (`internal/marketplace/writing_boxes.go`):** `orders.ready`, `orders.courier_code`; `CodesInPack` with `BoxPacker`
  (lines named by their place in the order), `BoxLabeler`, `SizedLabelFetcher`, `LabelSize` and the one refusal
  `label_size_unsupported`, `OrderReadier`, `CourierCodeChecker`, `BatchFound`, `CloseWait`, `CourierSchemes`, `WaitStatuses`.
- **The engine:** orders sent with the test mark and the way out; `UNPAID`/`PENDING` never sent; the label sizes read when the
  setting is filled, hourly and before a label; the boxes with their codes reported once, the verdict, each box's label in the
  fulfilment centre's size, «Готов к отгрузке» read first and written once; the courier's code checked at once, the answer kept
  before it is sent and the code blanked in the stored event; the day's shipment found, compared both ways (test orders left out),
  confirmed after the cutoff, its act sent. Wildberries' sticker in the fulfilment centre's size.
- **The adapter and the fakes** (helper agents): the nine writing methods of Яндекс Маркет with their limits, the one status
  function and its architecture test, the fake Яндекс Маркет's writing routes, shipments and code checks, the fake fulfilment centre's
  label sizes, courier-code orders and test orders, the contract suite's cases.
- **The wyo interface** (helper agent): the state «готов к отгрузке», boxes, the courier's checks, the shipment's kind, the label sizes
  on «Внешний API», the new notice and texts.
- **Documents:** README sections 1, 9, 12; `docs/external-api.md`, `docs/yandex.md`, `docs/fulfilment-centre-api.md`,
  `docs/development.md`, `api/yandex/README.md`; constitution 1.9.0; the register (items 42, 43; 24 and 25 widened).

## Components

Created: `CourierCodeChecks` (interface); in Go the box flow's files `internal/exchange/boxes.go`, `ready.go`, `courier.go`,
`labelsizes.go`, `internal/handover/found.go`, `internal/store/exchange_plan8.go`. Reused: `ParcelList`, `OrderLines`,
`TestOrderBadge`, `HandoverSteps`, `ExchangeStateBadge`, `ProblemLine`, `WarehouseMatchTable`, `OrderExchangeDetails`,
`HandoverDetails`, `StatusBadgeWithHelp`, `NoteBox`, `Stack`; the one caller, budget, journal, contract suite, the one comparison of a
batch (`marketplace.BatchDiffers`), the one schedule of the verdict, the one escaping of a marking code; the exchange rig's helpers
(moved from the Ozon rig to the shared rig) and the made-up hour's runner (`runJobsOn`), shared by the Ozon and Яндекс Маркет tests.

## Found while implementing

1. **A Яндекс Маркет order in `UNPAID` or `PENDING` would have been sent:** Plan 7 maps both to «Новый». Fixed: an adapter declares
   its waiting status words (`WaitStatuses`) and the store leaves those orders unsent. Test:
   `TestAnOrderWhoseMarketplaceStatusWaitsIsNotSentAndItsTestMarkAndSchemeNameGoWithTheOthers`.
2. **An order whose warehouse Яндекс Маркет did not tell was not under «Требуют внимания»** (plan, section B). Fixed: the reason and
   attention `warehouse_unknown`, the filter, the count and the bell's notice. Test:
   `TestANewOrderWhoseWarehouseTheMarketplaceDidNotTellNeedsTheOwnerWhileTheExchangeIsOn`.
3. **A cancellation after the box report was never passed on:** the skip that leaves an Ozon order to its packing's own reading also
   held the box flow, which has no such reading. Fixed in `internal/exchange/cancel.go`. Test:
   `TestAYandexMarketCancellationIsPassedOnBeforeAndAfterTheOrderIsReady`.
4. **«Run every job at once» did not read Яндекс Маркет's new orders** (they come by changes): neither on filling «Внешний API» nor
   by the test build's control address. Fixed: `sync.Engine.RunChanges`, called by the exchange's immediate reading. Test:
   `TestReadingTheNewOrdersAtOnceReadsYandexMarketsChangedOrdersToo`.
5. **A box report saved its history before its outcome**, so a lost report noted «packing finished» twice. Fixed: nothing is saved
   before the outcome (a report replaces the earlier). Test: `TestAYandexMarketWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice`.
6. **The adapter searched the day's shipment by the hand-over's orders**, so a shipment lacking them was never found and its
   difference never named. Fixed: found by day and warehouse. Test: `TestADailyShipmentThatDiffersFromTheHandOverIsNeverConfirmed`.
7. **sqlc drops a list parameter compared with a joined text and stops on angle brackets in a query's comment.** Worked around in
   `OrdersToSend` (a JSON list); no rule of the repository depended on it.
8. **The `conflict` code's description in the neutral specification did not cover the courier's answer.** Fixed in
   `api/fulfilment-centre.yaml`.
9. **A copy of the fake cabinet** in `e2e/specs/yandex.spec.ts` beside the new shared `FAKE_YANDEX_CABINET`. Fixed: one constant,
   both specs use it.
10. **`TestServeAnswersAndStopsWhenTheContextEnds` (`internal/server`) opened the whole wyo server inside the goroutine that
    serves, while its request waited at most 10 seconds:** on the Mac's whole run with the race detector it timed the opening
    (over 10 seconds, the stack showing `server.Init` still running after the test ended) and then failed from that goroutine. The
    test was wrong about what it times; the opening now comes before the serving, and its assertions are the same.
11. **Not sourced, cautious choices, register item 42:** the date form of `searchShipments`, whether `getShipment` lists orders being
    taken out, the shipment methods' real refusal words, `NOT_ON_VALIDATION`, a test order in a day's shipment, an Express order
    without `eacType` (`docs/yandex.md`, points 12 to 16).

## Rule R1

The check before the push is recorded in the session's final report.
