<!--
Sync Impact Report:
- Version change: 1.4.0 → 1.5.0 (MINOR) — Plan 6
  (`.agents/plans/wyo-ozon-fbs-exchange-with-a-fulfilment-centre-codes-packing-labels-shipment.md`): a rule widened, nothing
  removed. Principle XII: the exchange's flow is chosen by the parts an adapter claims (a marketplace that confirms an order, or one
  whose label exists only after the packing is reported), a marketplace may declare that a second sending of codes replaces the
  first, a batch is made to hold exactly the hand-over's orders and no others, an adapter declares which of the six hand-over steps it
  needs, and a label file is fetched without the key and only from the domain the adapter declares; principle IX: that domain is
  listed in the one list, as a wildcard entry (`*.ozone.ru`, https only, never the bare domain).
- Earlier: 1.3.0 → 1.4.0 (MINOR) — Plan 5
  (`.agents/plans/wyo-ozon-key-reading-adapter-orders-with-several-products-fake-ozon.md`): principle VI names Ozon's
  description file, the comparison of its later versions and the corrected temporary copy its generator reads (the
  maintainer, 2026-11-18: «Yes, fix them in a temporary copy, use the generator. Fix whatever must be fixed.»); principle IX —
  the marketplaces' hosts are declared by the adapters and listed by the gate in one place; principle X — keys of several
  fields packed by one function, order lines, the marketplace's own deadline, the adapter's declared limit headers and
  hosts, its own refusals and notes, the contract suite run for Ozon; principle XI — one lock per account, an order written
  only when something of it changed; rules R5 and R6 name their tests for every adapter; «Where code lives» gained
  `internal/marketplace/ozon`, `internal/adapterkit`, `internal/specfix`, `tools/specfix`, `internal/fakes/ozon`, `api/ozon/`.
- Earlier: 1.2.0 → 1.3.0 (MINOR) — Plan 4
  (`.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`): principle
  XII (the exchange with a fulfilment centre: alone by default, one client, never a blind write, never a cancellation at a
  marketplace) added with its tests; rule R6's test of the neutral API's one client is built; principle IX lists the fulfilment
  centre's purpose; «Where code lives» gained `internal/fulfilment`, `internal/exchange`, `internal/handover`.
- Earlier: 1.1.1 → 1.2.0 (MINOR) — Plan 3
  (`.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`): principles X (one adapter
  contract, one contract test suite) and XI (one synchronisation engine: one caller, one budget, one journal) added with their tests;
  rules R5 and R6 name their tests; the updater's copy of the database is made while the wyo server runs (principle VIII).
- Earlier: 1.1.0 → 1.1.1 (PATCH) — the maintainer, 2026-11-18: «Yes, I want the change!» (no second wait of about
  7 minutes after a merge) — a push to main no longer reruns the gate and the installer job on files their pull request's run already passed;
  «The pipeline» describes it, and the release's signing and installer steps of Plan 2.
- Earlier: 1.0.0 → 1.1.0 (MINOR) — Plan 2
  (`.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`):
  principles VIII (a seller's database is upgraded in place, for ever) and IX (every outgoing call goes through one gate) added with
  their tests; rule R3's Go test named; help-text rule 9's menu-place tokens arrived with the menu; «Where code lives» gained the
  packages of the database, the sign-in, the release and the installer.
- Earlier: none → 1.0.0 — the first version (Plan 1,
  `.agents/plans/wyo-repository-set-up-agent-rules-skills-constitution-tools-component-library-theme-fakes-fulfilment-centre-api.md`;
  the maintainer, 2026-11-17: «Don't forget to copy all the rules and skills and constitution from [the other] project, and then modify
  them according to technologies used here (GoLang, for example) so the coding is done in very similar way.» and «everything is a
  component, strict linting and checking, TDD, etc. Reusability and testability is a must!»). It is short on purpose: the maintainer's
  other repository's constitution grew to thousands of lines because every plan appended its details to it. Here a plan's details go to
  `docs/`; this file holds only rules that bind every plan.
- Principles added: R1 to R7 of the high-level plan as the first principles, each with the test that enforces it; I Language (Russian
  only, the machinery for more); II Test-first development with two coverage tiers on both sides; III Everything is a component and is
  reused; IV A rule is not finished until a policy test enforces it; V The browser holds no secret and calls nothing but the wyo server;
  VI Facts about a marketplace come only from its official documents; VII Language discipline of the code (TypeScript and Go).
- Carried over from the maintainer's other repository, rewritten for a Go server with a SvelteKit interface: test economy; forms; press
  feedback and single submission; one window component and the window's buttons; one vertical gap; mobile-first at 390 × 844 with
  44-pixel tap targets and a measured screens table; the nine help-text rules; commit messages; the plan implementation workflow; the
  agent interaction rule; governance. Left behind: its principles about its own subject, its accounts, roles and companies, its seed and
  staging rules, its deployment, its remote machines.
- Where the other text contradicted itself, one answer is written: branches are `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`; the
  whole gate runs once at the end of a plan, not after every change.
- Templates requiring updates: None
- Follow-up TODOs: the policy test of rule R7 (the register).
-->

# wyo Constitution

## The project

**wyo** is a free, open-source marketplace connector: a program a marketplace seller installs on a server the seller owns or rents.
After the seller types the marketplaces' own keys, it periodically reads the seller's orders from the marketplaces, shows them in one
list and, only when its setting «External API» is filled, exchanges orders, labels, statuses and marking codes with a fulfilment
centre's system through a neutral, documented format. It is written in Go (the wyo server, one file) with a SvelteKit interface (the wyo
interface) carried inside that file. The marketplaces are Wildberries and Ozon; more follow (`.agents/plans/Marketplace_connector_plan.md`,
«Stages»).

- **The parts, always named so:** the wyo server; the wyo interface; the installer (Plan 2); the fake Wildberries and the fake
  fulfilment centre (`cmd/wyofakes`). Never "the app".
- **The people:** the seller owns the server and types the keys; the owner is the person who signs in; the maintainer runs this project
  and decides its questions. A fulfilment centre's staff never install the program for a seller and never hold a seller's key.
- **The repository** is `https://github.com/prankyman/wyo`; the licence is Apache-2.0.
- **Plans** are in `.agents/plans/`; the register of requests and open decisions is `.agents/backlog.md`; research is in
  `.agents/research/`; a plan's details (screens, tables, calls) go to `docs/`, not to this file.

## Core Principles

### R1. No fulfilment product is named (NON-NEGOTIABLE)

No file, screen, text, example, test, commit message or package name of this repository names a fulfilment product, its website, its
domains or its owner. It also names neither the maintainer's other repository nor the words of that repository's own business. The
program's description says what it does and that it «can work with external fulfilment centres» (в том числе может работать с внешними
фулфилмент-центрами).

- **Test:** the check before every push (`AGENTS.md`, «The rules R1 to R7»): from outside the repository, every word of a list the
  maintainer keeps outside it is searched in every tracked file and in every commit message not yet on `origin/main`. The report of
  every plan states that the check ran and found nothing.
- A file copied from the maintainer's other repository loses every plan number, every date-and-name attribution and every comment
  pointing at the other project. "The maintainer" stands where a name stood. A dated reason is kept in neutral words («on 2026-11-07 the
  same bank fields had been drawn four times»).

### R2. The fulfilment-centre API is neutral (NON-NEGOTIABLE)

The format the program speaks to a fulfilment centre's system is described in `docs/fulfilment-centre-api.md` and
`api/fulfilment-centre.yaml`, in this repository's own words, as a specification any fulfilment centre's system may implement. It names
no product and no marketplace: a marketplace is a code inside the data. «External API» takes any address and any key.

