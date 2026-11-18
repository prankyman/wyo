# The maintainer's requests and open decisions — the register every plan reads

This file exists because a request the maintainer makes while testing one plan must not be lost when it belongs to a later one. Rules
(also in `AGENTS.md`, "The register"):

- **Before writing any plan**, read this file. Every open item whose area the plan touches goes into the plan — or the plan says, in its own
  "Not in this plan" section, why it stays open and names the item here.
- **When the maintainer asks for something that is not built in the current plan**, add it here in the same session, with the date and his
  words, before the session ends.
- **An item is closed only** when the plan that built it is merged, or when the maintainer says it is dropped. Closing moves it to "Closed"
  with the plan and commit that closed it. Never delete an entry.

## Open

### 33. The fulfilment-centre client ignores the neutral API's `Retry-After` (2026-11-18)

Found by Plan 5 (`.agents/plans/wyo-ozon-key-reading-adapter-orders-with-several-products-fake-ozon.md`) when every adapter came to
name its own limit headers: the neutral API answers `429 rate_limited` with `Retry-After` (`api/fulfilment-centre.yaml`), but the
fulfilment-centre client's limit group names no header, so after a 429 the caller waits one second, doubling, as it always did. Plan 5
kept that behaviour unchanged. To decide: name `Retry-After` in the group of `internal/fulfilment/endpoint.go`, with its test.

### 32. Ozon's notifications to a public address (2026-11-18)

Left out of Plan 5: Ozon's «пуш-уведомления». Ozon itself says the notice of a new order may come late and the lists must be read
regularly (`#tag/push_types` of `api/ozon/seller-api.json`); wyo reads the lists. To be taken up if the reading proves too slow.

### 31. Ozon's FBO orders older than 30 days (2026-11-18)

Left out of Plan 5: the FBO list is read for the last 30 days at each reading, because what its `since` compares is not sourced; an
older FBO order keeps the last state read. Revisited after the check of `since` on a real account (item 30). Plan 5 also did not build the sentence in an
older FBO order's details that says its state is no longer read (plan section B): the adapter's window is its own, and the wyo
interface has no way yet to know it; to be built with that revisit.

### 30. The checks of Plan 5 on the first real Ozon account (2026-11-18)

Plan 5 is built against the fake Ozon only; Ozon has no sandbox. Only reading calls are made, so the checks are safe on a working shop
(`.agents/research/ozon-official-seller-api-check-2026-11-18.md`, section 10, points 1, 2, 4, 5): the format of `/v1/roles`' list of
methods and which role opens each method Plan 5 calls; the HTTP status of each key error; what `since` and `to` of the FBO list
compare (then the FBO window may become a mark); whether the unprocessed list holds a posting the buyer cancelled before packing; how a
split made in the cabinet shows (the parts' numbers, the original's status); the real limit headers and the budget's agreement with
them. What they find becomes Plan 6's premises.

### 29. Stock (2026-11-18)

Register item 1's second half, left out of Plan 5: what lies in each marketplace's warehouses and the fulfilment centre's free stock
sent to every marketplace. A plan of its own after Яндекс Маркет, with the stock part of the neutral fulfilment-centre API first.

### 27. Several fulfilment centres at once (2026-11-18)

Left out of Plan 4 (`.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`,
«Not in this plan»): «Внешний API» holds one fulfilment centre, because the high-level plan names one setting. To be taken up if a
seller works with two.

### 26. Packing without a fulfilment centre (2026-11-18)

Left out of Plan 4: printing the marketplace's stickers from wyo itself for a seller who packs alone. The high-level plan
(`.agents/plans/Marketplace_connector_plan.md`, «Not in this plan») leaves it out; to be taken up if sellers ask.

### 25. What the owner still does in the Wildberries cabinet (2026-11-18)

Left out of Plan 4: cancelling an order at Wildberries from wyo (the wyo server never cancels: a cancellation by the seller is fined —
an architecture test forbids the call), moving an order back from a closed supply, orders Wildberries wants shipped again, passes to
Wildberries' warehouses, days off. The README (section 11) names each; to be taken up only on the maintainer's request.

### 24. Identifiers other than marking codes (2026-11-18)

Left out of Plan 4: a jewellery item's number (УИН), a phone's IMEI, the goods' GTIN, an expiry date and a customs declaration, which
Wildberries may require for an order. The order gets a problem that names the identifier and says it must be entered in the
Wildberries cabinet. A later plan sends them, once the neutral fulfilment-centre API carries them.

### 23. The checks of Plan 4 on the first real Wildberries account and the first real fulfilment centre (2026-11-18)

