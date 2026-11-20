# «Внешний API»: the exchange with a fulfilment centre, step by step

This page is for a seller's engineer who wants to know exactly what the wyo server does once «Внешний API» is filled, and for a
fulfilment centre's developer whose system will be called. The seller's own instructions are the README, section 12
(«Работа с фулфилмент-центром»). The format on the fulfilment centre's side is `docs/fulfilment-centre-api.md`; the plans that built
this are `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md` (Plan 4,
the walk of a marketplace that confirms orders, Wildberries: steps 1 to 5 below) and
`.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md` (Plan 6, the walk of a marketplace whose
label exists only after the packing is reported, Ozon: «The Ozon walk» below) and
`.agents/plans/wyo-yandex-market-fbs-and-express-exchange-with-a-fulfilment-centre-codes-labels-daily-shipment-courier-code.md` (Plan 8,
the walk of a marketplace that takes the boxes and the marking codes in one request, makes its own daily shipment and may send an order
alone with a courier's code, Яндекс Маркет: «The Яндекс Маркет walk» below). Every fact about Ozon cited below is from the research
`.agents/research/ozon-official-seller-api-check-2026-11-18.md` (written «OZC §n»); a gap in Ozon's documents is marked «not sourced».
Every fact about Яндекс Маркет is from `.agents/research/yandex-market-official-partner-api-check-2026-11-19.md` (written «YMC §n»), with
the same rule. What each Ozon request does on its own side, with its periods, is `docs/ozon.md`, section «Запросы обмена»; what each
Яндекс Маркет request does on its own side, with its periods, is `docs/yandex.md`, section «Запросы записи».

## Where it lives

| Part                                         | What it does                                                                                                           |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `internal/fulfilment`                        | The only package that knows the neutral API: the client, the refusals' table, the signature check.                     |
| `internal/exchange`                          | The exchange engine: sending, events, the label, the codes, the packing, cancellations, the notices and the views.     |
| `internal/handover`                          | The six steps of a hand-over, of which an adapter declares the ones it needs.                                          |
| `internal/marketplace/writing.go`            | The writing parts of the adapter contract; `internal/marketplace/wildberries`, `…/ozon` and `…/yandex` implement them. |
| `internal/outbound`, purpose fulfilment      | Lists the fulfilment centre's address only while «Внешний API» is filled.                                              |
| `src/routes/(app)/external-api`, `handovers` | The pages «Внешний API» and «Передачи».                                                                                |

Every request to the fulfilment centre goes through the same one caller as a request to a marketplace (`sync.Caller`): it is kept to
the specification's budget (300 a minute), names the program (`wyo/<version> (+https://github.com/prankyman/wyo)`), carries the key only
in `Authorization: Bearer <key>`, is never repeated blindly when it writes, and leaves one row of «Журнал запросов» marked
`fulfilment` (never a key, a query or a body).

## When nothing is sent

With «Внешний API» empty every job of the exchange returns at once and the outbound gate refuses any call to a fulfilment centre
before it leaves (rule R3). The jobs run while the setting is filled and its state is «Работает», «Нет связи», «Система недоступна» or
«Ответ не понят» (the states a later call mends); a refused key, a key that may only read, the API or the work switched off wait for
the owner's «Проверить снова».

## The jobs

| Job                 | Every                                                                                                                          | What it does                                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `exchange.orders`   | 1 minute (a marketplace is read at its adapter's own period: Wildberries every minute, Ozon and Яндекс Маркет every 5 minutes) | Reads the marketplaces' new orders, sends the unsent ones, goes on with interrupted hand-overs.               |
| `exchange.states`   | 5 minutes (as above)                                                                                                           | Reads the open orders' states, passes cancellations on.                                                       |
| `exchange.events`   | 1 minute (10 minutes beside notifications)                                                                                     | Reads `GET /api/v1/events?after=<mark>`, stores each event, applies them in order.                            |
| `exchange.verdicts` | 20 seconds                                                                                                                     | Asks what is due: the marking codes' verdicts, the packing that was reported, the labels that are made later. |
| `exchange.prune`    | nightly, 04:00–05:00 of the installation's zone                                                                                | Deletes marking codes 90 days after their order's hand-over and applied events after 30 days.                 |

Each moment is shifted by a few random seconds (rule R5).

## Step 1 — a new order

For every order of the seller's own warehouse (FBS) that is new at the marketplace, of an account whose key may write, whose
marketplace code and warehouse are matched, and which was never sent:

```
POST /api/v1/orders
{ "external_id": "<the marketplace's order number>", "marketplace": "<the matched code>", "group_id": "<the basket>",
  "warehouse": "<the matched warehouse>", "ship_by": "<created + the account's hours, 48 by default>",
  "lines": [{ "barcode": "…", "article": "…", "marketplace_item_id": "<the marketplace's size number>", "quantity": 1 }] }
```

The body also carries `"test": true` for a marketplace's test order and `"handover": "courier_code"` for an order that leaves alone with a
courier's code (both below); each is sent only when it applies, and left out they mean `false` and `shipment`.

`201` (or `200 duplicate`) → «передан»; `202 held` → «ждёт у фулфилмент-центра» with the reasons; a refusal (for example
`403 marketplace_not_enabled`, `409 external_id_reused`) → «не передаётся» with a problem and «Передать заново». An answer lost on the way
is harmless: the same order sent again is one order there.

## Events

Events are stored first (`fc_events`), a repeat is ignored, and they are applied in the order of their numbers, only up to the last
number read in order with `GET /events` — so a notification that arrives early waits for the events before it, and asking and
notifications together lose and double nothing. A notification is `POST /hooks/fulfilment/<random text>`, checked by the Standard
Webhooks signature and a time within five minutes, at most 256 KB, answered `204` at once; it starts a reading of the events.

## Step 2 — reserved, then the label (a marketplace that confirms orders)

This is the flow of an adapter that claims `orders.confirm` (Wildberries). An adapter that claims `orders.pack` (Ozon) writes nothing at
the reservation; its flow is «The Ozon walk» below. The engine chooses by the parts the adapter claims, never by the marketplace's name.

On `order.reserved` the engine reads the order's state at the marketplace (an order no longer new is not confirmed; a cancelled one is
passed on), puts it into the account's open supply of its group («wyo — сборка <date>», made when there is none; a supply found closed
is replaced), reads its label in the size the fulfilment centre names for the order's warehouse (58 × 40 mm when it names none; «The label
size» below) and sends it with `PUT /orders/{id}/parcels/1/label`, without a scan text. It waits for the
reservation on purpose: putting an order into a supply ends the buyer's free hour of cancellation, and a later cancellation by the seller
is fined.

