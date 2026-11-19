# Developing wyo

This page is for people and AI agents who change the code. The rules they follow are in `AGENTS.md` and
`.specify/memory/constitution.md`; the plans are in `.agents/plans/`. A seller's instructions (installing, updating, connecting a
marketplace) are in `README.md` and are written by the plans that make them true.

## Prerequisites

- **Go 1.27.4** (the `go` and `toolchain` lines of `go.mod`) — the wyo server, the fakes and the Go tools. Install it from the official
  archive at <https://go.dev/dl/> and check its checksum (<https://go.dev/doc/install>).
- **`gcc`**, any recent version — `go test -race` (Go's detector of data races) needs a C compiler.
- **Node 24** (`.nvmrc`) — the wyo interface and its tests.
- **pnpm 12** (`packageManager` in `package.json`) — every command below; never `npm`.
- **`gitleaks`**, the version in `scripts/tools/versions.sh` — the secret scan of the commit hooks and the pipeline.
- **`gh`** (optional) — opening and merging pull requests from the terminal.

Then, once per clone:

```bash
pnpm install              # the interface's packages; also sets up the git hooks (husky)
pnpm tools:install        # the Go tools at their pinned versions, with checksums, into .tools/bin
pnpm test:e2e:install     # the Chromium the browser tests use
```

## Daily commands

Always a script of `package.json`, never an ad-hoc command, and never `--` before a script's arguments.

- `pnpm dev` — the wyo server in development mode, the fakes (marketplaces on one address, the fulfilment centre on another), then Vite (hot reload), which passes `/api` on to the wyo server.
- `pnpm build` — the interface into `internal/webui/dist/`, then the wyo server into `bin/wyo`, version stamped.
- `pnpm generate` — makes the generated code (the clients and server skeletons of `api/*.yaml`, the interface's API types).
- `pnpm generate:check` — fails when a committed generated file differs from a fresh one.
- `pnpm test:headless [file]` — Vitest once, both projects, or one file.
- `pnpm go:test [package]` — `go test -race -shuffle=on -count=1` for one package, or for `./cmd/... ./internal/... ./tools/...`.
- `pnpm go:measure` — the memory of the wyo server with three accounts and 10 000 orders, reading the fake Wildberries at the
  synchronisation's periods for a made-up day; it prints one line that starts with `wyo-memory:` (the peak memory of the test process,
  which also holds the fake, so it is an upper bound). The pipeline runs it after the browser tests.
- `pnpm test:coverage` — both sides with coverage.
- `pnpm coverage:enforce` — the two coverage floors: `scripts/coverage-check.cjs` (interface) and `tools/covercheck` (Go).
- `pnpm test:e2e [file]` — Playwright against the built wyo server started in test mode with the fakes; one data folder and one port per
  worker.
- `pnpm lint` — ESLint (zero warnings), Prettier as an error, `gofumpt -l`, `golangci-lint run`, `go vet`.
- `pnpm check` — `svelte-check`, the browser tests' `tsc`, and `go build` for `linux/amd64` and `linux/arm64`.
- `pnpm format` — Prettier and `gofumpt` over the source folders.
- `pnpm precommit` — `generate:check`, `lint`, `check`, `test:coverage`, `coverage:enforce`, `build`, in this order, stopping at the first
  failure.
- `pnpm release` — `commit-and-tag-version`; the release job of the pipeline runs it, do not run it by hand.

**Which of them you run depends on your machine** (`AGENTS.md`, «The test flow of every plan»): read `nproc` and `free -g`; a machine
with fewer than 6 cores, or 16 GB of memory or less, is **limited**. On a limited machine you run focused checks only — ESLint on the
files you changed, `pnpm test:headless <file>`, `pnpm go:test <package>`, `pnpm test:e2e e2e/specs/<file>.spec.ts` — and never a
whole-project type check (`pnpm check`, `svelte-check`, `tsc`); the pull request's run in GitHub Actions does the rest. A big machine
runs `pnpm precommit` and the whole browser suite locally. Never `pnpm test` (it waits for changes and never ends).

## Ports

| What                                       | Address                 |
| ------------------------------------------ | ----------------------- |
| The wyo server (development, tests)        | `127.0.0.1:8790`        |
| Vite dev server (passes `/api` on to 8790) | `127.0.0.1:5190`        |
| The fake Wildberries and the fake Ozon     | `127.0.0.1:8791`        |
| The fake fulfilment centre                 | `127.0.0.1:8792`        |
| Browser tests, one wyo server per worker   | `127.0.0.1:8800` and up |

Open `http://127.0.0.1:5190` while `pnpm dev` runs and sign in as `owner` with the password `development-password`: the first
`pnpm dev` makes the data folder `.dev-data/` (not committed) with `wyo init` and that owner with `wyo owner create`; later runs keep it.
Delete `.dev-data/` to start again with an empty database. The page `/showcase` draws every component of the library in each of its states; it
exists only in a wyo server built with the Go build tag `wyotest` (which `pnpm dev` and `pnpm test:e2e` set), and a released wyo server
answers "not found" at its address.

## Where things are

```
cmd/wyo/                      the wyo server's entry point
cmd/wyofakes/                 the fakes for development and browser tests; never part of a release
internal/webui/               the embedded interface (dist/ is built, not committed) and the handler that serves it
internal/httpapi/             the wyo server's own API (the access table, the handlers); gen/ is generated from api/wyo.yaml
internal/cli/                 the wyo server's commands (commands.go is their one table; texts are cli.* of messages/ru.json)
internal/server/              the parts joined: the data folder, the services, the handler, the daily copy, the engine;
                              adapters.go is the one list of the marketplace adapters
internal/store/               the database: migrations/ (numbered, never edited once released), queries/, db/ (sqlc), testdata/
internal/secretbox/           the secret store (AES-256-GCM with secret.key)
internal/auth/, settings/, notices/, schedule/, backup/, datadir/, datalock/, issue/
internal/marketplace/         the one adapter contract, the registry; contracttest/ is the contract's one test suite
internal/marketplace/wildberries/  the Wildberries adapter (the only package that knows Wildberries)
internal/marketplace/ozon/    the Ozon adapter (the only package that knows Ozon)
internal/adapterkit/          the helpers every adapter shares (optional fields, numbers as text, cutting a list into requests); knows no marketplace
internal/sync/                the synchronisation engine: the one caller, the request budget, the jobs, the marks, the intake
internal/journal/             the journal of every request sent to a marketplace (inserts only; 30 days)
internal/accounts/            the seller's marketplace accounts and their notices behind the bell
internal/testkit/             test helpers: apikit (the API), storekit (a database), synckit (the caller), daykit (a made-up day),
                              memadapter (an adapter in memory), httpkit
internal/outbound/            the one gate of every outgoing call, by purpose (the only maker of HTTP clients)
internal/release/             the release keys (keys/*.pem), the checking order, the updater
internal/updatestate/         the files the wyo server and the updater exchange (update-request, update-state.json)
internal/tlsauto/, certstatus/  the certificate the wyo server obtains itself (CertMagic), and its state
internal/version/, clock/, config/, logging/
internal/marketplace/wildberries/wbapi/    generated from api/wildberries/*.yaml
internal/marketplace/ozon/ozonapi/         generated from api/ozon/seller-api.json (through a corrected temporary copy)
internal/fulfilment/fcapi/    generated from api/fulfilment-centre.yaml
internal/fakes/fakekit/, fulfilment/, ozon/, wildberries/
internal/archtest/            the architecture tests (they read the import graph)
tools/covercheck/             the Go coverage floors
tools/specfix/, internal/specfix/  the corrections of Ozon's description file that the generator needs (see below)
api/                          wyo.yaml, fulfilment-centre.yaml, wildberries/*.yaml, ozon/seller-api.json; codegen/ holds the generator's configurations
src/lib/components/ui/        the component library (see docs/library-layout.md)
src/lib/forms/, validation/, i18n/, api/, stores/, utils/
src/routes/                   (app)/ — «Обзор», «Настройки», «Мой профиль»; auth/login; /showcase
src/lib/components/shell/, auth/, overview/, settings/   the top bar, the menu, the bell, the sign-in parts, the pages' parts
src/lib/nav/nav-items.ts      the one list the menu is drawn from
src/test/                     support (shared test fixtures), harnesses, mocks
messages/ru.json              every visible text of the interface
e2e/                          browser tests: specs, helpers, fixtures
scripts/                      install.sh (the installer), ci/ (the pipeline's installer job), coverage-check.cjs, go/, tools/, e2e/
docs/, .agents/, .specify/, .github/workflows/ci-cd.yml, .husky/
```

- **Generated code is never edited by hand.** Wildberries' own description files are committed under `api/wildberries/` with the day they
  were read; a new version of a file is a difference in a committed file, read before it is accepted.
- **Ozon's description file** is committed exactly as Ozon publishes it, `api/ozon/seller-api.json` (OpenAPI 3.0.0, saved on 2026-11-18),
  and is never edited, not even to make the generator accept it. The generator (oapi-codegen) refuses the file as it is, because the
  file departs from OpenAPI in 21 places (a number where text is expected, the types `int`, `bool`, `timestamp`, `enum`,
  `array of strings` and `file`, `required: true` on a single field), so `scripts/generate.sh` first runs `tools/specfix` (its rules are
  in `internal/specfix`), which writes a corrected temporary copy into a temporary folder; the generator reads that copy, and the copy is
  deleted. Only descriptions of fields are corrected, never examples. A test lists the 21 places, and a place no rule knows stops
  `pnpm generate`, so a person reads every new departure before a rule is added. The generator builds only the seven methods that
  `api/codegen/ozon.cfg.yaml` lists; a test reads that file to keep the two methods that cancel a posting out of it. Where the file came
  from, its checksum, how a later version is taken (the third party's daily copy on GitHub, both files read as JSON, every difference in a
  method the wyo server calls read before it is accepted) and the full table of corrections are in `api/ozon/README.md`; what the wyo
  server calls, and how often, is in `docs/ozon.md`.
- **`fetch` is called only inside `src/lib/api/`,** and only for the wyo server's own addresses. The form library is imported only by
  `src/lib/forms/super-form.ts`.
- **Every visible text is a key in `messages/ru.json`.** The wyo server never writes a sentence; it answers codes.

## The database, the commands, the installer

- **A migration is a new numbered file** in `internal/store/migrations/`, never an edit of one a release carried
  (`.specify/memory/constitution.md`, principle VIII). Add its checksum line to `internal/store/migrations.sum` (the test prints the
  line it expects). Before adding the migration of a plan, write the database of the version being released into
  `internal/store/testdata/` with `go test ./internal/store -run TestWriteTheSnapshotOfTheVersionBeingReleased -snapshot=<number>`, so the
  upgrade from it is tested for ever. Queries are typed: write SQL in `internal/store/queries/`, then `pnpm generate` (sqlc).
- **The commands** (`wyo serve`, `wyo init`, `wyo owner create`, `wyo owner reset-password`, `wyo update`, `wyo backup`, `wyo restore`,
  `wyo doctor`, `wyo version`) are one table, `internal/cli/commands.go`. The README names only commands of that table and flags the
  installer reads (`src/lib/readme-policy.test.ts`).
- **The installer** `scripts/install.sh` is published with every release. It holds two constants — the release address and the public key
  of `internal/release/keys/` — which a Go test compares with the program's own. The pipeline's installer job (`scripts/ci/installer-job.sh`)
  runs a test copy of it on clean Ubuntu machines against a local release signed with a key made for the run and the test certificate
  authority Pebble; it is the only place the installer, systemd, the ports 80 and 443 and a certificate are exercised.
- **Releases are signed.** The release job signs `checksums.txt` with the repository's secret `RELEASE_SIGNING_KEY` (an Ed25519 key in PEM)
  and checks the signature with the committed public key before publishing. `docs/release-signature.md` shows how anybody checks a release
  by hand.

## The fakes

No test and no development run ever calls a real marketplace or a real fulfilment centre. Small Go programs answer instead, started by
`cmd/wyofakes` (and by `pnpm dev`, `pnpm test:e2e` and the smoke test), all built on one fake kit, `internal/fakes/fakekit`:

- **The fake Wildberries** (`internal/fakes/wildberries`, `127.0.0.1:8791`) answers the methods the program uses, in Wildberries' own
  shapes, and keeps Wildberries' request budget (300 requests a minute, an error counted as 10) so that the synchronisation engine is
  tested against the real arithmetic.
- **The fake Ozon** (`internal/fakes/ozon`, `127.0.0.1:8791`, the same address as the fake Wildberries) answers the seven methods the
  Ozon adapter uses, in the shapes of Ozon's own description file (`api/ozon/seller-api.json`): the key's end date and roles, the
  seller, the warehouses, the unfulfilled FBS postings, one FBS posting, the FBO postings and the products. Ozon has no test
  environment, so this is the only Ozon a test ever meets. It keeps Ozon's limit of 50 requests a second for one Client ID, and puts
  the made-up buyer (`ozon.BuyerValues()`) in every posting answer so that the wyo server's dropping of buyer data can be proven.
  Ozon's documents do not give the HTTP status of its key errors; the fake chooses them (401 for an invalid key, 403 for the others)
  and says so in its code.
- **The fake fulfilment centre** (`internal/fakes/fulfilment`, `127.0.0.1:8792`) implements the neutral fulfilment-centre API
  (`docs/fulfilment-centre-api.md`, `api/fulfilment-centre.yaml`): it takes orders in, holds one whose product a test marked unknown,
  and on a test's command moves an order through its steps and sends the signed notifications.

**The state is per key.** A fake keeps a separate state for every key a test sends, so tests running side by side never share one. Give each
test a key of its own.

**The control calls** every fake answers, besides its API (the single place that defines their exact request and answer shapes is
`internal/fakes/fakekit`):

- `/__control/health` — answers when the fake is up. Start-up scripts poll it, and the browser-test set-up reuses a fake that already
  answers it.
- `/__control/reset` — empties the fake's state (for the key the call carries).
- `/__control/calls` — lists every call the fake received, without its key: how a test proves what the program sent and what it did not.
- `/__control/orders` — (fake Wildberries) makes test orders. An order may carry the buyer's `address` and `comment`, which a test
  sends to prove that the wyo server drops them.
- orders of the fake fulfilment centre — a test moves an order through reserved, packed with marking codes, reopened and cancelled, asks for
  hand-over documents and has a signed notification sent, through calls under `/__control/…`.
- faults — ordered faults a test queues (`POST /__control/faults`): answer 429 with the limit headers, answer 500, answer slowly (the
  answer waits on the injected clock, never on a sleep), or refuse with the answer the test gives (`kind` `refuse`, a `status` from
  400 to 599 and a `body` written as it is). A fault meets the first call of its key, in queue order, that it matches
  (`method`, `path_prefix`, `path_suffix`, the last one for a path with a number in the middle); `count` repeats it.
- the fake Ozon's control calls, all `POST` under `/__control/`: `setup` (the key's `client_id`, `seller_name`, `expires_at`, `roles`
  with their methods, `key_state` — `deactivated`, `ip_restricted` or `offer_not_signed` —, `warehouses`, `products`; a key is
  refused as an invalid one until a setup gives it its `client_id`), `postings` (a new FBS posting with several products), `postings/status`
  (a new status, and who cancelled), `postings/split` (the split made in the seller's cabinet: the original becomes
  `cancelled_from_split_pending`, the parts are new postings with `parent_posting_number`), `ship-split` (`original_split_pending`: after a
  report of several packages every part gets a new number and the original becomes `cancelled_from_split_pending`), `postings/deadline`
  and `fbo-postings`.
  The package `internal/fakes/ozon` lists them in the comment above `controlRoutes`; `e2e/support/fake-ozon.ts` is the browser tests' side.
