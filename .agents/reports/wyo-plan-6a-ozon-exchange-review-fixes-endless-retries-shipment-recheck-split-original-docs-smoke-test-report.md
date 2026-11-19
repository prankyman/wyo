# Smoke test of Plan 6a (stage 7)

Plan: `.agents/plans/wyo-plan-6a-ozon-exchange-review-fixes-endless-retries-shipment-recheck-split-original-docs.md`
Date: 2026-11-19. Branch `fix/ozon-exchange-review-fixes`, uncommitted working tree.

Smoke Test PASSED (stage 7's two claims proved; one optional part, the new problem text «Маркетплейс отменил часть …», not produced).

Servers: started and stopped by this run — the wyo server built with the `wyotest` tag (`scripts/e2e/build.sh`, 12 s, rebuilt first
because `bin/wyo-test` was older than the changes) on 127.0.0.1:8790, and the fakes (the fake Wildberries and the fake Ozon on :8791,
the fake fulfilment centre on :8792). The machine had 7 GB available. One browser, one run at a time. Never a real marketplace.

## What was run

One wyo server with an empty data folder, one fake fulfilment centre key, three Ozon accounts (each on a made-up key of its own, so the
fake Ozon's call list is per account). The exchange was run through the test build's `POST /__test/exchange/run` 12 times, 25 seconds
apart (275 seconds), plus 4 more runs after the retry (about 100 seconds), alongside the wyo server's own jobs.

| Order | Account | Setup | Posting number |
| ----- | ------- | ----- | -------------- |
| A | «Ozon A (ярлык отклонён)» | one line, one parcel; the label request refused once (400, `POSTING_NOT_FOUND`) | `31000001-0001-1` |
| B | «Ozon B (две посылки)» | two lines packed in two parcels; `ship-split` `{"original_split_pending": true}` set before packing | `32000001-0001-1` |
| C | «Ozon C (часть отменена)» | as B, label requests refused 1000 times so the parts stay known and unlabelled; part `33000001-0001-3` cancelled at the fake Ozon after the report | `33000001-0001-1` |

## Claim 1 — a refused label request is not repeated every 20 seconds: PASSED

- `package-label/create` calls the fake Ozon received for account A, right after the first problem: **1**.
- After 12 runs of the exchange over 275 seconds: **1** (the count never changed in any of the 12 log lines).
- The order showed its problem: «Маркетплейс отказал на шаге «получение этикетки». Слова маркетплейса: «POSTING_NOT_FOUND»» (exchange state «проблема»).
- «Журнал запросов» across all three accounts held 5 `package-label/create` rows before and after the 12 runs (A 1 + B 2 + C 2; B and C make one request per parcel). The fake Ozon's own list for account A showed exactly 1 before the retry (the journal filtered to account A was read only after the retry).
- «Передать заново» pressed in the order's details (1280 × 800): **2** calls after (exactly one more, answered 200); the order went to «этикетка передана»; the parcel shows the label sent; the fake fulfilment centre holds `label_attached: true` for it and the problem was removed there. «Журнал запросов» filtered to account A: 2 rows (read through the page's own filter at 1280 and 390, 96 rows of the account in two pages).

## Claim 2 — an order packed in two parcels after «original split» is not cancelled: PASSED

- Order B's details show «Посылка 1 из 2» with part number `32000001-0001-2` and «Посылка 2 из 2» with `32000001-0001-3`, neither equal to the original `32000001-0001-1`; both «Этикетка передана фулфилмент-центру». The fake fulfilment centre holds the order `packed` with `label_attached: [true, true]`.
- B's packing was reported once (1 `ship` call) and 2 label requests were made (one per parcel).
- After 16 runs of the exchange in all (including the reading of states), the fake fulfilment centre's `/__control/calls` held **0** `POST /api/v1/orders/{id}/cancel` calls (it saw only `GET`, `POST /api/v1/orders`, `PUT …/problem`, `PUT …/parcels/N/label`, `DELETE …/problem`). B's exchange state stayed «этикетка передана», never «отменён».

## Observation (not a defect against the plan, for the maintainer to judge)

Order B's own marketplace status column «Состояние» in «Заказы» reads «Отменён», and its details show «Слова маркетплейса: posting_split_pending / cancelled_from_split_pending» (screenshot `…-phone-order-B-two-parcels.png`). That is the original posting's status as the fake now gives it after the split; the exchange column and the fulfilment centre are unaffected, but an owner reading the list sees an order «Отменён» whose labels are attached. The plan says what a real Ozon shows is not sourced.

## Not produced

The new problem text «Маркетплейс отменил часть …» (`problem.pack_part_cancelled`) was not seen. Order C reached the window (parts `33000001-0001-2` and `-3` known, part `-3` cancelled at the fake Ozon), but its label refusal («POSTING_NOT_FOUND») stayed the shown problem through 16 runs (state «упаковано» / «проблема», 0 cancels at the fulfilment centre). C was set up with the label refused so the window exists; without that refusal the labels are attached in the same run as the report and no window remains, so this can only be proved by the Go test named in the plan. This was not tried further.

## Screens, after each of which the four checks were run

Screenshots in this folder, named `<plan-name>-<viewport>-<screen>.png` (`phone` = 390 × 844, `desktop` = 1280 × 800):

- «Заказы» list: `…-phone-orders-list.png`, `…-desktop-orders-list.png`
- order details with the problem (A): `…-phone-order-A-problem.png`, `…-desktop-order-A-problem.png`; after the retry: `…-desktop-order-A-after-retry.png`
- order details with two parcels (B): `…-phone-order-B-two-parcels.png`, `…-desktop-order-B-two-parcels.png`
- order details of C: `…-phone-order-C-part-cancelled.png`, `…-desktop-order-C-part-cancelled.png`
- «Журнал запросов»: `…-phone-journal.png`, `…-desktop-journal.png`, and filtered to account A with the label rows: `…-phone-journal-account-A-label-rows.png`, `…-desktop-journal-account-A-label-rows.png`

Four checks, every screen listed above at both widths:
1. Console errors: none.
2. Raw text keys: none (every visible text is Russian; the marketplace's own words `POSTING_NOT_FOUND` and `posting_split_pending / cancelled_from_split_pending` are quoted from Ozon, not keys).
3. Sideways scroll at 390 × 844: none (document width 390 = window width on all of them).
4. Requests: only the wyo server's own address; no foreign address.

- R3 (no request left for anything but the fakes): passed — the browser asked only the wyo server; the wyo server asked only the fakes.
- R4 (a key shows only its last four characters): passed for what was looked at — the last page's source held neither the first characters of any of the three Ozon keys nor the fulfilment centre key. Not every page's source was searched.
- The sign-in page and «Обзор», «Настройки», «Мой профиль», the showcase page and the unknown-address checks of the skill were not repeated: nothing in this plan changes them.

Method note: the order's details were opened by the address `/orders?order=<id>` because on a phone the list's link text is covered by the stretched card link (an unmodified screen; a click on it timed out as «not visible» — the card itself is the click target). The sign-in went through the page's form.

Waiting for the maintainer: none.

## Defects

None found.

Cleanup: completed. The wyo server, the fakes (found by port, stopped by process id), the browser (no `ms-playwright` browser survived), the temporary data folder and the owner's password file were removed; the script and its logs remain in the session's scratch folder only.