## Step 3 — the marking codes

On `order.packing_finished` the codes of the units are kept exactly as scanned. When the marketplace takes marking codes for the order,
the codes sent before are removed first, unless the adapter declares that a second sending replaces the first (Ozon does; Wildberries
does not say so), then sent; the verdict is asked
every 20 seconds for 10 minutes, then every 5 minutes. Accepted or not required → «коды приняты»; «проверка не завершена и будет
продолжена» → shown, the hand-over is not stopped; refused or missing → a problem in the marketplace's own words, `PUT
/orders/{id}/problem`, cleared with `DELETE` when corrected codes pass. On `order.reopened` the codes are removed at the marketplace.
An order that requires another identifier (a jewellery number, a phone's number, a customs declaration) gets a problem that names it.

## Step 4 — the hand-over

On `handover.requested` the hand-over and its orders are recorded, and for each group of its orders (one supply holds one group) these
steps run, each stored before the next (`handover_steps`), so a stopped wyo server goes on from the last finished one. An adapter
declares which of the six it needs: Wildberries all six, Ozon four (1, 2, 5 and 6; the transport boxes and the shipping settings are
not Ozon's); the runner skips the others and asks for stored shipping settings only when step 4 is declared:

1. a new supply named after the hand-over;
2. exactly the hand-over's orders moved into it; their labels read again and compared with the ones attached — a changed label stops
   the hand-over with a problem naming the orders;
3. the transport boxes the hand-over names, added to its first supply;
4. the shipping type, city and kind of point from the account's settings, the point the owner chose (never guessed), the hand-over's day;
5. the supply closed;
6. `PUT /handovers/{id}/documents` with each supply's code (`handover_code`, the supply's number as its scan code) and its boxes'
   labels (`box_label`), the supplies' numbers as `external_id`.

A refusal becomes a problem of the hand-over (`PUT /handovers/{id}/problem`) in the marketplace's words; «Повторить» goes on from the
failed step. `handover.request_withdrawn` before the closing moves the orders back to their assembly supply and deletes the empty one;
after the closing nothing can be undone.

## Step 5 — cancellations

Every 5 minutes the open orders' states are read; an order the marketplace cancelled is cancelled at the fulfilment centre
(`POST /orders/{id}/cancel`, or `DELETE /held-orders/{id}` while it is held), and the answer is kept as it is. A cancellation by the
warehouse (`order.cancelled` with `by: warehouse`) or a change waiting for a person (`order.change_waits`) needs the owner. **The wyo
server never cancels an order at a marketplace**; an architecture test fails if the marketplace's cancel method is ever used.

## The Ozon walk

Ozon has no confirmation of an order, and its label exists only after the packing is reported (OZC §8), so the engine, finding that the
adapter claims `orders.pack`, takes this flow. Every request to Ozon leaves through `sync.Caller` and is one row of «Журнал запросов»;
every request to the fulfilment centre is one row marked `fulfilment`. A request that writes to Ozon is never repeated blindly: the state
it changes is read first and what is done is skipped. The calls, in the order the exchange makes them (the periods are in `docs/ozon.md`):

1. **Reading Ozon** (every 5 minutes while «Внешний API» is filled): `POST /v2/warehouse/list`, `POST /v4/posting/fbs/unfulfilled/list`,
   `POST /v3/posting/fbs/get`, `POST /v3/product/info/list` (OZC §5, §6). Nothing is written to the fulfilment centre by reading.
2. **A new posting is sent** (an Ozon FBS posting of a matched warehouse in the state «Новый»).
    - Fulfilment centre: `POST /api/v1/orders` with the posting number as `external_id`, the order number as `group_id`, **every line**
      (`article`, `barcode`, the SKU as `marketplace_item_id`, `quantity`) and Ozon's own deadline (`shipment_date`, OZC §6) as `ship_by`.
      `201` or `200 duplicate` → «передан»; `202 held` → «ждёт у фулфилмент-центра» with the reasons; a refusal → «не передаётся» with a
      problem.
    - Ozon: nothing is written. The posting stays `awaiting_packaging` there until the packing is reported.
3. **Reserved** (`order.reserved`). Fulfilment centre: the event, read with `GET /api/v1/events`. Ozon: nothing is written.
4. **Packing finished** (`order.packing_finished`: the contents are final, each unit's marking code is attached).
    - Fulfilment centre: the event carries the parcels with their units and codes, kept exactly as scanned.
    - Ozon: `POST /v3/posting/fbs/get`, then `POST /v6/fbs/posting/product/exemplar/create-or-get` (which products need a marking code,
      which another identifier — OZC §7), then **`POST /v6/fbs/posting/product/exemplar/set`**: for each product that takes a code, each unit's code
      on its own exemplar, **the full set every time** (OZC §7), the separator written once as `\u001d`. A product that takes no code gets none.
      The order's state becomes «коды проверяются».
    - Ozon needs another identifier than a marking code (a customs declaration number, a batch registration number, a jewellery number, an
      IMEI, a weight) or a count of boxes for a multi-box product: fulfilment centre: `PUT /api/v1/orders/{id}/problem` naming it; the
      owner enters it in the Ozon cabinet and presses «Передать заново». The country of origin is not checked by the wyo server: Ozon's own
      refusal of the packing (step 6) names it.
    - Ozon needs a marking code and the fulfilment centre sent none: a problem of its own, «Маркетплейс требует код маркировки, а фулфилмент-центр не прислал ни одного».
    - No product takes a code: nothing is sent; the order's codes are recorded as not taken and the packing is reported at once (step 6).
5. **The verdict:** `POST /v5/fbs/posting/product/exemplar/status`, every 20 seconds for 10 minutes, then every 5 minutes. A 200 of `set`
   only means Ozon took the task (OZC §7). `ship_available` → «коды приняты» and step 6; `validation_in_process` or a mark `processing` →
   asked again; a mark `failed` («система не успела обработать запрос», OZC §7) → the full set is sent once more (a resend that never reached Ozon is
   not counted and is made at the next check), a second `failed` is a problem; a mark with `error_codes` → fulfilment centre: `PUT /api/v1/orders/{id}/problem` with Ozon's codes in Ozon's words (their words
   are not sourced, OZC §7), cleared with `DELETE /api/v1/orders/{id}/problem` when corrected codes arrive with a new
   `order.packing_finished`. `order.reopened` before the report writes nothing to Ozon (the next `order.packing_finished` sends the full set
   again); after the report it is a problem that says changing codes at Ozon is not sourced, and it stays when Ozon then shows the posting
   packed — only «Передать заново» clears it.
6. **The packing is reported.**
    - Fulfilment centre: `GET /api/v1/orders/{id}` for the parcels. If the parcels' contents are not exactly Ozon's lines (the fulfilment
      centre cut the order, `order.reduced`), the report is not made and the order gets a problem; the owner removes the products in the Ozon
      cabinet.
    - Ozon: `POST /v3/posting/fbs/get` (a posting already `awaiting_deliver` is not reported again; a cancelled one is passed on, step 9; after a report that split the
      posting, each part is read by its own number, and a part cancelled is a problem naming it, never a cancellation passed on),
      then **`POST /v4/posting/fbs/ship`**: one package for each parcel with its products by SKU and quantity, and `with.additional_data`.
      **Several parcels split the posting**: Ozon makes a posting of each parcel's contents (OZC §7), and its answer names each part's number.
      A 200 is not the end («Ответ с кодом 200 не гарантирует успешную сборку заказа», OZC §7): `POST /v3/posting/fbs/get` is read every
      20 seconds for 10 minutes, then every 5 minutes, until the status is `awaiting_deliver`. The substatus `ship_failed` makes the report
      again, at most three times; then, and for each refusal in Ozon's words (`HAS_INCORRECT_PRODUCT_QUANTITY`,
      `EXEMPLAR_INFO_NOT_FILLED_COMPLETELY`, `INCORRECT_OVH_FOR_POSTING`, OZC §7), the order gets a problem. `POSTING_ALREADY_SHIPPED`
      means it is done. Each parcel takes the part with the same contents; the numbers are stored on the parcels and shown in the order's
      details.
7. **Labels** — 60 seconds after Ozon shows the posting packed (OZC §8 recommends 45–60 seconds).
    - Ozon: **`POST /v3/posting/fbs/package-label/create`** for the parts, the task of the warehouse's label type (`carriage_label_type`);
      `POST /v2/posting/fbs/package-label/get` every 10 seconds for 5 minutes, then every 5 minutes (in practice every 20 seconds at best:
      the job wakes at that period), until `completed`; then `GET` of the `file_url` link **without the key**, only over https and only
      from a host under `ozone.ru` (OZC §8: the domain of Ozon's own example link; the real host is not sourced), at most 5 MB, only a file
      that begins as a PDF. A part among `unprinted_postings`, or `error`, is a problem in Ozon's words.
    - Fulfilment centre: `PUT /api/v1/orders/{id}/parcels/{n}/label` as `application/pdf` for each parcel, **without a scan text** (which of the
      posting's two barcodes a scanner reads is not sourced, OZC §8). The order's state becomes «этикетка передана».
8. **Packed, and the hand-over** (`order.packed`, then `handover.requested`).
    - Fulfilment centre: `GET /api/v1/handovers/{id}` for the hand-over's parcels. A shipment is made for each Ozon warehouse among them
      (OZC §9: for FBS `delivery_method_id` is the warehouse's number).
    - Ozon, four steps, each stored before the next. **Batch** — `POST /v2/carriage/delivery/list` for the warehouse and the day; a shipment
      of the status `new` already there is reused, otherwise, when Ozon lists the action `create`, `POST /v1/carriage/create`; when Ozon
      cannot make one yet or answers `POSTING_NOT_FOUND`, the hand-over waits with Ozon's words and tries again every 5 minutes, reading the
      list first, so a shipment is never made twice. **Move** — a new shipment takes in every posting ready for shipment (OZC §9), so
      `POST /v1/carriage/set-postings` rewrites its list to exactly the parts of the hand-over's parcels (a posting Ozon refuses is a
      problem naming it), and `POST /v2/posting/fbs/act/get-postings` reads it back and compares it both ways: a difference is a problem
      that names the postings, and a shipment with other postings is never approved. Ozon says the method is not available to sellers
      outside Russia (OZC §9): its refusal is a problem. **Close** — `POST /v1/carriage/get` and the shipment's list read again and compared with the hand-over's both
      ways (a difference rewrites the list once more; a second difference is the problem naming the postings, and nothing is approved),
      and `POST /v1/carriage/approve` unless it is already approved (without `containers_count`, which is for entrusted acceptance only, OZC §9). **Documents** — `POST
