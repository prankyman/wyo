---
name: go-testing
description: 'Use when writing or reviewing tests of Go code: the wyo server, the fakes and the Go tools (test-first, table-driven).'
metadata:
    author: local
    version: '1.0.0'
---

# Go Testing (the wyo server, the fakes, the tools)

Testing guidance for the Go code of this repository. Read this before writing or changing any Go test. It is the Go counterpart of the
`sveltekit-testing` skill (`.agents/skills/sveltekit-testing/SKILL.md`): every rule below stands where an interface rule stands, so the two
sides are tested in the same way.

**Read alongside:** `.specify/memory/constitution.md` (principle II is the authority this skill implements; principle III governs reuse;
principle IV, policy tests; rules R3 and R4, the fakes and the key canary).

---

## The rules

### 1. Tests first, failing first

A test is written BEFORE the code it checks, and it fails first (red, green, refactor).

- Show the failure: run the new test before the code exists or while the behaviour is missing, and put the failing output into the progress
  note. A test that never failed proves nothing.
- A compile error is not a red test. Write the smallest stub that compiles (a function that returns the zero value), then see the assertion
  fail.
- **`t.Skip` is never a fix.** Neither is a retry, a longer wait or a loosened assertion. A skipped test is deleted or fixed in the same
  plan.

### 2. One behaviour per test; enumerations are table-driven tests with named cases

A test covers one behaviour and may carry as many assertions as that behaviour needs. Splitting one behaviour across several test functions
to hold one assertion each multiplies set-up for nothing.

A list of inputs and expected outputs is ONE test with a table of named cases and `t.Run(name, …)` — the counterpart of `it.each`:

```go
func TestRefusalCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{name: "unknown address", status: http.StatusNotFound, want: "not_found"},
		{name: "wrong method", status: http.StatusMethodNotAllowed, want: "method_not_allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, refusalCode(tc.status))
		})
	}
}
```

More than about 10 new cases in one table is a question to ask yourself: are they behaviours or assertions? The reviewer will ask.

### 3. `x_test.go` beside `x.go`; the test uses the public behaviour

- A test file sits next to the file it tests, in the same folder: `x_test.go` beside `x.go`.
- The test is in the package `x_test` (the external test package) and uses only what the package exports, unless that is impossible; then
  the reason is one comment line at the top of the test file.
- Every new source file has its test beside it. A package that only holds generated code needs none; its owner's tests prove it.

### 4. No stand-in for the database

Every test that needs the database opens a NEW SQLite file in `t.TempDir()`, applies the REAL migrations and uses the REAL code. No
in-memory stand-in, no hand-written fake of the storage layer, no mock of a query. A test that proves a mock proves nothing about the
program. (Plan 2 brings the database and the helper that opens it; this rule is written now so that no test is ever written another way.
There is no database before Plan 2.)

### 5. No real marketplace and no real fulfilment centre, ever

A test starts the fakes of `internal/fakes/` in its own process (`httptest` servers) and points the code at their addresses:

- the fake Wildberries (`internal/fakes/wildberries`) answers the methods the program uses, in Wildberries' own shapes;
- the fake fulfilment centre (`internal/fakes/fulfilment`) implements the neutral fulfilment-centre API;
- both are built on the fake kit (`internal/fakes/fakekit`): a state of its own for every test (chosen by the key the test sends),
  `/__control/health`, `/__control/reset`, `/__control/calls` (every received call without its key), and ordered faults (answer 429 with the
  limit headers, answer 500, answer slowly).

A test never reaches any other address. A test that would is a defect, because it would call a real marketplace with a real key (rule R3).
Use a key of your own for each test, so that tests running side by side do not share a state.

### 6. Time is given, never taken

- Code reads the time from an injected clock (`internal/clock`); a function that needs "now" takes the clock as an argument.
- Tests use fixed moments **at midday in a named time zone** (`time.Date(2026, 10, 1, 12, 0, 0, 0, moscow)`), never midnight, never the
  machine's time zone.
