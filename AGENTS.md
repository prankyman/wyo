# Project Rules

These rules apply to every AI agent working in this repository — `wyo`, hosted at `github.com/prankyman/wyo` — whatever the tool or
model.

## Where agent material lives

- `.agents/skills/<name>/SKILL.md` - skills: `sveltekit-testing`, `go-testing`, `smoke-testing`.
- `.agents/plans/` - implementation and smoke-test plans. Name each file after what the plan implements.
- `.agents/reports/` - reports and findings produced by executing a plan.
- `.agents/research/` - research a plan relies on (a marketplace's official documents, checked and dated).
- `.agents/backlog.md` - the register: the maintainer's requests and open decisions not built yet; every plan reads it (see "The
  register").
- `.specify/memory/constitution.md` - the project constitution.

## Constitution

CRITICAL: Before writing any code, plans, or tasks, you must read and adhere to the rules defined in `.specify/memory/constitution.md`.
These are the non-negotiable architectural and behavioural principles for this project. Its first principles are rules R1 to R7 of the
high-level plan (`.agents/plans/Marketplace_connector_plan.md`, «The rules that bind every stage»); each of them has a test, named in
the constitution.

## The register

`.agents/backlog.md` holds every request the maintainer made that is not built yet, and every decision still waiting for him. It is
binding:

- Before writing ANY plan, read it. Every open item in the plan's area goes into the plan, or the plan's "Not in this plan" section
  names the item and says why it stays open.
- When the maintainer asks for something the current plan does not build, add it to the register in the same session, with the date and
  his words.
- An item is closed only when the plan that built it is merged or the maintainer drops it; it then moves to "Closed" with that plan and
  commit. Never delete an entry.

## Name the program

Always say which program you mean:

- **the wyo server** — the Go program, one file named `wyo`;
- **the wyo interface** — its pages in a browser, built by SvelteKit and carried inside the wyo server;
- **the installer** — the script that puts the wyo server on a seller's server (Plan 2);
- **the fake Wildberries**, **the fake Ozon** and **the fake fulfilment centre** — the small Go programs that answer as Wildberries, as
  Ozon and as a fulfilment centre's system would, with made-up data (`cmd/wyofakes`; the two fake marketplaces on one address).

Never write "the app" on its own — in code comments, messages, documents, plans and reports alike. The maintainer's other repository is
called exactly that: «the maintainer's other repository».

## The rules R1 to R7

The constitution holds all seven. Two bind every command you run:

- **R1 — no fulfilment product is named, anywhere.** No file, screen, text, example, test, commit message or package name names a
  fulfilment product, its website, its domains or its owner, and none names the maintainer's other repository or any of its words. The
  list of words is kept by the maintainer outside this repository and is never copied into it. **Before every push** the agent searches,
  from outside the repository, the whole tree (`git grep -n -i -e <word>` over every tracked file) and every commit message not yet on
  `origin/main` (`git log origin/main..HEAD --format=%B`) for each word of that list, and pushes only when nothing is found. A copied
  file loses every plan number, every date-and-name attribution and every comment that points at the other project; "the maintainer"
  stands where a name stood, and a dated reason is kept in neutral words. The report of every plan states that the check ran and found
  nothing.
- **R3 — alone by default** and **R4 — a key never leaves the server and is shown only by its last four characters**: no test, no
  development run and no pipeline run calls a real marketplace or a real fulfilment centre; the fakes answer instead.

## Testing

When testing, use the three skills of this repository, always:

- `.agents/skills/sveltekit-testing/SKILL.md` — every test of the wyo interface (Vitest, components, browser tests with Playwright).
- `.agents/skills/go-testing/SKILL.md` — every test of Go code (the wyo server, the fakes, the tools).
- `.agents/skills/smoke-testing/SKILL.md` — a smoke test of the built wyo server in a real browser, and any look at the interface and
  its interactions.

Whenever a plan is written, it must include a request to all AI agents implementing it to follow `AGENTS.md` principles (global and
project) and to use these skills for all test work.

- `pnpm precommit` is the one whole local gate: `pnpm generate:check`, `pnpm lint`, `pnpm check`, `pnpm test:coverage`,
  `pnpm coverage:enforce`, `pnpm build`, in this order, stopping at the first failure. Do not run those steps separately one after
  another when you mean the whole gate.
- Never `pnpm test` (it waits for changes and never ends). Never put `--` before a script's arguments: `pnpm go:test ./internal/webui`,
  not `pnpm go:test -- ./internal/webui`.
- When a long command prints what you need, save its output to a temporary file and read that; do not run the same long command again
  just to filter its output. Delete the file afterwards.

### The test flow of every plan

At the start of every plan, run `nproc` (the number of processor cores) and `free -g` (the memory in GB). The machine is **limited**
when it has fewer than 6 cores OR 16 GB of memory or less — the development machine `server` (4 cores, 15 GB) is limited. Every other
machine is **big**.

1. **Focused runs first, on any machine.** Make the change test-first; then run exactly the files that changed or failed, nothing else:
   ESLint on the changed files, `pnpm test:headless <file>` for one interface test file, `pnpm go:test <package>` for one Go package,
   `pnpm test:e2e e2e/specs/<file>.spec.ts` for one browser test file. Every new source file has its unit test beside it. At most 2–3
   test files run in one go; a failure is fixed with another focused run of the failed file.
2. **The whole local gate, `pnpm precommit`, once, by the main agent, as the final local step — on a big machine only.** A limited
   machine never runs `pnpm precommit` (it holds `pnpm check`): it keeps to focused runs, and the pull request's run is its first whole
   gate. **On a limited machine the whole-project type checks — `pnpm check`, `svelte-check`, any `tsc` with any configuration — never
   run locally** (they ran the earlier machine out of memory); they run in the pull request's GitHub Actions run. Write test code and helpers type-safely (a typed
   stub, no tuple index on an untyped mock, no property a union type may lack), and never fix a type error blind: read every error the
   pull request's run prints, fix all of them, push again. A big machine runs `pnpm precommit` and the whole browser suite
   (`pnpm test:e2e`) locally until both pass.
