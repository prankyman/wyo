# The repository of the marketplace connector made ready for development: the rules for AI agents, the three testing skills and the constitution carried over from the maintainer's other repository and rewritten for a Go server with a SvelteKit interface; the Go and interface tools with their strict checks; the component library with its own, more colourful theme; the fake Wildberries and the fake fulfilment centre; the neutral fulfilment-centre API; the pipeline in GitHub Actions

> **Plan 1** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). It is **Stage 0** of the high-level plan `.agents/plans/Marketplace_connector_plan.md`.
> Written 2026-11-17 from `origin/main` at `ab8838f` (one commit: the high-level plan and the marketplaces' saved pages).
> Research: `.agents/research/wildberries-official-description-files-check-2026-11-17.md` (cited «WBC»); the earlier research the
> high-level plan cites as «MC», «FA», «WBA», «WBF», «WBP» is kept by the maintainer outside this repository.
>
> **The plans that follow.** Stage 1 of the high-level plan is three plans (the maintainer, 2026-11-17: «3»), each releasable alone:
> Plan 2 `.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`,
> Plan 3 `.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`,
> Plan 4 `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`.
>
> **Branch** `feat/repository-set-up`. One plan = one branch = one squashed commit = one pull request. The commit is `feat:` (it
> makes the first release, 0.0.1) — never `feat!:` and never a `BREAKING CHANGE:` footer: nothing exists yet that could break.

**Every AI agent implementing this plan follows the `AGENTS.md` principles — global (`~/.agents/AGENTS.md`) and this repository's own
(`AGENTS.md`, which this plan creates in its first stage) — and uses this repository's testing skills for all test work as soon as
section C has created them: `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the interface, `go-testing`
(`.agents/skills/go-testing/SKILL.md`) for Go, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the
main agent runs `pnpm precommit`, whole-codebase checks and full test suites; a subagent runs only its own test files and lints only
its own changed files; at most 3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a
component and is reused".

## Terms used below

- **The connector**, **wyo** — the program the high-level plan describes. Its parts are always named: **the wyo server** (the Go
  program, one file named `wyo`), **the wyo interface** (its pages in a browser, built by SvelteKit and carried inside the wyo
  server), **the installer** (the script of Plan 2). Never "the app".
- **The maintainer** — the person who runs this project. **The maintainer's other repository** — the earlier project whose rules,
  skills, constitution and component library this plan carries over. Its name and place are never written in this repository
  (rule R1 of the high-level plan); the implementing agent is told them outside it.
- **The fake Wildberries**, **the fake fulfilment centre** — two small Go programs that answer as Wildberries and as a fulfilment
  centre's system would, with made-up data, so that no test ever calls a real one.
- **The neutral fulfilment-centre API** — the data format of rule R2: orders in, labels in, statuses and marking codes back, hand-over
  documents.
- **A description file** — an OpenAPI file: a machine-readable list of an API's calls, fields and limits, from which code is generated.
- **A policy test** — a unit test that reads the project's own source files and fails the build when a rule is broken (a second copy
  of a component, a colour written by hand, a marketplace named outside its adapter).
- **The gate** — the whole set of checks a commit must pass: secret scan, commit-message check, formatting and lint, type checks, every
  test with its coverage floor, the build, the browser tests.
- **The register** — `.agents/backlog.md`: every request of the maintainer that is not built yet and every decision waiting for him.

## The maintainer's request and answers (2026-11-17) — nothing is open

- «Create, based on the [high-level plan] stage 0 and stage 1 plans. Don't forget to copy all the rules and skills and constitution
  from [the other] project, and then modify them according to technologies used here (GoLang, for example) so the coding is done in
  very similar way.»
- «I want you to follow all the best practices collected over the months of coding [the other] project - everything is a component,
  strict linting and checking, TDD, etc. Reusability and testability is a must!»
- «I want you to select a slightly different UI theme, so the project visually and in style did not look too similar to [the other]
  project. Let it be a bit more colorful but the same UI library must be used.»
- «I want you to reuse as much functionality as possible from [the other project] as well, as some things probably are already
  implemented [there].» — «When I work with you here in this project, I want the same feel as if I work in [the other project].»
- Stage 1 as one plan or three: «3». The numbers are Plan 1 (this one) and Plans 2, 3 and 4.
- The interface's languages: «Russian only. But do create the languages capability.»
- Where the whole gate runs: «GH Actions? Are they free?» — they are, for a public repository on GitHub's standard machines
  (`https://docs.github.com/en/billing/concepts/product-billing/github-actions`, read 2026-11-17: «The use of standard GitHub-hosted
  runners is free: In public repositories»); then «If the GH Actions passes fine for you - not a problem!»
- Pull requests: «No, I want you to merge». The maintainer put a working token into the development machine's environment
  (`GH_TOKEN`, set in `~/.profile`; checked 2026-11-17: it reads the repository, its pull requests, its pipeline runs and its secrets).
- Instructions: «Be very thorough with instructions on how to install it and use it, most probably - in README.md, unless you have a
  better proposition.» — section H.

## Today (2026-11-17, `origin/main` at `ab8838f`)

- The repository holds two things: `.agents/plans/Marketplace_connector_plan.md` and `docs/marketplaces/` (31 saved pages and
  description files of the marketplaces). No code, no `AGENTS.md`, no licence file, no pipeline.
- The development machine (`server`: 4 processor cores, 15 GB of memory — a **limited** machine by the rule of section B) has
  Node 24.21.0, pnpm 12.10.1 and `gitleaks`; it has **no** Go, no C compiler (`gcc`), no Docker and no `gh` command. `sudo` works
  without a password there.
