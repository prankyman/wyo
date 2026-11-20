---
name: sveltekit-testing
description: 'Use when writing or reviewing tests of the wyo interface (SvelteKit, Svelte 5, Vitest, Playwright, MCP Playwright).'
metadata:
    author: local
    version: '1.0.0'
---

# SvelteKit Testing (Vitest, Playwright and MCP Playwright)

Testing guidance for the wyo interface. Read this before writing or changing any interface test. Go code (the wyo server, the fakes, the
tools) is tested by the `go-testing` skill (`.agents/skills/go-testing/SKILL.md`); it is the Go counterpart of this one.

**Read alongside:** `.specify/memory/constitution.md` (principle II is the authority this skill implements; III governs reuse; IV, policy
tests; I, language; V, what the browser may call).

---

## Components are tested once (constitution, principle III)

Everything that appears in more than one place is one component, and it is tested once, in its own `<Component>.svelte.test.ts`, in every
state and with every prop. A page's test checks only what the page adds — which component it draws, with which data and props, and what its
own actions do — and never tests the component's boxes, texts or answers again. When a component replaces copies, the copies' tests are
deleted, and what they proved that the component's test does not yet prove moves into the component's test. A browser test walks a component
once per flow, not once per page that draws it. A component the library lacks is copied from the maintainer's other repository with its test
(`docs/library-layout.md`), never written again.

## 1. Test Economy (NON-NEGOTIABLE)

This section exists because, in the maintainer's other repository, the suite grew to 8,431 tests across 135,000 lines — more test code
than source code — while only ~10% of the runtime was spent actually executing test bodies. The
rules below are the ones that keep that from happening again. **They bind as hard as the quality
rules in §5.**

### 1.1 One behaviour per test, not one assertion per test

A test covers **one behaviour** and may carry as many `expect()` calls as that behaviour needs.
Splitting a single behaviour across several `it()` blocks multiplies the most expensive part of a
test — setup, render, mock wiring — to buy nothing.

```typescript
// ❌ WRONG — four renders, four setups, one assertion each
it('renders the title', () => {
    render(Banner, { props });
    expect(screen.getByText('Stopped')).toBeInTheDocument();
});
it('renders the reason', () => {
    render(Banner, { props });
    expect(screen.getByTestId('reason')).toHaveTextContent('Keys expired');
});
it('renders the deadline', () => {
    render(Banner, { props });
    expect(screen.getByTestId('deadline')).toHaveTextContent('7 days');
});
it('renders the contact', () => {
    render(Banner, { props });
    expect(screen.getByTestId('contact')).toHaveTextContent('owner@example.com');
});

// ✅ RIGHT — one render, one behaviour ("a stopped connection shows its full notice")
it('shows the reason, the 7-day deadline and the platform contact', () => {
    const { container } = render(Banner, { props });
    expect(screen.getByText('Stopped')).toBeInTheDocument();
    expect(container.querySelector('[data-test="reason"]')).toHaveTextContent('Keys expired');
    expect(container.querySelector('[data-test="deadline"]')).toHaveTextContent('7 days');
    expect(container.querySelector('[data-test="contact"]')).toHaveTextContent('owner@example.com');
});
```

### 1.2 One `render()` per test, maximum

If a test needs two renders it is either two behaviours or a case for `it.each`. A component mount
is the single most expensive thing a test can do.

### 1.3 Parameterise enumerations with `it.each`

Any block that walks a list of statuses, currencies, roles, or option values must use `it.each`,
never copy-paste.

```typescript
it.each([
    ['pending', true],
    ['confirmed', true],
    ['shipped', false]
])('allows editing in %s only when the order is not yet shipped', (status, editable) => {
    expect(canEdit(status)).toBe(editable);
});
```

### 1.4 Never assert the same string in two locales

Russian is the only language (constitution, principle I), so there is nothing to double today; the rule keeps the suite from
doubling the day a second language file exists.

