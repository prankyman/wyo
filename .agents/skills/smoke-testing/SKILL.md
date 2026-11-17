---
name: smoke-testing
description: 'Use to smoke-test the built wyo server in a real browser with MCP Playwright: build, start with the fakes, check, report.'
metadata:
    author: local
    version: '1.0.0'
---

# Smoke Testing wyo

This skill is the browser-level check that the built wyo server and the wyo interface actually work end to end, driven through MCP
Playwright. It covers what exists today and is extended in the same plan that adds a screen: a screen without smoke coverage is not
finished.

Read alongside: `.specify/memory/constitution.md` (rules R3 and R4; «Every screen is mobile-first»), `AGENTS.md` («The test flow of every
plan») and the `sveltekit-testing` skill (§5.7 and §9).

## When to Use This Skill

- Before merging a plan that adds or fixes a screen or a call (the Plan Implementation Workflow, step 4, requires it for every feature and
  bug fix; it is skipped for documentation-only and chore work).
- After a significant change of the wyo server, of the wyo interface or of a fake.
- To look at or analyse the interface and its interactions in a real browser.

A smoke test that passed is not repeated without a reason.

## What Exists Today

| Address                | What it is                                                                                  |
| ---------------------- | ------------------------------------------------------------------------------------------- |
| `/`                    | The start page: shows the program's name and the version the wyo server reports             |
| `/showcase`            | Every component of the library in each of its states (only in a wyo server built for tests) |
| `GET /api/v1/meta`     | The wyo server's own API, one call: `{ "name", "version", "mode" }`                         |
| any other `/api/…`     | `404` and `{"error":{"code":"not_found"}}`                                                  |
| any other page address | The start page (the interface decides what to draw)                                         |

Each plan that adds a screen adds its rows here and its steps to Phase 2, in the same plan. Plan 2 adds sign-in; Plan 3 the keys and the
order list; Plan 4 «External API».

The fakes (`cmd/wyofakes`): the fake Wildberries on `127.0.0.1:8791`, the fake fulfilment centre on `127.0.0.1:8792`. Both answer
`/__control/health` when up, `/__control/reset` empties their state, `/__control/calls` lists the calls they received (without keys).

## Where the smoke test runs

**Locally, always:** one browser against the built wyo server started in test mode with the fakes, the signed-in people opened one after
another, never side by side (on a limited machine — fewer than 6 cores or 16 GB of memory or less — a smoke run of a few pages holds about
1 GB of browser memory; check `free -g` shows at least 3 GB available before you start). There are no remote machines in this repository.

Write the script so it can run by `node <script>` from the repository's root as well as through MCP Playwright:

```javascript
// node <script> from the repository's root
const { chromium, selectors } = require(require.resolve('@playwright/test', { paths: [process.cwd()] }));
const BASE = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:8790';
const OUT = process.env.SMOKE_OUT || process.argv[2]; // the screenshots' folder
selectors.setTestIdAttribute('data-test');
(async () => {
    const browser = await chromium.launch();
    try {
        // … the checks; read passwords from the environment and never print them
    } finally {
        await browser.close(); // always: a thrown assertion must not leak the browser
    }
})().catch((error) => {
    console.error(error);
    process.exit(1);
}); // a failed check exits non-zero
```

## Credentials

> **Passwords and keys are not written down here, anywhere.** A password, a key or a session cookie never goes into a tool call, a file, a
> commit, a report, a screenshot filename or the chat.

There is no sign-in before Plan 2. When sign-in exists, the owner's address and password for a smoke run come from the environment of the
shell that starts the run (the plan that builds sign-in names the variables); read them at the moment you need them and fill the field
inside the page. The same holds for a marketplace key: use the made-up key of the fakes (the fake Wildberries accepts any key and keeps a
state per key), typed inside the page, never printed. Where a screenshot would show a key, take a screenshot of the single element being
checked, or none.

Every `data-test` attribute is the way to select an element (`page.locator('[data-test="…"]')`); `page.getByTestId` looks for `data-testid`,
so use `data-test` selectors or set the test-id attribute as in the script above.

## Smoke Testing Workflow

### Phase 1: Build and start

1. `free -g` shows at least 3 GB available? If not, stop and report; do not run a smoke test on a starved machine.
2. Check whether the wyo server already answers: `curl -s http://127.0.0.1:8790/api/v1/meta`. If it does, check that it runs the current
   code (it was started after your last change; the binary `bin/wyo` is newer than your last edit) and use it. Never start a second wyo
   server on another port. Never stop a server that was already running before this session, except to restart it when it runs old code.
3. Otherwise build and start, in background mode, only what you need:

