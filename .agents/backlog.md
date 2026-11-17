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

### 9. A policy test for rule R7 (free and open) (2026-11-17)

Found by Plan 1 while writing the constitution: every other rule R1 to R6 names the test that enforces it, but rule R7 (Apache-2.0, no paid
edition, no licence key) has only the file `LICENSE`. "A rule is not finished until a policy test enforces it" (constitution, principle IV).
To build: a policy test that reads `package.json`, `go.mod`'s module files and any other manifest and fails when one names a licence other
than Apache-2.0, or when a source file asks for, checks or sells a licence key. It belongs to the first plan that adds a manifest beyond the
ones Plan 1 creates, or to Plan 2 (the release), whichever comes first. The maintainer decides when.

### 8. The components the library does not hold yet (2026-11-17)

The maintainer, 2026-11-17: «I want you to reuse as much functionality as possible from [the other project] as well, as some things probably
are already implemented [there].» and «When I work with you here in this project, I want the same feel as if I work in [the other project].»
Plan 1 copies the components every screen needs (`docs/library-layout.md` lists them). **Copied by the plan that first needs one, with its
test, never rewritten:**

- the searchable picker `Combobox` (the only user of the package `bits-ui`), `FilePicker`, `CodeNameSelect`, `DateRangeFields`,
  `TickAllBox`, `FileFormatButtons`, `StepsCard`, `RowButtonForm`, `ReasonDialog`;
- the list component `DataList` with `ColumnHeading` and `ColumnLegend` — Plan 3 (the order list);
- the shell parts (the header, the menu), the sign-in parts and the notification bell — Plan 2.

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

Nothing is closed yet.