- **The "same UI library" is the other repository's own component library**, not a library from outside: about 66 components with a
  test file beside each, on Svelte 5 and Tailwind CSS 4, with icons from `phosphor-svelte`. Its colours are written into each component
  as Tailwind palette classes (a warm grey, a black main button); it has no theme file and no dark mode.
- That library was written for pages rendered on a server with SvelteKit form actions. The wyo interface is built as static files and
  has no SvelteKit server side (high-level plan, «What it is built of»), so the few components that touch form actions or a
  server-given language need the small changes section E lists.
- The other repository has **no** Wildberries client and **no** fake Wildberries. Both are written new, from Wildberries' own
  description files, which were downloaded on 2026-11-17 and still hold every method the high-level plan needs (WBC §2).
- The format of the neutral fulfilment-centre API exists: the maintainer provides it as a guide and a description file (outside this
  repository). It names no marketplace in its own text: a marketplace is a code inside the data.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (section B).
2. **The tools are installed on the development machine**, each by its official instruction, read on the day: Go (the newest stable
   release, 1.27.4 on 2026-11-17 by `https://go.dev/VERSION?m=text`; the official archive with its checksum checked), `gcc` (Go's
   detector of data races, `go test -race`, needs a C compiler), and the `gh` command. Approving this plan approves these three
   installations.
3. The token works: `gh auth status` with `GH_TOKEN`, and a pull request of an empty test branch can be opened and closed.
4. **A generator turns Wildberries' description files into Go code that compiles.** The default is `oapi-codegen`
   (`https://github.com/oapi-codegen/oapi-codegen`, v2.8.2 on 2026-11-17), limited to the methods of WBC §2. If it cannot read the
   files, the second choice is `ogen` (`https://github.com/ogen-go/ogen`); if neither can, the types of those methods are written by
   hand and a unit test compares their field names with the files. The choice is recorded in the constitution.
5. **The interface built as static files works inside the Go program:** `@sveltejs/adapter-static` with a fallback page and server
   rendering switched off (`https://svelte.dev/docs/kit/adapter-static`), embedded with Go's `embed` and the `all:` prefix
   (`https://pkg.go.dev/embed` — files whose names begin with `_`, such as SvelteKit's `_app` folder, are skipped without it). Proven
   by a Go test: the start page, a file under `_app/` and an unknown page address are all served; an unknown `/api/…` address is not.
6. **The form library works without a SvelteKit server:** `sveltekit-superforms` in its single-page mode with the `zod` 4 browser
   adapter (the library's documentation, read on the day), proven by one form of the showcase page (section E) whose server is a stub.
7. **A language prefix works in static files:** an address `/xx/…` of a second language reaches the same page as `/…` through
   SvelteKit's `reroute` hook in the browser (the documentation of `hooks`, read on the day), proven with a test-only second
   language. If it cannot be done, stop and ask.
8. **Tailwind CSS 4 theme variables give the classes section F needs** (`bg-brand-600`, `text-ink-800`, `bg-ok-50`): its
   documentation on `@theme`, read on the day, and one built page.
9. The versions of the interface's packages are **the ones the other repository has on the day of copying** (on 2026-11-17: Svelte 5.57,
   SvelteKit 2.70, Vite 8, Tailwind CSS 4.3, Vitest 5, Playwright 1.63, TypeScript 6, ESLint 10, `zod` 4.6, `sveltekit-superforms` 2.30,
   `phosphor-svelte` 3.1, Node 24), so that every copied component and test behaves as it does there. Newer versions are a later plan.
10. GitHub's standard machine `ubuntu-24.04` runs the whole gate, the browser tests included, in under 20 minutes. Measured on the
    first run and written into `AGENTS.md`.

## A. What is carried over, what is rewritten, what is left behind

The other repository's rules were read in full for this plan. Its `AGENTS.md` (135 lines), its constitution (7 001 lines, of which
about 1 250 are not about its own subject), its testing skill (833 lines) and its smoke-testing skill (1 933 lines) are the sources.

| Source                                    | Carried over as it is                                                                                                                                                                                                                                                                                                                                                                | Rewritten for this stack                                                                                                                                                                                                                                                        | Left behind                                                                                                                 |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| The rules for agents                      | Where agent material lives; the register and its three duties; "name the program"; "everything is a component and is reused" (five points); the rule about reading long files; documentation first; `pnpm` and the scripts of `package.json` only; no explicit `any`; no command over 2 minutes; heavy checks only by the main agent; at most 3 subagents                              | The test flow of every plan (section B); the migration rule (worded for the wyo server's own migrations, Plan 2); the rule R1 and its check before every push                                                                                                                    | Everything about the other project's machines for remote runs, its cluster, its database tools and its other programs       |
| The constitution                          | Test-first with its two coverage tiers; test economy; everything is a component; every rule paired with a policy test and a reason; the forms' behaviour; press feedback and single submission; one window component; one vertical gap; mobile-first at 390 × 844 with 44-pixel tap targets and the measured screens table; the nine help-text rules; commit messages; the plan implementation workflow; the agent interaction rule; governance | Language (one language now, the machinery kept — section G); where code lives (Go packages and the interface's folders); the database rules (Plan 2); "the browser holds no secret and makes no outside call"; rules R2–R7 of the high-level plan as the first principles     | Its 50 principles about its own subject; its accounts, roles and companies; its seed and staging rules; its deployment      |
| The testing skill                         | Components tested once; test economy; the two Vitest projects chosen by file name; shared test fixtures; test quality rules (specific assertions, `data-test`, real data, console output spied and asserted, dates at midday); coverage; no fixed waits in browser tests; the checklist                                                                                                | The commands; the part about server code becomes a pointer to the new Go skill; browser tests run against the built wyo server, one data folder and one port per worker                                                                                                           | Its tenant and capability probes; its fixtures for form actions and for its database                                        |
| The smoke-testing skill                   | The purpose; the `data-test` convention; passwords never in a tool call; the checks after every screen (console errors, raw text keys, sideways scroll at 390 pixels); two browser contexts for two people; human-only checks reported as waiting, never as passed; the report's format and place; closing every browser; "a smoke test finds and reports, it does not fix"             | Starting the wyo server with the fakes; Linux commands; two new steps: with «External API» empty no request leaves for anything but the fake marketplaces (R3), and a key shows only its last four characters (R4)                                                               | Its 1 300 lines of walks through the other project's screens                                                                |
| The Go testing skill                      | —                                                                                                                                                                                                                                                                                                                                                                                    | **New** (section C): the Go counterpart of every rule above                                                                                                                                                                                                                    | —                                                                                                                           |