Plan 4 is built against the two fakes only. To check (the plan's «After the release»): a real order printed at 58 × 40 mm from the
sticker the wyo server attached, and the text its code gives to a scanner (then the scan text can be sent with the label); whether a
sticker changes when its order moves between two open supplies (the cautious choice: the labels are read again after the move and
compared); whether a second sending of codes replaces the first (the cautious choice: the earlier ones are removed first); how long a
verdict stays unfinished; which supplies Wildberries refuses without a transport box; the shipping points' list and the closing of a
supply on a real account (the sandbox has neither, so a test token is not capable of the hand-over's parts); the time between a
hand-over's request and its documents; notifications received at a real address behind a real certificate. Register item 5's points 1
to 6 and 8 are the same checks. What they find becomes the next plan's premises.

### 22. A fulfilment centre that names transport boxes (2026-11-18)

Found while implementing Plan 4: the fake fulfilment centre never names a number of transport boxes in a hand-over (`transport_boxes`
is empty), so the step «Добавление коробов» is proven through the hand-over's runner with a recorded hand-over, not through the fake's
own `handover.requested`. Likewise the fake Wildberries cannot refuse putting an order into a supply, or closing a supply for missing
shipping, on command: the exchange's tests wrap it with scripted refusals in Wildberries' own error shape
(`internal/exchange/rig_test.go`, `refuse`). To decide: a control that names boxes and a fault kind «refuse with this code», in the
plan that next touches the fakes.

Half done by Plan 5 (2026-11-18): the fake kit has the fault kind `refuse` (a given status and body, a given number of times,
`internal/fakes/fakekit/faults.go`), used by the fake Ozon's and the Ozon adapter's tests; the boxes named by the fake fulfilment centre
and the end of the exchange tests' own scripted refusals are Plan 6's.

### 21. The phone's menu strip cuts long labels (2026-11-18)

Found by the smoke test of Plan 3: at 390 × 844 the strip at the bottom holds five items and «Ещё», so «Маркетплейсы» and «Журнал
запросов» are drawn cut («Маркетп…», «Журнал з…»). It is the library's rule as copied from the maintainer's other repository (five
items, a label cut to its width), so Plan 3 did not change it. To decide: a short label of its own for the strip (a key such as
`nav.journal.short`), fewer items in the strip, or two lines of small letters.

### 20. Wildberries' product cards, prices of orders, old orders (2026-11-18)

Widened by Plan 5 (2026-11-18): Ozon's product cards and prices; an Ozon order's lines show the name and the barcodes, the cards are the
contract part `products.read` of a later stage.

Left out of Plan 3 (`.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`, «Not in this
plan»): the order list shows the product's kind, brand and size the orders feed gives — photographs and full names need reading the
product cards (the contract part `products.read`, a later stage); the prices of orders are read with the money (Stage 3 of the
high-level plan); orders older than 90 days and Wildberries' archive are not read, because the feed keeps 90 days and history before the
installation was not asked for.

### 19. Orders the seller delivers to the buyer itself; tokens «Базовый» and «Сервисный» (2026-11-18)

Widened by Plan 5 (2026-11-18): Ozon's rFBS warehouses (`is_rfbs`) are stored with the mark «доставляет продавец» and their orders are
never read, for the same reason; the account's card counts them.

Left out of Plan 3: Wildberries' seller-warehouse schemes where the seller delivers to the buyer need the buyer's address, which wyo does
not store (rule R4 and the plan's «no buyer data»). A token of the type «Базовый» is refused, because its limits are too low for this work
(the orders feed once in three hours), and «Сервисный» is refused, because it belongs to a cloud service of Wildberries' catalogue. To be
taken up if sellers ask.

### 18. The checks of Plan 3 on the first real Wildberries account (2026-11-18)

Plan 3 is built against the fake Wildberries only (the maintainer, 2026-11-17: «Right now you'll have to do all work pretty much blindly»).
With a test token first (the sandbox), then with a Personal token (Plan 3, section I): that the token's fields are as Wildberries'
description says; the two hosts' `/ping`; the real limit headers and the budget's agreement with them; whether a new cabinet answers 403
until seller-warehouse sales are switched on, and in which words (the wyo server guesses the code `seller_warehouse_off`); whether a token
with the access level «только чтение» reads the orders (the methods carry Wildberries' mark `x-readonly-method`, which its documents do
not explain; until then the instructions ask for «чтение и запись»); whether «В доставке» is the right state for an FBO order the feed
does not mark cancelled (the feed has no status words — a choice of Plan 3, not a sourced fact); how soon a feed row follows its order;
and item 5's points 1 and 3. What they find becomes Plan 4's premises.

### 17. A help «?» button carries the same name as the button it explains (2026-11-18)

Found by the smoke test of Plan 2: `ButtonWithHelp` gives its «?» button the accessible name of the button beside it (for example both
are «Сделать копию сейчас»), so a screen reader hears two buttons with one name, and a browser test that looks a button up by its name
finds two. It is the library's convention as copied from the maintainer's other repository, so Plan 2 did not change it. To decide: a
name of its own for every «?» (for example «Что делает «Сделать копию сейчас»»), changed in `ButtonWithHelp` once, with its test.

### 16. The maintainer's checks of Plan 2 on a real server (2026-11-18)

Plan 2 (`.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`, «After
the release») is proven on GitHub's clean Ubuntu machines only, with a test certificate authority. The maintainer, 2026-11-17: «If the GH
Actions passes fine for you - not a problem!» — and checks a real server himself afterwards: a server of 1 core and 1 GB with Debian 12
or 13 (the pipeline has only Ubuntu) and a real domain; the installer from the README; a real Let's Encrypt certificate; the memory after
a day; an update from this release to the next one; a copy restored on a second server. What the checks find becomes the next plan's
premises.

### 15. Systems without systemd or OpenSSL 3, a Docker image, Windows (2026-11-18)

Left out of Plan 2: the installer refuses such a system and says why. To be taken up if sellers ask.

### 14. A second sign-in step and a list of allowed addresses (2026-11-18)

Left out of Plan 2: a code from a phone as a second step of signing in, and a list of addresses the wyo interface may be opened from.
Later, if asked.

### 13. A server without a domain name (2026-11-18)

Left out of Plan 2: a certificate for a bare address. The maintainer's request names a domain («asks the domain name»).

### 12. E-mail from wyo (2026-11-18)

Left out of Plan 2: a forgotten password by mail and notices by mail. A seller's server has no mail set up; a forgotten password is set
again on the server (`sudo wyo owner reset-password`) and notices live behind the bell.

### 11. An offline page and installing the wyo interface as an application on a phone (2026-11-18)

Left out of Plan 2: not asked for.

### 10. Postgres (2026-11-18)

The high-level plan: "not offered in the first stages". The database is SQLite in one file (`.specify/memory/constitution.md`,
principle VIII). To be taken up when a seller's data outgrows it.

### 9. A policy test for rule R7 (free and open) (2026-11-17)

Found by Plan 1 while writing the constitution: every other rule R1 to R6 names the test that enforces it, but rule R7 (Apache-2.0, no paid
edition, no licence key) has only the file `LICENSE`. "A rule is not finished until a policy test enforces it" (constitution, principle IV).
To build: a policy test that reads `package.json`, `go.mod`'s module files and any other manifest and fails when one names a licence other
than Apache-2.0, or when a source file asks for, checks or sells a licence key. It belongs to the first plan that adds a manifest beyond the
ones Plan 1 creates, or to Plan 2 (the release), whichever comes first. The maintainer decides when. Plan 2 (2026-11-18) added no
manifest and no feature switch, and its text does not name this test, so the item stays open.

### 8. The components the library does not hold yet (2026-11-17)

The maintainer, 2026-11-17: «I want you to reuse as much functionality as possible from [the other project] as well, as some things probably
are already implemented [there].» and «When I work with you here in this project, I want the same feel as if I work in [the other project].»
Plan 1 copies the components every screen needs (`docs/library-layout.md` lists them). **Copied by the plan that first needs one, with its
test, never rewritten:**

- the searchable picker `Combobox` (the only user of the package `bits-ui`), `FilePicker`, `CodeNameSelect`, `FileFormatButtons`,
  `StepsCard`, `RowButtonForm`, `ReasonDialog`;
- ~~the list component `DataList` with `ColumnHeading` and `ColumnLegend`, `TickAllBox`, `DateRangeFields`~~ — copied by Plan 3 on
  2026-11-18 (`docs/library-layout.md`);
- ~~the shell parts (the header, the menu), the sign-in parts and the notification bell — Plan 2~~ — copied by Plan 2 on 2026-11-18
  (`docs/library-layout.md`); the rest of this list stays open.

### 7. More people who sign in (2026-11-17)

The first version has one owner who signs in. The high-level plan (`.agents/plans/Marketplace_connector_plan.md`, «What it is built of»)
describes "more people added by the owner". The maintainer, 2026-11-17: «What more people? … Is this a sought-after functionality?» Left out
until sellers ask for it. To decide when asked: who the people are, what each may see and do, and how an owner adds and removes them.

### 6. The trade-mark check of the name wyo (2026-11-17)

The maintainer, 2026-11-17: «Let's forget about the trademark, so far.» A web search on 2026-11-17 found no software product named «WYO».
Set aside; to be taken up if the maintainer asks, before the first public announcement.

### 5. The checks of the first real Wildberries account or test token (2026-11-17)

Nine facts that no official description file settles (`.agents/research/wildberries-official-description-files-check-2026-11-17.md`,
section 7). The maintainer holds no Wildberries key yet («Right now you'll have to do all work pretty much blindly»); every plan is built
against the fake Wildberries, and these are checked on the first real account or on Wildberries' test token, which works against its sandbox
with generated data:

1. Does `GET /api/v3/orders/new` still list an order the buyer cancelled within the first hour?
2. Does `PUT …/meta/sgtin` replace the codes sent before or add to them?
3. Which `wbStatus` a buyer's cancellation shows after the order is on assembly.
4. Which text the order sticker's code gives to a scanner (the field `barcode` of the sticker's answer is the likely one).
5. Whether a sticker stays valid when its order is moved between two open supplies.
6. Which supplies are refused with `FulfillmentSupplyBoxesRequired` when they have no transport boxes.
7. Which of the two limits a test token really meets in the sandbox: 150 requests a minute or 1 request a second.
8. How long the verdict `pending` of a marking code lasts in practice.
9. Whether the sandbox closes a supply without shipping parameters (the shipping methods exist only on the production host).

What a check finds becomes the next plan's premises; a fact is written only from the official documents or from the check, with the date.

### 4. Stage 6 — Мегамаркет; Авито and Лемана ПРО on demand (2026-11-17)

`.agents/plans/Marketplace_connector_plan.md`, «The marketplaces, in order» and «Stages 1–6». Мегамаркет after its documents are read
for the FBS steps (`docs/marketplaces/` holds its API descriptions). Авито and Лемана ПРО last, and only on demand: no ready client
exists, and Лемана ПРО issues keys only after an application; the advice given to the maintainer is to leave both out of the first
stages. A plan of its own, written when Stage 5 is released.

### 3. Stage 5 — supplies to marketplace warehouses through the fulfilment centre (2026-11-17)

A supply request sent, the packed boxes' contents received, the marketplace's box file and labels produced. Needs the supply parts of the
neutral fulfilment-centre API (`docs/fulfilment-centre-api.md`), which Plan 1 does not write. A plan of its own, written when Stage 4 is
released.

### 2. Stages 3 and 4 — sales and money; planning (2026-11-17)

Stage 3: sales and returns, the marketplaces' commissions, logistics and advertising spend read in; the cost price typed per product; profit
per product and per marketplace; the fulfilment centre's charges where its API gives them. Stage 4: how many days each product's stock will
last at the present speed of sales, what to buy and when, a purchase list as a file. A plan of its own for each, written when the stage
before it is released. Left out on purpose (other products do them, each adds a risky write to a marketplace): automatic price changes,
advertising bid management, automatic answers to reviews, competitor tracking.

### 1. Stage 2 — Ozon and Яндекс Маркет; stock (2026-11-17)

Keys, the order list and the same exchange with a fulfilment centre for Ozon and Яндекс Маркет. Stock: what lies in each marketplace's
warehouses, and the fulfilment centre's free stock sent to every marketplace that sells from it, so a sale on one lowers what the others
offer. Needs the stock part of the neutral fulfilment-centre API, which Plan 1 does not write. A plan of its own, written when Stage 1
(Plans 2, 3 and 4) is released.

## Closed

### 28. The new orders are rewritten every minute while «Внешний API» is filled (2026-11-18)

Found by Plan 4's budget test: with the exchange on, the engine reads the new orders every minute and writes every order that is still
new at the marketplace again, and every reading of the states writes every open order again. Correct, but slow with many open orders
(about 1.5 ms a query under the race detector), so the test of a whole hour runs with 50 orders and the test of 500 orders runs for its
six busiest minutes. To decide: write an order only when something of it changed.

Closed by Plan 5 (`.agents/plans/wyo-ozon-key-reading-adapter-orders-with-several-products-fake-ozon.md`, section C), 2026-11-18: a
reading compares what it read with the stored order (its state, its deadline, its lines) and writes nothing when they are equal
(`TestAReadingThatChangedNothingWritesNothingButItsRecordOfTheReading`, `internal/sync`).