```bash
pnpm build                                   # the interface into internal/webui/dist/, then the wyo server into bin/wyo
go run -tags wyotest ./cmd/wyofakes          # the two fakes, on 127.0.0.1:8791 and :8792
# then the wyo server in test mode, started as scripts/e2e/ starts it for the browser tests (read it: flags, data folder, port)
```

4. **Wait for the answer, not for a log line:** poll `http://127.0.0.1:8790/api/v1/meta` until it answers `200` (up to 60 seconds), and
   `http://127.0.0.1:8791/__control/health` and `http://127.0.0.1:8792/__control/health` for the fakes. If a poll never answers, read the
   background command's output, report the specific message and stop.
5. Use a data folder of your own for the run (a new temporary folder) and delete it at the end.

### Phase 2: Browser testing with MCP Playwright

The MCP Playwright tools are named `browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_console_messages`,
`browser_network_requests`, `browser_resize`, `browser_take_screenshot`, `browser_close`; the session may prefix them. Use whatever
prefix the session shows. `browser_type` fills the whole text at once by default; pass `slowly: true` to type character by character.

**After EVERY screen, check four things** (a screen is not passed until all four are clean):

1. **Console errors:** `browser_console_messages` with `level: 'error'` — none. Document every message and the page it came from.
2. **Raw text keys:** the snapshot shows no text that looks like a key (`common.`, `app.`, a dotted lowercase word) — every visible text is
   Russian (constitution, principle I).
3. **Sideways scroll at 390 pixels:** at 390 × 844 the page does not scroll sideways (a dense table may scroll inside its own panel).
4. **Requests:** `browser_network_requests` lists no request to any address but the wyo server's own (R3, below).

Test the screens in this order — each step leaves the browser in the state the next needs:

1. **The meta call** (`/api/v1/meta`): navigate directly; expect a `200` JSON answer with `name` `wyo`, a `version` and `mode` `test`.
2. **The start page** (`/`) at 1280 × 800 and at 390 × 844: the name and the version the wyo server reports are drawn; take a screenshot of
   each for the report; run the four checks.
3. **An unknown page address** (`/does-not-exist`): the start page answers (a reload on any page works). **An unknown API address**
   (`/api/does-not-exist`): `404` with `{"error":{"code":"not_found"}}`.
4. **The showcase page** (`/showcase`) at 1280 × 800 and at 390 × 844: every component is drawn in each of its states; nothing is cut off,
   overlapping or unreachable; every tap target looks at least 44 × 44 pixels; take a screenshot of each width; run the four checks.