**Rule R1 governs every copy.** A copied file loses every plan number, date-and-name attribution and comment that points at the
other project; "the maintainer" stands where a name stood. The dated reasons that make the rules convincing are kept in neutral words
("on 2026-11-07 the same bank fields had been drawn four times, each with its own tests"). The check of section J runs before the push.

## B. `AGENTS.md`, `CLAUDE.md` and the test flow of every plan

`AGENTS.md` (and `CLAUDE.md`, seven lines that point to `~/.agents/AGENTS.md` and to `AGENTS.md`, as in the other repository) holds:

1. **Where agent material lives:** `.agents/skills/<name>/SKILL.md`, `.agents/plans/` (each file named after what the plan implements),
   `.agents/reports/`, `.agents/research/`, `.agents/backlog.md` (the register), `.specify/memory/constitution.md` (the same place as
   in the other repository, so that the habit carries over).
2. **The constitution is read before any code, plan or task.**
3. **The register** — read before any plan; a request the current plan does not build is added in the same session with the date and
   the maintainer's words; an entry is closed, never deleted.
4. **Name the program** — the terms of this plan.
5. **Rule R1 and its check** (section J), and rules R2–R7 by reference to the constitution.
6. **Testing** — which skill for which work; `pnpm precommit` as the one whole local gate; never `pnpm test` (it waits for changes
   and never ends); never `--` before a script's arguments.
7. **The test flow of every plan.** At the start `nproc` and `free -g` are read: fewer than 6 processor cores, or 16 GB of memory or
   less, is a **limited** machine; anything bigger is a **big** machine.
    - **Focused runs first, on any machine.** The change is made test-first; then exactly the files that changed are run: ESLint on
      the changed files, `pnpm test:headless <file>` for an interface test file, `pnpm go:test <package>` for one Go package,
      `pnpm test:e2e e2e/specs/<file>.spec.ts` for one browser test file. Every new source file has its unit test beside it.
    - **The whole local gate, `pnpm precommit`, once, by the main agent, as the final local step** — and on a limited machine only
      while it ends in under 10 minutes and `free -g` shows at least 4 GB available; when it no longer does, the limited machine keeps
      to focused runs and the pull request's run is the first whole gate (the plan that meets this writes the measured time here).
    - **The pull request's run in GitHub Actions is the gate on a clean machine.** The work is committed with `git commit --no-verify`
      (later `--amend --no-verify --no-edit`), pushed, the pull request opened, the run watched to its end. A failure is fixed
      locally with focused runs of the failed files, amended and pushed again (`git push --force-with-lease --no-verify`, the plan's
      own branch only). **A run in GitHub Actions is never used to find out whether one test passes.**
    - **The smoke test** runs locally against the built wyo server and the fakes, one browser.
8. **Tooling and commands**, **everything is a component and is reused**, **reading files**, **documentation first** — as carried over.
9. **A commit of documents only** ends its subject with `[skip ci]` and is made and pushed with `--no-verify`; only the secret scan
   runs for it.

## C. The three skills

- **`sveltekit-testing`** — the other repository's skill with the changes of the table in section A. The commands become this
  repository's (section D). It keeps the rule about Svelte 5 reactivity being testable, the procedure for a component whose coverage
  reads low only because of its `$props()` line, and the checklist before committing.
- **`go-testing`** — new, written before the first Go test of this plan. Its rules, each the counterpart of an interface rule:
    1. Tests first, failing first; how to show the failure (the failing output goes into the progress note); `t.Skip` is never a fix.
    2. One behaviour per test, enumerations as table-driven tests with named cases (the counterpart of `it.each`); more than about
       10 new cases in one table is a question to ask oneself.
    3. `x_test.go` beside `x.go`; the test uses the package's public behaviour (package `x_test`) unless that is impossible.
    4. **No stand-in for the database:** every test that needs one opens a new SQLite file in `t.TempDir()`, applies the real
       migrations and uses the real code (Plan 2 brings the database; the rule is written now).
    5. **No real marketplace and no real fulfilment centre, ever:** the fakes of section I, started in the test's own process.
    6. **Time is given, never taken:** code reads the time from an injected clock; tests use fixed moments at midday in a named time
       zone; the linter forbids `time.Now` outside the clock package and `time.Sleep` in tests.
    7. **Logs are captured and asserted**, and a canary test proves a key never reaches a log line, a journal row, an answer of the
       wyo server's own API or a request to a fulfilment centre.
    8. The router's table of who may call what is tested by walking every registered path with every method.
    9. `go test -race -shuffle=on -count=1`; a parser of anything a marketplace sends has a fuzz test; expected files live in
       `testdata/` and are rewritten only by an explicit flag, and their difference is read before committing.
    10. Coverage: the script of section D; an exclusion needs a written reason.
    11. The commands table and "never change what a test checks to make it pass", word for word from the global rules.
