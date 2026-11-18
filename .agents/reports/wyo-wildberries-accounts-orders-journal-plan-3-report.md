# Plan 3 — Wildberries accounts, the order list, the request journal, and updates with a short stop: report

**DONE.** Released as **0.0.4** on 2026-11-18. Merged as `e92d5af` (pull request #7, `https://github.com/prankyman/wyo/pull/7`, one
commit `a103996`).

- The plan: `.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`, with its section K
  added on 2026-11-18 at the maintainer's request: «Make sure the installation script not only installs the app, but also checks for
  updates and updates it with minimal downtime.» His answers the same day: in this plan's commit; the copy of the database made while
  the wyo server runs, «Make sure it cleans up after the upgrade»; the installer run again goes through the updater; the daily look
  «between 1 am MSK and 3 am MSK».

## The gate

| Where                                         | What                                                                                                                                         | Result                                                     |
| --------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| Development machine (limited: 4 cores, 15 GB) | Focused runs of each changed Go package (both builds) and test file; the whole lint; `pnpm generate:check`; browser specs one file at a time | green; no whole-project type check (the machine's rule)    |
| GitHub Actions, `ubuntu-24.04` (gate)         | Secret scan, commit message, generate, lint, type checks, tests, build, browser tests, memory                                                | 9 min 29 s (run 44280633130)                               |
| Gate — interface                              | Unit tests with coverage                                                                                                                     | 1 282 tests in 189 files; 95.58 % of 6 998 statements      |
| Gate — Go                                     | Tests with coverage, both builds                                                                                                             | 92.7 % of 6 894 statements in 123 files, every file ≥ 84 % |
| Gate — browser                                | Browser tests against the built wyo server and the fakes                                                                                     | 77 tests, 44.7 s                                           |
| Installer job, three clean machines           | `ubuntu-24.04`, `ubuntu-22.04`, `ubuntu-24.04-arm`                                                                                           | green, about 2.5 minutes each                              |

**The pull request needed four runs.** Run 1: three type errors that no local check could reach (`src/lib/validation/list-filters.ts`,
`src/lib/lists/filter-words.test.ts`) and two warnings. Run 2: two files under the per-file coverage floor (`internal/release/timer.go`
80 %, `internal/server/adapters.go` 80 %) — their untested error paths got tests. Run 3: two browser tests in `e2e/specs/orders.spec.ts`
saw one order of two — **a real defect, below**. Run 4: green.

## Measured (plan section H), 2026-11-18

The test `TestTheMemoryOfADayWithThreeAccountsAndTenThousandOrders` (`internal/server/memory_wyomeasure_test.go`, tags `wyotest
wyomeasure`, step «Memory of the wyo server …» of the gate): three accounts, 10 090 orders in the database, the scheduler run through
a made-up day against the fake Wildberries. Peak memory 62.6 MB (`peak_rss_kb=62648`), Go heap in use 3.8 MB, 426 journal rows. The
test process also holds the fake Wildberries, so the wyo server's own figure is lower. The README's requirements carry the number.

## Premises (Stage 0), all held

1. Limited machine → focused local runs; the pull request's run is the first whole gate.
2. Wildberries' six description files, downloaded again on 2026-11-18, are byte for byte the committed copies under `api/wildberries/`.
3. How a seller makes the token: Wildberries' article
   `https://seller.wildberries.ru/instructions/ru/ru/material/how-to-create-update-or-delete-a-wb-api-token` («Обновлено 18.05.2026»),
   which agrees with the saved PDF; `dev.wildberries.ru` refuses scripts (status 498).
4. The token's own fields (`exp`, `sid`, `acc`, `s` with bits from 0, `t`, `id`, `for`) are confirmed by `api/wildberries/01-general.yaml`,
   section «Как устроен токен» — `.agents/research/wildberries-token-check-2026-11-18.md`.
5. `sqlc` and the migration rules of Plan 2 in place; this plan's migration is `0002`. The database of 0.0.3 was already the snapshot
   `internal/store/testdata/0001.db` (no migration changed since Plan 2).

## Updates with a short stop (section K)

- The updater copies the database **while the wyo server runs**, read-only (`store.CopyFile`), then stops, swaps and starts it: the wyo
  server is down only while the new version starts. A failed copy never stops the wyo server. The price, told to the maintainer
  before the work: a put-back loses the changes of the second or two between the copy and the stop.
- After a successful update it deletes the downloads, a leftover `wyo.new`, the folders of interrupted runs and trims the
  before-update copies to three at once (`backup.Prune`, the one pruning function); one `wyo.previous` stays.
- The installer run again on a server whose wyo server runs hands a newer release to the updater (the request file `install`, then
  `wyo-update.service`), and restarts the wyo server only when the program, `wyo.env` or `wyo.service` changed.
- The daily look: `OnCalendar=*-*-* 01:00:00 Europe/Moscow`, `RandomizedDelaySec=2h`, `FixedRandomDelay=true` (one stable moment per
  server, spread over the two hours: rule R5). The timer's text lives once, in the program (`release.TimerUnit`); the installer asks the
  program for it (`wyo update --timer-unit`) and every updater run of a new version rewrites an older timer. Sources: systemd.time(7)
  «Calendar Events», systemd.timer(5), systemd NEWS 235 and 247, read 2026-11-18.
- **Proven on the three clean machines** by the installer job: a second run of the installer with a newer release went through the
  updater (copy, new version, no failure, no request file, no leftovers, key file unchanged, the owner signs in); an earlier timer was
  put right by the installer and by the updater, both without restarting the wyo server; the timer reads `01:00:00 Europe/Moscow`.

## Smoke test (locally, the built wyo server in test mode with the fakes, one browser)

Signing in (a wrong password refused); «Маркетплейсы» empty, the add form, an account added with a made-up token — the card shows only
the key's last four characters and the box emptied; three orders made in the fake appeared after «Обновить», and the button showed «Обновить
можно через 5 мин.»; a cancellation in the fake showed after the next reading; the filter by state in the address; the details beside
the list with the history «Отменён покупателем»; «Журнал запросов» and its filter; «Обзор»; «Настройки». Every screen at 390 × 844 and
1280 × 800 with screenshots; after each: no console error (one 401 line was the wrong password typed on purpose), no raw text key, no
sideways scroll, no request to any address but the wyo server's, the key nowhere on the page.

**Findings, all fixed in this plan:**

1. **The orders feed was read in a loop** (about 12 requests in one minute). Cause: the fake Wildberries kept a row's change time to the
   nanosecond but wrote it in whole seconds, so the same rows came back. The fake now filters by the written second, and the engine ends a
   reading whose page does not move its mark, whatever a marketplace answers.
2. **An order changed in the same second as the feed's mark was never read** (found by run 3 of the pull request, reproduced by
   `TestARowChangedInTheSecondOfTheMarkIsReadAndEveryOrderKeptOnce`). Wildberries writes `lastChangeDate` in whole seconds and answers
   only rows changed later than `dateFrom`. The adapter now asks from one second before the mark; the rows of that second come again and
   are kept once by their number. Two existing tests asserted the old rule (the exact `dateFrom` values; «the end of the feed has no
   rows») and were corrected to the new one, which the shown defect requires.
