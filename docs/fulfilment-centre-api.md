# The fulfilment-centre API

The data format between a seller's own system and a **fulfilment centre**: the system that stores the seller's goods, picks and
packs the seller's orders and hands the parcels to a carrier. Orders go in, labels go in, statuses and marking codes come back,
and at the end the documents of the hand-over go in. Either side can be any program that keeps this format.

The machine-readable description is [`api/fulfilment-centre.yaml`](../api/fulfilment-centre.yaml) (OpenAPI 3.1). This page is its
explanation in prose. The Go code generated from the description is in `internal/fulfilment/fcapi/`; it is never edited by hand
(`pnpm generate`).

The format names no product and no marketplace. A marketplace is a **code** inside the data (the `marketplace` field of an order),
taken from the list the fulfilment centre returns for the seller's key.

## Words used

- **Seller** — the owner of the goods. A key names exactly one seller.
- **Sender** — the seller's own system that calls the API. The fulfilment centre never learns what it is.
- **Held order** — an order the fulfilment centre received but could not turn into a working order yet (an unknown product, an
  unknown warehouse, the seller's work stopped). It is kept with its reasons, tried again, and never dropped.
- **Hand-over** — the closed parcels of one seller, one warehouse and one marketplace that leave with the carrier together on
  one day.
- **Event** — one stored fact about an order or a hand-over, with a growing number (`sequence`).

## How a call is made

| What         | Rule                                                                                                                                                                                                 |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Address      | Every address starts with `/api/v1`.                                                                                                                                                                 |
| Key          | The header `Authorization: Bearer <key>`. The key is an opaque secret text, shown once when it is created. It names one seller.                                                                      |
| Access       | A key is `read_write` or `read_only`. A read-only key is refused with `read_only_key` on every write.                                                                                                |
| Body         | JSON, both ways. Unknown fields of a request are refused (`400`), so no buyer data can travel in an order.                                                                                           |
| Times        | ISO 8601 in UTC. Quantities are whole numbers. A marking code is its exact text, group separator included (`\u001d` in JSON).                                                                        |
| Lists        | `limit` (50 by default, at most 200) and `cursor`; the answer has `next_cursor`, `null` on the last page. Events use `after` and `next_after`.                                                       |
| Limits       | 300 requests a minute per key (over it, `429 rate_limited` with `Retry-After`); a JSON body of 1 MB; 8 MB for the calls with files, a file of 5 MB. Events, journal and deliveries are kept 30 days. |
| Repeating    | Every write can be repeated: an order by its `external_id`; a label, a set of documents and a problem by `PUT`; a cancel and a cut by stating the wanted end state.                                  |
| Others' data | A record of another seller is answered exactly like one that does not exist (`404 not_found`).                                                                                                       |

### Refusals

A refusal is `{ "error": { "code", "params"?, "issues"? } }`. `issues` is a list of `{ path, code }`: where in the body the fault is
and what it is. Values of the body never come back. The order of checks for a write is: no key (`401`), a read-only key (`403`),
a body over its limit (`413`), a body that is not JSON or not of the right shape (`400` with `issues`), then the call's own rules.
Nothing is stored for a refused request.

| Code                      | Status | When                                                                                                                                                                                                                                                              |
| ------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bad_request`             | 400    | The body or a parameter is wrong; `issues` names the places.                                                                                                                                                                                                      |
| `unauthorized`            | 401    | No key, a mistyped key, or a revoked key.                                                                                                                                                                                                                         |
| `forbidden`               | 403    | The seller's work is switched off at the fulfilment centre.                                                                                                                                                                                                       |
| `api_off`                 | 403    | The seller switched the API off; a right key does not work until it is switched on.                                                                                                                                                                               |
| `read_only_key`           | 403    | A write with a read-only key.                                                                                                                                                                                                                                     |
| `marketplace_not_enabled` | 403    | The order's marketplace is not enabled for this seller.                                                                                                                                                                                                           |
| `not_found`               | 404    | No such record for this seller.                                                                                                                                                                                                                                   |
| `not_api_order`           | 409    | Only an order that came through this API can be changed.                                                                                                                                                                                                          |
| `external_id_reused`      | 409    | A repeat of an order number with other lines.                                                                                                                                                                                                                     |
| `handed_over`             | 409    | The order is already handed over; too late to change it.                                                                                                                                                                                                          |
| `conflict`                | 409    | The call cannot be done now: the order is cancelled, the documents were not asked for, the hand-over is completed, the scan code belongs to another parcel, the order does not leave with a courier's code, or a courier's code was already answered differently. |
| `too_large`               | 413    | The body is over its limit.                                                                                                                                                                                                                                       |
| `rate_limited`            | 429    | More than 300 requests a minute.                                                                                                                                                                                                                                  |
| `unavailable`             | 503    | The fulfilment centre is in maintenance.                                                                                                                                                                                                                          |

## The calls

| Call                                                        | What it does                                                                                                                                                             |
| ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET /api/v1/account`                                       | The seller's name, whether the API is on, this key's access, the **enabled marketplaces** `[{ code, name }]`, the limits, the fulfilment centre's clock.                 |
| `GET /api/v1/warehouses`                                    | The working warehouses: `code`, `name`, `city`, `label_sizes`. A `code` goes into an order's `warehouse`.                                                                |
| `POST /api/v1/orders`                                       | Send an order in (below).                                                                                                                                                |
| `GET /api/v1/orders`                                        | The seller's orders however they came; filters `updated_since`, `status`, `marketplace` (a code, or `none` for the seller's own shop), `external_id`, `limit`, `cursor`. |
| `GET /api/v1/orders/{id}`                                   | One order.                                                                                                                                                               |
| `PATCH /api/v1/orders/{id}`                                 | The hand-over deadline `ship_by` and the packer's note `packing_note`.                                                                                                   |
| `POST /api/v1/orders/{id}/cancel`                           | Cancel, with a `reason`.                                                                                                                                                 |
| `POST /api/v1/orders/{id}/reduce`                           | Cut: for each named line, how many units are wanted now.                                                                                                                 |
| `PUT` / `DELETE /api/v1/orders/{id}/parcels/{number}/label` | The label file of a parcel.                                                                                                                                              |
| `PUT` / `DELETE /api/v1/orders/{id}/problem`                | A problem the sender reports about the order.                                                                                                                            |
| `PUT /api/v1/orders/{id}/courier-code-checks/{check_id}`    | The answer to a code a courier named for an order that leaves alone (below).                                                                                             |
| `GET /api/v1/held-orders`                                   | The orders that wait, with their reasons.                                                                                                                                |
| `GET` / `DELETE /api/v1/held-orders/{id}`                   | One received order in any state / withdraw a waiting one.                                                                                                                |
| `GET /api/v1/events`                                        | The events (`after`, `types`, `limit`).                                                                                                                                  |
| `GET /api/v1/handovers`, `GET /api/v1/handovers/{id}`       | The seller's hand-overs (`state=open` leaves only those that have not left).                                                                                             |
| `PUT /api/v1/handovers/{id}/documents`                      | The hand-over's documents.                                                                                                                                               |
| `PUT` / `DELETE /api/v1/handovers/{id}/problem`             | A problem the sender reports about the hand-over.                                                                                                                        |