/v2/posting/fbs/act/check-status` until `ready` (`in_process`, `error` and `cancelled` wait and ask again, as Ozon says; a posting of
      the hand-over among `removed_from_act` is a problem that names it), then `POST /v2/posting/fbs/act/get-barcode/text`, `POST
/v2/posting/fbs/act/get-barcode` and `POST /v2/posting/fbs/act/get-pdf`. Whether the last two answer with the file itself or with an
      object holding it is not sourced (OZC §9): both are read, by one function.
    - Fulfilment centre: `PUT /api/v1/handovers/{id}/documents` with `external_id` the shipment's number, the hand-over barcode as
      `handover_code` (Ozon's text of it as `scan_code`) and the shipping list with the waybill as `act` (one PDF, OZC §9).
    - The transport boxes the fulfilment centre names are not sent to Ozon (Ozon has none outside entrusted acceptance). A refusal of
      Ozon is `PUT /api/v1/handovers/{id}/problem` in Ozon's words, and «Повторить» goes on from the failed step.
    - `handover.request_withdrawn` before the approval: `POST /v1/carriage/cancel` for the shipment of the hand-over (read first; one that is not
      `new` is left), which also cancels a shipment the owner had made in the cabinet and the wyo server found and used; after the approval
      nothing is undone.
9. **Cancellations.** Ozon: `POST /v3/posting/fbs/get` for every open order every 5 minutes. A cancellation (also after the packing,
   `cancelled_after_ship`, OZC §6) is passed on: fulfilment centre: `POST /api/v1/orders/{id}/cancel`, `DELETE /api/v1/held-orders/{id}` while
   held; its answer is kept as it is. **The wyo server never cancels a posting at Ozon and never removes a product from one**: the generated
   client holds neither `/v2/posting/fbs/cancel` nor `/v2/posting/fbs/product/cancel` (`TestTheCancelMethodsOfOzonAreNeverGeneratedInThisRepository`,
   `internal/archtest`).
10. **Handed over** (`order.handed_over`): the order's state becomes «передан в доставку».

An Ozon key that lacks the access level `Posting FBS` (the methods of steps 4 to 8 are not in its list, OZC §3) is accepted for reading and
marked «только чтение»; the orders of that account are not sent to the fulfilment centre until the key is replaced.

## The Яндекс Маркет walk

Яндекс Маркет takes the boxes and the marking codes in one request, makes its own daily shipment, and wants a last write («Готов к
отгрузке») before an order may leave, so the engine, finding that the adapter claims `orders.pack` with `CodesInPack`, `orders.ready`
and `labels.fetch`, takes this flow, and finding that its batch is found and never made, runs the hand-over differently (below). It does
this by the parts the adapter claims, never by the marketplace's name. Every request to Яндекс Маркет leaves through `sync.Caller` and
is one row of «Журнал запросов»; every request to the fulfilment centre is one row marked `fulfilment`. A request that writes is never
repeated blindly: the state it changes is read first and what is done is skipped. The key needs the access level «Обработка заказов и
учёт товаров» (`inventory-and-order-processing`) or `all-methods`; a key that may only read is marked «только чтение» and its orders are
not sent (YMC §2, §5). Only FBS and Express orders of a matched warehouse are exchanged; FBY and DBS never. With «Внешний API» empty
nothing of this runs and Яндекс Маркет gets only the readings of Plan 7 (rule R3). Each call below is written as the neutral API's call,
then Яндекс Маркет's method; the periods and limits of the methods are `docs/yandex.md`, section «Запросы записи» (limits: YMC §5).

1. **Reading Яндекс Маркет** (every 5 minutes while «Внешний API» is filled; a notification starts a reading at once): the readings of
   Plan 7 (`docs/yandex.md`, «Чтение заказов»). Nothing is written by reading.
2. **A new order is sent** (an FBS or Express order of a matched warehouse with the status `PROCESSING` and the substatus `STARTED`,
   YMC §7; `UNPAID` and `PENDING` wait).
    - Fulfilment centre: `POST /api/v1/orders` with the order number as `external_id`, **every line** (`article` from `offerId`, the
      `barcode`s, `quantity`; an item's `id` is its line's number in the order, not a product's number, so no marketplace product number is
      sent), Яндекс Маркет's shipment day and time (`shipmentDate`, `shipmentTime`) as `ship_by`, `test` (the order's `fake`), and
      `handover` (`courier_code` for Express, `shipment` otherwise). A changed shipment day is passed on with `PATCH /api/v1/orders/{id}`.
      `201` or `200 duplicate` → «передан»; `202 held` → «ждёт у фулфилмент-центра» with the reasons; a refusal → «не передаётся» with a
      problem and «Передать заново».
    - Яндекс Маркет: nothing is written. **An order whose warehouse the reading could not tell** (the shop has several warehouses of the
      scheme) is not sent: «склад не определён» under «Требуют внимания» and in the bell.
3. **Reserved** (`order.reserved`). Nothing is written to Яндекс Маркет.
4. **Packing finished** (`order.packing_finished`: the contents are final, each unit's marking code is attached).
    - Яндекс Маркет: `POST /v1/businesses/{businessId}/orders` (the order is read first: cancelled → step 10; already `READY_TO_SHIP` →
      the boxes are skipped, because a layout cannot change after it, and the walk goes to step 6).
    - Fulfilment centre: `GET /api/v1/orders/{id}` for the parcels and their units.
    - Яндекс Маркет: **`PUT /v2/campaigns/{campaignId}/orders/{orderId}/boxes`** (`setOrderBoxLayout`), once: one box for each parcel;
      each box lists the order's items it holds by their `id` with `fullCount` and, for an item that takes codes (`requiredInstanceTypes`
      with `CIS` or `CIS_OPTIONAL`), the units' codes in `instances[].cis`, each written exactly as scanned with its separator once as
      `\u001d` (the one function and its test of Plans 4 and 6). Never `allowRemove`, never `partialCount`. The answer gives each box's
      `boxId`, stored on the parcel. A later sending replaces the earlier until the order is `READY_TO_SHIP` (YMC §10). The order's state
      becomes «коды проверяются».
    - **The contents must equal the order.** Parcels that do not hold exactly Яндекс Маркет's items (the fulfilment centre cut the order,
      `order.reduced`) are not reported: fulfilment centre: `PUT /api/v1/orders/{id}/problem`; the owner removes the goods in the
      cabinet (the wyo server never removes them) and presses «Передать заново». The same for a product split over several boxes and for a
      unit that needs `UIN`, `RNPT` or `GTD` (YMC §7): the neutral API carries none of them, so the problem names them and says they are
      entered in the cabinet. A code needed and none sent: a problem, «Маркет требует код маркировки».
5. **The verdict:** `POST /v2/campaigns/{campaignId}/orders/{orderId}/identifiers/status` (`getOrderIdentifiersStatus`) every 20 seconds
   for 10 minutes, then every 5 minutes (the one schedule function of Plans 4 and 6), until every code is final; no item takes codes →
   nothing to wait for. Accepted → «коды приняты». Refused → fulfilment centre: `PUT /api/v1/orders/{id}/problem` with Яндекс Маркет's
   own words, cleared with `DELETE` when corrected codes arrive with a new `order.packing_finished` (the whole layout is sent again; it
   replaces the earlier). `order.reopened` before «Готов к отгрузке» writes nothing; the next `order.packing_finished` sends the whole
   layout again. After it the layout cannot change (YMC §10): a problem that says so, for the owner and the fulfilment centre; it stays
   until «Передать заново».
6. **Labels, then «Готов к отгрузке».**
    - Яндекс Маркет: **`GET /v2/campaigns/{campaignId}/orders/{orderId}/delivery/shipments/{shipmentId}/boxes/{boxId}/label`**
      (`generateOrderLabel`) for each box; the `shipmentId` is outdated and any number is taken (YMC §10). The `format` is the size the
      fulfilment centre names (below, «The label size»). The file is read from the API's own host, with the key, through the one caller,
      at most 5 MB, only when it begins as a PDF.
    - Fulfilment centre: `PUT /api/v1/orders/{id}/parcels/{n}/label` as `application/pdf` for each parcel, **without a scan text** (which
      of the label's barcodes a scanner reads is not sourced). The order's state becomes «этикетка передана». A size Яндекс Маркет does not
      make is a problem on the order, «Маркет не делает ярлыки размера W × H мм — выберите на складе фулфилмент-центра 58 × 40, 40 × 58,
      75 × 120 или A4».
    - On `order.packed`, with the codes accepted and every label attached, Яндекс Маркет: the order is read (already `READY_TO_SHIP` →
      nothing), then **`PUT /v2/campaigns/{campaignId}/orders/{orderId}/status`** (`updateOrderStatus`) with `PROCESSING` and
      `READY_TO_SHIP`, from the one function that may call this generated method; no other status is ever sent (principle XII). The order's
      state becomes «готов к отгрузке». A refusal is a problem in Яндекс Маркет's words.
7. **Express: the order enters no hand-over** (step 8 does not run for it); it leaves as step 9 says.
8. **The daily shipment (FBS)** — `handover.requested`. Fulfilment centre: `GET /api/v1/handovers/{id}` for the hand-over's parcels. See
   «The daily shipment» below.
9. **Express: the courier's code** — `order.courier_code_entered`. See «The courier's code» below.
10. **Cancellations.** Яндекс Маркет: the readings and notifications (`ORDER_CANCELLED`, `ORDER_STATUS_UPDATED`) tell the engine that Яндекс
    Маркет cancelled an order. It is passed on, before and after «Готов к отгрузке»: fulfilment centre: `POST /api/v1/orders/{id}/cancel`,
    `DELETE /api/v1/held-orders/{id}` while held; its answer is kept as it is. **The wyo server never cancels an order at Яндекс Маркет,
    never removes goods from one (`allowRemove` is never sent), never moves an order to another shipment and never sends a DBS order**:
    `updateOrderItems`, `acceptOrderCancellation` and `transferOrdersFromShipment` are not in the generated client
    (`internal/archtest`, `yandexNever`), and the status method is called by one function only
    (`TestTheStatusMethodOfYandexMarketIsCalledOnlyByItsOneReadyFunction`).
11. **Handed over** (`order.handed_over`): the order's state becomes «передан в доставку».

### The daily shipment

Яндекс Маркет makes one shipment for each warehouse and day itself, by the schedule set in the cabinet (YMC §11); nothing can be added to
it. The engine therefore **finds, compares and confirms; it never makes or fills**. On `handover.requested` it runs four steps for each
Яндекс Маркет warehouse among the hand-over's orders, each stored before the next, so a stopped wyo server goes on from the last
finished one:

1. **Find (`batch`).** Яндекс Маркет: `PUT /v2/campaigns/{campaignId}/first-mile/shipments` (`searchShipments`) for the hand-over's day
   with the hand-over's order numbers, then `GET …/first-mile/shipments/{shipmentId}` (`getShipment`). No shipment yet → the hand-over
   waits with «Маркет ещё не сформировал отгрузку» and the step is tried again every 5 minutes while the request stands.
2. **Compare (`move`).** The one function of Plan 6a compares the shipment with the hand-over both ways; **test orders are left out**
   (the fulfilment centre never hands them over; whether Яндекс Маркет puts one into a real shipment is not sourced). Nothing is moved.
    - In the shipment, not in the hand-over → fulfilment centre: `PUT /api/v1/handovers/{id}/problem` naming the orders: the owner moves
      them to the next shipment or cancels them in the cabinet and presses «Повторить». The wyo server never moves them
      (`transferOrdersFromShipment` is never generated) and **never confirms such a shipment**.
    - In the hand-over, not in the shipment → a problem naming them (they may belong to another day's shipment).
3. **Confirm (`close`).** Яндекс Маркет: the shipment is read; already confirmed → nothing; `CONFIRM` among `availableActions` →
   **`POST …/first-mile/shipments/{shipmentId}/confirm`** (`confirmShipment`). Яндекс Маркет's refusals (YMC §4, §11): «Cutoff time for
   shipments has not been reached yet» → the hand-over waits with these words and the time is tried every 5 minutes; «Some orders have not
   been processed yet…» → a problem naming the orders not yet «Готов к отгрузке»; «Some orders are in the process of being excluded from
   shipment… wait up to 30 minutes» → the step waits and reads again.
4. **Documents (`documents`).** When `DOWNLOAD_ACT` is offered, Яндекс Маркет: **`GET …/first-mile/shipments/{shipmentId}/act`**
   (`downloadShipmentAct`, a PDF); fulfilment centre: `PUT /api/v1/handovers/{id}/documents` with the act as kind `act`, the shipment's
   number as `external_id`. **The waybill** of a `WITHDRAW` shipment is an `.xls` file that the neutral API does not take (PDF, PNG or
   JPEG): it is not sent, and the hand-over's page says that the owner downloads it in the cabinet.

Both kinds of shipment — `WITHDRAW` («Маркет забирает со склада») and `IMPORT` («Склад везёт в сортировочный центр или пункт приёма») —
walk the same way; the page of the hand-over names the kind. The transport boxes the fulfilment centre names are not sent (pallets and
their labels are other Яндекс Маркет methods that this walk does not call); the page says so. A refusal of Яндекс Маркет is
`PUT /api/v1/handovers/{id}/problem` in its words, and «Повторить» goes on from the failed step. `handover.request_withdrawn` before the
confirmation: nothing was written to Яндекс Маркет, so nothing is undone; after it nothing is undone, and the page says so.

### The courier's code

An Express order is sent with `handover: courier_code` and enters no hand-over. When the courier comes, a worker at the fulfilment centre
types the code the courier names:

1. Fulfilment centre: the event `order.courier_code_entered` with `details.check_id` and `details.code`. The engine applies it **at once,
   ahead of other work of the account**, within the budget. A notification brings it at once; read every minute it may wait up to a
   minute, so «Внешний API» asks the owner to set the events mode to notifications beside the Express warehouses.
2. Яндекс Маркет: **`PUT /v2/campaigns/{campaignId}/orders/{orderId}/verifyEac`** (`verifyOrderEac`) with the code (YMC §12).
3. Fulfilment centre: `PUT /api/v1/orders/{id}/courier-code-checks/{check_id}` with `{ result, attempts_left?, message? }`:
    - `ACCEPTED` → `accepted` (the worker gives the parcels to the courier; `order.handed_over` follows);
    - `REJECTED` → `refused` with `attempts_left` (Яндекс Маркет's `attemptsLeft`);
    - `NEED_UPDATE` → `new_code_needed` («Маркет отправит курьеру новый код»);
    - a refusal of the request itself (the order in a status that takes no code, a key without the level) → `refused` with Яндекс Маркет's
      words as `message`, and a problem on the order for the owner.
4. **The code is never kept.** It is in the stored event only until the answer is sent, then blanked in the event row; it is in no log line,
   journal row, answer of the wyo server's own API or error text. The order's details show each check (when, the answer, the attempts
   left) and never the code.

The code's kind is the order's `delivery.eacType`, given only «Если у магазина настроена работа с кодами подтверждения»
(`COURIER_TO_MERCHANT`: the courier names the code, YMC §12). An Express order without it is still sent as a courier-code order; what
Яндекс Маркет answers for it is not sourced.

### The label size

The fulfilment centre names a label size for each of its warehouses and each marketplace in `label_sizes` of `GET /api/v1/warehouses`
(`docs/fulfilment-centre-api.md`). The engine reads them when «Внешний API» is filled, every hour, and before a label is asked for when
the last reading is older than an hour, and keeps them for each fulfilment-centre warehouse and marketplace (`fc_label_sizes`).
`labels.fetch` receives the size of the order's warehouse and each adapter maps it to what its marketplace makes; **the wyo server chooses
nothing the fulfilment centre names**:

| Marketplace   | Sizes made (`format` or size)                                                            | No size named               | Another size                                                                               |
| ------------- | ---------------------------------------------------------------------------------------- | --------------------------- | ------------------------------------------------------------------------------------------ |
| Яндекс Маркет | 58 × 40 → `A9_HORIZONTALLY`, 40 × 58 → `A9`, 75 × 120 → `A7`, 210 × 297 → `A4` (YMC §10) | Яндекс Маркет's own default | A problem on the order: «Маркет не делает ярлыки размера W × H мм — …» (step 6)            |
| Wildberries   | 58 × 40 and 40 × 30 (the two sizes of its sticker method)                                | 58 × 40, as before          | The same problem in Wildberries' words; its tests of the 58 × 40 sticker stay as they were |
| Ozon          | Unchanged: the label type follows the warehouse's own setting at Ozon (Plan 6)           | Unchanged                   | Unchanged                                                                                  |

### The test mark and `handover`

A marketplace's test order (Plan 7's `fake` of Яндекс Маркет) is sent with `test: true`. The fulfilment centre packs it like any other
order and never puts it into a hand-over, never hands it over, never bills, pays or counts it. The exchange treats it like any other
order in every step above (Яндекс Маркет accepts the writes of a test order, YMC §9), except that it is left out of the shipment's
comparison; the owner cancels it in the cabinet after the try, and the wyo server never counts it in a total. `handover` says how an order
leaves: `shipment` (the default: in a day's hand-over with the others) or `courier_code` (alone, to a courier who names a code; the
adapter names which of its orders leave so — for Яндекс Маркет the scheme Express).

## What never travels

A marketplace key never goes to a fulfilment centre and the fulfilment centre's key never goes to a marketplace; neither, nor a marking
code, nor a courier's code, reaches a log line, a journal row or an answer of the wyo server's API (the request for an Ozon label file carries no key at all, and
its journal row names the path `{label file}` and never the link, which may hold a token); a buyer's address, comment or region is dropped where the
marketplace's answer arrives. The tests: `TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide` (`internal/server`) and the exchange's own.

## For development and tests

A build made with the `wyotest` tag accepts a plain `http://` address for «Внешний API» (the fake fulfilment centre listens on
`http://127.0.0.1:8792`) and answers `POST /__test/exchange/run` by running every job of the exchange at once, so a browser test does
not wait for the jobs' periods. A released wyo server has neither.
