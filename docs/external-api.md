# «Внешний API»: the exchange with a fulfilment centre, step by step

This page is for a seller's engineer who wants to know exactly what the wyo server does once «Внешний API» is filled, and for a
fulfilment centre's developer whose system will be called. The seller's own instructions are the README, section 10
(«Работа с фулфилмент-центром»). The format on the fulfilment centre's side is `docs/fulfilment-centre-api.md`; the plan that built
this is `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md` (Plan 4).

## Where it lives

| Part                                     | What it does                                                                                                   |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `internal/fulfilment`                    | The only package that knows the neutral API: the client, the refusals' table, the signature check.             |
| `internal/exchange`                      | The exchange engine: sending, events, the label, the codes, cancellations, the notices and the views.          |
| `internal/handover`                      | The six steps of a hand-over.                                                                                  |
| `internal/marketplace/writing.go`        | The writing parts of the adapter contract; `internal/marketplace/wildberries` implements them for Wildberries. |
| `internal/outbound`, purpose fulfilment  | Lists the fulfilment centre's address only while «Внешний API» is filled.                                      |
| `src/routes/(app)/external-api`, `handovers` | The pages «Внешний API» and «Передачи».                                                                    |

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

| Job                 | Every                                         | What it does                                                                                 |
| ------------------- | --------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `exchange.orders`   | 1 minute                                      | Reads the marketplaces' new orders, sends the unsent ones, goes on with interrupted hand-overs. |
| `exchange.states`   | 5 minutes                                     | Reads the open orders' states, passes cancellations on.                                       |
| `exchange.events`   | 1 minute (10 minutes beside notifications)     | Reads `GET /api/v1/events?after=<mark>`, stores each event, applies them in order.            |
| `exchange.verdicts` | 20 seconds                                    | Asks the marking codes' verdicts that are due.                                                |
| `exchange.prune`    | nightly, 04:00–05:00 of the installation's zone | Deletes marking codes 90 days after their order's hand-over and applied events after 30 days. |

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

`201` (or `200 duplicate`) → «передан»; `202 held` → «ждёт у фулфилмент-центра» with the reasons; a refusal (for example
`403 marketplace_not_enabled`, `409 external_id_reused`) → «не передаётся» with a problem and «Передать заново». An answer lost on the way
is harmless: the same order sent again is one order there.

## Events

Events are stored first (`fc_events`), a repeat is ignored, and they are applied in the order of their numbers, only up to the last
number read in order with `GET /events` — so a notification that arrives early waits for the events before it, and asking and
notifications together lose and double nothing. A notification is `POST /hooks/fulfilment/<random text>`, checked by the Standard
Webhooks signature and a time within five minutes, at most 256 KB, answered `204` at once; it starts a reading of the events.

## Step 2 — reserved, then the label

On `order.reserved` the engine reads the order's state at the marketplace (an order no longer new is not confirmed; a cancelled one is
passed on), puts it into the account's open supply of its group («wyo — сборка <date>», made when there is none; a supply found closed
is replaced), reads its label (58 × 40 mm) and sends it with `PUT /orders/{id}/parcels/1/label`, without a scan text. It waits for the
reservation on purpose: putting an order into a supply ends the buyer's free hour of cancellation, and a later cancellation by the seller
is fined.

## Step 3 — the marking codes

On `order.packing_finished` the codes of the units are kept exactly as scanned. When the marketplace takes marking codes for the order,
the codes sent before are removed first (whether a second sending replaces the first is not sourced), then sent; the verdict is asked
every 20 seconds for 10 minutes, then every 5 minutes. Accepted or not required → «коды приняты»; «проверка не завершена и будет
продолжена» → shown, the hand-over is not stopped; refused or missing → a problem in the marketplace's own words, `PUT
/orders/{id}/problem`, cleared with `DELETE` when corrected codes pass. On `order.reopened` the codes are removed at the marketplace.
An order that requires another identifier (a jewellery number, a phone's number, a customs declaration) gets a problem that names it.

## Step 4 — the hand-over

On `handover.requested` the hand-over and its orders are recorded, and for each group of its orders (one supply holds one group) these
steps run, each stored before the next (`handover_steps`), so a stopped wyo server goes on from the last finished one:

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

## What never travels

A marketplace key never goes to a fulfilment centre and the fulfilment centre's key never goes to a marketplace; neither, nor a marking
code, reaches a log line, a journal row or an answer of the wyo server's API; a buyer's address, comment or region is dropped where the
marketplace's answer arrives. The tests: `TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide` (`internal/server`) and the exchange's own.

## For development and tests

A build made with the `wyotest` tag accepts a plain `http://` address for «Внешний API» (the fake fulfilment centre listens on
`http://127.0.0.1:8792`) and answers `POST /__test/exchange/run` by running every job of the exchange at once, so a browser test does
not wait for the jobs' periods. A released wyo server has neither.