- Write **one** test proving the component's locale _plumbing_ works — that the `lang` prop reaches
  `t()` and the rendered text changes. Not one per string.
- Write a second locale test **only** when the component has genuinely locale-conditional logic
  (different layout, different formatting, a locale-specific branch).
- Bundle completeness is covered centrally, once, by `message-bundles.test.ts`, which asserts every key used in `src/`
  exists in every `messages/*.json`. That protects against missing translations far better than
  hand-mirrored assertions did, and costs one test instead of hundreds.

### 1.5 Responsive layout is an e2e concern

CSS media queries do not apply in jsdom, therefore a jsdom test that renders a component and asserts
"the mobile menu is present" asserts nothing about mobile.

- jsdom tests may cover **JS-driven** responsive behaviour (a `matchMedia` listener, a
  ResizeObserver callback, a breakpoint held in `$state`).
- Everything that depends on CSS belongs in Playwright (`e2e/`), where a real viewport exists (the screen sweep measures every page at
  390 × 844, 640, 768, 1024 and 1280 pixels).

### 1.6 Do not test the framework

No tests whose only claim is that Svelte props, bindings, or `{#if}` work. Those are Svelte's tests.
Test _your_ logic and _your_ rendered output.

### 1.7 Never hand-roll a shared mock

See §3. Hand-rolling `$lib/i18n/i18n`, the form library's instance or a page's data is forbidden.

### 1.8 Budget check before you commit

If a change adds more than ~10 tests to one file, stop and ask whether they are behaviours or
assertions. The reviewer will.

---

## 2. Two Vitest projects: know which one your file lands in

`vitest.config.ts` defines **two projects**, because most test files never touch a DOM and would each pay the cost of building a jsdom
they never use.

| Project | Collects                                       | Environment | Setup file               |
| ------- | ---------------------------------------------- | ----------- | ------------------------ |
| `node`  | every `*.test.ts` that is NOT a component test | `node`      | `src/test/setup.node.ts` |
| `jsdom` | `**/*.svelte.test.ts`                          | `jsdom`     | `src/test/setup.ts`      |

**Naming decides routing, so naming is not cosmetic:**

- Component test → **`ComponentName.svelte.test.ts`** → jsdom.
- Anything else (stores, pure logic, helpers, policy tests) → `name.test.ts` → node.

A non-component test that genuinely needs DOM globals (`localStorage`, `File`, `EventSource`) must
be added to `DOM_DEPENDENT_NON_COMPONENT_TESTS` in `vitest.config.ts` **with a comment saying why**.
Keep that list short — it is an escape hatch, not a dumping ground.

> Do **not** reach for a `// @vitest-environment jsdom` docblock instead. It would give the file a
> jsdom inside the node project's pool settings, which is a combination that has been shown not to survive.

### `browser` is `false` in the node project

The node project aliases `$app/environment` to a `browser = false` variant. This is correct for
isomorphic code, and it is what stops `src/lib/i18n/` touching `document` at module load.

### Why `isolate: false` is not used

It is the biggest speed lever on paper and it is **not safe here**. In the maintainer's other repository it made component tests leak
module and DOM state into each other (84–190 failures out of 566 in the jsdom project) and made a `vi.mock` in one file silently win for
later files in the same worker, with a different set of files failing on each run. Consolidating mocks onto §3 is the prerequisite for
ever revisiting this. Do not enable it.

---

## 3. Shared fixtures — `src/test/support/` (NON-NEGOTIABLE)

In the maintainer's other repository, 144 test files had once hand-rolled the same texts mock in 61 different shapes, and two of those
shapes were outright wrong, so every test written against them verified the mock rather than the code. Shared fixtures end that: **a
mock, a stand-in or a data builder that two files need is ONE helper in `src/test/support/`**, and the guard
`test-helper-duplication.test.ts` fails a test file that declares a helper whose home is elsewhere (one home per test fixture, like
`helper-duplication` for source code).