- **Test:** `reference-policy.test.ts` fails when the specification names a marketplace or a product.
- Only the fulfilment-centre client (`internal/fulfilment/`) knows this format; an architecture test keeps it so (R6).

### R3. Alone by default (NON-NEGOTIABLE)

With «External API» empty the program talks only to the marketplaces. It never sends anything anywhere else — no usage statistics, no
update check that carries data about the installation, no font or script fetched from another site. A test, a development run and a
pipeline run never call a real marketplace or a real fulfilment centre either: the fakes answer.

- **Tests:** the browser test «no request leaves for any address but the wyo server's own» (Plan 1); a Go test of every outgoing call's
  address (principle IX, `internal/outbound`); the smoke test's step «with External API empty no request leaves for anything but the
  fake marketplaces» (`smoke-testing` skill).

### R4. The seller installs it and types the keys (NON-NEGOTIABLE)

The program runs on a server the seller owns or rents, and only the seller's own people sign in. A marketplace key never leaves that
server: it is not sent to a fulfilment centre, not written to a log, not sent to the browser after it was saved — the interface shows
only the last four characters. Wildberries' own rule forbids handing the Personal token to third parties or using it in cloud services
and allows it in the seller's own programs deployed on the seller's own or rented infrastructure; the documents tell a fulfilment
centre's staff never to install the program for a seller or hold a seller's key.

