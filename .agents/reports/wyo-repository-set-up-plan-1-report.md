# Plan 1 — the repository set up: report

**DONE.** Released as **0.0.1** on 2026-11-17 (`https://github.com/prankyman/wyo/releases/tag/v0.0.1`: `wyo_linux_amd64`,
`wyo_linux_arm64`, `checksums.txt`; the checksums and `wyo --version` checked after download).

- The plan: `.agents/plans/wyo-repository-set-up-agent-rules-skills-constitution-tools-component-library-theme-fakes-fulfilment-centre-api.md`.
- Pull request #2 (the plan's one commit, merged as `8e37a78`) and pull request #3 (`a865717`, the release job's fix below).

## The gate

| Where                                         | What                                                                                  | Result                                                               |
| --------------------------------------------- | ------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Development machine (limited: 4 cores, 15 GB) | Go tests with coverage, both builds (`scripts/go/coverage.sh`, `coverage-enforce.sh`) | 93.9 % of 2 717 statements in 41 files, every file ≥ the floor; 23 s |
| Development machine                           | Go lint (`gofumpt`, `golangci-lint`, `go vet`), `generate:check`                      | 0 issues                                                             |
| Development machine                           | Interface unit tests with coverage                                                    | 845 tests in 112 files, 94.78 %, every file ≥ the floor; 45 s        |
| Development machine                           | Browser tests                                                                         | 22 tests in 6 files; 7 s                                             |
| GitHub Actions, `ubuntu-24.04`                | The whole gate, the type checks included (first green run 44180551170)                | 4 min 31 s                                                           |

The whole-project type check never ran on the development machine (the maintainer's rule for a limited machine); the first pull
request run found one error (`src/lib/forms/super-form.ts`, a form's success answer typed against an open message type), fixed
before the merge.

## Premises

1. Limited machine → focused local runs, the pull request's run as the first whole gate.
2. Go 1.27.4 (official archive, checksum checked) in `~/.local/go` — the install into `/usr/local` with `sudo` was refused by the
   agent's permission settings; `gcc` 14.2 from Debian; `gh` 2.104.0 (official archive, checksum checked) in `~/.local/bin`.
3. The token opens and closes a pull request (test pull request #1, closed).
4. **oapi-codegen v2.8.2** reads Wildberries' files: one Go package per file (the files share type names); recorded in the
   constitution. `ogen` was not needed.
5. The interface built by `adapter-static` and embedded with `all:dist` is served by the wyo server (Go test).
6. Single-page forms with `zod` work without a server (`src/lib/forms/super-form.svelte.test.ts`).
7. The language prefix `/xx/…` works in static files (`src/hooks.ts`, tested with a test-only language; checked in a browser).
8. Tailwind `@theme` gives `bg-brand-600`, `text-ink-800`, `bg-ok-50` (built CSS read); the palette is switched off.
9. The interface's packages at the other repository's versions (caret ranges; Playwright resolved to 1.65.1).
10. The whole gate in GitHub Actions: 4 min 31 s, written into `AGENTS.md`.

## Rule R1 and the neutral API

- The search of rule R1 ran over every tracked file and every commit message before each push: nothing found. One accepted hit:
  the bare word «WMS» in Wildberries' own downloaded description file (`api/wildberries/01-general.yaml`, line 34, «ERP, WMS, OMS,
  CRM»), which is not on the list.
- The one-time comparison of `api/fulfilment-centre.yaml` with the format the maintainer provided: «OK: 21 operations, 15 schemas,
  18 events, every path, method, parameter, field, required list, enum and limit equals the format file», with 9 deliberate
  differences (the stock and supply parts of later stages left out; generator naming hints).

## Where the plan was not followed to the letter

- **`pnpm precommit` never runs on a limited machine** (`AGENTS.md`, the constitution and the testing skill say so): it holds the
  whole-project type check.
- **`AttentionIcon`** was 16 × 16 px; the showcase sweep found it. Its button is now a 44 × 44 tap box with the triangle inside at
  its size; its test of the triangle's size now reads the triangle, not the button.
- **The release job** committed through the repository's own pre-commit hook, which needs Go tools the job does not install; the
  first release run stopped before its commit (nothing pushed). Pull request #3 commits and pushes with `--no-verify`.
- **Wildberries' generated code** is four packages (`wbapi/general`, `items`, `orders`, `reports`), not one.
- **Extra Go packages:** `buildinfo`, `refusal`, `server`, `covercheck`, `testkit/httpkit` — each one job, used by several.
- **The Standard Webhooks test vector** comes from the reference library's own test with the example secret the specification
  publishes (the specification's own example has no secret).
- **`.gitleaks.toml`** allows four exact values: two made-up canary keys of tests, the specification's example secret and one line
  of the compressed description the generator embeds (it changes with `api/fulfilment-centre.yaml`).
- **Register item 9** (a policy test for rule R7) was added: the plan names no test for R7.
- **The fakes' unsourced choices** (which refusal a cancel meets, whether codes are replaced, the sticker's text, the 429 headers'
  rounding, …) are commented in the code; each is a check of the first real account (register item 5).
- Button and badge letters are now ordinary letters, medium weight, at the sizes the library had (`text-xs` on a medium button).