The changes of an order (`PATCH`, cancel, reduce, the label, the problem, the answer to a courier's code) are allowed **only for an order that came through the
API**; any other order answers `409 not_api_order` (it can still be read).

### Sending an order

The body of `POST /orders`:

- `external_id` — the sender's own number, 1–64 characters, required. With the marketplace it identifies the order.
- `marketplace` — a code from `GET /account`, or absent for the seller's own shop.
- `group_id` — the buyer's basket, optional.
- `warehouse` — a code from `GET /warehouses`; may be left out when the fulfilment centre has one working warehouse.
- `ship_by` — the hand-over deadline, ISO 8601 with a zone, required.
- `packing_note` — up to 300 characters.
- `test` — `true` for a marketplace's test order; left out, it is `false` (below).
- `handover` — how the order leaves: `shipment` (the default) or `courier_code` (below).
- `lines[]` — 1 to 200 lines: `article`, `barcode` and/or `marketplace_item_id` (at least one) and `quantity` (a whole number from 1).

How a line's product is found: first by `barcode` (unique per seller), then by `article` (the product's code), then by
`marketplace_item_id`. A barcode of a box or a pallet does not name a product. When an article and a barcode are both given and
point at two different products, the order is held (`line_conflict`): the fulfilment centre will not guess. Two lines of one product
are added up.

