# The final tests on the Mac mini; GitHub Actions only scans, tests the installer, builds and releases

Register item 39 (`.agents/backlog.md`). The maintainer, 2026-11-19: «настроишь такую-же систему для wyo - гонять финальные тесты сначала
на mac mini m4 (and/or webserver? split 70|30%? think if the webserver is needed in this case), а на GH - отключи их все, ибо он страшно
медленный». A pull request's run in GitHub Actions took 16 to 28 minutes.

Every AI agent implementing this plan follows `AGENTS.md` (global `~/.agents/AGENTS.md` and this repository's), and uses the testing
skills of this repository (`.agents/skills/go-testing`, `sveltekit-testing`, `smoke-testing`) for all test work.

## Decisions (the maintainer, 2026-11-19)

1. **The Mac mini M4 only** (macOS, Apple M4, 12 cores, 24 GB; its address is in the ignored scripts/remote/hosts.env). The web server is not used:
   wyo's suites are small (112 browser tests in about a minute), a split would add a second machine for minutes saved at most, and the
   web server carries a production service of the maintainer's other repository.
2. **The Go tests run on the Mac too** (macOS): wyo has no Linux-only source file.
3. **The installer's test stays in GitHub Actions** (three clean Ubuntu machines, 22.04, 24.04 and ARM, with systemd; about 2.5 minutes,
   side by side): it cannot run on a Mac.
4. **GitHub does not check that the Mac's run passed** (the maintainer, 2026-11-19: «сделай так же, без отметки», as in the
   maintainer's other repository). The agent pushes only a commit `pnpm remote:gate` passed, and runs it again after an amend or
   rebase that changes code; one that touches only documents or `.github/workflows/` needs no new run. A mark on the commit (the commit status `remote-gate`, set by a script after the push) was built first and dropped: the
   token of the development machine may not set commit statuses (HTTP 403, «forbidden»).

## What is built

### A. The Mac's set-up, once — `pnpm remote:setup` (`scripts/remote/setup-mac.sh`)

- A clone of its own, `~/wyo-runs/wyo`, from `https://github.com/prankyman/wyo.git` (the repository is public: no key on the Mac).
- Go at the version of `go.mod`'s `toolchain` line, the official darwin-arm64 archive from go.dev, its SHA-256 checked, into
  `~/wyo-runs/go`; Node at `.nvmrc` through the Mac's nvm; pnpm through corepack; `pnpm tools:install`; Chromium for Playwright.
- `scripts/tools/install.sh` gains pinned darwin-arm64 archives (with SHA-256) of sqlc and shellcheck in `scripts/tools/versions.sh`.

### B. The final run — `pnpm remote:gate` (`scripts/remote/gate.sh` on the development machine, `scripts/remote/mac-gate.sh` on the Mac)

- Carries the committed HEAD only (uncommitted changes of tracked files are refused; untracked files are listed and never carried) as a
  git bundle over SSH: nothing is pushed anywhere for the run.
- On the Mac, under a run lock (`lockf`), in a tmux session of its own (a dropped connection does not end it), with a 55-minute
  watchdog: the commit checked out detached, `pnpm install --frozen-lockfile`, the commit messages of `origin/main..HEAD`
  (commitlint), `pnpm precommit` (generate check, lint, type checks, tests with coverage, coverage floors, build), `pnpm test:e2e` (the
  whole browser suite), `pnpm go:measure`.
- The report comes back into `.remote-runs/<stamp>/` (ignored by git): `summary.md`, `run.log`, the browser report.

### C. GitHub Actions (`.github/workflows/ci-cd.yml`)

- `gate` keeps only the secret scan (gitleaks); it drops the installs, the commit-message check, `pnpm precommit`, the browser tests
  and `pnpm go:measure`. It does not check the Mac's run (decision 4).
- `installer`, `release`, `released-installer` and the `tested` rule (a merge whose files are the pull request's last commit's) stay.
- A merge that is not byte for byte the pull request's last commit is run on the Mac before it is pushed to main.

### D. A rule for main (ruleset)

- A repository ruleset on `main`: the check `gate` must pass before a merge, the release job (GitHub Actions) may push its version
  commit. Not created: the token may not (HTTP 403, «forbidden»); the maintainer can add it by hand.

### E. Documents

- `AGENTS.md` and `docs/development.md`: the test flow — focused runs locally, then `pnpm remote:gate`, push only the commit it passed, open
  the pull request, merge only on a green run. The Mac's host and port stay out of the repository: `scripts/remote/hosts.env` (ignored
  by git; `scripts/remote/hosts.env.example` committed).
- The register: item 39 closed by this plan.

## Not in this plan

- The web server and a 70/30 split (decision 1).
- Register item 38's documents `error` / `cancelled` (waits for item 34).

## Workflow

One branch `chore/final-tests-on-the-mac-mini`, one commit, one pull request. Its own gate is the first real `pnpm remote:gate` on the
Mac; the pull request merges only when GitHub's run is green. The R1 search runs before every push.