**Import via the `$test` alias.** Always use a lazy async factory so the call site is immune to `vi.mock` hoisting:

```typescript
// texts — replaces every hand-rolled copy
vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock());
// …or with options:
vi.mock('$lib/i18n/i18n.js', async () => (await import('$test/support/i18n')).createI18nMock({ locale: 'ru' }));
```

**`createI18nMock` is faithful to the real module** — `t(key, lang?, params?)` with `{param}` interpolation and fallback to the default
language, and a `localizePath` that leaves the default language unprefixed. If your test disagrees with it, the test is asserting the old
mock's bug. **The documented exception:** a test needing a _mutable_ language (flipping between cases) keeps a bespoke `vi.hoisted` mock.
Say so in a comment.

**Forms — every form is a superform in single-page mode (constitution, «Forms»), and their tests use these fixtures, never a hand-made
copy:**

- `superform` — `createSuperFormMock`, `savedAnswer`: a `superForm()` instance handed to a component as a prop.
- `superform-answer` — `answerForm`, `postedJson`: a component test that sends a real superform and answers it as the wyo server would;
  `settle`, `flushTicks`: one tick, or several Svelte updates in a row.
- `render-page` — `renderPageWith(Page, data, props?)`: a route's page rendered with its data.

A form is sent as JSON through `src/lib/api/`, so a test stubs the one function of that folder that sends — never the global `fetch` in a
component test — and answers it with a success or with the refusal `{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }`.

**The stand-ins for SvelteKit's `$app/*` modules** (`navigation`, `page-state.svelte`, `reactive.svelte`) and the helpers that several
component tests share (`overlay-a11y` — `describeOverlayA11y`, `page-help` — `expectPageHelp`, `svelte-markup`) live in the same folder.
Before writing a stand-in, look there. The fixtures that exist are the ones the component library was copied with
(`docs/library-layout.md`); a plan that needs a new one adds it to `src/test/support/` and nowhere else.

---

## 4. Test-first development

- Vitest tests are mandatory for stores, helpers and components with non-trivial logic.
- Tests are written **before** implementation; Red-Green-Refactor. Show the failure first.
- Component tests use only the public API — props, callbacks, rendered DOM. Never reach into internal state.
- Tests introduce no new logic, no deprecated APIs, and no lint errors.
- **Never change what a test checks to make it pass** (constitution, principle II; `AGENTS.md`). Find the cause first; "flaky" is not a
  cause. If the program is wrong, fix the program.

**The test flow depends on the machine** (`AGENTS.md`, «The test flow of every plan»). Read `nproc` and `free -g` at the start of the plan.
Fewer than 6 cores, or 16 GB of memory or less, is a **limited** machine; anything bigger is **big**.

- **Focused runs first, on any machine:** ESLint on the files you changed, `pnpm test:headless <file>` for an interface test file,
  `pnpm test:e2e e2e/specs/<file>.spec.ts` for one browser test file, `pnpm go:test <package>` for Go. At most 2–3 test files at once.
- **The whole gate runs on the Mac mini, only by the main agent: `pnpm remote:gate`** (`AGENTS.md`, «The test flow of every plan»): the
  commit messages, `pnpm precommit` and the whole browser suite. **On a limited machine no whole-project type check ever runs locally**
  (`pnpm check`, `svelte-check`, any `tsc` with any configuration): it runs on the Mac. So write test code type-safely — a typed stub, no
  tuple index on an untyped mock, no property that a union type may lack — and never fix a type error blind: read every error of the
  run's report, fix all of them, then run again.
- **The Mac's run is the gate; GitHub Actions runs no test and does not check the Mac's run**, so only a commit the Mac passed is
  pushed. Neither is used to find out whether one test passes. A failure is fixed
  locally with focused runs of the failed files.
- A subagent runs only its own test files and lints only its own changed files — never a whole suite.