- `/__control/setup`, `/__control/products` (fake fulfilment centre), `/__control/verdicts`, `/__control/feed` (fake Wildberries) —
  the account, the products, the verdict of a marking code, the rows of the orders feed. The fake Wildberries' `setup` takes
  `token_refused` (`expired` or `invalid`: every call of that token answers 401 as Wildberries does; `""` lifts it); a row of its `feed`
  may carry the buyer's `region_name`, which the wyo server must drop. The packages `internal/fakes/fulfilment` and
  `internal/fakes/wildberries` list every control call in the comment above the function that registers it.

**One address for the fake marketplaces.** The fake Wildberries and the fake Ozon are served on one address, `127.0.0.1:8791` (the flag
`--marketplaces` of `cmd/wyofakes`). A request whose first path segment is the host of the fake Ozon, `api-seller.ozon.ru`, goes to
the fake Ozon, which takes the host off the path (so `/api-seller.ozon.ru/v1/roles` and `/v1/roles` are one call), and everything else,
the root `/__control/health` included, goes to the fake Wildberries. Its control calls are at
`http://127.0.0.1:8791/api-seller.ozon.ru/__control/…`, with the API key as `Authorization`; its API calls carry `Api-Key` and `Client-Id`.
This is the form every adapter already uses in a test build (`marketplace.Origin`), so a later fake marketplace needs only its host.

