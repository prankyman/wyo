# Browser tests

Playwright tests of the wyo interface, run against the BUILT wyo server — never against the Vite dev server. Rules: `AGENTS.md`,
«The test flow of every plan»; the skill: `.agents/skills/sveltekit-testing/SKILL.md`, §5.7.

## Running

```bash
pnpm test:e2e:install                              # once: the Chromium the tests use
pnpm test:e2e e2e/specs/start-page.spec.ts         # one spec file
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
2. **Every worker** starts a wyo server of its own the first time one of its tests needs it (`wyo` in `e2e/fixtures/test.ts`): test
   mode, port `8800 + the worker's parallel index`, running in an empty data folder under the worker's output folder
   (`<output>/wyo-data/worker-<index>`). It waits for `GET /api/v1/meta` to answer — never a fixed time — and stops the server when the
   worker ends. The fixture also sets `baseURL`, so `page.goto('/showcase')` opens the worker's own server.
3. **Every test** imports `test` and `expect` from `e2e/fixtures/test.ts`. The browser console is collected: any console error or
   uncaught error of the page fails the test. `requests` records every request the test's pages make.

A spec that needs no browser (the screens table) imports `test` from `@playwright/test` instead and starts no server.

## Where things are

| Path                                 | What                                                                                                                     |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| `e2e/specs/`                         | the tests; a test file stays under about two minutes                                                                     |
| `e2e/fixtures/test.ts`, `console.ts` | the test with the worker's wyo server, the console check and the request log                                             |
| `e2e/helpers/phone.ts`               | the phone's rules (390 x 844): tap targets, sideways scroll, covered, overlap, selects (`measurePhoneScreen` and others) |
| `e2e/helpers/box-level.ts`           | the boxes of a form stand level, labels close to their boxes, nothing overlaps (at 640, 768, 1024, 1280 px)              |
| `e2e/helpers/stack-gaps.ts`          | the one 16 px gap between the blocks of a column                                                                         |
| `e2e/helpers/sweep.ts`               | `sweepScreen`: one page opened once, measured by all of the above                                                        |
| `e2e/helpers/requests.ts`            | the request recorder behind rule R3                                                                                      |
| `e2e/support/screens.ts`             | the screens table: one row for every page under `src/routes/`                                                            |
| `e2e/support/phone-exceptions.ts`    | the exceptions list of the phone's rules; EMPTY, an entry needs a written WCAG reason and may only be removed later      |
| `e2e/support/process.ts`             | starting, waiting for and stopping a program (`wyo-server.ts`, `fakes.ts` use it)                                        |

## Rules a spec keeps

- Select by `data-test` (`testIdAttribute` is set), never by a Tailwind class or a position.
- No `waitForTimeout` and no `networkidle`: wait for the condition (an element, an answer of a request).
- Work in the worker's own data folder; use your own key on the fakes; call no address but the wyo server's own and the fakes'.
- Layout is measured here and nowhere else (jsdom lays nothing out). A new page gets a row in `e2e/support/screens.ts` and a call
  `sweepScreen(page, screenOf('<route>'))` in the spec the row names; `e2e/specs/screens-table.spec.ts` fails otherwise.
- A sweep finding is never silenced by loosening the measurement. The only list of exceptions is `e2e/support/phone-exceptions.ts`.
- Specs are type-checked by `pnpm check` (`tsc --noEmit -p e2e/tsconfig.json`), which a limited machine leaves to the pipeline: write
  them type-safely.