- **Tests:** the canary test of the `go-testing` skill (a key never reaches a log line, a journal row, an answer of the wyo server's own
  API or a request to a fulfilment centre); the interface test that a key is drawn as its last four characters (the plan that builds the
  keys' screen names it); the smoke test's step on the last four characters.

### R5. Nothing is hidden from a marketplace (NON-NEGOTIABLE)

The program names itself and its version in every request, keeps inside each marketplace's documented request limits and waits as the
marketplace's answer tells it. No request disguising, no proxies, no rotation of addresses. Polling moments are spread at random only so
that many installations do not ask in the same second.

- **Tests:** `TestEveryRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader` (`internal/sync`, against the fake Wildberries) and
  `TestEveryOzonRequestNamesTheProgramAndCarriesTheKeyOnlyInItsHeader` (against the fake Ozon),
  `TestAnHourOfReadingsOf500OzonPostingsNeverExceedsOzonsLimit` (50 requests a second per Client ID) and
  `TestAnHourAtFullLoadNeverExceedsWildberriesBudget` (`internal/sync`: the engine driven for a made-up hour at full load against the
  fake's own arithmetic — 300 requests a minute, an error counted as 10); the outbound gate's marketplace purpose uses no proxy
  (`internal/outbound`).

### R6. One contract per side (NON-NEGOTIABLE)

One marketplace-adapter contract that every marketplace implements, one synchronisation engine, one fulfilment-centre client. Nothing
marketplace-specific outside its adapter. Every piece of interface used twice is one component (principle III).

- **Tests:** the architecture tests in `internal/archtest` (generated packages are imported only by their one owner; nothing under
  `internal/` imports the fakes except tests and `cmd/wyofakes`; `TestEveryAdapterStandsAloneInThisRepository` — an adapter never
  imports another adapter and makes no request itself; `TestEveryMarketplaceIsNamedOnlyByItsAdapterInThisRepository` (a table of every adapter);
  `TestOnlyTheFulfilmentPackagesAndTheFakeFulfilmentCentreMayImportItsClient` — only the fulfilment-centre client knows the neutral
  API; `TestNoFolderOfTheExchangeNamesAMarketplaceInThisRepository`); `src/lib/marketplace-name-policy.test.ts` — no file under `src/` names a marketplace
  outside the message keys `marketplace.<code>.*`; the component policy tests of principle III.

### R7. Free and open (NON-NEGOTIABLE)

Apache-2.0, no paid edition, no licence key. `LICENSE` is the Apache License 2.0 text with the copyright line «Copyright 2026 The wyo
authors». No code asks for, checks or sells a key to unlock a feature.

- **Test:** a policy test that `package.json` and every other manifest name the same licence is on the register (follow-up in the Sync
  Impact Report); until it exists, a pull request that adds a manifest or a feature switch is reviewed against this rule.

### I. Language: Russian only, with the machinery for more (NON-NEGOTIABLE)

- The interface is in **Russian only**; the machinery for more languages exists and is kept working. `messages/ru.json` is the one
  language file; `DEFAULT_LOCALE = 'ru'` is the only language written in code. A second file, say `messages/en.json`, makes that
  language exist with no code change: its pages answer under `/en/…` (SvelteKit's `reroute` hook in the browser), and the language
  switch appears by itself, because it is drawn only when more than one language exists.
- **Every visible text is a key** in `messages/`, found by `t(key, lang)`, with `{name}` placeholders and the plural, number and date
  helpers of `src/lib/i18n/`. No sentence is written in a component, a script or Go.
- **The wyo server never writes a sentence for a person.** It answers codes with parameters
  (`{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }`); every sentence is in `messages/`. A second language never
  needs a change in Go.
- Dates and numbers are formatted with the language's code, never with a region written by hand.
- **Tests:** `message-bundles.test.ts` (every key used in `src/` exists in every file; every file has the same keys),
  `locale-argument-policy.test.ts` (no language written by hand), and the prefix and the switch tested with a second language that
  exists only in the tests. No test asserts the same text in two languages.

### II. Test-first development (NON-NEGOTIABLE)

Tests are written BEFORE the code they check and fail first: red, green, refactor. Showing the failure is part of the work (the failing
output goes into the progress note). All tests pass before review. A test of a component uses only its public behaviour — props,
callbacks, rendered output — and introduces no new logic, no deprecated feature and no lint error. `t.Skip`, `it.skip`, a retry or a
loosened assertion is never a fix.

**Coverage is enforced in two tiers on both sides, both hard gates**, by `pnpm coverage:enforce`:

1. **Aggregate.** The code stays at or above the floor.
2. **Every file.** Every file the coverage report measures is at or above the floor. No category of file is exempt for being a page, a
   component or a script.

**Each floor is declared in exactly one place:** the interface's in the constant in `scripts/coverage-check.cjs`, the Go side's in the
constant in `tools/covercheck` (a small Go program that reads the coverage profile of `go test -coverpkg` and computes each file's share
of covered statements). No document repeats the figures, so changing a floor is a one-line edit and never an amendment.

- **A file that cannot be meaningfully tested is removed from measurement, never left below the bar.** The exclusion is declared beside
  the tool's configuration, in the same commit, with a written reason naming what makes the file untestable — what a test would assert
  and why that is worthless. "Environment-dependent" is not a reason. Generated code is excluded as generated. A file with one
  untestable branch stays measured.
- There is no third, advisory tier: a warning nobody is obliged to act on is technical debt with a progress bar.
- **Coverage is a floor, not a target.** A test is never added solely to raise a percentage. A file below the floor is exactly one of
  two things: missing a test for behaviour someone would notice (an error path, a guard, a refusal), or a file that belongs in the
  exclusions.

**Test economy (NON-NEGOTIABLE).** A test covers one _behaviour_ and may carry as many assertions as that behaviour needs; splitting one
behaviour across several tests to hold one assertion each is forbidden, as is more than one `render()` per interface test. Enumerations
are `it.each` in the interface and table-driven tests with named cases in Go. More than about 10 new cases in one place is a question to
ask oneself. Responsive layout is a browser-test concern, because CSS does not apply in the unit-test environment. Shared fixtures are
imported from `src/test/support/` and never hand-rolled per file. Test dates are at midday in a named time zone, never at midnight.

**Never make a test pass by changing what it checks (NON-NEGOTIABLE).** A failing test is a fact about the program. You may NEVER change
a test only so that it passes: not its expected values, not what it counts, not its filters, not its waits, not by adding a case that
hides the failure, not by skipping, retrying or loosening it (`toBeGreaterThan(0)` in place of an exact number is loosening).

1. When a test fails, find the CAUSE first. "Flaky" is not a cause; it is a name for a failure whose cause is not known yet.
2. When the cause is not known, say so plainly. Do NOT guess a cause and edit the test to match the guess. Put the evidence (the state
   of the queue, the rows, the times) into the failure message WITHOUT touching the assertions, and reproduce the failure by running the
   test repeatedly.
3. A test may be changed only after the cause is shown, and only when the test itself was wrong about the program's behaviour. Say what
   was wrong with the test, in plain words, and ask the maintainer when the change makes the test weaker.
4. If the program is wrong, fix the program, not the test.
5. Tests are written BEFORE the code they check, and fail first. Read the matching testing skill before writing any test.

Why: on 2026-11-13 an integration test failed once in a whole run; its cause was unknown, and it was changed to count another file's job
so it would pass. The guess about the cause was also wrong. A test that passes without proving the behaviour is worse than no test.

The full standards are in the skills: `sveltekit-testing`, `go-testing`, `smoke-testing` (`.agents/skills/<name>/SKILL.md`).

### III. Everything is a component and is reused (NON-NEGOTIABLE)

Every piece of the interface that appears in more than one place is ONE component in `src/lib/components/<area>/`, used everywhere it
appears and never written again: a group of fields (an address, bank details, a company's legal identity), a card, a table, a button
with its answer, a block of a printed document. The same holds for logic: one function, in a pure module or a package, called from every
place that needs it — never a copy. A page assembles components and passes them its data and actions; a component takes the differences
between places as props (read-only or editable, which lookup it offers, its test-id prefix) instead of being copied with changes. This
holds in Go as in the interface: one function for one job, one table of values, one fixture.

- **Before writing any piece of interface or logic, search for the component or function that already does it, and use it.** When the
  same piece is about to be written a second time, the first one is turned into a component and both places use it, in the same plan.
- **A piece the component library lacks is copied from the maintainer's other repository, never invented.** The plan that first needs it
  copies it with its test (`docs/library-layout.md` lists what is in the library and what waits).
- **A copy found later is a defect:** the plan that finds it turns it into a component and replaces every copy.
- **Tests follow the components:** a component is tested once, in its own test file, in every state and with every prop; a page's test
  checks only what the page adds — which component it draws, with which data, and what its actions do — and never tests a component's
  insides again. When a component replaces copies, the copies' tests are deleted. A browser test walks a component once per flow.
- **Every plan names the components it creates and the ones it reuses**, and its review checks that no piece is written twice.
- **Enforced** by the copied policy tests (principle IV and the list there), `helper-duplication` (one home file per helper) and
  `test-helper-duplication` (one home per test fixture).
- Why: on 2026-11-07 the same bank fields had been drawn four times and the same legal-identity fields four times in the maintainer's
  other repository, each with its own tests — more code, more tests and longer test runs for nothing.

**The column of blocks is `Stack`.** Every page and every stack of blocks is the one `Stack` component, never a hand-written
`flex flex-col` with a gap, `space-y-*` or a block's own margin (see «One vertical gap everywhere»).

### IV. A rule is not finished until a policy test enforces it (NON-NEGOTIABLE)

A **policy test** is an ordinary unit test that reads the project's own source files and fails the build when a rule is broken. A rule
written in this file or in an agent's instructions without such a test is a wish. So:

- A new rule is written together with its policy test, in the same plan, and the rule names the test.
- A policy test's allow-list starts empty and holds only exceptions, each with a written reason; an exception is the complete record of
  what does not follow the rule. A test never holds a list of things "left to convert".
- Where the rule is about Go, the policy test is an ordinary Go test (`internal/archtest`) or a linter rule in the committed
  `golangci-lint` configuration (`forbidigo`, `depguard`).
- The policy tests copied with the component library (Plan 1) are: `modal-window-policy`, `window-buttons-policy`, `note-box-policy`,
  `empty-state-policy`, `stack-policy`, `form-control-policy`, `field-label-policy`, `number-field-policy`, `form-buttons-policy`,
  `page-width-policy`, `disclosure-form-policy`, `section-form-policy`, `collapsible-section-policy`, `press-feedback-policy`,
  `status-tones-policy`, `select-arrow-policy`, `page-help-policy`, `help-placement-policy`, `helper-duplication`,
  `test-helper-duplication`; the new ones are `theme-policy`, `message-bundles`, `locale-argument-policy`, `reference-policy` and the
  architecture tests in Go.

### V. The browser holds no secret and calls nothing but the wyo server (NON-NEGOTIABLE)

- The wyo interface is built as static files (`@sveltejs/adapter-static`, a fallback page, server rendering off) and carried inside the
  wyo server (Go's `embed` with the `all:` prefix). It has no SvelteKit server side: nothing in `src/` reads a file, a database or the
  environment.
- **A marketplace key, a fulfilment-centre key, a password hash or a session secret is never sent to the browser** after it is saved.
  The browser shows at most the last four characters of a key.
- **`fetch` is called only inside `src/lib/api/`** (an ESLint rule), and only for addresses of the wyo server itself. Interface types of
  the wyo server's API are generated from `api/wyo.yaml` with `openapi-typescript` and never edited by hand.
- **Every write is a JSON call** to the wyo server's API, checked there again; the wyo server is the authority and answers codes
  (principle I). The form library may be imported only by the project's wrapper `src/lib/forms/super-form.ts`.
- The Go handler of the interface serves files of `internal/webui/dist/`, answers every other address that does not begin with `/api/`
  with the start page (so a reload on a page works), and answers an unknown `/api/…` address with a `404` and
  `{"error":{"code":"not_found"}}`. The page `/showcase` exists only in a wyo server built for development and tests (the Go build tag
  `wyotest`); a released wyo server answers "not found" at its address, and a Go test of the release build proves it.

### VI. Facts about a marketplace come only from its official documents

A fact about a marketplace — an address, a field, a limit, a rule — is written only from the marketplace's official documents, with the
address and the date it was read; a gap is marked "not documented" and never filled from memory. The documents saved on the day are in
`docs/marketplaces/`. The program's clients are generated from the marketplaces' own description files, committed under
`api/<marketplace>/` with the day they were read, and a new version of a file is a difference in a committed file, read before it is
accepted. Wildberries and the fulfilment-centre API: Go code generated by oapi-codegen v2.8.2 (premise 4 of Plan 1, proven 2026-11-17: it reads Wildberries' files with `include-operation-ids` and `resolve-type-name-collisions`; one package per Wildberries file, because the files share type names). Generated
code is never edited by hand, and `pnpm generate:check` fails when a committed generated file differs from a fresh one.

- **Ozon's file** (`api/ozon/seller-api.json`) is committed exactly as Ozon publishes it; `api/ozon/README.md` gives its address,
  day and checksum, and how a later version is taken: the copy of the day is read as JSON beside the committed file and every
  difference in a method the wyo server calls is read before it is accepted. The file departs from OpenAPI where the generator
  refuses it, so `pnpm generate` corrects a temporary copy (`tools/specfix`, rules in `internal/specfix`; the maintainer, 2026-11-18)
  and generates from that: only descriptions of fields are corrected, never examples, each correction is listed in
  `api/ozon/README.md`, and a kind of departure no rule knows stops the generation
  (`TestOzonsFileGetsExactlyTheCorrectionsItsReadmeLists`).

### VII. Language discipline of the code

- **TypeScript:** strict mode; no explicit `any` (use `unknown`); no implicit `any`; interfaces and types of a folder live in its own
  `types.ts` beside the code that uses them (a type shared across areas lives in the `types.ts` of the folder that owns the concept;
  there is no system-wide `types` folder); unused names only with a leading underscore.
- **Go:** `gofumpt` formatting; `golangci-lint` with the committed configuration, strict from the first commit (unchecked errors,
  `staticcheck`, `gosec`, closed response bodies, requests with a context, exhaustive `switch`, misspellings); a `//nolint` names its
  linter and its reason; `forbidigo` forbids `time.Now` outside the clock package, the default HTTP client, `fmt.Print*` and reading the
  environment outside the configuration package. Time is given, never taken: code reads the time from an injected clock.
- **Both:** a file over 800 lines is a failure (tests and generated code exempt) — split it along its seams; functions are small and
  have one job; no deprecated feature of a package unless a newer one is not available.
- **Versions are pinned:** `.nvmrc`, `packageManager` in `package.json`, the `go` and `toolchain` lines of `go.mod`,
  `scripts/tools/versions.sh`. The interface's packages are at the versions the maintainer's other repository had on 2026-11-17; newer
  ones are a later plan.

### VIII. A seller's database is upgraded in place, for ever (NON-NEGOTIABLE)

A seller's database (`/var/lib/wyo/wyo.db`, SQLite) is never rebuilt: every release upgrades it where it lies, on the seller's own server,
with nobody there to repair it by hand.

- **Migrations** are numbered SQL files inside the program (`internal/store/migrations/`). At start the wyo server applies the ones a
  database lacks, all of one start in one transaction: every pending migration is applied, or none.
- **An applied migration is never edited.** The table `schema_migrations` keeps each one's checksum; a wyo server whose own copy of an
  applied migration differs refuses to start and names it. A schema change is always a NEW numbered file.
- **The upgrade is tested from every released version:** `internal/store/testdata/` keeps a database file of every released version that
  had one, and a test upgrades each to the newest.
- **The wyo server is the only writer.** One connection writes; reads use their own. Nobody edits the file by hand; an agent looks into a
  database only through a copy opened read-only. Queries are typed Go generated by `sqlc` from `internal/store/queries/` (generated code,
  never edited by hand; `pnpm generate:check` compares it).
- **An update stops the wyo server only for the swap.** The updater copies the database before it while the wyo server runs, read-only
  (`store.CopyFile`), so a failed copy never stops the wyo server; a put-back restores that copy. (`internal/release` updater tests.)
- **Secrets are stored encrypted** by the one secret store `internal/secretbox` (AES-256-GCM with the key file `secret.key`, the row's name
  bound into the encryption). A wyo server without its key file refuses to start rather than make a new key over existing secrets.
- **Tests:** the migration tests of `internal/store` (a new database; a second start changes nothing; a changed applied migration is
  refused; a failing migration leaves nothing behind; every `testdata/` database upgrades), the committed `internal/store/migrations.sum`
  with the test that fails when a listed file changes, and the tests of `internal/secretbox`.

### IX. Every outgoing call goes through one gate (NON-NEGOTIABLE)

Every address the wyo server or the updater may call is listed in one place, `internal/outbound`, by purpose: the release address and the
certificate authority (Plan 2), the marketplaces (Plan 3), the fulfilment centre's address only while «External API» is filled (Plan 4,
`TestTheFulfilmentCentresAddressIsListedOnlyWhileTheSettingIsFilled`).
The marketplaces' hosts are declared by the adapters (`marketplace.Adapter.Hosts`), and the wyo server gives the gate the hosts of the
adapters it registers, so the one list is built in one place at start (`TestEveryHostOfEveryAdapterIsOnTheGatesListInARelease`).
A marketplace's file host, from which labels and documents are fetched, is listed the same way, as one wildcard entry: `*.domain`
names every host under that domain, over https on the standard port only, and never the bare domain, another domain that merely ends
with the same letters, or a domain of fewer than two labels (Ozon: `*.ozone.ru`; the adapter's file route also carries that host and
no key). `internal/outbound` hands out the only HTTP clients; a client refuses, before anything leaves, a request to an address outside its
purpose's list, and follows a redirect only inside it. The look for a newer release names neither the domain nor the installed version.
This is rule R3 seen from the Go side.

- **Tests:** `TestACallOutsideTheListIsRefusedBeforeItLeaves` and `TestARedirectIsFollowedOnlyInsideTheList` (`internal/outbound`),
  `TestAWildcardHostOfAnAdapterAllowsExactlyTheHostsUnderItsDomain` (`internal/outbound`); the
  `forbidigo` rule of `.golangci.yml` that forbids `http.Client`, `http.Transport` and `http.DefaultTransport` anywhere but
  `internal/outbound`, tests, the fakes and the test kit; the updater's test that a look sends exactly three plain downloads.

### X. One adapter contract, one contract test suite (NON-NEGOTIABLE)

`internal/marketplace` holds the one list of things every marketplace adapter can be asked to do (`key.check`, `warehouses.read`,
`orders.new`, `orders.states`, `orders.feed`, and the parts of later plans and stages, named). An adapter says which parts it supports;
the wyo interface draws a marketplace's key form, mark and capability-bound columns from `GET /api/v1/marketplaces`, never from a
marketplace's name. Each adapter is one package (`internal/marketplace/<code>`), the only place that knows its marketplace: its generated
client, its hosts, its limit groups with the headers by which its marketplace tells its budget, its states table, its refusals. An
adapter drops a buyer's name, phone, e-mail, address, comment and region at its edge.

- **A key of several fields** (Ozon: the Client ID and the API key) is one text of the secret store, made and read by one function
  (`marketplace.PackKey`, `UnpackKey`); a key of one field is its value, as every release before kept it. `Authorize` is given the
  fields; the last four characters shown come from the secret field only, and a field that is not secret is shown in full.
- **An order is lines:** each product with its quantity (`order_lines`), never a product kept in two places; the marketplace's own
  deadline (`GivesDeadline`, `Order.ShipBy`) wins over the creation plus the account's hours; a split order names the one it came from.
- **A marketplace's own words:** a refusal's or a note's text is looked up first in the adapter's family
  (`marketplace.<code>.refusal.<code>`, `marketplace.<code>.note.<code>`), then in the shared one, by one function
  (`src/lib/marketplaces/refusals.ts`); a shared text holds only neutral words.
- **Shared helpers** of adapters live once in `internal/adapterkit`, never copied into an adapter.
- **Tests:** `RunAdapterContract` (`internal/marketplace/contracttest`) is written once and run for every adapter against its fake: an
  adapter that claims a part passes that part's cases (`TestTheAdapterKeepsTheContractAgainstTheFake` for Wildberries and for Ozon);
  `TestANewOrderIsReadWithoutTheBuyersAddressOrComment`, `TestANewPostingOfSeveralProductsIsOneOrderWithItsLinesAndWithoutTheBuyer`
  and `TestAnOzonKeyStaysInTheApiKeyHeaderAndTheBuyerNowhere`; the packed key's tests (`internal/marketplace`); the architecture
  tests of rule R6.

### XI. One synchronisation engine: one caller, one budget, one journal (NON-NEGOTIABLE)

Every request to a marketplace goes through `sync.Caller` (`internal/sync`): it takes the key from the secret store only for the call,
names the program, keeps the budget of each account and limit group, the 30-second deadline and the repeats (a reading after 1 and 3
seconds on a network or server error; never a client error; never a writing call), refuses a redirect, and writes one row of the journal
(`internal/journal`) for every request — never a key, a query or a body; the journal's table refuses a change of a row and keeps 30 days.
The jobs and their periods live in the engine; a failed job leaves its mark; five failed readings in a row make an account «Нет связи»;
a refused key stops its account's readings. «Обновить» is answered at most once in 5 minutes. The readings of each account are guarded by
a lock of their own, so an account that waits for its budget never holds up another's
(`TestAnAccountThatWaitsDoesNotHoldUpTheReadingsOfAnother`). A reading writes an order only when something of it changed — its state,
its deadline, its lines — so a reading that changed nothing writes nothing but its record of the reading
(`TestAReadingThatChangedNothingWritesNothingButItsRecordOfTheReading`).

- **Tests:** the tests of `internal/sync` (among them `TestAReadingIsRepeatedAfterOneAndThreeSecondsButAClientErrorOrAWriteNever`,
  `TestATooManyRequestsAnswerStopsTheGroupAndTheWaitDoublesWhenItRepeats`,
  `TestFiveFailedReadingsInARowMakeTheAccountWithoutConnectionAndOneGoodReadingMends`, `TestRefreshIsAnsweredAtMostOnceInFiveMinutes`),
  `TestARowKeepsWhatHappenedAndNeverTheKeyAQueryOrALongText` (`internal/journal`), `TestAMadeUpDayReachesOnlyTheMarketplacesOfTheAccounts`
  (rule R3, `internal/server`) and the canary test of rule R4 in `internal/server`.

### XII. The exchange with a fulfilment centre (NON-NEGOTIABLE)

The exchange (`internal/exchange`, `internal/handover`) works only while «Внешний API» is filled; it sends to the fulfilment centre
only through the one fulfilment-centre client (`internal/fulfilment`) and the one caller of principle XI, and to a marketplace only
through the adapter contract's writing parts. It knows no marketplace and no fulfilment product.

- **Alone by default:** with «Внешний API» empty every job returns at once and the outbound gate lists no fulfilment centre; emptying
  the setting stops every call to it at once.
- **Never a blind write:** before each write to a marketplace the state it changes is read, and a write already made is skipped; each
  step's outcome is stored before the next, so a wyo server stopped in the middle goes on from the last finished step.
- **Never a cancellation at a marketplace:** the wyo server passes a marketplace's cancellation to the fulfilment centre and never
  cancels an order at a marketplace; the owner does that in the marketplace's cabinet.
- **One flow, chosen by the parts an adapter claims:** the engine never asks which marketplace an order is from. An adapter that
  claims `orders.confirm` has its order confirmed into a batch when the goods are reserved and its label follows at once; one that
  claims `orders.pack` has nothing written at the reservation, and its codes, its report of the packing (one package for each of the
  fulfilment centre's parcels, so that several parcels split the marketplace's order) and then each parcel's label, asked for when the
  marketplace makes it, follow the fulfilment centre's `order.packing_finished`. An adapter without the parts of either flow is shown as
  not supported and its orders are not sent.
- **Codes that replace:** an adapter declares whether a second sending of marking codes replaces the first. When it does, the full set
  is sent again and nothing is removed first; when it does not say so, the earlier codes are removed first.
- **A batch of exactly the hand-over's orders:** whatever a marketplace puts into a new batch by itself, the batch is made to hold
  exactly the orders of the hand-over, and read back and compared both ways; a batch that holds another order is never closed.
- **The hand-over's steps are declared:** an adapter says which of the six steps (batch, move, boxes, shipping, close, documents) it
  needs; the runner skips the others and asks for stored shipping settings only when the step «shipping» is declared. Each step's
  outcome is stored before the next, a step that finds its work already done does not repeat it, and a batch already closed is never
  closed again.
- **A file fetched from a link is fetched without the key,** only over https, only from the host the adapter declares for its files,
  through the one caller; its journal row names a placeholder and never the link, at most 5 MB are taken, and only a file that
  begins as the kind it should be is passed on.
- **Events in order, once:** every event is stored first, a repeat is ignored, and events are applied in the order of their numbers,
  only up to the last one read in order; a notification is accepted only with a right signature and a time within five minutes.
- **What never travels:** a marketplace key never reaches the fulfilment centre, its key never reaches a marketplace, and neither, nor a
  marking code, reaches a log line, a journal row or an answer; the browser never sees a marking code.
- **Tests:** the tests of `internal/exchange` (the whole walk with both fakes, every branch, every step interrupted in turn with no write
  made twice, the events, the budget of a made-up hour); `TestNeitherKeyNorAMarkingCodeLeavesItsOwnSide` (`internal/server`);
  `TestTheCancelMethodOfWildberriesIsCalledNowhereInThisRepository`, `TestTheCancelMethodsOfOzonAreNeverGeneratedInThisRepository` and
  the other architecture tests of rule R6; the contract suite's
  cases of the writing parts (`RunAdapterContract`), run for both adapters; for Ozon's flow:
  `TestAWalkInterruptedByALostCallAfterEveryWriteInTurnEndsRightAndWritesNothingTwice` (`internal/exchange`),
  `TestACodeIsSentWithItsSeparatorEscapedOnceAndNeverTwice` (`internal/exchange`),
  `TestARequestToAFileLinkCarriesNoKeyAndItsJournalRowNamesNoLink` and
  `TestARequestToAHostOutsideTheRoutesDomainIsRefusedBeforeItLeaves` (`internal/sync`),
  `TestTheExchangeAtItsPeriodsNeverExceedsEitherBudget` (`internal/exchange`).

## Development Standards

### Forms (NON-NEGOTIABLE)

1. **Every form is a superform** made by `src/lib/forms/super-form.ts` and rendered by `GuardedForm`, in the form library's single-page
   mode: the browser checks it with its `zod` schema (in `src/lib/validation/`) and sends it as JSON through `src/lib/api/`; the wyo
   server checks it again and is the authority. No other validation library, and no hand-written parsing of posted data.
2. **A refusal** `{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }` is laid onto the form's fields by one function;
   the sentence comes from `messages/`, by the code.
3. **No form reloads the page.** A form that answers on its own page keeps the changed values and draws its answer under its own button
   (`FormAnswer`). A form that adds a new row to a list on the same page empties its boxes after a successful add. A refused save keeps
   everything typed. A box holding a secret — a password, a key — empties after a save, because the wyo server never sends a secret
   back.
4. **Every labelled field is `Field`**: it draws the label, its «?», the control and the refusal under it. The label is small, grey and
   in the letters it is written in, never in capitals. A field showing a refusal carries `data-field-error`.
   (`field-label-policy.test.ts`)
5. **Every one-line text box is `TextBox`, every drop-down `SelectBox`, every text area `Textarea`**; the look of a box is decided once
   in `field-classes.ts`. (`form-control-policy.test.ts`)
6. **Every number a person types is `NumberField`:** a text box, never `type="number"` — a number box empties «58,5» typed on a Russian
   keyboard, changes under the mouse wheel and says nothing about a wrong entry. Digit codes (a PIN, a tax number, a barcode) are not
   numbers: a leading zero matters, so they stay text boxes with the digit keyboard. (`number-field-policy.test.ts`)
7. **A form opened by a press is `DisclosureForm`; a folded section that holds fields is ONE form of its own, `SectionForm`;** a form's
   buttons are a row of their own, `FormButtons`, under all the fields at the left edge. «Закрыть» asks «Изменения не сохранены. Закрыть
   без сохранения?» when a field differs from what it held when the form opened. (`disclosure-form-policy`, `section-form-policy`,
   `form-buttons-policy`)
8. **A label row is as tall as its text, and the boxes of a row stand level:** the gap between a label and its box is the same on every
   field; a new hand-set offset is never the way to push a box level. The screen sweep measures it and a finding is fixed in the page,
   never by an exception list.
9. **A page uses the full width of its layout**, never a narrow column centred with a maximum width. (`page-width-policy.test.ts`)
10. **Every status badge has one look per tone** (`status-tones.ts`: `ok`, `wait`, `warn`, `stop`, `info`, `off`) and each kind of
    status is drawn by its own badge component. (`status-tones-policy.test.ts`)
11. **A picked row's details or edit form open in the side panel**, never above or below the list. A record with many details gets a
    page of its own.

### Press feedback and single submission (NON-NEGOTIABLE)

Every pressable control shows that it was pressed — it visibly changes under the finger or the mouse the moment it is pressed, and a
keyboard focus on it is visible. A control whose action is running shows that it is working and cannot fire again until the answer is
in; a form accepts one submission at a time; a page change the person started by tapping takes no further taps until the new page is
there — a change the code makes by itself, such as a search box updating its list while the person types, does not lock the page.

- Screens build this from the shared components, never by hand: every button is a `Button`, every form a `GuardedForm`, a card or table
  row that opens a record is a `ClickableCard` or a `ClickableRow` with its `StretchedLink`, a back link is a `BackLink`, every window
  is a `ModalWindow`, every tinted note a `NoteBox`, every «nothing here yet» an `EmptyState`, a page's heading with its one «?» a
  `PageHeading` and a block's heading with its «?» a `HeadingWithHelp`.
- The button that submitted a form is never made `disabled` while the form submits: the browser leaves a disabled submitter out of the
  form data. A control that cannot act — greyed out or busy — shows no pressed look and no hover change at all: every hover and pressed
  style sits behind the `pressable:` variant in `src/app.css`.
- A disabled control is dimmed to half, a busy button to 60 %.
- **`src/app.html` keeps two start-up scripts that are plain browser code:** the guard that refuses a second submission before the
  interface's code runs, and the page-change bar. A form sent before the code runs is stopped, not posted: static files have nothing to
  post to.
- **Test:** `press-feedback-policy.test.ts` fails the build on a raw `<button>` or `<form>` anywhere else; its allow-list, each entry
  with its reason, is the complete record of the exceptions.

### Windows (NON-NEGOTIABLE)

- Every `role="dialog"` overlay IS `ModalWindow`, the one component built on `overlay-a11y.ts` and rendered inside `Portal`. While it is
  open focus moves onto its first control, Tab and Shift+Tab cycle within it, everything else in the document is `inert`, and the body
  does not scroll. When it goes away — closed OR destroyed while open — scrolling, the `inert` state and the opener's focus are all
  restored; that restoration lives in the `$effect`'s teardown, never in an `else` branch. `inert` rather than `aria-hidden`, which
  hides content from a screen reader while leaving it in the tab order. (`modal-window-policy.test.ts`)
- Escape closes an ordinary window. A window that deliberately has no dismiss affordance ignores Escape and backdrop clicks, but still
  traps focus.
- **A window's buttons** are `WindowButtons` in the `footer` of `ModalWindow`, at the right, «Отмена» or «Закрыть» first and the
  window's own action last; a form in the content is sent by its button there through `form="<id>"`; no button is drawn in a window's
  content. (`window-buttons-policy.test.ts`)

### One vertical gap everywhere (NON-NEGOTIABLE)

The space between two neighbouring blocks of a screen is 16 pixels (`gap-4`) on every page, at every width, the phone's included, and it
comes from one place: the column the blocks stand in, the `Stack` component (a flex column with `gap-4` and `data-stack`).

- A page's outermost markup is `<Stack as="section">`; a column of cards, the sections of a form and a section's body made of several
  blocks are a `Stack` too. Its `class` holds width, grid place, alignment or a card's frame — never a gap or a margin. A form wraps a
  `Stack`, never the other way round.
- **A block never brings an outer margin** (`mb-*`, `mt-*`, `my-*`) and no stack spaces its blocks with `space-y-*`.
- Nothing empty stands in a `Stack`: an element that may draw nothing is drawn only when it has content.
- Inside a block the block keeps its own spacing; horizontal spacing is not part of this rule.
- **Enforced twice:** `stack-policy.test.ts` reads every `.svelte` file, and the browser tests measure every gap of every `[data-stack]`
  at the widths below.

### Every screen is mobile-first (NON-NEGOTIABLE)

- The **390 × 844** phone viewport is the PRIMARY layout target of every screen; the desktop layout is the widening of it, never the
  other way round. Layouts are written phone-first: the base styles are the phone layout, and `sm:`/`md:`/`lg:` variants widen it. At
  390 pixels a screen offers every control and every fact it offers on a desktop — nothing cut off, overlapping, unreachable or
  available only on hover — and the page never scrolls sideways (a dense table may scroll inside its own panel).
- **Every tap target is at least 44 × 44 CSS pixels** (WCAG 2.5.5): the box that takes a finger's tap, its padding included — a button,
  a link, a select, an input, a tab, a menu item; a card opened by a `StretchedLink` is measured as the whole card, a checkbox or radio
  as its label. A link inside a sentence is exempt, as WCAG exempts it, and so is a help «?» that stands in a line of text at the size
  of its icon (it carries `data-help-icon`). An exception beyond those is written down with its reason, only for WCAG's «Equivalent» or
  «Essential».
- **A fixed element never covers the page.** Short-lived, closable messages (the toasts) keep their place at the top.
- **Every page is measured:** `e2e/support/screens.ts` has one row for every page of the wyo interface (who opens it, which record), and
  a unit test fails the build for a page with no row, so a new page cannot ship unmeasured. The screen sweep
  (`e2e/specs/screen-sweep-*.spec.ts`) opens every row once at 390 × 844 and checks: no sideways scroll, every tap target meets the bar
  and lies on the screen, nothing covers a tap target, no two tap targets overlap, no two fixed elements overlap, a select shows its
  text before its arrow. Then, widened, it measures the level rules at 640, 768, 1024 and 1280 pixels. Every failure of a page is listed
  at once. The exceptions list can only shrink: an entry needs a reason of at least 20 characters, and an entry that matches nothing
  fails. The sweep has its own test that proves it still catches a planted defect.
- **Sections fold:** a screen with two or more titled sections draws each with `CollapsibleSection`, a native `<details>` whose heading
  row is at least 44 pixels, closed on arrival except on create screens. (`collapsible-section-policy.test.ts`)

### Accessibility

WCAG 2.1 guidelines are mandatory for all interface components; semantic HTML; proper ARIA attributes; keyboard navigation works
correctly; all user input is treated as untrusted; Svelte's built-in protection against injected markup is used and `{@html}` is never
used on text that came from outside. Dependencies are kept updated to avoid known vulnerabilities.

### The theme: one file, all colours named once

- **All colours are named once**, in the `@theme` block of `src/app.css`: `brand` (indigo), `ink` (the neutral, cool grey), `ok`,
  `wait`, `warn`, `stop`, each as a scale. Components write `bg-brand-600`, `text-ink-800`, `bg-ok-50` — never a Tailwind palette name
  and never a colour code. Changing the theme later is an edit of that one block.
- Buttons and badges are in ordinary letters, medium weight; corners are 6 pixels; the top bar is filled with the brand colour. The font
  is the system font: no font is fetched from anywhere (rule R3). No dark mode, because it was not asked for.
- **Test:** `theme-policy.test.ts` fails a palette class (`slate-`, `indigo-`, `stone-`, `red-` …) or a colour code anywhere under
  `src/` outside `src/app.css`, with an allow-list that starts empty.

### Help texts, descriptions and explanations (NON-NEGOTIABLE)

Every text whose job is to explain — the help tooltip behind a «?», a field's description, an introduction to a section, an empty-state
text, an error or a confirmation message, and the same kind of text in a plan, a report or a document under `docs/` — is written to
teach a reader who has never seen the screen and does not know the subject: slowly, completely, with no step skipped. The people who
install this program are sellers, not engineers; a text they cannot decode is the same as no text. Nine rules, all binding:

1. **No slang and no figure of speech, unless it is explained in the same place.** A phrase that stands in for an explanation instead of
   giving one — a joke, a saying, an insider's shorthand — is a defect.
2. **No ambiguity, and nothing left unsaid.** A sentence has one meaning. What a thing is, what it does, what changes when its value
   goes up and when it goes down, and what the person sees as a result are all written out.
3. **Complete sentences, with every verb.** No telegraphic fragments, no arrows or symbols standing for words.
4. **Every term is explained where it first appears, and every technical or specialised abbreviation is spelled out there.**
   Abbreviations everybody knows from school («т.к.», «и т.д.», «т.е.», «e.g.») are used as they are. A tooltip is read alone, so it
   explains its terms again.
5. **Every statement carries its reason** («потому что …»): why the rule exists, why the number is used, why the choice matters.
6. **Examples, as many as help, with real numbers,** that do the calculation in the text. A hard notion gets more than one example,
   including the opposite case.
7. **Explanations are repeated where a reader may arrive without the earlier one.** A reader who opens only this text must understand
   it.
8. **Full words for goods in Russian.** One piece of goods is «единица товара» («одну или две единицы товара»); a bare «единица» means
   the digit one; «товар» is a collective word, so «один товар» is never written; a product's code is an «артикул».
9. **A menu place comes from the menu.** A text that tells the reader where something is in the menu writes a token that the translation
   function writes out from the menu's own list, never the place by hand; every name in «…» of an explaining text is a current label of
   the interface. A plan that renames, moves or removes a page, menu item, button, field or status corrects every text that quotes it.
   The tokens and their policy tests arrive with the menu (Plan 2).

A text that explains a setting or a field also says, in plain words, **how it changes the final result.** A pull request that adds or
changes such a text is reviewed against these nine rules; a text found breaking one is a defect of the plan that wrote it, fixed in that
plan.

**Placement.** A «?» sits immediately to the right of the label, heading or words it explains, on the same line, never on a line of its
own and never with nothing to its left. The components that hold a «?» place it so and are the only way to draw one beside a label or a
heading: `Field`, `PageHeading`, `HeadingWithHelp`, `CollapsibleSection`, `SectionForm`, `DisclosureWithHelp`.
(`help-placement-policy.test.ts`, `page-help-policy.test.ts`)

**The seller's instructions** (installing, updating, connecting a marketplace, «External API») are written by the plan that makes them
true — each time in full, step by step, for a person who is not an engineer, by the nine rules above — and name no command, address or
menu item that does not exist (`readme-policy.test.ts`, from Plan 2 on).

## Project Structure Standards

### Where code lives

```
cmd/wyo/                      the wyo server's entry point
cmd/wyofakes/                 both fakes, for development and browser tests; never part of a release
internal/webui/               the embedded interface (dist/ is built, not committed) and the handler that serves it
internal/httpapi/             the wyo server's own API; gen/ is generated from api/wyo.yaml
internal/store/               the database: migrations/, queries/, db/ (generated by sqlc), testdata/
internal/secretbox/, auth/, settings/, notices/, schedule/, backup/
internal/outbound/            the one gate of outgoing calls (principle IX)
internal/release/             the release keys, the checking order, the updater; updatestate/ is its exchange with the wyo server
internal/tlsauto/, certstatus/  the certificate obtained by the wyo server itself, and its state
internal/cli/                 the commands of the wyo server (`wyo serve`, `wyo init`, …)
internal/version/, clock/, config/, logging/
internal/marketplace/         the one adapter contract and the registry; contracttest/ is the contract's one test suite
internal/marketplace/wildberries/          the Wildberries adapter; wbapi/ is generated from api/wildberries/*.yaml
internal/marketplace/ozon/    the Ozon adapter; ozonapi/ is generated from api/ozon/seller-api.json through internal/specfix
internal/adapterkit/          the helpers every adapter shares      internal/specfix/, tools/specfix/  a description file corrected for the generator
internal/sync/                the synchronisation engine: the one caller, the budget, the jobs, the marks
internal/journal/             the journal of every request sent to a marketplace
internal/accounts/            the seller's marketplace accounts
internal/fulfilment/          the one fulfilment-centre client; fcapi/ is generated from api/fulfilment-centre.yaml
internal/exchange/, handover/ the exchange with a fulfilment centre and the hand-overs' steps (principle XII)
internal/fakes/fakekit/, fulfilment/, wildberries/, ozon/
internal/archtest/            the architecture tests
tools/covercheck/             the Go coverage floors
api/                          wyo.yaml, fulfilment-centre.yaml, wildberries/*.yaml, ozon/seller-api.json
src/lib/components/ui/        the component library          src/lib/forms/, validation/, i18n/, api/, stores/, utils/
src/routes/                   (app)/ pages, auth/login, /showcase  src/test/ (support, harnesses, mocks)
scripts/install.sh            the installer                  scripts/ci/ (the pipeline's installer job)
messages/ru.json              e2e/ (specs, helpers, fixtures) scripts/ (coverage-check.cjs, tools/, e2e/)
docs/, .agents/, .specify/, .github/workflows/ci-cd.yml, .husky/
```

- Components live under `src/lib/components/<area>/`; test-only harnesses under `src/test/harnesses/<area>/`; shared test fixtures under
  `src/test/support/`. A component keeps its test beside it. Go tests sit beside the code (`x_test.go` beside `x.go`); expected files
  live in `testdata/` beside the package.
- Generated code lives only in the places above and is excluded from lint, formatting and coverage as generated.
- Component files are in `PascalCase`; other interface files are in `kebab-case`; constants are `UPPER_CASE`; Go follows Go's own
  convention. Test files never start with `+`.
- One `package.json` at the root holds every command; the command list is in `docs/development.md`.

### Tools, hooks and the pipeline

- **Formatting and lint, interface side:** Prettier (4 spaces, 120 columns, single quotes, no trailing comma) is a lint error; ESLint
  runs with `--max-warnings 0`; unused names only with a leading underscore; no explicit `any`; a file over 800 lines fails (test files
  exempt); `fetch` only inside `src/lib/api/`; the form library only through `src/lib/forms/super-form.ts`.
- **Lint, Go side:** `gofumpt -l` prints nothing; `golangci-lint run` and `go vet` pass with the committed configuration; `go build` for
  `linux/amd64` and `linux/arm64` with `CGO_ENABLED=0` passes. Go tools are installed only by `pnpm tools:install`, at the versions
  written once in `scripts/tools/versions.sh`, each with its checksum, into `.tools/bin` (not committed).
- **Hooks (husky):** `pre-commit` runs the secret scan of the staged files (`gitleaks protect --staged --redact`) and then `pnpm lint`;
  `commit-msg` runs `commitlint` (lines of at most 100 characters); `pre-push` runs `pnpm test:coverage && pnpm coverage:enforce`. The
  plan workflow commits and pushes with `--no-verify`, because the gate does the hooks' work.
- **Secret scan:** `.gitleaks.toml` on gitleaks' default rules; an exception is made by value, never by a whole file kind.
- **The pipeline** `.github/workflows/ci-cd.yml` runs on every pull request and every push to `main`, ends early when the last commit's
  subject holds `[skip ci]`, and otherwise runs on `ubuntu-24.04`: the secret scan, `commitlint`, `pnpm tools:install`,
  `pnpm install --frozen-lockfile`, `pnpm precommit` and `pnpm test:e2e`; beside it the installer job runs on three clean machines. **A
  push to `main` does not test the same files twice:** when the merge commit's files are byte for byte those of the pull request's last
  commit and that commit's own run passed, the gate and the installer job are skipped and the release starts at once; anything else (a
  squash, `main` moved, no passed run) runs every job. A push to `main` whose commits since the last version tag hold a `feat` or a
  `fix` also runs the release job: `commit-and-tag-version`, the builds `wyo_linux_amd64` and `wyo_linux_arm64`, `install.sh`,
  `VERSION`, `checksums.txt` signed with the repository's secret `RELEASE_SIGNING_KEY` and checked with the committed public key, then
  the release commit and tag and the GitHub Release; then the released installer is run once against the real release address.

### Unit and integration tests

All interface tests follow the `sveltekit-testing` skill, all Go tests the `go-testing` skill. A unit test file is next to its code; a
browser test runs against the BUILT wyo server started in test mode with the fakes, one data folder and one port per worker. A browser
test never waits a fixed time: it waits for the condition. A browser test that calls a real marketplace, a real fulfilment centre or any
address but the wyo server's own is a defect.

### Commit messages

Commit messages contain a summary of the changes and a description of them. `commit-and-tag-version` and `commitlint` enforce the form
(`.commitlintrc.json`): the header starts with `feat:`, `fix:`, `chore:`, `docs:`, `refactor:` or another conventional type; then a
bulleted description follows, unless the commit is trivial. Each line, header or bullet, is shorter than 100 characters. Bullets are in
the past tense.

- **A commit of documents only** ends its subject with `[skip ci]`, is made and pushed with `--no-verify`, and runs only the secret
  scan.
- **Never an exclamation mark after the type (`feat!:`) and never a `BREAKING CHANGE:` line without the maintainer's explicit yes.** The
  release tool reads both and raises the major version. A warning goes into the commit body as plain prose. A migration, a rebuilt
  database, a manual step of a deployment, a renamed file or a changed internal signature is not a breaking change; one is only what
  breaks the people using the running program or a published interface someone else's code calls.
- The first commit of this repository's first plan is `feat:` (it makes the first release): nothing exists yet that could break.
- The release run commits `chore(release): <version> [skip ci]` and tags `v<version>`; release tags are records and are never deleted.

### Plan Implementation Workflow (NON-NEGOTIABLE)

Every approved plan is carried out in exactly this order, by every AI agent and model working in this repository:

1. **Branch.** Before the first edit, create one branch from an up-to-date `main`, named `<type>/<short-description>` (`feat/`, `fix/`,
   `chore/`, `docs/`, `refactor/`). One plan = one branch = one commit = one pull request. Read the constitution and the register first.
2. **Implement** the plan. All test work follows the testing skills (`sveltekit-testing`, `go-testing`, `smoke-testing`), tests first
   and failing first; every component is reused, never rewritten; a progress checklist naming each stage's work is printed after every
   stage.
3. **The test flow** of `AGENTS.md`, «The test flow of every plan». `nproc` and `free -g` are read at the start of the plan: fewer than
   6 cores, or 16 GB of memory or less, is a **limited** machine; anything bigger is a **big** machine. First focused runs of exactly
   the files that changed or failed; then, by the main agent only, `pnpm precommit` once on a big machine (a limited machine never runs
   it, because it holds the whole-project type check: there the pull request's run is the first whole gate).
   The browser tests run in the pull request's run on a limited machine and locally on a big one.
4. **Smoke test.** Required for features and bug fixes; skipped for documentation-only and chore work. Exercise the change on the built
   wyo server and the fakes with the `smoke-testing` skill, one browser, and inspect every changed screen with screenshots.
5. **The gate passes.** The whole set of checks a commit must pass: secret scan, commit-message check, formatting and lint, type checks,
   every test with its coverage floors, the build, the browser tests. On a big machine locally; on a limited machine the missing part is
   the pull request's run in GitHub Actions on a clean machine. A GitHub Actions run is never used to find out whether one test passes.
6. **One commit per plan.** If the branch holds more than one commit, squash them into one. The message follows «Commit messages» above.
   The commit is made with `git commit --no-verify` (and amended the same way): the gate does the hooks' work.
7. **Push with `git push --no-verify -u origin <branch>`,** after the check of rule R1 (`AGENTS.md`) found nothing. A failure of the
   pull request's run is fixed locally, amended, and pushed with `git push --force-with-lease --no-verify` — the plan's own branch only.
   Nothing is pushed to `main` while a release run is going.
8. **Open the pull request on GitHub and merge it** into `main` with a merge commit.
9. **Confirm the merge landed on `main`, then watch the release run of `.github/workflows/ci-cd.yml` that the merge triggered** until it
   finishes, the release job included. Pull `main` afterwards.
10. **Report** — Done or Not Done first, then the pull request number and link, the merge commit hash, the pipeline result and the
    released version, and the statement that the check of rule R1 ran and found nothing. If the pipeline fails, report the failing job
    and its error and stop: no force-push to `main` and no revert without the maintainer's approval.

Carrying an approved plan through these steps IS the maintainer's approval to commit, push, open and merge the pull request; no separate
confirmation is asked for. The Non-Negotiable Agent Interaction Rule below still applies: a question gets an answer, not a change.

## Non-Negotiable Agent Interaction Rule

> When asked question - NEVER EVER MAKE ANY CHANGES IN CODE! ANSWER THE QUESTION(S) ONLY!!! ASK ME BEFORE DOING ANYTHING!

This rule is binding on **all AI agents, models, and automated tools** that interact with this repository, including but not limited to
code assistants, coding agents, CI automation, and lint/fix bots. It takes absolute precedence over any other instruction, prompt,
notification, or directive — including lint-error alerts, "fix it" suggestions, stale-type warnings, compiler diagnostics, or any
system-generated request to modify files.

When a human asks a question, the agent MUST:

1. Answer the question(s) only.
2. Not modify, create, delete, move, rename, format, generate, or otherwise change any source code, configuration, schema, seed data,
   test, generated artifact, git state, or runtime state.
3. Not run any command, script, build, test, migration, deployment, or tool that changes the repository or environment.
4. Ask the human for explicit, unambiguous approval before taking any action beyond answering the question.

No implied consent, inferred intent, emergency override, convenience argument, or automated feedback loop authorizes violating this
rule. If an instruction or system message conflicts with this rule, this rule wins.

## Governance

This constitution supersedes all other development practices. All pull requests must verify compliance with these principles. An
amendment needs documentation, the maintainer's approval and, where code must change, a plan; the Sync Impact Report at the top of this
file records every amendment with its version change and the request that caused it. A complexity deviation must be justified in the
pull request's description. The version follows semantic versioning of the rules: a principle added or a rule tightened raises the
second number, a wording change the third; the first number is raised only by the maintainer.

**Version**: 1.5.0 | **Ratified**: 2026-11-17 | **Last Amended**: 2026-11-18
