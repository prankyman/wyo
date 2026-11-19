# Plan 6a — the fixes from the final review of Plan 6: report

**DONE.** Released as **0.0.8** (`https://github.com/prankyman/wyo/releases/tag/v0.0.8`) on 2026-11-19. Merged as `2d6276f`
(pull request #11, `https://github.com/prankyman/wyo/pull/11`, one commit `573fd00`); the release commit is `a2db778`.

- The plan: `.agents/plans/wyo-plan-6a-ozon-exchange-review-fixes-endless-retries-shipment-recheck-split-original-docs.md`.
- The check of rule R1 (every word of the list the maintainer keeps outside the repository, over every tracked and new file and the
  commit message) ran before the push and found nothing.
- No database migration. Nothing is left to build in this plan. What stays open is register item 38 (what the review left, and three
  findings of this work) and item 34 (the checks on a real Ozon account).

## What was fixed (the plan's points)

1. **A refused label is no longer asked for, read or reported again every 20 seconds.** Every problem of the label step clears the
   parcel's next reading (`labelProblem`, `labelFailed` in `internal/exchange/pack.go`).
2. **A refused packing report is no longer sent again every 20 seconds.** Every problem of the packing step clears the order's next
   reading (`packProblem`, `packFailed`). «Передать заново» and a new `order.packing_finished` start the step again.
3. **The Ozon shipment is read and compared with the hand-over again before it is approved** (`internal/handover/steps.go`, `close`).
   A difference rewrites the list once; a second difference is the problem `batch_differs` naming the postings, and nothing is approved.
   The comparison is one function, `marketplace.BatchDiffers`, used by the Ozon adapter's `FillBatch` and by `close`; the
   «what is in one list and not in the other» part, `marketplace.Without`, also replaced the Wildberries adapter's own loop.
4. **The original of the wyo server's own split is never passed on as a cancellation.** `cancelled_from_split_pending` reads as
   `Split`, never `Cancelled` (`internal/marketplace/ozon/pack.go`). After a report that split the posting the parts are read by
   their own numbers (`packState`, `splitParts`); a cancelled part is the new problem `pack_part_cancelled`, which names it; the
   reading of states skips such an order (`internal/exchange/cancel.go`). A split made in the cabinet before any report is still a
   cancellation. The fake Ozon gained the control `ship-split`.
5. **A resend of the codes that never reached Ozon is not counted** and is made at the next check (`checkedAgain`,
   `internal/exchange/codes.go`).
6. **The problem «reopened after the packing was reported» stays** when Ozon then shows the posting packed and the label is attached
   (`clearSettled`).
7. **A refused reading of a posting names «Posting FBS read-only»** (`wordRefusal`, `internal/marketplace/ozon/refusal.go`).
8. **The documents** (`docs/ozon.md`, `docs/external-api.md`, `README.md`, `docs/development.md`) say the real periods of a waiting
   hand-over, the label files' host `*.ozone.ru` (fetched without the key), and the changes above.

## Found and fixed beyond the plan

- **The wyo interface never named the postings of a shipment that differs from the hand-over.** The wyo server sends them as one text
  («200-1, 200-2»); the interface chose the sentence that names them only for a list. `problem-text.ts` now reads both.
- **The reading of states would have passed the split original on as a cancellation too** (the plan named only the packing's reading).

## Tests

All written first and seen failing first; existing tests only gained assertions.

- `internal/exchange/ozon_refused_once_test.go` — 4 packing refusals and 2 label refusals, each run for 30 minutes of made-up time:
  exactly one problem sent to the fulfilment centre, exactly one «problem» row in the history, the exact count of reports and label
  requests, then exactly one new attempt after «Передать заново». Before the fix one of them reported the problem 90 times.
- `ozon_labels_test.go` — the existing «not made» test gained the same two counts.
- `ozon_codes_test.go` — a lost resend of the codes; a reopening before Ozon shows the posting packed.
- `ozon_packing_test.go` — the split original and a cancelled part.
- `ozon_handover_test.go` — a posting falling into the shipment between «move» and «close», once and twice.
- `internal/marketplace` (`BatchDiffers`, 9 cases), `internal/marketplace/ozon/pack_test.go` (3), the contract suite (a split order
  reads `Split`), `internal/fakes/ozon` (the two new controls), `ProblemLine.svelte.test.ts` (3 cases).
- Local focused runs on the limited machine (4 cores, 15 GB): every changed package green; the whole `internal/exchange` package
  (the interrupted-walk and budget tests among them) green in 249 s; `golangci-lint` on the changed packages: 0 issues.
- The pull request's first run in GitHub Actions failed on one test, `TestNoGoSourceFileIsTooLong` (`internal/archtest`): `pack.go`
  had grown to 813 lines, over the limit of 800. The label step moved to `internal/exchange/labels.go`, unchanged.
- The second run in GitHub Actions passed: the whole gate (generate, lint, type checks, tests with coverage, build), the browser tests,
  the memory test of 10 000 orders, and the installer on three systems.

## Smoke test

PASSED: `.agents/reports/wyo-plan-6a-ozon-exchange-review-fixes-endless-retries-shipment-recheck-split-original-docs-smoke-test-report.md`.
A refused label request stayed at one `package-label/create` through 12 runs of the exchange; «Передать заново» made exactly one more and
the label was attached. A posting packed in two parcels with the original shown split got both labels and no cancellation at the
fulfilment centre through 16 runs. The new problem text was not reachable in the browser (only its Go and component tests show it).
The screenshots stay beside the report, not committed, as in earlier plans.