The answers:

- `201` `{ "state": "accepted", "order": {…} }` — a working order exists and its goods are reserved.
- `202` `{ "state": "held", "held": {…} }` — received, waits. The reasons are in `held.reasons`. The fulfilment centre tries again by
  itself and sends `order.accepted` when the order is taken in.
- `200` with `"duplicate": true` — a repeat of an order already taken in: the same order, no second one.
- `409 external_id_reused` — a repeat with other lines. `403 marketplace_not_enabled` — the marketplace is not enabled.

A held order is in one of the states `held_link`, `held_warehouse`, `held_work`, `held_refused`, `accepted`, `withdrawn`. Withdrawing
one (`DELETE /held-orders/{id}`) ends its waiting; the sender then reads the state `withdrawn`.

### Test orders

A marketplace's test order has no buyer and no money. With `test: true` the order is picked and packed like any other, but the
fulfilment centre never puts it into a hand-over, never hands it over, and never bills, pays or counts it. The order object returns
`test` as it was sent.

### Orders that leave alone, with a courier's code

An order sent with `handover: courier_code` does not go into a day's hand-over with the others. Its parcels are packed and wait. When
a courier comes for them, a worker at the fulfilment centre types the code the courier names, and the fulfilment centre reports the
event `order.courier_code_entered` with `details.check_id` (the id of this check) and `details.code` (the typed text). For such an
order `ship_by` is the time by which it must be packed. The order object returns `handover` as it was sent (`shipment` or
`courier_code`).

The sender checks the code with its marketplace and answers with `PUT /orders/{id}/courier-code-checks/{check_id}` and
`{ result, attempts_left?, message? }`:

- `result` is `accepted` (the marketplace accepted the code: the worker gives the parcels to the courier, the order becomes `shipped`
  and `order.handed_over` follows), `refused` (the code is wrong) or `new_code_needed` (the marketplace sends the courier a new
  code).
- `attempts_left` is how many tries of the code the marketplace still allows, from 0 to 1000, when it says; `message` is the
  marketplace's own words for the worker, up to 300 characters. Both may be `null` or left out.
- The answer is `200` with `{ state, order }`. The same answer sent again is `200` again; a different answer to an answered check is
  `409 conflict`. A check or an order of another seller, or one that does not exist, is `404 not_found`; an order that was not sent
  with `handover: courier_code` is `409 conflict`.
- **The code is kept short.** It is read from the check when the event is read or sent: once the check is answered, or about 10
  minutes after the code was typed, `details.code` is `null`. A sender that reads the event later knows only that a code was typed.
- Without an answer the worker may type a new code; that is a new check, with a new `check_id`, and a new event.

### The label size of a warehouse

Each warehouse in `GET /warehouses` carries `label_sizes`: a list of `{ marketplace, width_mm, height_mm }`, one for each marketplace
whose label size the warehouse chooses, because its printer takes that size. The sender asks the marketplace for a label of exactly
that size. A marketplace with no entry has no size the warehouse chooses, and the marketplace's own default is used. A warehouse with
no choice returns an empty list.

### The label that comes afterwards

A marketplace issues its label after packing, so an order is created without one and the label follows:
`PUT /orders/{id}/parcels/{number}/label` with `{ content_type, data, scan_code? }` — `content_type` is `application/pdf`,
`image/png` or `image/jpeg`, `data` is the file in base64 (at most 5 MB). The file is checked by its bytes; an empty file, a file
too large or of another type is `400` with the issue path `data`. `scan_code` is the text a scanner reads from the label, when the
sender knows it: then the parcel can be closed only by scanning exactly that code. A label cannot be sent once the order is handed
over (`409 handed_over`) or cancelled (`409 conflict`). `DELETE` removes the label while the parcel is open.

### Changing an order

- **Cancel** — `200 cancelled`: the order is cancelled and the goods are free again. `202 waiting`: a picker is walking the shelves
  with the order; the fulfilment centre repeats the cancel by itself. `202 needs_person`: a parcel is already closed and only a
  person at the fulfilment centre can cancel it. `409 handed_over`: too late. Cancelling a cancelled order succeeds.
- **Reduce** — `200 reduced`, `202 waiting`, `202 needs_person`, `409 handed_over` as above. A number above the current one is `400`
  with the issue code `above_current`; an unknown line is `400` with `unknown_line`. Bringing every line to zero is a cancel.
