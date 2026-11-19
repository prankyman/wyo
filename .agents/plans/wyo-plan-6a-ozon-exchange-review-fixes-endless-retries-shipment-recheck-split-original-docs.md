# Plan 6a — fixes from the final review of Plan 6: refused labels and packing reports no longer retried every 20 seconds for ever, the Ozon shipment re-checked before approval, a split posting's original never passed on as a cancellation, the documents corrected

> **Plan 6a** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`).
> It fixes what the final diff review of Plan 6 found
> (`.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md`, merged as `4fd626c`, commit
> `4813baa`, released as 0.0.7; its report `.agents/reports/wyo-ozon-fbs-exchange-plan-6-report.md`). Written 2026-11-19 from `main` at
> `b88b903`. It sits after Plan 6 and before the next stage, hence «6a».
>
> **Branch** `fix/ozon-exchange-review-fixes`. One plan = one branch = one squashed commit = one pull request. The commit is `fix:` —
> never `fix!:` and never a `BREAKING CHANGE:` footer. **No database migration.** No change to the constitution: every fix brings the
> code back to principle XII as written (`.specify/memory/constitution.md`, «XII. The exchange with a fulfilment centre»).

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most 3
subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Findings fixed here (each confirmed by reading the code at `b88b903`)

### 1. A refused label is asked for, or read, every 20 seconds for ever — BUG

`internal/exchange/pack.go`, `labelStep` / `askLabel` / `readLabel` (lines ~532–670). When Ozon refuses the label request, the label
task ends `error`, a part is among `unprinted_postings`, the label is missing, or the fulfilment centre refuses the label file, the
order gets a problem — but the parcel's next reading time (`exchange_parcels.label_next_at`) stays in the past. The query
`ListDueLabelChecks` (`internal/store/queries/exchange.sql:275`) does not skip orders with a problem, and the job runs every
20 seconds. **Effect:** Ozon's `/v3/posting/fbs/package-label/create` (a write) is called again every 20 seconds after a refusal, and
in every case the fulfilment centre gets the same problem again (`PUT /api/v1/orders/{id}/problem`) and the order's history gets a new
«problem» row every 20 seconds. Principle XII: «Never a blind write».

**Fix:** every path in the label step that records a problem also clears that parcel's next reading time and saves the parcel; the
labels are asked for again only by «Передать заново» (`resetLabels`, already sets it to now) or a new fulfilment-centre event.

### 2. A refused packing report is sent again every 20 seconds for ever — BUG

`internal/exchange/pack.go`, `packStep` / `report` (lines ~248–398). The next reading of the packing
(`exchange_orders.pack_next_at`) is cleared on three problem paths but not on four others: Ozon refusing the reading of the posting
(`PackState`), the fulfilment centre refusing to give its order (`fcProblem`), the parcels' contents differing from Ozon's lines
(`pack_contents`), and Ozon refusing the report itself (`Pack`). `ListDuePackChecks` (`exchange.sql:146`) does not skip orders with a
problem. **Scenario:** the first report gets no answer (network) → retried 20 s later → Ozon refuses it with
`HAS_INCORRECT_PRODUCT_QUANTITY` → problem recorded, `pack_next_at` still in the past → every 20 seconds the posting is read and
**`/v4/posting/fbs/ship` is sent again**, plus a new problem to the fulfilment centre and a new history row each time.

**Fix:** the same as point 1 — every problem path of the packing step clears `pack_next_at`. «Передать заново» already plans the
report again (`internal/exchange/runnow.go`); its test is extended to prove it after each of these four refusals.

### 3. The Ozon shipment is approved without re-reading what it holds — BUG against principle XII

`internal/handover/steps.go`, `close` (~line 315) reads the batch (`state.Orders` is filled by `internal/marketplace/ozon/handover.go`,
`Batch`) and ignores its list; `CloseBatch` approves. The only comparison is the read-back at the end of the step «move»
(`FillBatch`). Ozon puts every ready posting into a shipment of status `new` (OZC §9), so a posting that becomes ready between «move» and
«close» (or a hand-over resumed after a restart) is approved with the shipment. Principle XII: «a batch that holds another order is never
closed».

**Fix:** before closing, the runner compares the batch's orders with the hand-over's orders both ways, with **the same one function** the
read-back of «move» uses (moved to `internal/marketplace` or `internal/handover` if it now lives only in the Ozon adapter — one function,
never a copy); a difference does not close, it re-runs «move» once (rewriting the list) and compares again; a second difference is the
existing problem `batch_differs` naming the postings. Wildberries' step «close» keeps its behaviour (its supply holds only what wyo put
into it; the comparison finds no difference).

### 4. After wyo's own split, the original posting may be read as cancelled and passed on as a cancellation — RISK (not sourced)

`internal/marketplace/ozon/pack.go:179-181` (`PackState`) counts `cancelled_from_split_pending` («отменён из-за разделения
отправления», OZC §6) as a cancellation. After a report with several parcels, `internal/exchange/pack.go` (`packStep`,
`settledAtMarketplace`) still reads **the original number**. If real Ozon shows the original as `cancelled_from_split_pending` after the
split, the wyo server **cancels the order at the fulfilment centre although the packing succeeded**. The fake Ozon
(`internal/fakes/ozon/ship.go`) keeps the original as part 1, so no test can see it. `PackState`'s own comment says «Several parts of a
split are each read as a posting of their own» — the code does not do that.

**Fix (the cautious choice, until the check on a real account — register item 34):**

- `cancelled_from_split_pending` is a state of its own in `PackState` («split»), never `Cancelled`.
- After a report whose parcels carry part numbers, the packing is read from **the parts**: packed when every part is packed; a part
  cancelled is a problem on the order naming the part (never a cancellation passed on — the owner decides, the wyo server never guesses
  a cancellation); the original shown «split» is not a problem.
- Before any report (a split made in the Ozon cabinet, Plan 6 section B), «split» is passed on as a cancellation, as today.
- The fake Ozon gains a control that makes the original `cancelled_from_split_pending` after a split, so both behaviours are tested.

### 5. The one resend of the marking codes is used up when the resend never reached Ozon — RISK

`internal/exchange/codes.go:372-393` (`checkedAgain`) counts the resend and saves before sending, and drops a network failure
(`keepStopped`). **Effect:** on the next check the owner gets the problem «codes could not be checked» although the codes were sent only
once. **Fix:** a resend that did not reach the marketplace is not counted and is tried at the next check (Ozon's codes replace —
principle XII «Codes that replace» — so a resend after a lost answer writes nothing wrong).

### 6. The problem «reopened after the packing was reported» is wiped 20 seconds later — BUG

`internal/exchange/codes.go:396-409` records `pack_reopened_after_report` and keeps `pack_next_at`; the next reading finds the posting
packed, and `packConfirmed` (`pack.go`) calls `clearProblem`, which removes the problem here and at the fulfilment centre. **Effect:**
the owner and the fulfilment centre never see that the parcel was reopened after Ozon was told it was packed. **Fix:** `packConfirmed`
keeps a `pack_reopened_after_report` problem; only «Передать заново» or the owner's action clears it.

### 7. A refused reading of a posting names the write level — NIT

`internal/marketplace/ozon/pack.go:147-164` (`posting`, `/v3/posting/fbs/get`) passes refusals through `writeRefusal`, which always
names `Posting FBS` (write). That method is in the read category (`categoryPostingFBS`, «Posting FBS read-only»,
`internal/marketplace/ozon/ozon.go:44,63`), as `orders.go:159,242` already treats it. **Fix:** the posting reading uses the read
category's refusal.

### 8. The documents say things the code does not do — DOCS

- `docs/ozon.md`, the periods table (~line 191): a hand-over waiting for Ozon is «каждые 5 минут» with a constant `retryEvery` that does
  not exist. The code (`internal/handover/handover.go:44-62`): a shipment Ozon cannot make yet every 5 minutes (`retryBatchEvery`);
  documents not made yet every 20 seconds during the first 10 minutes after the request, then every 5 minutes (`retryDocumentsFast`,
  `retryDocumentsFor`). The same wrong sentence: `docs/ozon.md` ~line 168 (`act/check-status`) and `README.md` ~line 1044.
- `docs/ozon.md`, «Адрес» (lines 30–47): says wyo talks to one Ozon address and the firewall needs only `api-seller.ozon.ru`. Since
  Plan 6 `Hosts()` (`internal/marketplace/ozon/ozon.go:236`) also lists `*.ozone.ru`, where label files lie. **Effect:** a seller who
  follows the section blocks every label. The section names both, says the label files are fetched without the key, and the firewall
  sentence lists both. The README's firewall sentence, if it has one, the same.
- Points 3, 4 and 6 change behaviour that `docs/ozon.md`, `docs/external-api.md` and README «Работа с фулфилмент-центром» describe; each
  description is updated in the same commit.

## Tests (written first, failing first — the `go-testing` skill)

Each test below fails on `b88b903` and passes after the fix. Counts are exact numbers, never «at least».

- Points 1 and 2, in `internal/exchange` with the fake Ozon and the fake fulfilment centre (the kit's fault «refuse with this answer»):
  for each refusal path, run the job for 30 minutes of made-up time; the fake Ozon's call log shows the refused write **exactly once**,
  the fake fulfilment centre got the problem **exactly once**, the order's history holds **exactly one** «problem» row; then «Передать
  заново» makes exactly one new attempt. The existing `ozon_labels_test.go` case «the task ends in an error» gains these counts (it
  asserts only the create call today — the test is extended, nothing it checks is loosened).
- Point 3, in `internal/handover`: a posting made ready in the fake Ozon (its control `late`) between «move» and «close» → «move» runs
  once more, the approved shipment holds exactly the hand-over's postings; the posting added a second time → no approval, the problem
  `batch_differs` naming it. The Wildberries hand-over tests pass unchanged.
- Point 4: the fake's new control (original `cancelled_from_split_pending` after a split) — the walk of two parcels ends with both labels
  attached and **no** `POST /api/v1/orders/{id}/cancel` at the fake fulfilment centre; a part cancelled → a problem naming the part, no
  cancellation passed on; a cabinet split before any report → cancellation passed on (as today). The contract suite case for `PackState`
  covers «split».
- Point 5: a resend that meets no connection → the next check resends, no problem; a second `failed` after a resend that reached Ozon →
  the problem, as today.
- Point 6: reopened after the report → the posting read as packed → the problem still there, still at the fake fulfilment centre.
- Point 7: `internal/marketplace/ozon` — a missing-role refusal on `/v3/posting/fbs/get` names «Posting FBS read-only».
- The interrupted-walk test (`TestAWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice`) and the budget test
  (`TestTheExchangeAtItsPeriodsNeverExceedsEitherBudget`) pass unchanged.
- The interface: one new problem text (point 4, a part cancelled) is a key of `messages/ru.json` drawn by the existing `ProblemLine`;
  its existing test gains the one case. No new component.

## Components

**Created:** none. **Reused:** `ProblemLine` (the new problem's text), the one batch-comparison function (point 3, moved out of the Ozon
adapter if it lives only there), the one schedule of asking again, the one fulfilment-centre client, the one caller.

## Stages

1. `nproc` and `free -g` (the machine `server` is limited: focused runs only, no local whole-project type check). Branch from an
   up-to-date `main`.
2. Points 1, 2, 6 (`internal/exchange`) — tests first.
3. Point 4 (`internal/marketplace/ozon`, the fake Ozon, `internal/exchange`) and point 7.
4. Point 3 (`internal/handover`, the comparison function).
5. Point 5.
6. Point 8 and the descriptions of points 3, 4, 6; the new message key.
7. Smoke test (the `smoke-testing` skill, the built wyo server with the fakes): an Ozon order whose label request the fake refuses —
   «Журнал запросов» shows one `package-label/create`, not one every 20 seconds; «Передать заново» makes one more; an Ozon order packed
   in two parcels with the fake's «original split» control — both labels attached, no cancellation; every changed screen at a computer's
   width and at 390 × 844.
8. The gate: the test flow of `AGENTS.md` (limited machine — the pull request's run is the first whole gate); the check of rule R1;
   one squashed `fix:` commit; push; pull request; merge by the agent; the release run watched to its end; `main` pulled; the report
   starting with Done / Not Done.

A progress checklist naming each stage's work is printed after every stage.

## Not in this plan (each goes to the register at the merge, as a new item «What the final review of Plan 6 left», with its reason)

- **Parts matched to parcels by position** when Ozon's answer to the packing report lacks `additional_data`
  (`internal/marketplace/ozon/pack.go:128-142`, `internal/exchange/pack.go:184-197`): with two parcels of different contents a different
  order of the parts would swap their labels. The shape of the answer is not sourced; it goes to register item 34 (the real-account
  check), and the fallback is decided after it.
- **A stop of the wyo server between a write to Ozon and its record** (`pack.go` `report`, `codes.go` codes accepted): the order keeps no
  next reading and waits for the next fulfilment-centre event. A narrow window; the interrupted-walk test covers lost answers, not a
  process stop at that line.
- **Marking codes of a product that is not in the posting are dropped without a word** (`internal/marketplace/ozon/codes.go`, `marksOf`):
  Ozon then refuses the packing report, and that refusal is shown in Ozon's words.
- **The 5 MB limit is applied after Ozon's own API answer is read** (`handover.go` documents, `files.go:61`): the host is Ozon's API and
  the call has a 30-second limit.
- **The outbound gate allows `*.ozone.ru` for every marketplace adapter, not only Ozon's** (`internal/outbound/outbound.go`): the route
  table keeps every key off those hosts.
- **Documents `error` / `cancelled` wait and are asked again every 5 minutes**, as Plan 6 section E says; how Ozon wants them «запрошены
  повторно» is not sourced (register item 34).
- Register items 35 (the found shipment cancelled on withdrawal and the rest), 36 (splitting before packing) and 37 (the smoke test's
  findings outside Plan 6) stay open: each waits for the maintainer's decision or a real account, and none is a defect this review found.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read the constitution and the register before the first edit.
2. Tests first, failing first; never change what a test checks to make it pass — the only changes to existing tests are added
   assertions (point 1).
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. Everything is a component and is reused: one comparison of a batch's orders, one schedule of asking again, one fulfilment-centre
   client, one caller; search before writing. Every subagent prompt repeats this rule.
5. No migration; nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and `marketplace.<code>.*` message keys;
   no fulfilment product named anywhere (rule R1) — the check before every push.
6. No write to a marketplace repeated blindly; no order cancelled at a marketplace by the wyo server; a fact about Ozon only from its
   official documents, a gap marked «not sourced».
7. One branch, one commit `fix:`, no `!` and no `BREAKING CHANGE:` without the maintainer's yes.