- `forbidigo` in the committed `golangci-lint` configuration forbids `time.Now` outside the clock package and `time.Sleep` in tests. To wait
  for something, wait for the condition (`require.Eventually`, a channel, a `sync.WaitGroup`), never for a fixed time.

### 7. Logs are captured and asserted; a key never leaks

- A test that drives a path which logs captures the log (the logger writes to a buffer the test owns) and ASSERTS the line. Never leave log
  output unhandled; never suppress it without asserting it.
- **The canary test** proves a key never reaches a log line, a journal row, an answer of the wyo server's own API or a request to a
  fulfilment centre. It saves a made-up key with a recognisable value, drives every path that touches it, and fails when the value is found
  in any of those four places. Every plan that adds a place a key can flow to adds that place to the canary (rule R4).

### 8. The router's table of who may call what is tested by walking every path

The router is tested by walking EVERY registered path with EVERY method, and asserting the table of who may call what: the status and the
refusal code of each combination. A path nobody wrote a row for fails the test, so a new endpoint cannot ship unguarded.

### 9. The race detector, shuffled order, fuzz tests and test data files

- Tests run as `go test -race -shuffle=on -count=1` (`pnpm go:test` does it). A test that depends on another test's order or leftovers is
  broken; fix it, do not pin the order. Every test that can run beside others calls `t.Parallel()`.
- A parser of anything a marketplace or a fulfilment centre sends has a **fuzz test** (`FuzzXxx`) with a committed seed corpus under
  `testdata/fuzz/`.
- Expected files (answers of the fakes, generated documents, label pictures) live in `testdata/` beside the package. They are rewritten
  only by an explicit flag (`-update`) that the test itself defines, and the difference is READ before it is committed. A changed
  expected file is a change in behaviour, never a formality.

### 10. Coverage

The Go side has the same two hard tiers as the interface side: the whole and EVERY file at or above one floor, declared in exactly one
place: the constant in `tools/covercheck`. `pnpm test:coverage` writes the profile (`go test -coverpkg`), and `pnpm coverage:enforce`
runs `tools/covercheck` over it. Read the figure there; do not write it in a test, a comment or a document.

- A file below the floor is exactly one of two things: missing a test for behaviour someone would notice (an error path, a refusal, a
  limit), or a file with no testable surface. Write the test, or exclude the whole file.
- **An exclusion needs a written reason** beside its entry: what a test would assert and why that is worthless. "Hard to test" is not a
  reason. Generated code is excluded as generated.
- Coverage is a floor, not a target. Never add a test only to raise a percentage; never add one that asserts a fallback exists.

### 11. The commands, and never change what a test checks

The commands table is below. The last rule, word for word from the global rules, is binding:

> **NEVER make a test pass by changing what it checks (the maintainer, 2026-11-13) — NON-NEGOTIABLE, every project, every agent**
>
> A failing test is a fact about the program. You may NEVER change a test only so that it passes: not its expected values, not what it
> counts, not its filters, not its waits, not by adding a case that hides the failure, not by skipping, retrying or loosening it
> (`toBeGreaterThan(0)` in place of an exact number is loosening).
>
> 1. When a test fails, find the CAUSE first. "Flaky" is not a cause; it is a name for a failure whose cause is not known yet.
> 2. When the cause is not known, say so plainly. Do NOT guess a cause and edit the test to match the guess. Put the evidence (the state of
>    the queue, the rows, the times) into the failure message WITHOUT touching the assertions, and reproduce the failure by running the test
>    repeatedly.
> 3. A test may be changed only after the cause is shown, and only when the test itself was wrong about the program's behaviour. Say what
>    was wrong with the test, in plain words, and ask the maintainer when the change makes the test weaker.
> 4. If the program is wrong, fix the program, not the test.
> 5. Tests are written BEFORE the code they check, and fail first (red, green, refactor). Read the project's testing skill before
>    writing any test.
>
> Why: on 2026-11-13 an integration test failed once in a whole run; its cause was unknown, and it was changed to count another file's job
> so it would pass. The guess about the cause was also wrong; the maintainer caught it. A test that passes without proving the behaviour is
> worse than no test.

