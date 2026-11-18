# Plan 6 — the whole Ozon FBS exchange through the neutral fulfilment-centre API: report

**DONE.** Released as **0.0.7** (`https://github.com/prankyman/wyo/releases/tag/v0.0.7`) on 2026-11-18. Merged as `4fd626c` (pull request #10, `https://github.com/prankyman/wyo/pull/10`, one
commit `4813baa`); the release commit is `9107a15`.

- The plan: `.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md`.
- The check of rule R1 (every word of the list the maintainer keeps outside the repository, over every tracked and new file and the
  commit message) ran before every push and found nothing.
- Nothing is left to build in this plan. What is not proven — everything that needs a real Ozon account and a real fulfilment centre —
  is in register item 34; the places where the code does less than the plan says are in register item 35; what the smoke test found
  and did not belong to this plan is in item 37.

## What was built

1. **The writing half of the adapter contract is neutral** (plan section A). The exchange chooses its flow by the parts an adapter
   claims (`orders.confirm` — Wildberries; `orders.pack` — Ozon). Labels may come later (ask, get a ticket, read the ticket); codes may
   replace the ones sent before and carry the product each lies on; a batch can be made to hold exactly the hand-over's orders; the
   adapter declares which of the six hand-over steps it needs, what its documents are, and how often its new orders and states are
   read (`Description.HandoverSteps`, `CodesReplace`, `OrdersPeriod`, `StatesPeriod`; `internal/marketplace/writing.go`). One function
   names a batch (`AssemblyBatchName`, `HandoverBatchName`), where two copies were. The contract suite's label and document cases no
   longer demand a PNG and a scan text of every adapter: that was a Wildberries-only fact written into the shared suite.
2. **The Ozon adapter's writing parts** (`internal/marketplace/ozon`): marking codes sent on their own exemplars and their verdict; the
   packing report with the fulfilment centre's parcels (several parcels split the posting); a label for each part, fetched only from
   `https` and a host under `ozone.ru`, without the key (a route that carries `NoKey`), at most 5 MB, only a file that begins as a PDF;
   the day's shipment found or made, rewritten to exactly the hand-over's postings and read back, approved, and its documents (the
   shipping list, the hand-over barcode with its text). A key that cannot write is marked read-only with the level to add (`Posting FBS`).
3. **Parcels** (migration `0005_parcels_ozon_exchange`): each order's parcels with the marketplace's number of each part, the label's
   ticket and fingerprint; the label of every existing order became its parcel 1.
4. **The second flow of the exchange** (`internal/exchange/pack.go`, `codes.go`, `deadline.go`; `internal/handover`): nothing is
   written at the reservation; after packing is finished the codes are sent, their verdict waited for, the packing reported and read
   until it took effect, and the labels asked for and attached for each parcel. A changed deadline is passed on to the fulfilment
   centre. The hand-over runs only the steps its adapter declares and tries again when the marketplace has not made a shipment or
   its documents yet.
5. **The fakes**: the fake Ozon's writing routes and controls, the fake fulfilment centre's parcels and transport boxes, and the kit's
   fault «refuse with this answer» for both marketplaces (register item 22, closed).
6. **The interface**: `ParcelList` in the order's details, the declared steps in «Передачи» with a note about transport boxes,
   automatic assembly in the warehouses' list, the new problems and history lines in words, and a line for each account whose
   marketplace has no hand-over settings.
7. **README, `docs/ozon.md`, `docs/external-api.md`, constitution 1.5.0** (principles XII and IX widened), and the register.

## The gate