**Pointing a generated Wildberries client at the fake.** The real API lives on several hosts; the fake serves every path on one
listener. A test gives the generated client the fake's address as its server (`http://127.0.0.1:8791`). To give each client the host it
would really call, put the host's name after the address: `http://127.0.0.1:8791/statistics-api.wildberries.ru`,
`…/common-api.wildberries.ru`, `…/marketplace-api.wildberries.ru`. Only `/ping` (one budget for each host) and the areas a test
denies (`/__control/setup`, `denied`) depend on the host. The token is the key: `Authorization: <token>`, as Wildberries has it. The
neutral fulfilment-centre API is the generated client at `http://127.0.0.1:8792/api/v1` with `Authorization: Bearer <key>`.

**Pointing the wyo server at the fake.** A wyo server built with the `wyotest` tag reads `WYO_MARKETPLACE_URL` (a released one refuses
to start with it): every adapter then puts its hosts' names after that address, and the outbound gate allows it. `pnpm dev` and the
browser tests' start of the wyo server (`e2e/support/wyo-server.ts`) set it to the fake marketplaces, `http://127.0.0.1:8791`, so no
development run and no test reaches a real marketplace. The gate of a `wyotest` build lists no real marketplace host at all: started
without `WYO_MARKETPLACE_URL`, it refuses every marketplace call before anything leaves.