3. **The pull request's run in GitHub Actions is the gate on a clean machine** (`.github/workflows/ci-cd.yml`). Commit with
   `git commit --no-verify` (later `--amend --no-verify --no-edit`), push, open the pull request, watch the run to its end. A failure is
   fixed locally with focused runs of the failed files, amended and pushed again (`git push --force-with-lease --no-verify`, the plan's
   own branch only). **A run in GitHub Actions is never used to find out whether one test passes.** Measured on the first green
   run (2026-11-17, Plan 1): the whole gate took 4 min 31 s on `ubuntu-24.04` — `pnpm precommit` 2 min 15 s, the browser's install
   50 s, the browser tests under 15 s.
4. **The smoke test** runs locally against the built wyo server and the fakes, one browser (`smoke-testing` skill). It is skipped for
   work that is documents only.

Only the **main agent** runs the heavy checks: `pnpm precommit`, `pnpm check`, `pnpm lint` over the whole tree, `pnpm test:coverage`,
`pnpm test:e2e` over the whole suite, `go test ./...` over the whole module, and any other whole-codebase check or full test suite. A
subagent runs only its own test files, a lint of its own changed files and the `go test` of its own package. Several agents
type-checking or testing at the same time used up the machine's memory and crashed it. **At most 3 subagents run at the same time**,
each with an explicit list of files that no other agent touches. Every prompt given to a subagent repeats both rules above and the rule
"everything is a component and is reused".

Three things keep a shared working tree safe while subagents work in it: never run a command that rewrites the working tree
(`git stash`, `git checkout --`, `git reset`, `git restore`; compare with `git show HEAD:<path>` instead); never start a server in a
process a subagent's end could kill; and a browser-test run of one agent passes `--output=<a folder of its own>`, because a run without
it empties `test-results/` under another run.

### Commits, branches and pull requests

- **One plan = one branch = one commit = one pull request.** Create the branch once, before the first edit, from an up-to-date `main`,
  named `<type>/<short-description>` with `feat/`, `fix/`, `chore/`, `docs/` or `refactor/`. If the branch ends with several commits,
  squash them into one before pushing, with a message whose bullets cover every part.
- **A commit of documents only** ends its subject with `[skip ci]` and is made and pushed with `--no-verify`; only the secret scan
  (`gitleaks`) runs for it.
- **Commit messages** follow `.specify/memory/constitution.md`, «Commit messages». Never an exclamation mark after the type (`feat!:`)
  and never a `BREAKING CHANGE:` line without the maintainer's explicit yes: the release tool reads them and raises the major version. A
  warning goes into the commit body as plain prose.