| Where                                         | What                                                                                                                               | Result                                                      |
| --------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| Development machine (limited: 4 cores, 15 GB) | Every touched Go package in both builds, the whole Go lint, `pnpm generate:check`, the whole interface unit suite, 9 browser specs | green; no whole-project type check (the machine's rule)     |
| GitHub Actions, `ubuntu-24.04` (gate)         | Secret scan, commit message, generate, lint, type checks, tests with coverage, build, browser tests                                | 27 min 20 s (run 44416635610)                               |
| Gate — interface                              | Unit tests with coverage                                                                                                           | 218 files; 96.08 % aggregate                                |
| Gate — Go                                     | Tests with coverage, both builds                                                                                                   | 92.5 % of 13 110 statements in 199 files, every file ≥ 84 % |
| Gate — browser                                | Browser tests against the built wyo server and the fakes                                                                           | 112 tests, 54.4 s                                           |
| Installer job, three clean machines           | `ubuntu-24.04`, `ubuntu-22.04`, `ubuntu-24.04-arm`                                                                                 | green                                                       |

**The pull request needed four runs.** Run 1: the secret scan flagged a made-up token in a test (`internal/sync/caller_nokey_test.go`);
it is now allowed by its exact value in `.gitleaks.toml` with its reason, as the file's own rule says. Run 2: three files under the
per-file coverage floor (`internal/exchange/deadline.go` 57.9 %, `internal/exchange/pack.go` 81.7 %, `internal/marketplace/ozon/pack.go`
81.8 %): the branches that met a refused or an unanswered call got tests. Run 3: one browser test saw a phone-width finding that the
local font did not show: the option «Не передавать заказы этого аккаунта» needed 283.8 px in a 270 px select; the text is now «Не
передавать заказы». Run 4: green.

## Bugs the tests and the smoke test found, and fixed

- The query that lists the orders to send used `LIMIT sqlc.arg(batch)`, which sqlc numbered `?2`; with two marketplaces named the
  limit became the second marketplace's name and **nothing was sent** (found by the canary test with Wildberries and Ozon together).
- Marking codes lost their product (`unitsOf` did not fill the order line), so Ozon was told a code was missing.
- «Передать заново» did nothing for the packing report and the labels of an Ozon order.
- A report that failed three times was read again for ever, and the fulfilment centre's problem was sent every tick.
- A posting packed in the cabinet, or by automatic assembly, got stuck on the codes.
- The label step compared its parcels with the real clock instead of the moment it was asked for, so «run everything now» never
  finished the labels (found by the first browser spec).
- The fulfilment-centre client had no route for `PATCH /orders/{id}`: the caller refused every deadline change.
- A cancelled order was asked for its verdict every 5 minutes for ever.
- The history said «the fulfilment centre sent codes» for an order that had none; a reopened parcel and a changed deadline had no
  text; the journal named its jobs in English; an empty parcel number was drawn as nothing; a code equal to the words was said twice.

## Premises (Stage 0)

1. `nproc` 4 and 15 GB: limited, so focused local runs; the pull request's run is the first whole gate.
2. Ozon's file: unchanged since Plan 5 (`api/ozon/README.md`); the generator took all 17 writing methods, among them
   `/v2/posting/fbs/act/get-postings` whose `id` is an `int` (the existing corrections already turn it into `integer`). Ozon's file
   leaves `type: array` out in places, so some generated fields are `interface{}` and the adapter marshals and re-reads them.
3. No findings of Plan 5's checks with a real account existed, so the cautious choices stand (register item 34).
4. The neutral specification still equals the format the maintainer provides: not checked again outside this repository, because the
   specification was not changed.
5. A seller's key must be allowed to write: the key check asks for all 17 writing methods and marks the key read-only otherwise. The
   role that opens the shipment methods is **not sourced** (they are in none of Ozon's role-table rows): the level to add is a guess.

## Where the code does less than the plan says

Register item 35 holds each, with its reason: a shipment found and reused is cancelled on a withdrawal; the country of origin is not
asked for before packing; the documents are asked again every 20 seconds for 10 minutes (the plan: every 10 seconds for 5), and
labels at the 20-second beat of the job; the part numbers are lost when the answer of the report is lost.

## After the release

Register item 34: the checks with the first real Ozon account and the first real fulfilment centre. Nothing in this repository calls a
real marketplace or a real fulfilment centre (rule R3).