**The request budget of the fake Wildberries** is Wildberries' own: for the Marketplace category a burst of 20 calls refilled one every
200 ms (300 a minute when spread evenly), a call that meets a 4xx costing 10, `X-Ratelimit-Remaining` on every answer but 429 and
`X-Ratelimit-Retry`, `-Limit`, `-Reset` on 429; the cancel, the marking-code, the ping, the seller-information and the statistics
methods have the budgets of their own that the description files give. It counts on the injected clock, so a test moves the clock
instead of waiting.

The fake Wildberries answers the marking-code verdicts a test chooses, returns a small real sticker picture of 58 × 40 mm, closes a supply
with each of its documented refusals, and so on; the fake fulfilment centre signs its notifications by the Standard Webhooks scheme
(<https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md>), checked against a published test vector: the specification itself publishes no signature that can be reproduced (its example
shows a signature and no secret), so the unit test takes the vector of the specification's own reference libraries
(`libraries/php/tests/WebhookTest.php` of the same repository, read on 2026-11-17) with the signing secret that the specification's
endpoint-verification extension publishes. Notifications are sent once each, in order; a failed one is not retried by the fake
(the contract's eight attempts are not played), a test asks for it again with `POST /__control/resend`.

**Driving the exchange with a fulfilment centre** (Plan 4, `docs/external-api.md`). A `wyotest` build accepts a plain `http://` address
for «Внешний API», so the page takes the fake fulfilment centre, `http://127.0.0.1:8792`, with any key of your own. The exchange's jobs
run every minute; a `wyotest` build also answers `POST /__test/exchange/run` (no sign-in) by running every job of the exchange at once,
so a browser test or a person at `pnpm dev` plays the fulfilment centre's part with its control calls (reserve, pack with
`marking_code`s, request a hand-over's documents) and sees the result at once. The fake Wildberries changes an order's sticker on
`POST /__control/orders/{id}/new-sticker`, and `/__control/setup` with `shipping_points` makes a shipping point disappear. A released wyo
server answers neither the test address nor a plain `http://` address.

A fact about a marketplace is written only from its official documents with the address and the date read (`docs/marketplaces/` holds the
pages saved on 2026-11-17); a gap is marked "not documented" and goes to the checks of the first real account (`.agents/backlog.md`).

## Before you push

1. The check of rule R1 (`AGENTS.md`, «The rules R1 to R7»): from outside the repository, search every tracked file and every commit message
   not yet on `origin/main` for each word of the maintainer's list. Push only when nothing is found.
2. One plan = one branch = one commit = one pull request. Commit and push with `--no-verify`; the pull request's run does the hooks' work.
3. A commit of documents only ends its subject with `[skip ci]`.