To reproduce an intermittent failure, run the ONE test many times in a row (the usage line at the top of `scripts/go/test.sh` says how
to pass `-run` and `-count` to it), and keep the output in a temporary file in the scratchpad instead of running the same long command
again.

---

## Commands

| Command                           | What it does                                                                         |
| --------------------------------- | ------------------------------------------------------------------------------------ |
| `pnpm go:test <package>`          | `go test -race -shuffle=on -count=1` for ONE package: use while writing and fixing   |
| `pnpm go:test`                    | The same for `./cmd/... ./internal/... ./tools/...` — main agent only                |
| `pnpm test:coverage`              | Interface and Go tests with coverage — main agent only                               |
| `pnpm coverage:enforce`           | The two floor scripts: `scripts/coverage-check.cjs` and `tools/covercheck`           |
| `pnpm lint`                       | ESLint, `gofumpt -l`, `golangci-lint run`, `go vet` — main agent over the whole tree |
| `pnpm exec eslint <files>`        | Lint of the interface files you changed                                              |
| `pnpm generate`, `generate:check` | Make the generated code; fail when a committed generated file is stale               |
| `pnpm tools:install`              | Install the Go tools at the pinned versions into `.tools/bin`                        |
| `go vet ./<package>/...`          | A subagent may run it over its own package                                           |
| `go build ./<package>/...`        | A subagent may run it over its own package                                           |

- **Never** a bare tool when a script exists. Go tools are used from `.tools/bin` through the scripts, never from a global install.
- **Never `go test ./...` over the whole module, and never `golangci-lint run` over the whole module, in a subagent.** Only the main
  agent runs whole-codebase checks and full test suites; a subagent runs the tests of its own package and the lint of its own changed
  files. Check `free -g` shows at least 3 GB available before a run.
- **Never `--` before a script's arguments:** `pnpm go:test ./internal/webui`, not `pnpm go:test -- ./internal/webui`.
- No command runs longer than 2 minutes. A test that runs longer than 20 seconds probably waits for something that never comes.

## Where tests and fixtures live

- `x_test.go` beside `x.go`; expected files in `testdata/` beside the package; fuzz corpora in `testdata/fuzz/`.
- A fixture used by two test files is ONE helper in one place (a `internal/<area>/<area>test` package or the fake kit), never a copy: the
  rule "everything is a component and is reused" holds in tests too. Search before writing a helper.
- The architecture tests are in `internal/archtest` (ordinary tests that read the import graph); a new rule that is about imports gets a row
  there, not a new mechanism.

## Checklist before committing

**Economy and correctness**

- [ ] Written before the implementation, and I saw it fail (the output is in the progress note)
- [ ] One behaviour per test; enumerations are one table with named cases; no more than about 10 new cases in one table without a reason
- [ ] `x_test.go` beside `x.go`, package `x_test`
- [ ] Assertions are specific and hard-coded; expected files read and understood before committing
- [ ] No `t.Skip`, no `time.Sleep`, no `time.Now`, no retry

**The world around the code**

- [ ] The database, if any, is a real SQLite file in `t.TempDir()` with the real migrations
- [ ] Only the fakes are called; each test uses a key of its own
- [ ] Logs are captured and asserted; the key canary covers every new place a key can flow to
- [ ] A new route is in the router's walk of every path and method
- [ ] A new parser has a fuzz test

**Gates**

- [ ] `pnpm go:test <package>` passes under `-race -shuffle=on`
- [ ] The new files reach the floor of `tools/covercheck`, or are excluded with a written reason
- [ ] `go vet` and the lint of the changed files pass
- [ ] No test was changed to make it pass; if one was, the cause was shown and said in the report