**Frugality.** `pnpm precommit` is the final gate, not an iteration tool. While fixing what a run surfaced, re-run only the specific
check that failed — the single test file, the lint of the changed files — never the whole suite per fix. Do not run the same long
command twice just to filter its output differently: save the output to a temporary file and read that.

### Commands

| Command                              | Use                                                                           |
| ------------------------------------ | ----------------------------------------------------------------------------- |
| `pnpm test:headless`                 | whole suite, both projects — main agent only                                  |
| `pnpm test:headless <pattern>`       | one file / directory — use while fixing                                       |
| `pnpm test:coverage`                 | interface and Go tests with coverage — main agent only                        |
| `pnpm coverage:enforce`              | the gate described in §8 — main agent only                                    |
| `pnpm test:e2e [file]`               | browser tests against the built wyo server                                    |
| `pnpm go:test <package>`             | one Go package (`go-testing` skill)                                           |
| `pnpm exec eslint <files>`           | lint of the files you changed                                                 |
| `pnpm exec prettier --write <files>` | format the files you changed                                                  |
| `pnpm build`                         | the interface into `internal/webui/dist/`, then the wyo server into `bin/wyo` |
| `pnpm precommit`                     | the whole local gate — main agent only                                        |

**Never** use bare `npx`/`vitest`/`eslint` — always the `pnpm` script or `pnpm exec`.

**DO NOT use `pnpm test`** (watch mode; it hangs agents on interactive prompts).

**DO NOT pass `--` to these scripts** (e.g. `pnpm run test:headless -- src/lib/forms`) — it silently runs the _entire_ suite instead of the
filtered subset.

### Workflow when fixing failures

1. Fix one file at a time with `pnpm test:headless <failing_file>`. Do not run the whole suite between every edit.
2. When the individual files pass, the main agent runs `pnpm precommit` once if the machine allows it, commits, pushes and watches the pull
   request's run. A failure there is fixed locally, with focused runs of the failed files, amended and pushed again.

**Before the first push of a plan**, run locally every test file the plan added or changed — unit and browser, including a spec that was
only edited and including a unit test for each new source file. No GitHub Actions run is a place to find out whether one test passes.

---

## 5. Test quality rules

### 5.1 Specific assertions

```typescript
expect(buttons.length).toBeGreaterThan(0); // ❌ lazy
expect(element).toBeTruthy(); // ❌ lazy
expect(buttons).toHaveLength(2); // ✅
expect(screen.getByText('Submit')).toBeInTheDocument(); // ✅
```

Assert the DOM as actually rendered, not as assumed.

### 5.2 Selectors

- Prefer `data-test` attributes; add them to the component when needed.
- No jsdom-unsupported pseudo-selectors (`:not()`, `:hidden`, `:visible`).
- Escape Tailwind classes: `.md\\:hidden`.
- Scope ambiguous queries with `within(container)` or `container.querySelector('#id')` rather than
  indexing into `getAllByRole(...)[0]`.

### 5.3 Real data, hard-coded assertions

- Data from a JSON file → import the real file. Data from the database → mock it.
- Assert **hard-coded** expected values, so an accidental change to a data source fails the test.
- Before changing an existing assertion, `git diff` the data source. If it changed unintentionally,
  ask — do not re-point the assertion at the new value.

### 5.4 Silence expected console output

Any test that deliberately drives a `console.error`/`warn`/`log` path must spy on it, suppress it,
**and assert the message**. Never leave it unhandled; never suppress without asserting.

```typescript
// an illustration of the shape, not a file of this repository
it('reports a network failure to the log, without the key', async () => {
    const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    sendMock.mockRejectedValue(new TypeError('Failed to fetch'));

    await expect(saveKey(SAVED_KEY)).resolves.toEqual({ ok: false, code: 'network' });
    expect(consoleSpy).toHaveBeenCalledWith('saveKey: the request failed:', 'Failed to fetch');
    expect(JSON.stringify(consoleSpy.mock.calls)).not.toContain(SAVED_KEY);
    consoleSpy.mockRestore();
});
```