- **Deadline and note** — `PATCH` `{ ship_by?, packing_note? }`; `409 handed_over` after the hand-over.
- **Problem** — `PUT …/problem` `{ code, message }` (the message up to 300 characters); `DELETE` clears it. An order has one
  problem. The fulfilment centre shows it to its people; only the sender clears it. Allowed at any stage.

Every successful change answers with `state` (`cancelled`, `reduced` or `updated`) and the order as it is now, or `202` with `state`
`waiting` or `needs_person`.

### The order object

`id`, `number` (the fulfilment centre's own number), `external_id`, `group_id`, `marketplace` (a code or `null`), `via` (how the
order came: `api`, `cabinet`, `file`, `office`, `marketplace`, `shop`), `warehouse`, `status` (`accepted`, `waitingStock`,
`reserved`, `picking`, `packing`, `packed`, `shipped`, `cancelled`), `warehouse_status` (twelve detailed values), `ship_by`,
`packing_note`, `test`, `handover`, `lines[]` (`line`, `article`, `quantity`, `cancelled_quantity`, `packed_quantity`, `marking_required`), `parcels[]`
(`number`, `closed_at`, `handed_over_at`, `label_attached`, `units[]` — each unit's line and marking code), `problem`,
`handover_id`, `sequence`, `updated_at`. There is no buyer data.

Whether a line needs marking codes is decided by the fulfilment centre's product card; the order object only tells it in
`marking_required`, so the sender can see a disagreement with its marketplace and report a problem.

## Events

The events the fulfilment centre reports:

| Event                        | When                                                                                        |
| ---------------------------- | ------------------------------------------------------------------------------------------- |
| `order.accepted`             | The order is created: at once or from the waiting list.                                     |
| `order.held`                 | The order is received and waits.                                                            |
| `order.reserved`             | The goods are reserved.                                                                     |
| `order.picking_started`      | The order is released to picking.                                                           |
| `order.picked`               | The pick is complete.                                                                       |
| `order.packing_finished`     | **The contents are final**; carries each unit's marking code.                               |
| `order.label_attached`       | A label is attached to a parcel (`details.parcel`).                                         |
| `order.parcel_closed`        | A parcel is closed (`details.parcel`).                                                      |
| `order.packed`               | The last parcel is closed.                                                                  |
| `order.reopened`             | A parcel is reopened or packing restarted: the codes sent before no longer hold.            |
| `order.reduced`              | Lines were cut.                                                                             |
| `order.change_waits`         | A cancel or a cut waits for a person (`details.field`, `details.why`).                      |
| `order.cancelled`            | The order is cancelled (`details.by`: `outside_system` or `warehouse`).                     |
| `order.handed_over`          | The last parcel left.                                                                       |
| `order.courier_code_entered` | A courier named a code for an order that leaves alone (`details.check_id`, `details.code`). |
| `handover.requested`         | The fulfilment centre asks for the hand-over's documents.                                   |
| `handover.request_withdrawn` | It withdrew the request.                                                                    |
| `handover.completed`         | Every parcel of the hand-over left.                                                         |
| `ping`                       | A test notification, sent on request.                                                       |

An event is `{ id, type, occurred_at, sequence, data }`, where `data.order` (or `data.held`, `data.handover`) is the object as it is
**when the event is read or sent**, and `data.details` holds the event's small facts. A receiver discards an event whose `sequence`
is lower than one it already applied. Events of an order that did not come through the API are reported too: the sender may follow
every order of the seller. An event never carries a buyer's data.

The codes of the units are sent in `order.packing_finished`; the sender reports them to its marketplace. When packing restarts or a
parcel is reopened, `order.reopened` says the earlier codes are void and the next `order.packing_finished` carries the new ones.

### Reading the events

`GET /api/v1/events?after=<number>&types=a,b&limit=` returns the events after the number, oldest first. `next_after` is the last
number of the page when more events wait behind it, otherwise `null`. A reader keeps the last number it applied and asks again from
there. Events are kept 30 days. A number is given when the event is first read or sent, in the order the events were saved, so every
new number is higher than every number already handed out.

### Receiving notifications

The same events can be received as **signed notifications**: a `POST` with the event as the body to an address the seller gave
(`https://` only). They follow the Standard Webhooks scheme
(<https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md>):

- Headers: `webhook-id` (the event's id; the same on every attempt), `webhook-timestamp` (seconds since 1970-01-01 UTC, of this
  attempt) and `webhook-signature` (one or more `v1,<signature>` separated by a space).
- The signed text is `<webhook-id>.<webhook-timestamp>.<body>` exactly as sent; the signature is the base64 of its HMAC-SHA256.
- The secret is `whsec_` followed by the base64 of 32 random bytes. **The decoded bytes**, not the text, are the HMAC key. It is
  shown once, when the address is added.
- A receiver checks the signature **and** that the timestamp is within five minutes of its own clock, and uses `webhook-id` to
  recognise a notification it has already processed.
- Any `2xx` answer is success. A redirect, any other answer, or no answer within 10 seconds is a failure; the notification is sent
  again. There are 8 attempts in all — at once, then after 1 minute, 5 minutes, 30 minutes, 2 hours, 5 hours, 10 hours and 10
  hours (about 27 hours), each pause spread by 20 % either way at random so that a receiver that was down does not see every delayed
  notification return in the same second. The notifications of one address are sent one at a time in the events' order.
- An address that has delivered nothing for 3 days is switched off by the fulfilment centre; the events of the last 30 days can
  still be read with `GET /events`.

## Hand-overs and their documents

A marketplace gives the documents of a hand-over (the code the receiver scans, a label per transport box, an act) only after its own
supply is closed, and a closed supply takes no more orders. So the **fulfilment centre asks** for them when the parcels are ready.

- `GET /api/v1/handovers` and `GET /api/v1/handovers/{id}`: `id`, `number`, `warehouse`, `marketplace`, `planned_date`, `state`
  (`open`, `documents_requested`, `documents_ready`, `completed`), `transport_boxes`, `external_id`, `parcels[]` (`order_id`,
  `external_id`, `number`, `scan_code`), `documents[]` (`kind`, `title`; no file), `problem`.
- **The request** is the event `handover.requested`. From then on the hand-over takes no more parcels: a parcel closed later goes to
  the next day's hand-over. The request can be withdrawn (`handover.request_withdrawn`) until the documents arrive.
- **The answer** is `PUT /handovers/{id}/documents` with `{ external_id, documents: [{ kind, title, content_type, data, scan_code? }] }`.
  `kind` is `handover_code` (the code the receiver scans), `box_label`, `act` or `other`; PDF, PNG or JPEG, each at most 5 MB, at most
  60 files, the body at most 8 MB. A repeat replaces the earlier set. `409 conflict` when the documents were not asked for, or the
  hand-over is completed.
- **A problem** — `PUT /handovers/{id}/problem` `{ code, message }` (or `DELETE`): the sender could not close its side. The request
  can be made again once the problem is cleared.
- The hand-over itself is done by the fulfilment centre: each parcel is scanned, the hand-over is closed, and `handover.completed`
  is reported.

## Who installs the sender, and who holds the marketplace's key

The sender is installed by the seller, on a server the seller owns or rents, and the seller alone types the marketplaces' keys into it.
**A fulfilment centre's staff never install the sender for a seller and never hold a seller's marketplace key**, and this API has no
place for one: an order carries the marketplace's code, never its key, and the fulfilment centre's key is the only key the sender
sends here.

The reason is the marketplaces' own rule. The first marketplace the connector supports says it in its knowledge-base article
«Облачный сервис или локальное решение: как отличить и какой токен выбрать» (updated 16.09.2026): «Персональный токен нельзя
передавать третьим лицам и использовать с облачными сервисами.» Its personal token is meant «только вашим собственным программам — в
том числе корпоративным системам (on-premise), развёрнутым на собственной или арендованной инфраструктуре». A sender installed and run
by the fulfilment centre would be a third party holding the seller's key.

## What is not here yet

Products, stock, announced deliveries, returns, delivery of the seller's own-shop orders by delivery services, and supplies to
marketplace warehouses belong to later stages of the connector's plan (the high-level plan,
`.agents/plans/Marketplace_connector_plan.md`, "Stages 1–6"); the stages that use them add them to `api/fulfilment-centre.yaml`. The
`delivery` object of an order, which belongs to the delivery-service part, is left out until then; a reader tolerates fields it does
not know.