- **`smoke-testing`** — the harness of the other repository's skill with the changes of the table in section A.
- `skills-lock.json` names the three skills as local.

## D. The tools and the checks

One `package.json` at the root holds every command, as in the other repository; Go code lives in `cmd/`, `internal/` and `tools/`,
the interface in `src/`, `static/`, `messages/` and `e2e/` — the same places as in the other repository, so that copied files keep
their paths.

| Command                          | What it runs                                                                                                                                                                                                 |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pnpm tools:install`             | Installs the Go tools at the versions written once in `scripts/tools/versions.sh`, each with its checksum, into `.tools/bin` (not committed). The pipeline runs the same script.                              |
| `pnpm dev`                       | The wyo server in development mode and the fakes, then Vite on its own port, passing `/api` on to the wyo server.                                                                                             |
| `pnpm generate`, `generate:check` | Makes the generated code (section I and the wyo server's own API); the second fails when a committed generated file differs from a fresh one.                                                                |
| `pnpm lint`                      | ESLint with `--max-warnings 0` and Prettier as a lint error; `gofumpt -l` printing nothing; `golangci-lint run`; `go vet`.                                                                                    |
| `pnpm check`                     | `svelte-check`, the browser tests' `tsc`, and `go build` for `linux/amd64` and `linux/arm64` with `CGO_ENABLED=0`.                                                                                            |
| `pnpm test:headless [file]`      | Vitest once, both projects, or one file.                                                                                                                                                                      |
| `pnpm go:test [package]`         | `go test -race -shuffle=on -count=1` for the named package, or for `./cmd/... ./internal/... ./tools/...`.                                                                                                    |
| `pnpm test:coverage`, `coverage:enforce` | Both sides with coverage; then the two floor scripts.                                                                                                                                                 |
| `pnpm build`                     | The interface into `internal/webui/dist/`, then the wyo server into `bin/wyo` with its version stamped.                                                                                                       |
| `pnpm test:e2e [file]`           | Playwright against the built wyo server started in test mode with the fakes; one data folder and one port per worker.                                                                                         |
| `pnpm precommit`                 | `generate:check`, `lint`, `check`, `test:coverage`, `coverage:enforce`, `build` — in this order, stopping at the first failure.                                                                              |
| `pnpm format`, `release`         | Prettier and `gofumpt` over the source folders; `commit-and-tag-version`.                                                                                                                                     |

- **Formatting and lint, interface side:** the other repository's `.prettierrc` (4 spaces, 120 columns, single quotes, no trailing
  comma) and the skeleton of its `eslint.config.js`: unused names allowed only with a leading underscore, no explicit `any`, a file
  over 800 lines is a failure (test files exempt), and two "one way in" rules — `fetch` may be called only inside `src/lib/api/`,
  and the form library may be imported only by the project's wrapper `src/lib/forms/super-form.ts`.
- **Lint, Go side:** `golangci-lint` (v2.15.0 on 2026-11-17) with a committed configuration that is strict from the first commit:
  unchecked errors, `staticcheck`, `gosec`, closed response bodies, requests with a context, exhaustive `switch`, misspellings; a
  `//nolint` must name its linter and its reason; `forbidigo` forbids `time.Now` outside the clock package, the default HTTP client,
  `fmt.Print*` and reading the environment outside the configuration package; a file over 800 lines fails (tests and generated code
  exempt).
- **Coverage floors — two tiers, both hard, on both sides.** The interface keeps the other repository's script: the whole and
  **every file** at or above one constant in `scripts/coverage-check.cjs`. Go gets the same two tiers from a small Go program,
  `tools/covercheck`, which reads the coverage profile of `go test -coverpkg` and computes each file's share of covered statements;
  its constant lives in that program only. Both start at the other repository's figure for its interface. No document repeats the
  figures; a file is excluded only with a written reason beside its entry; generated code is excluded as generated.
- **Architecture tests in Go** (`internal/archtest`, ordinary tests that read the import graph): generated packages are imported only
  by their one owner; nothing under `internal/` imports the fakes except tests and `cmd/wyofakes`; later plans add their own rows
  (an adapter never imports another adapter; only the fulfilment-centre client knows the neutral API).
- **Hooks (husky), as in the other repository:** `pre-commit` — the secret scan of the staged files (`gitleaks protect --staged
  --redact`), then `pnpm lint`; `commit-msg` — `commitlint` with the conventional rules, lines of at most 100 characters;
  `pre-push` — `pnpm test:coverage && pnpm coverage:enforce`. The plan workflow commits and pushes with `--no-verify`, because the
  gate does the hooks' work.
- **Secret scan:** `.gitleaks.toml` on gitleaks' default rules; an exception is made by value, never by a whole file kind.
- **Versions pinned:** `.nvmrc` (Node 24), `packageManager` in `package.json`, the `go` and `toolchain` lines of `go.mod`,
  `scripts/tools/versions.sh`.
- **The small files of the other repository, cut to what applies here:** `.gitignore`, `.gitattributes`, `.npmrc`,
  `.prettierignore`, `.commitlintrc.json`, `.versionrc.json`, `.vscode/extensions.json`.