Hoist the spy into `beforeEach`/`afterEach` when 2+ tests in a block trigger the same method. Don't
add a spy to a test that triggers nothing — an unused spy is dead code. Verify with
`pnpm test:headless --reporter=verbose <file>`; a clean file has zero `stderr |`/`stdout |` blocks.

### 5.5 File organisation

- Co-locate tests with the code under test.
- Test files must not start with `+` (breaks SvelteKit routing): `page.svelte.test.ts`, not
  `+page.svelte.test.ts`.
- Naming decides the Vitest project — see §2.

### 5.6 Test dates at midday, never at midnight

A date-only string (`'2026-10-01'`) parses as midnight UTC. Formatting it renders the previous
calendar day in every timezone west of Greenwich, so a test asserting the rendered date passes in
one place and fails in another. Any other midnight instant behaves the same way: wherever the local
day has not started yet, the test sees the wrong day.

- Write test dates at noon, in local time, with no offset: `'2026-10-01T12:00:00'`. It renders the
  same calendar day in every timezone on Earth.
- This applies to every date fed to `formatDate`/`formatDateTime`, to any component that formats
  dates, and to any test that compares calendar days, month boundaries or "today" / "overdue" logic.
- Do not rely on the machine's timezone. If the code under test depends on "now", freeze it with
  `vi.useFakeTimers()` / `vi.setSystemTime(...)` at a midday instant too.
- The same rule is recorded in `.specify/memory/constitution.md`, in principle II; the Go side states it in the `go-testing` skill.

### 5.7 Browser tests run side by side, against the built wyo server, with no fixed waits

Browser tests (`e2e/specs/*.spec.ts`, Playwright) run against the **BUILT** wyo server (`pnpm build`), never against the Vite dev
server, and only ever call the wyo server and the fakes. `pnpm test:e2e` starts, for every worker, **a wyo server of its own in test
mode** (the Go build tag `wyotest`) with **its own empty data folder and its own port** (from 8800 up) and the two fakes
(`cmd/wyofakes`: the fake Wildberries and the fake Ozon on one address, and the fake fulfilment centre), so workers share nothing. A new browser test therefore:

- **works in its worker's data folder by default** and never assumes another test's rows; a test that needs rows makes them through the wyo
  server's own API or the fakes' control calls, and cleans up what it leaves;
- **uses its own key on the fakes**, never a shared default state; the fakes keep a state per key, and `/__control/reset` empties it;
- **calls no address but the wyo server's own and the fakes'** — never a real marketplace, never a real fulfilment centre (rule R3). The
  suite has one test that fails when any request leaves for another address;
- **never waits a fixed time:** no `waitForTimeout`, no `networkidle`; wait for the condition (the row, the status, the hydrated page, the
  answer of the request: `waitForResponse`); the suite runs with the visitor's «less motion» setting (`reducedMotion: 'reduce'`), so an
  animation needs no pause;
- **selects by `data-test`,** never by a Tailwind class or a position;
- **stays under about 2 minutes per file:** a file with independent tests is split into files; a serial story stays one file;
- **measures layout only here,** never in jsdom: the screens table (`e2e/support/screens.ts`) has one row for every page, and the screen
  sweep measures them (constitution, «Every screen is mobile-first»).

---

## 6. Server code is Go

The wyo server is Go. Its handlers, its database code, its adapters and its fakes are tested by the `go-testing` skill
(`.agents/skills/go-testing/SKILL.md`): real SQLite files in a temporary folder, the fakes instead of real marketplaces, injected time,
captured logs, the key canary, the router's table walked over every path and method. This skill does not repeat them. The interface has
no server side of its own: nothing in `src/` reads a file, a database or the environment (constitution, principle V).