3. The order history showed «Новый» twice and did not say who cancelled — an event is now written only when the connector's state or
   who cancelled changes, and carries who cancelled.
4. The details panel on a phone had no side padding — fixed once in `RowDetailsPanel`, measured by the browser tests.
5. A cancelled or handed-over order still showed its deadline — only `new` and `assembling` show it now.
6. Texts that claimed more than the sources say: the 48 hours as «the marketplace fines after this» (now: wyo's default, check your
   marketplace's rule) and «POST — передать данные» in the journal's help.

## Rule R1 and the secret scan

The search of rule R1 ran over every tracked and new file and over the commit message before each push: nothing found. The secret
scan flagged seven made-up test keys (`internal/journal`, `internal/sync`, `internal/httpapi`, `internal/marketplace/wildberries`
tests); each is an exception by its exact value in `.gitleaks.toml`, with its reason.

## Where the plan was not followed to the letter

- **The token's access level:** section J says «только чтение» is enough. Wildberries' documents do not say whether a read-only token
  reads orders (its methods carry `x-readonly-method`, which the documents do not explain), and Plan 4 writes; the instructions ask for
  «чтение и запись» and say why. The check is in register item 18.
- **Choices not sourced, recorded for the first real account (register item 18):** an FBO order the feed does not mark cancelled is «В
  доставке»; the 403 of a cabinet without seller-warehouse sales is guessed as `seller_warehouse_off`.
- **Additions:** `marketplace_busy` (a 429 while a key is checked, not «no connection»); a test build of the wyo server lists no real
  marketplace host at all, so a run without the fake's address calls nothing (rule R3); the deadline is computed, not stored; «Обновить»'s
  five minutes live in memory and end with a restart; the component `FactList` and `AccountChangeForm`; `internal/accounts`,
  `internal/testkit/daykit`, `synckit`, `memadapter`.
- **Tests first:** most packages were written test-first with the failing run recorded by the helpers; `internal/marketplace`,
  `internal/journal`, the caller and the adapter, the R3 and R4 tests and some copied interface helpers were written with their code. The
  coverage-gap tests of run 2 test behaviour that existed.
- The journal rows of a new key's first check carry no account: they are written before the account exists, and the journal's rows
  cannot be changed.

## The register

Items 18 (the checks on the first real account), 19 (seller-delivered orders; «Базовый» and «Сервисный» tokens), 20 (product cards,
prices, old orders) and 21 (the phone strip cuts long labels) added; item 8's list components copied.

## Release

**0.0.4**, released 2026-11-18 (`https://github.com/prankyman/wyo/releases/tag/v0.0.4`, release commit `4fb7de6`). The merge's run
(44282344921) skipped the gate and the installer job, because the merge commit held byte for byte the files of the pull request's last
commit, whose run passed; the release job and the released installer then passed — the first run that proves that chain whole. Checked by
hand after download: the signature of `checksums.txt` is accepted by the committed public key, every file matches its checksum, `wyo
version` prints 0.0.4, and `wyo update --timer-unit` prints the Moscow window.

The update from 0.0.3 to 0.0.4 on an installed server is made by 0.0.3's updater (copy after the stop, as before); from 0.0.4 on, the
updater copies first, and its first run rewrites the timer to the Moscow window.