- Never add uncommitted code to git and commit it without the maintainer's approval. Approving a plan whose workflow includes
  committing, pushing and merging counts as that approval.
- Nothing is pushed to `main` while a release run is going.

## Migrations

NEVER edit a database migration that has already been released. A schema change is always a NEW numbered file in
`internal/store/migrations/`. A seller's database is upgraded where it lies and never rebuilt: the wyo server records each applied
migration's checksum and refuses to start when its own copy of an applied migration differs, so an edited migration stops every
installation that applied it. `internal/store/migrations.sum` holds the checksums and a test fails when a listed file changes; the databases
of released versions in `internal/store/testdata/` are upgraded by a test. Constitution, principle VIII; `docs/development.md`, «The
database, the commands, the installer».

## Tooling and commands

1. Use `pnpm` instead of `npm`.
2. Always use scripts defined in the `scripts` section of `package.json` instead of ad-hoc shell command sequences.
3. No explicit `any` types in TypeScript.
4. Monitor and stop execution of terminal commands if they run unusually long. If a test file runs longer than 20 seconds, there is
   probably a loop that never exits.
5. Max run length of any terminal command is 2 minutes. If longer — stop it and find out why. The one exception is a run in GitHub
   Actions.
6. Go tools are installed only by `pnpm tools:install` at the versions written once in `scripts/tools/versions.sh`.

## Everything is a component and is reused

NON-NEGOTIABLE; the full rule is in `.specify/memory/constitution.md`, «Everything is a component and is reused».

1. Every piece of the interface that appears in more than one place — a group of fields, a card, a table, a button with its answer, a
   block of a printed document — is ONE component in `src/lib/components/<area>/`, used everywhere it appears. The same for logic: one
   function, called everywhere, never a copy.
2. Before writing a piece, search for the component or function that already does it, and use it. About to write it a second time: turn
   the first one into a component and use it in both places, in the same plan. A piece the component library lacks is copied from the
   maintainer's other repository by the plan that first needs it (`docs/library-layout.md`), never invented.
3. A copy found later is a defect: the plan that finds it turns it into a component and replaces every copy.
4. A component is tested once, in its own test file, in every state; a page's test checks only what the page adds and never tests a
   component's insides again; a browser spec walks a component once per flow.
5. Every plan names the components it creates and reuses; every subagent prompt repeats this rule.

On 2026-11-07 the same bank fields had been drawn four times and the same legal-identity fields four times in the other repository, each
with its own tests — more code, more tests and longer test runs for nothing.

## Never make a test pass by changing what it checks

NON-NEGOTIABLE. A failing test is a fact about the program. You may NEVER change a test only so that it passes: not its expected values,
not what it counts, not its filters, not its waits, not by adding a case that hides the failure, not by skipping, retrying or loosening
it (`toBeGreaterThan(0)` in place of an exact number is loosening). Find the cause first; "flaky" is not a cause. If the program is
wrong, fix the program. The full text is in the constitution and in both testing skills.

## Reading files

- NEVER use generic single-command file readers (like `cat` or standard view tools) on files exceeding 500 lines.
- If a file is long, you MUST read it using a paginated loop (specific line-range offsets) until the final line is reached.
- Do not accept `[Truncated]` data structures; run `sed` or `awk` to grab subsequent chunks if text cuts off.

## Documentation first

Never provide advice, root cause analysis or expert statements without first researching official documentation. This applies when
giving a root cause, explaining why an error occurred, giving expert technical advice, diagnosing bugs, recommending solutions or
workarounds, or explaining framework or library behaviour. Before providing any explanation or advice:

1. Search for official documentation related to the issue.
2. Read the relevant documentation thoroughly.
3. Base your explanation on the documentation, not on assumptions or general knowledge.
4. Provide direct links to the documentation sources, and quote or reference specific sections when applicable.
5. If documentation is unclear or unavailable, say so explicitly.

A fact about a marketplace is written only from its official documents, with the address and the date it was read.

## Where to read next

- `docs/development.md` — prerequisites, daily commands, where things are, how the fakes are driven.
- `docs/library-layout.md` — what the component library holds and what waits to be copied.
- `.agents/plans/Marketplace_connector_plan.md` — the high-level plan; the numbered plans beside it are its stages.