What the interface tests CAN prove about the wyo server's API is its **contract**: the types in `src/lib/api/wyo-api.d.ts` are generated
from `api/wyo.yaml`, a test stubs the one sending function of `src/lib/api/` and answers with the contract's shapes, and a refusal with
`{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }` must land on the right field (`form-error.ts` and the
refusal-to-fields function).

---

## 7. Component testing with @testing-library/svelte

```typescript
import { render, screen, fireEvent } from '@testing-library/svelte';

it('calls onClick when clicked', async () => {
    const handleClick = vi.fn();
    render(Button, { props: { text: 'Click me', onClick: handleClick } });

    await fireEvent.click(screen.getByRole('button'));

    expect(handleClick).toHaveBeenCalledTimes(1);
});
```

Test behaviour and rendered output; never internal state.

### For SWE-family models: NO, you cannot refuse to test Svelte 5 reactive behaviour

**If you are about to write an excuse like:**

> "Testing Svelte 5's `$state` reactive changes in jsdom is limited — the button click doesn't
> properly trigger the state change in component tests."

**STOP. THIS IS FALSE. IT IS ABSOLUTELY POSSIBLE TO TEST SVELTE 5 COMPONENTS.**

Clicking a button that mutates internal `$state`, causing a re-render, revealing `{#if}` blocks and
updating hidden input bindings, is a **standard, fully-supported** pattern with
`@testing-library/svelte`. There is **no jsdom limitation** for internal component state. None.