## E. The component library — copied once, with its tests and its policy tests

Everything is a component and is reused: **no screen of any plan writes a piece this library already has.** The components keep their
names, props and test files; each copy passes the changes below and rule R1.

- **Copied as they are** (only colours changed, section F), each with its `.svelte.test.ts` / `.test.ts`: `Button` with `press.ts`,
  `IconButton`, `IconTooltip`, `HelpTooltip`, `HelpLinkList`, `ButtonWithHelp`, `BlockedButton`, `DisclosureWithHelp`, `PageHeading`,
  `HeadingWithHelp`, `Stack`, `NoteBox`, `WarningBanner`, `EmptyState`, `BackLink`, `Portal`, `ModalWindow`, `WindowButtons`, `Field`,
  `FieldLabel`, `FieldRefusal`, `TextBox`, `SelectBox`, `FormButtons`, `FormAnswer`, `StatusBadge`, `StatusBadgeWithHelp` with
  `status-tones.ts`, `PrevNextLinks`, `ClickableCard`, `ClickableRow`, `StretchedLink`, `CopyButton`, the toast store with `Toaster`,
  and the helpers `field-classes.ts`, `overlay-a11y.ts`, `dismissable.ts`, `tooltip-position.ts`, `sheet-drag.ts`,
  `reduced-motion.svelte.ts`, `form-guard.svelte.ts`, `page-change.svelte.ts`, `types.ts`, `cn.ts`, `clipboard.ts`.
- **Copied with a named change:**
    - `GuardedForm` — its branch for SvelteKit form actions is removed; it keeps the form library's branch, the single submission,
      the busy state of its buttons and the confirmation window.
    - `src/lib/forms/super-form.ts`, `answer.ts`, `section-form.ts` — the form library in its single-page mode: a form is checked in
      the browser by its `zod` schema and sent as JSON through `src/lib/api/`; the wyo server checks it again and is the authority.
      Its refusal `{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }` is laid onto the form's fields by one function.
    - `ConfirmationModal`, `SlideOverPanel`, `RowDetailsPanel`, `CollapsibleSection`, `StepStrip`, `Tabs`, `Pagination`,
      `FilterSection`, `RemovableChip`, `FilteredListNote`, `NumberField` with `number-input.ts`, `Textarea`, `FieldMessage`,
      `SubmitWithAnswer`, `SectionForm`, `DisclosureForm`, `DisclosureClose`, `PageChangeGuard`, `PopupCloseButton`, `AttentionIcon`,
      `ShowOnceSecret` (its one import from outside the library is brought inside) — the language comes from the page's address in
      the browser (section G), no longer from a server.
- **Policy tests copied with the components** (each allow-list emptied of the other project's entries): `modal-window-policy`,
  `window-buttons-policy`, `note-box-policy`, `empty-state-policy`, `stack-policy`, `form-control-policy`, `field-label-policy`,
  `number-field-policy`, `form-buttons-policy`, `page-width-policy`, `disclosure-form-policy`, `section-form-policy`,
  `collapsible-section-policy`, `press-feedback-policy`, `status-tones-policy`, `select-arrow-policy`, `page-help-policy`,
  `help-placement-policy`, `helper-duplication` (one home file per helper) and `test-helper-duplication` (one home per test fixture).
- **Not copied yet — copied by the plan that first needs one, never rewritten:** the searchable picker (`Combobox`, the only user of
  the package `bits-ui`), `FilePicker`, `CodeNameSelect`, `DateRangeFields`, `TickAllBox`, `FileFormatButtons`, `StepsCard`,
  `RowButtonForm`, `ReasonDialog`, the list component `DataList` with `ColumnHeading` and `ColumnLegend` (Plan 3), the shell parts,
  the sign-in parts and the notification bell (Plan 2). `docs/library-layout.md` lists what is in the library and what waits.
- **Test set-up copied:** the two Vitest projects chosen by file name (`*.svelte.test.ts` in a browser-like environment, everything
  else without one), the stand-ins for SvelteKit's `$app/*` modules, the `$test` alias, the shared fixtures that are not about the
  other project's subject (the texts fixture, the form fixtures for the single-page mode, `renderPageWith`).
- **`src/app.html`** keeps the two start-up scripts that are plain browser code: the guard that refuses a second submission before
  the interface's code runs, and the page-change bar (in the theme's colour). A form sent before the code runs is stopped, not posted:
  static files have nothing to post to.
- **The showcase page** `/showcase` draws every copied component in each of its states under the new theme. It is how the
  maintainer judges the theme, and it is the page the browser tests measure at 390 pixels. It exists only in a wyo server built for
  development and tests (the Go build tag `wyotest`, which `pnpm dev` and `pnpm test:e2e` set); a released wyo server answers
  "not found" at its address, and a Go test of the release build proves it.

## F. The theme — one file, visibly not the other project's

The other project is warm grey with black buttons, capital letters on buttons and badges, and 4-pixel corners. wyo:

| What                  | wyo                                                                                                                                             |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Brand colour          | Indigo: the main button, links, the chosen menu item, the focus ring, the page-change bar, the informational tone                                 |
| Neutral colour        | Cool grey (Tailwind's `slate`) for text, borders and the page background                                                                         |
| The top bar           | Filled with the brand colour, white text and icons (the other project's bar is white)                                                            |
| Tones of statuses     | Done — emerald; waiting — sky; attention — amber; stopped — red; information — the brand colour; switched off — grey. One file decides them.     |
| A marketplace's mark  | A small coloured dot beside each marketplace's name; the colour's name comes from the marketplace's adapter (Plan 3), never from a screen         |
| Letters               | Buttons and badges in ordinary letters, medium weight (not capitals with wide spacing)                                                           |
| Corners               | 6 pixels on boxes, buttons and cards                                                                                                             |
| Unchanged on purpose  | The system font (no font is fetched from anywhere, rule R3); 44-pixel tap targets; the 16-pixel gap between blocks; every size and behaviour     |

- **All colours are named once**, in the `@theme` block of `src/app.css`: `brand`, `ink` (the neutral), `ok`, `wait`, `warn`, `stop`,
  each as a scale. Components write `bg-brand-600`, `text-ink-800`, `bg-ok-50` — never a Tailwind palette name and never a colour code.
  Changing the theme later is an edit of that one block.
- **`theme-policy.test.ts`** fails a palette class (`slate-`, `indigo-`, `stone-`, `red-`, …) or a colour code anywhere under `src/`
  outside `src/app.css`, with an allow-list that starts empty.
- A disabled control is dimmed to half, a busy button to 60 %, as in the other project. No dark mode: it was not asked for.

## G. Language: Russian only, with the machinery for more

- The other repository's own small text system is copied (it uses no outside library): `t(key, lang)`, texts in `messages/*.json`
  found by pattern, `{name}` placeholders, plural and number and date helpers, `form-error.ts`.
- **One file, `messages/ru.json`.** `DEFAULT_LOCALE = 'ru'` is the only language written in code. A second file, say
  `messages/en.json`, added later makes that language exist with no code change: its pages answer under `/en/…` (premise 7), and the
  language switch (`LanguageSwitcher`, copied by Plan 2 with the top bar) appears by itself, because it is drawn only when more than
  one language exists.
- **The wyo server never writes a sentence for a person.** It answers codes with parameters; every sentence is in `messages/`. So a
  second language never needs a change in Go.
- Tests: `message-bundles.test.ts` (every key used in `src/` exists in every file; every file has the same keys),
  `locale-argument-policy.test.ts` (no language written by hand), and the prefix and the switch tested with a second language that
  exists only in the tests. No test asserts the same text in two languages.
- Dates and numbers are formatted with the language's code, never with a region written by hand.

## H. The documents

- **`LICENSE`** — Apache-2.0. **`README.md`** — in Russian, because the people who install wyo are Russian-speaking sellers, with one
  English paragraph at the top saying what the program is. Plan 1 writes: what wyo does, that it "can work with external fulfilment
  centres", what it never does (R3, R4, R5), its state ("not installable yet; the installer comes with the next release"), and the
  developer's start. **The seller's instructions are written by the plans that make them true** — installing and updating by Plan 2,
  connecting Wildberries by Plan 3, «External API» by Plan 4 — each time in full, step by step, for a person who is not an engineer,
  by the constitution's help-text rules. `readme-policy.test.ts` (from Plan 2 on) fails when the README names a command, an address
  or a menu item that does not exist.
- **`docs/development.md`** (English): prerequisites, daily commands, where things are, how the fakes are driven.
- **`docs/library-layout.md`**: the library's components and what waits (section E).
- **`.agents/backlog.md`** — the register, started with: Stages 2–6 of the high-level plan; the checks of the first real account
  (WBC §7); the trade-mark check the maintainer set aside; "more people who sign in" (the maintainer, 2026-11-17: «What more
  people? … Is this a sought-after functionality?» — left out until sellers ask); the components not copied yet.
- **`.specify/memory/constitution.md`**, version 1.0.0, short on purpose (the other repository's grew to 7 001 lines because every
  plan appended its details; here a plan's details go to `docs/`). It opens with the change log in the other repository's format.
    1. The project in ten lines; the first principles are rules R1–R7 of the high-level plan, each with its test (R1 by the check of
       section J; R2 by `reference-policy.test.ts` of section I; R3–R6 by tests the plans that build them name).
    2. Language (section G). Test-first, the two coverage tiers, test economy, "never change what a test checks".
    3. Everything is a component and is reused, with "a rule is not finished until a policy test enforces it".
    4. **The browser holds no secret and calls nothing but the wyo server; every write is a JSON call; the wyo server answers codes.**
    5. Marketplace facts only from official documents with the address and the date (rule 7 of the high-level plan).
    6. Development standards: forms; press feedback and single submission; windows; one vertical gap; mobile-first, tap targets and
       the screens table; accessibility; help texts and the place of a «?»; file size; naming (component files in `PascalCase`, other
       interface files in `kebab-case`, Go by Go's own convention). Where the other text contradicted itself, one answer is written:
       branches are `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`; the whole gate runs once at the end, not after every change.
    7. Where code lives (section K). Commit messages. **The plan implementation workflow** in ten steps: branch; implement; the test
       flow of section B; the smoke test; the gate; one commit; push; the pull request on GitHub, merged by the agent with a merge
       commit; the release run watched to its end; the report that starts with Done or Not Done. Carrying an approved plan through
       these steps is the maintainer's approval to commit, push, open and merge.
    8. The non-negotiable agent interaction rule, word for word. Governance and the version line.

## I. The fakes and the neutral fulfilment-centre API

- **`docs/fulfilment-centre-api.md` and `api/fulfilment-centre.yaml`** — the specification of rule R2, in English, written in this
  repository's own words from the format the maintainer provides, naming no product and no marketplace. This plan writes the parts
  Stage 1 uses; the stock and supply parts are added by the stages that use them (high-level plan, «Stages 1–6»).
    - **How a call is made:** the key in `Authorization: Bearer …`; JSON; times in UTC; lists by `limit` and `cursor`; a refusal is
      `{ "error": { "code", "params"?, "issues"? } }`; the documented limits (300 requests a minute, bodies of 1 MB, files of 5 MB).
    - **The calls:** `GET /api/v1/account`, `GET /api/v1/warehouses`; `POST /api/v1/orders` (accepted, held with reasons, or a repeat
      recognised by the sender's own order number), `GET /api/v1/orders`, `GET` and `PATCH /api/v1/orders/{id}`,
      `POST …/cancel`, `POST …/reduce`, `PUT` and `DELETE …/parcels/{number}/label`, `PUT` and `DELETE …/problem`;
      `GET /api/v1/held-orders`, `GET` and `DELETE /api/v1/held-orders/{id}`; `GET /api/v1/events`; `GET /api/v1/handovers`,
      `GET /api/v1/handovers/{id}`, `PUT …/documents`, `PUT` and `DELETE …/problem`.
    - **The events** the fulfilment centre reports (`order.accepted`, `order.held`, `order.reserved`, `order.picking_started`,
      `order.picked`, `order.packing_finished` with each unit's marking code, `order.label_attached`, `order.parcel_closed`,
      `order.packed`, `order.reopened`, `order.reduced`, `order.change_waits`, `order.cancelled`, `order.handed_over`,
      `handover.requested`, `handover.request_withdrawn`, `handover.completed`, `ping`), read with `GET /api/v1/events` or received as
      signed notifications: the Standard Webhooks scheme (`https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md`),
      with its published test vector as a unit test.
    - **`reference-policy.test.ts`** fails when the specification names a marketplace. **A one-time comparison**, made by the
      implementing agent outside this repository, shows that every path, field and list of values equals the format the maintainer
      provided; its outcome is one line of the report.
- **Generated code, never edited by hand:** from `api/fulfilment-centre.yaml` the types, a client and a server skeleton
  (`internal/fulfilment/fcapi`); from Wildberries' files, committed under `api/wildberries/` with the day they were read, the types
  and a client for the methods of WBC §2 (`internal/marketplace/wildberries/wbapi`); from `api/wyo.yaml` — the wyo server's own API,
  which in this plan has one call, `GET /api/v1/meta` (name, version, mode) — the server skeleton and, with `openapi-typescript`,
  the interface's types in `src/lib/api/`. A new Wildberries file is a difference in a committed file, read before it is accepted.
- **The fake kit** `internal/fakes/fakekit` — written once, used by both fakes: a state of its own for every test (chosen by the key
  the test sends), `/__control/health`, `/__control/reset`, `/__control/calls` (every received call without its key), and ordered
  faults (answer 429 with the limit headers, answer 500, answer slowly).
- **The fake fulfilment centre** `internal/fakes/fulfilment` implements the generated server skeleton of the neutral API: it takes
  orders in, holds one whose product a test marked unknown, and on a test's command through `/__control/…` moves an order through
  reserved, packed with marking codes, reopened, cancelled, asks for hand-over documents, and sends the signed notifications.
- **The fake Wildberries** `internal/fakes/wildberries` answers the methods of WBC §2 in the files' own shapes: `/ping`, the seller's
  information, warehouses, new orders, states, supplies, stickers (a small real picture of 58 × 40 mm), marking codes with verdicts a
  test chooses, transport boxes, the shipping point and method, the closing of a supply with each of its documented refusals, the QR
  code. It keeps Wildberries' budget (300 requests a minute, an error counted as 10) so that the synchronisation engine of Plan 3 is
  tested against the real arithmetic. Test orders are made through `/__control/orders`.
- **`cmd/wyofakes`** starts both fakes on `127.0.0.1` for `pnpm dev`, the browser tests and the smoke test. It is never part of a
  release.

## J. The check of rule R1 before every push

The list of words is kept by the maintainer outside this repository and is never copied into it. Before every push the agent
searches, from outside the repository, the whole tree (`git grep -n -i -e <word> …` over every tracked file) and every commit message
not yet on `origin/main` (`git log origin/main..HEAD --format=%B`) for each word of the list, and the push happens only when nothing
is found. `AGENTS.md` describes the step without the list. The report of every plan states that the check ran and found nothing.

## K. Code — where things live

```
cmd/wyo/                      the wyo server's entry point (this plan: serve the interface and /api/v1/meta, print the version)
cmd/wyofakes/                 both fakes for development and browser tests
internal/webui/               the embedded interface (dist/ is built, not committed) and the handler that serves it
internal/httpapi/             the wyo server's own API; gen/ is generated from api/wyo.yaml
internal/version/, clock/, config/, logging/
internal/marketplace/wildberries/wbapi/    generated from api/wildberries/*.yaml
internal/fulfilment/fcapi/    generated from api/fulfilment-centre.yaml
internal/fakes/fakekit/, fulfilment/, wildberries/
internal/archtest/            the architecture tests
tools/covercheck/             the Go coverage floors
api/                          wyo.yaml, fulfilment-centre.yaml, wildberries/*.yaml
src/lib/components/ui/        the component library            src/lib/forms/, validation/, i18n/, api/, stores/, utils/
src/routes/                   the start page and /showcase     src/test/ (support, harnesses, mocks)
messages/ru.json              e2e/ (specs, helpers, fixtures)  scripts/ (coverage-check.cjs, tools/, e2e/)
docs/, .agents/, .specify/, .github/workflows/ci-cd.yml, .husky/
```

Components this plan creates: none of its own design — it brings the library of section E and adds the theme tokens.
Components it reuses: every one of section E.

## L. The pipeline — `.github/workflows/ci-cd.yml` on GitHub Actions

- **On every pull request and every push to `main`:** a first job reads the last commit's subject and ends the run when it holds
  `[skip ci]`. The gate job, on `ubuntu-24.04`: the secret scan of the pushed commits (a pinned gitleaks with its checksum, findings
  hidden), `commitlint` over the same commits, `pnpm tools:install`, `pnpm install --frozen-lockfile`, then `pnpm precommit` and
  `pnpm test:e2e`. The reports of the tests and of the coverage are kept with the run.
- **On a push to `main` whose commits since the last version tag hold a `feat` or a `fix`:** the release job runs
  `commit-and-tag-version` (the other repository's `.versionrc.json`: features and fixes shown, chores hidden), commits
  `chore(release): <version> [skip ci]`, tags `v<version>`, pushes both, builds `wyo_linux_amd64` and `wyo_linux_arm64` and
  `checksums.txt`, and publishes them as a GitHub Release whose text is the version's part of `CHANGELOG.md`.
- **The signature of a release is added by Plan 2**, together with the installer and the updater that check it, so that a signature
  is never made by one plan and first checked by another. This moves one clause of the high-level plan's Stage 0, step 4.
- Nothing is pushed to `main` while a release job runs.

## M. Tests

Written before the code they check, failing first; the skill of section C for each kind.

- **Go:** the handler of `internal/webui` (premise 5); `GET /api/v1/meta`; the fake kit (state per key, reset, the call log without
  keys, each fault); the fake fulfilment centre against the generated client of the neutral API, call by call, and its signature
  against the published test vector; the fake Wildberries against the generated Wildberries client, method by method, with its budget
  and each documented refusal; `tools/covercheck` on a made-up profile (a file under the floor fails, an excluded file without a
  reason fails); the architecture tests.
- **Interface unit tests:** every copied component's own test file, green under the new theme; the policy tests of section E with
  empty allow-lists; `theme-policy`; `message-bundles`; `locale-argument-policy`; the language prefix and the switch with the
  test-only language; the refusal-to-fields function; `reference-policy`.
- **Browser tests:** the start page shows the name and the version the wyo server reports; the showcase page at 390 × 844 and at 640,
  768, 1024 and 1280 pixels — no sideways scroll, every tap target at least 44 × 44 pixels and uncovered, the 16-pixel gap between
  blocks — with the sweep's own test that proves it still catches a planted defect; no request leaves for any address but the wyo
  server's own (R3); the screens table has a row for every page.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. `AGENTS.md`, `CLAUDE.md`, the three skills, the constitution, the register, `LICENSE`, the README.
3. The tools, the commands, the hooks, the two coverage scripts, the architecture tests. 4. `cmd/wyo` with `internal/webui` and
   `GET /api/v1/meta`; the interface's skeleton, the text system, the theme. 5. The component library with its tests and policy tests,
   in batches of components that do not depend on one another; the showcase page. 6. The neutral specification and the generated
   code. 7. The fake kit, the fake fulfilment centre, the fake Wildberries. 8. The browser tests and the screens table. 9. The
   pipeline. 10. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill): the built wyo server started with the fakes; the start page and the showcase page
  opened at a computer's width and at 390 × 844, a screenshot of each for the maintainer; the console checked; no request to any
  other address.
- **Gate:** the test flow of section B; the check of section J; squash; push; the pull request; the merge by the agent; the release
  run watched to its end (the first release, 0.0.1, with its two files and `checksums.txt`); `main` pulled; the report.

## Not in this plan

- **The signature of releases, the installer, the database, sign-in, HTTPS, updates** — Plan 2.
- **Wildberries keys, the adapter, the synchronisation engine, the order list, the journal** — Plan 3. **«External API» and the
  exchange** — Plan 4.
- **The stock and supply parts of the neutral API** — the stages that use them (high-level plan, Stages 2 and 5).
- **Machines for remote runs** like the other project's: GitHub Actions is the clean machine; nothing else is set up.
- **A second language's texts, a dark theme, a fetched font:** not asked for.
- **Newer versions of the interface's packages** than the other repository's: a later plan, so that this one changes one thing at a
  time.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and, once stage 2 has written it, this repository's `AGENTS.md`; read the constitution and the
   register before the first edit after that.
2. Use the testing skills for every test: `sveltekit-testing`, `go-testing`, `smoke-testing`. Tests first, failing first; never
   change what a test checks to make it pass; a copied test that fails under the new theme or the new stack has its cause found first.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each with its own list of files, each told these rules.
4. Everything is a component and is reused: a component is copied once, with its test, and never written again; a piece missing from
   the library is copied from the list of section E, not invented. Every subagent prompt repeats this rule.
5. Rule R1 binds every file and every commit message; the check of section J before every push. Rules R2–R7 bind every file.
6. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about a marketplace
   is written only from its official documents, with the address and the date.
7. Every visible text is a key in `messages/ru.json`, written by the help-text rules; no sentence in Go.
8. Name the program every time: the wyo server, the wyo interface, the fake Wildberries, the fake fulfilment centre.
9. One branch, one commit, no `!` and no `BREAKING CHANGE:`; a progress checklist after each stage; the final report starts with
   Done / Not Done.

## Estimate (inference, not a sourced fact)

The largest part is the component library: about 60 components and 20 policy tests to copy, re-colour and make green — mechanical
work that three subagents can share by component groups. The two fakes are the largest new code (about 30 Wildberries methods and
20 calls of the neutral API). The rules, skills and constitution are writing, not code.
