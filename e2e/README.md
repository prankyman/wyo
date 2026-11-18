# Browser tests

Playwright tests of the wyo interface, run against the BUILT wyo server — never against the Vite dev server. Rules: `AGENTS.md`,
«The test flow of every plan»; the skill: `.agents/skills/sveltekit-testing/SKILL.md`, §5.7.

## Running

```bash
pnpm test:e2e:install                              # once: the Chromium the tests use
pnpm test:e2e e2e/specs/sign-in.spec.ts            # one spec file
pnpm test:e2e                                      # the whole suite (a big machine; a limited one leaves it to the pipeline)
```

A run that must not empty `test-results/` under another run passes `--output=<a folder of its own>`. `E2E_WORKERS` sets the number of
workers (default 2). A failed test is never retried.

## What a run does

1. **Global setup** (`e2e/global-setup.ts`) runs `scripts/e2e/build.sh`: the interface with `vite build` into `internal/webui/dist/`
   unless a real build is there and no source file is newer than it; then `bin/wyo-test` (the wyo server built with the Go tag
   `wyotest`, version `0.0.0-e2e`) and `bin/wyofakes` (the fake Wildberries and the fake fulfilment centre). Then it starts the fakes
   once for the whole run (`e2e/support/fakes.ts`): on `127.0.0.1:8791` and `127.0.0.1:8792`, and waits for `/__control/health`. When both
   already answer (`pnpm dev` is running) they are used as they are and left running.
2. **Every test** runs against a wyo server (`server` in `e2e/fixtures/test.ts`), in test mode, in an empty data folder with the owner
   already made by the wyo server's own command (`wyo owner create`, `e2e/support/owner.ts`: login `owner`), and waits for
   `GET /api/v1/meta` to answer — never a fixed time. By default it is **the worker's shared server**, started the first time a test
   of the worker needs it, on port `8800 + the worker's parallel index`, in `<output>/wyo-data/worker-<index>`, and stopped when the
   worker ends. A test that changes what other tests would find — the owner's password, a setting, the count of failed sign-ins, the
   updater's request — runs on **a server of its own**: `test.use({ serverKind: 'own' })` for a file or a block. It starts for the
   test, on port `8900 + the index`, in a data folder under the test's output folder, and stops at its end. The fixture also sets
   `baseURL`, so `page.goto('/settings')` opens the test's own server.
3. **Every test** imports `test` and `expect` from `e2e/fixtures/test.ts`. The browser console is collected: any console error or
   uncaught error of the page fails the test. A test that provokes a refusal on purpose (a call with no session answers 401) names it
   in `expectedRefusals` — the status and the end of the address — and only the browser's line for exactly that refusal is left out.
   `requests` records every request the test's pages make. `ownerPage` is `page` with the owner signed in.

A spec that needs no browser (the screens table) imports `test` from `@playwright/test` instead and starts no server.

## Where things are

| Path                                 | What                                                                                                                             |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------- |
| `e2e/specs/`                         | the tests; a test file stays under about two minutes                                                                             |
| `e2e/fixtures/test.ts`, `console.ts` | the test with the wyo server, `ownerPage`, the console check and the request log                                                 |
| `e2e/helpers/phone.ts`               | the phone's rules (390 x 844): tap targets, sideways scroll, covered, overlap, selects (`measurePhoneScreen` and others)         |
| `e2e/helpers/box-level.ts`           | the boxes of a form stand level, labels close to their boxes, nothing overlaps (at 640, 768, 1024, 1280 px)                      |
| `e2e/helpers/stack-gaps.ts`          | the one 16 px gap between the blocks of a column                                                                                 |
| `e2e/helpers/sweep.ts`               | `sweepScreen`: one page opened once, measured by all of the above                                                                |
| `e2e/helpers/requests.ts`            | the request recorder behind rule R3                                                                                              |
| `e2e/helpers/sign-in.ts`             | signing in: through the API (`signInThroughApi`, for every spec that is not about signing in) and through the page               |
| `e2e/helpers/open-screen.ts`         | `openScreen`: open a page and wait until its `ready` element is drawn; `visitScreen`: the same, signed in when the page needs it |
| `e2e/helpers/sections.ts`            | open the folded sections of a page (`openAllSections`, `openSection`)                                                            |
| `e2e/helpers/policy-violations.ts`   | record every refusal by the page's security policy, including those the browser keeps out of its console                         |
| `e2e/helpers/update-state.ts`        | write the updater's file `update-state.json` into a server's data folder, for a notice behind the bell                           |
| `e2e/helpers/accounts.ts`            | a marketplace account added and the orders read through the wyo server's own API, for specs not about the form                   |
| `e2e/support/fake-wildberries.ts`    | made-up tokens in Wildberries' shape (`makeToken`, a new one per test) and the fake Wildberries' control calls                   |
| `e2e/support/screens.ts`             | the screens table: one row for every page under `src/routes/`                                                                    |
| `e2e/support/phone-exceptions.ts`    | the exceptions list of the phone's rules; EMPTY, an entry needs a written WCAG reason and may only be removed later              |
| `e2e/support/owner.ts`, `texts.ts`   | the test owner and the made-up passwords; `ruText(key, params)` reads a text from `messages/ru.json`                             |
| `e2e/support/wyo-server.ts`          | starts a wyo server in an empty folder, with the owner made first; `restartWyoServer` runs it again on the same data             |
| `e2e/support/process.ts`             | starting, waiting for and stopping a program (`wyo-server.ts`, `fakes.ts` use it)                                                |

## Rules a spec keeps

- Select by `data-test` (`testIdAttribute` is set), never by a Tailwind class or a position.
- No `waitForTimeout` and no `networkidle`: wait for the condition (an element, an answer of a request).
- **Sign in once per test, through the API** (`ownerPage`, or `signInThroughApi` in a block that walks every row of the screens table):
  the sign-in form is walked in one spec only (`sign-in.spec.ts`). A spec asserts a text through `ruText`, not by copying it.
- A test of the sweep for a page with folded sections has a second sweep with `{ before: openAllSections }`, so the boxes inside are
  measured too.
- Work in the worker's own data folder; use your own key on the fakes; call no address but the wyo server's own and the fakes'.
- Layout is measured here and nowhere else (jsdom lays nothing out). A new page gets a row in `e2e/support/screens.ts` and a call
  `sweepScreen(page, screenOf('<route>'))` in the spec the row names; `e2e/specs/screens-table.spec.ts` fails otherwise.
- A sweep finding is never silenced by loosening the measurement. The only list of exceptions is `e2e/support/phone-exceptions.ts`.
- Specs are type-checked by `pnpm check` (`tsc --noEmit -p e2e/tsconfig.json`), which a limited machine leaves to the pipeline: write
  them type-safely.