- `@testing-library/svelte` officially supports Svelte 3, 4 and 5 —
  [github.com/testing-library/svelte-testing-library](https://github.com/testing-library/svelte-testing-library)
- Runes testing patterns (`$state`, `$derived`, `$effect`, `flushSync`, `untrack`) are documented at
  [Sveltest — Runes Testing](https://www.sveltest.dev/docs/runes-testing)

**The ONLY real documented limitation** ([issue #394](https://github.com/testing-library/svelte-testing-library/issues/394))
is that `$bindable` props passed _into_ a component don't reflect back to the test's outer scope.
That is about prop binding, not about `let x = $state(...)` inside the component.

**A real failure is almost certainly one of these mundane bugs:**

- Ambiguous selectors (`getByRole` matching 2+ elements — scope with `within()` or an id)
- Querying the wrong element by DOM order (`getAllByRole('combobox')[0]` grabbing an unrelated `<select>`)
- Forgetting `await` before `fireEvent.*`
- Forgetting `flushSync()` when asserting `$derived` **values directly** in `.test.svelte.ts` files
  (not needed when asserting via rendered DOM — `render`/`fireEvent` already flush)

**When stuck, in order:** the testing-library README → the Sveltest runes docs → the
[DOM query docs](https://testing-library.com/docs/queries/about) → search the exact error plus
"svelte 5". If still stuck, say so and ask — do **not** invent a framework limitation to skip a test.

---

## 8. Coverage

Enforced by `pnpm coverage:enforce` (`scripts/coverage-check.cjs` for the interface, `tools/covercheck` for Go) in **two tiers, both hard**:

1. **Aggregate.** The code must stay at or above the threshold.
2. **Every measured file.** Every file in the coverage report must reach the threshold. Pages,
   components and scripts included — there is no exempt category.

The interface's threshold lives in exactly one place, the `THRESHOLD` constant in `scripts/coverage-check.cjs`; the Go side's, in the
constant of `tools/covercheck`. Read it there; do not hard-code the figure in a test, a comment or a document.

**There is no warning tier.** A tier that reports files but never fails the build does not hold a line: in the maintainer's other repository
eleven files sat below the bar for weeks, one of them at 33%, while every build passed.

**Coverage is a floor, not a target.** Do not add tests solely to raise a percentage. That rule
has NOT changed — what changed is the remedy. A file below the threshold is one of exactly two
things:

- **Missing a test for behaviour someone would notice** — an error path, a guard, a refusal, a
  feature toggle, a branch that changes what an office user or a floor worker sees. Write it.
- **Genuinely untestable** — no surface a test could assert anything real about. Then exclude the
  whole file in `coverage.exclude` in `vitest.config.ts`, in the same commit, with a written
  reason saying what a test would be asserting and why that is worthless. "It is environment-dependent" is not a reason — that exact
  phrase once hid two fully tested files. Generated code is excluded as generated.

Tick a number up with an assertion that proves nothing and you have made the suite slower and the
gate a liar at the same time. Exclude the file instead, and say why.

**When a file is under the bar, find out WHY before writing a test.** A gap in a real behaviour —
an error path, a guard, a refusal, a feature toggle — is a defect; fix it. A `?? ''` fallback or a
genuinely unreachable defensive branch is not, and it no longer has a warning to hide in either: it
has to be restructured so the branch is reachable, proven unreachable in the PR and left for the
aggregate to absorb, or — if the whole file has no testable surface — excluded with a reason. What
is forbidden is the third option people reach for: a test that asserts the fallback exists.

The gate compares the **exact** percentage. It used to round first, so a file at 89.66% reported as
90 and cleared a 90 gate — do not reintroduce that.

**A Svelte component reading low is often the compiler, not the tests.** v8 maps
several statements to a single source line in compiled Svelte, so statement coverage systematically
under-reports components. A small component can read well under the floor with every behaviour tested:
every uncovered statement but one is a duplicate entry on a line that IS covered, and the one left is the
`let { … }: IProps = $props()` destructure, which runs on every render but which v8 never counts as run.

Before writing a test to move a component's number, check whether its uncovered statements sit on
lines that are already covered:

```bash
node -e "const m=require('./coverage/coverage-final.json')[process.argv[1]];
const cov=new Set();for(const[i,c]of Object.entries(m.s))if(c>0)cov.add(m.statementMap[i].start.line);
for(const[i,c]of Object.entries(m.s))if(c===0&&!cov.has(m.statementMap[i].start.line))
console.log('real gap, line',m.statementMap[i].start.line);" "$PWD/src/lib/components/ui/Example.svelte"
```

If it prints nothing, every uncovered statement sits on a line that IS covered: the file is as
covered as the compiler allows and a test cannot move it. That is the one case for
`coverage.exclude` on a component — record this command's empty output as the written reason, so
the next reader can re-run it rather than take your word for it.

---

## 9. Visual analysis with MCP Playwright

Analyse a component in a real browser before writing tests that depend on its DOM.

**Dev server:** `pnpm dev` starts the wyo server in development mode (`127.0.0.1:8790`), the two fakes (the fake Wildberries on `8791`, the
fake fulfilment centre on `8792`) and Vite (`127.0.0.1:5190`, passing `/api` on to the wyo server). Open `http://127.0.0.1:5190`; the page
`/showcase` draws every component of the library in each of its states (it exists only in a wyo server built for development and tests).

1. If it is running, use it.
2. If it runs old code — started before a change of `api/wyo.yaml` or of the Go code, so it holds the old server — stop the processes you
   started and run `pnpm dev` again.
3. If nothing listens on 5190, start it with `pnpm dev` in background mode.
4. **Never start a second server on another port.** Never stop a dev server that was already running before this session, except to restart
   it as in item 2.

Commands: `browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_resize`,
`browser_take_screenshot`.

**Cleanup** — stop only a server you started, and stop it as soon as you no longer need it (the dev servers and a browser together hold
more than 2 GB): find the process with `ss -ltnp 'sport = :5190'`, then `kill <PID>`. The bash tool's command id is not the process id.

---

## 10. Debugging failures

1. jsdom has no CSS — media queries never apply (§1.5).
2. **jsdom lays nothing out, so every element reports zero size and zero position.** See §10.1.
3. `await` every `fireEvent`; use `waitFor` for async settling.
4. Check the mock is the shared one (§3) and matches the real module's signature.
5. Resolve selector ambiguity by scoping, not by indexing.
6. A failing test is a fact about the program: find the cause first (constitution, principle II).

### 10.1 Nothing has a size or a position in jsdom

jsdom parses HTML and builds a DOM. It never lays the page out. There are no boxes, so every
element in every test reports that it occupies no space on screen, whether it is visible, hidden,
or does not exist at all:

| Call                                               | Real browser                       | jsdom, always                                 |
| -------------------------------------------------- | ---------------------------------- | --------------------------------------------- |
| `el.getClientRects()`                              | one rect per box the element draws | empty list                                    |
| `el.getBoundingClientRect()`                       | the element's real box             | all zeroes                                    |
| `el.offsetWidth` / `offsetHeight` / `offsetParent` | real values                        | `0`, `0`, `null`                              |
| `window.getComputedStyle(el).display`              | what CSS resolved to               | the inline value only — no stylesheet applies |

**What this breaks.** Any code that asks "is this element actually on screen?" answers "no" for everything under test. A typical case is
a control that exists twice — once for a wide screen, once for a phone — where the code picks the copy with a non-empty
`getClientRects()`. Under jsdom that is neither copy, so the code finds nothing, and a test that expected a picked element fails for a
reason that has nothing to do with the code being tested.

**What to do.** Give the element a size for the duration of the test. This is a stub of the
BROWSER, not of the code under test, so it does not violate the "never mock the thing you are
testing" rule:

```typescript
beforeEach(() => {
    // jsdom performs no layout, so every element reports zero client rects and any
    // "is this on screen?" check answers no. Treat anything attached to the document as visible.
    vi.spyOn(Element.prototype, 'getClientRects').mockImplementation(function (this: Element) {
        return (this.isConnected ? [{}] : []) as unknown as DOMRectList;
    });
});
```

**What NOT to do.** Do not soften the production code so it passes without the stub — "treat an
element with no size as visible" is wrong in a real browser, where a zero-size element genuinely
cannot be pointed at. And do not assert real geometry (positions, widths, overlaps) in jsdom at
all: those numbers are fabricated. Geometry belongs in Playwright (`e2e/`), where a real viewport
exists — the same rule §1.5 states for CSS.

---

## Checklist before committing

**Economy**

- [ ] Each test covers one behaviour; no assertion-per-test splitting
- [ ] At most one `render()` per test
- [ ] Enumerations use `it.each`
- [ ] No locale-pair duplication; no CSS-dependent viewport tests in jsdom
- [ ] No assertions on element size or position in jsdom; visibility checks stubbed per §10.1
- [ ] No tests that only prove the framework works
- [ ] A component is tested once, in its own file; a page's test never re-tests a component

**Correctness**

- [ ] Written before implementation, and I saw it fail
- [ ] Specific assertions; real data sources; hard-coded expected values
- [ ] Data-source changes verified with `git diff` before touching assertions
- [ ] `data-test` selectors, correctly scoped
- [ ] Shared fixtures from `$test/support/` — no hand-rolled texts, form or page mocks
- [ ] Correct filename so the file lands in the right Vitest project (§2)
- [ ] Console output spied, suppressed and asserted; verified with `--reporter=verbose`
- [ ] Test dates are at midday in local time (`'2026-10-01T12:00:00'`), never date-only or midnight (§5.6)
- [ ] No test was changed to make it pass; if one was, the cause was shown and said in the report

**Browser tests**

- [ ] Runs against the built wyo server and the fakes only; its own key on the fakes; no fixed waits
- [ ] A new page has its row in the screens table

**Gates**

- [ ] My own files pass `pnpm test:headless <file>` and `pnpm exec eslint <file>`; no whole suite, no type check run by a subagent or on
      a limited machine
- [ ] The main agent's `pnpm remote:gate` passes on the Mac mini
- [ ] Manual browser verification done where the interface changed
- [ ] Dev servers started this session have been stopped; pre-existing ones left running