5. **R3 — with «External API» empty no request leaves for anything but the fake marketplaces.** While you walk the screens above (and every
   screen later plans add), read the list of requests the browser sent (`browser_network_requests`) and the fakes' call lists
   (`/__control/calls`): the browser calls only the wyo server's own address; the wyo server calls only the fake Wildberries, and the fake
   fulfilment centre receives nothing while «External API» is empty. Any other address — a font, a script, a statistics call — is a failure.
   (The wyo server's own outgoing addresses are checked by its Go tests; this step checks the whole built program.)
6. **R4 — a key shows only its last four characters.** From Plan 3 on, when a screen shows a saved marketplace key, check that the page
   shows at most its last four characters: the snapshot, the page source (`browser_evaluate` on `document.documentElement.outerHTML`,
   searched for the key's first characters, never printed) and the network answers must not contain the rest of the key. Until a screen
   shows a key this step is **not applicable**; write "not applicable: no screen shows a key yet", never "passed".
7. **Two people at once** (from Plan 2 on, when more than one person can sign in): use a second browser **context** per person, never a
   second browser, and open them one after another.

A check that only a human can do (a printed sticker on paper, a phone camera, a real marketplace account) is reported as **waiting for the
maintainer**, never as passed.

**Responsive testing:** the start page and the showcase page are checked at 1280 × 800 and at 390 × 844. The 390 × 844 phone is the
primary layout (constitution, «Every screen is mobile-first»); the widths 640, 768, 1024 and 1280 are measured by the browser tests'
screen sweep, not by hand.

```javascript
await browser_resize({ width: 1280, height: 800 });
await browser_take_screenshot({ filename: 'start-desktop.png' });
await browser_resize({ width: 390, height: 844 });
await browser_take_screenshot({ filename: 'start-phone.png' });
```

### Phase 3: Automated testing

The automated gate is not part of the smoke test; the main agent runs it as `AGENTS.md` («The test flow of every plan») requires. A smoke
test reports the status of those runs only if they were run, never runs them itself, and never starts a heavy check.

### Phase 4: Cleanup

**Stop only what you started in Phase 1:** the wyo server, the fakes, any helper. Find a process by its port and stop it by its process id
(the bash tool's command id is not the process id):

```bash
ss -ltnp 'sport = :8790'      # then: kill <PID>   (8791 and 8792 for the fakes)
```

Verify the ports are free, delete the temporary data folder and any temporary file.

**Close the browser — mandatory, even when the run failed or you were interrupted.** A Playwright browser holds an exclusive lock on its
profile; a browser left running blocks every other session from starting one:

```javascript
await browser_close();
```

Then verify no automation browser survived, and stop it if one did:

```bash
ps -eo pid,args | grep '[m]s-playwright' | grep -E 'chrom'     # only PIDs from this list may be stopped
```

**Never stop the user's own browser.** An automation browser has `ms-playwright` in its command line; the user's has not.

## Checklist for Smoke Testing

Before reporting the smoke test as complete:

- [ ] The machine had at least 3 GB of memory available; the wyo server and the fakes answered before the first step
- [ ] The meta call, the start page, an unknown page address, an unknown API address and the showcase page checked
- [ ] The start page and the showcase page seen at 1280 × 800 and at 390 × 844, a screenshot of each
- [ ] After every screen: no console error, no raw key, no sideways scroll at 390, no request to a foreign address
- [ ] R3 checked: with «External API» empty, no request left for anything but the fake marketplaces
- [ ] R4 checked, or written as "not applicable" with the reason
- [ ] No password, key or cookie appears in a tool call, a file, a screenshot name or the report
- [ ] The wyo server, the fakes and the helpers you started are stopped; the browser is closed; no automation browser survived
- [ ] The report is written to `.agents/reports/` (below); defects are listed, none fixed

## Error Handling

**If the wyo server does not start:** check that nothing else listens on `8790` (`ss -ltnp 'sport = :8790'`), that `pnpm build` ended
without an error, and read the background command's output. Report the specific message; do not test against a half-up program.

**If `/api/v1/meta` answers but a page does not load:** the interface was not built into `internal/webui/dist/` before the wyo server was
built (`pnpm build` does both, in this order). Report it.

**If a fake does not answer `/__control/health`:** start `go run -tags wyotest ./cmd/wyofakes` again and check the ports `8791` and `8792`.

**If the browser fails to navigate:** verify the server is running and the address is right; report the specific error.

**If console errors are found:** document every message and the page it came from; check whether it is critical or a warning; report all of
them before going on.

**If cleanup fails:** stop the leftover process by hand, close the browser, confirm with `ps` that nothing survived, and report it.

## Reporting Format

Write the report to `.agents/reports/<plan-name>-smoke-test-report.md`, where `<plan-name>` is the file name of the plan being verified.
Put the screenshots next to it, named `<plan-name>-<viewport>-<screen>.png`. The report never contains a password, a key or a session
cookie. **A smoke test finds and reports; it does not fix.** A defect is listed with its page, its viewport and its console output.

**Successful smoke test:**

```
Smoke Test PASSED

Servers: reused / started and stopped (the wyo server, the fake Wildberries, the fake fulfilment centre)
- Meta call: passed
- Start page at 1280 and 390: passed
- Unknown page address and unknown API address: passed
- Showcase page at 1280 and 390: passed
- After every screen: console clean, no raw keys, no sideways scroll, no foreign request: passed
- R3, with External API empty no request left for anything but the fake marketplaces: passed
- R4, a key shows only its last four characters: not applicable, no screen shows a key yet
Waiting for the maintainer: <the human-only checks, or "none">
Cleanup: completed successfully
```

**Failed smoke test:**

```
Smoke Test FAILED

Failure Point: [specific step and screen that failed]
Error Message: [actual error message]
Details: [URL, viewport, console output]

Steps Tested Before Failure:
- Meta call: passed
- Start page: failed [details]

Cleanup: [status]
```

## Common Issues and Solutions

**Port already in use:** first check whether it is simply the server that was already running before this session — if so, use it, do not
stop it. Otherwise find the unrelated process with `ss -ltnp` and report it; do not stop a process you did not start.

**The showcase page answers "not found":** the wyo server was built without the `wyotest` tag; it is the released build's correct answer.
Start the test-mode server instead.

**A page shows old data after a change:** the wyo server in memory is older than your change; restart the one you started.

**A browser test passed but the smoke test fails:** the browser tests run in their own data folders and ports; the smoke test runs the whole
built program once. Report the difference; do not change a test to hide it.

## Integration with Other Skills

- **sveltekit-testing** (`.agents/skills/sveltekit-testing/SKILL.md`) — for the unit, component and browser tests the smoke run complements.
- **go-testing** (`.agents/skills/go-testing/SKILL.md`) — for the Go tests that check the wyo server's own outgoing addresses and the key
  canary.

## Notes

- Keep smoke test results (the reports) for historical comparison.
- Update this skill in the same plan that adds a screen, a `data-test` id or a language.
- A smoke test that finds nothing wrong proves only what it looked at: say what it looked at.
