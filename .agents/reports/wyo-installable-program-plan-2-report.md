# Plan 2 — the installable program, sign-in, installer, updates, signed releases: report

**DONE.** Released as **0.0.2** on 2026-11-18 (`https://github.com/prankyman/wyo/releases/tag/v0.0.2`: `wyo_linux_amd64`,
`wyo_linux_arm64`, `install.sh`, `VERSION`, `checksums.txt`, `checksums.txt.sig`). The release was checked by hand after download:
the signature of `checksums.txt` is accepted by the committed public key, every file matches its checksum, `wyo version` prints 0.0.2.

- The plan: `.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`.
- Pull request #4 (`https://github.com/prankyman/wyo/pull/4`), one commit, merged as `51ecad7`; the release commit is `d10ae94`.

## The gate

| Where                                         | What                                                                          | Result                                                      |
| --------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------- |
| Development machine (limited: 4 cores, 15 GB) | Focused runs: each changed Go package (both builds), each changed test file   | green; no whole-project type check (the machine's rule)     |
| GitHub Actions, `ubuntu-24.04` (gate)         | Secret scan, commit messages, generate, lint, type checks, tests, build       | 6 min 32 s (run of `dfa940d`)                               |
| Gate — interface                              | Unit tests with coverage                                                      | 1 083 tests in 155 files; 95.44 %, every file ≥ the floor   |
| Gate — Go                                     | Tests with coverage, both builds                                              | 92.3 % of 5 132 statements in 91 files, every file ≥ 84 %   |
| Gate — browser                                | Browser tests against the built wyo server                                    | 55 tests in 13 files, 21.4 s                                |
| Installer job, three clean machines           | `ubuntu-24.04`, `ubuntu-22.04`, `ubuntu-24.04-arm`                            | green, about 3 minutes each                                 |
| After the release                             | The released installer from the real release address, `--behind-proxy`        | green: downloaded, signature and checksums checked, units, owner signs in |

**The installer job proves**, on each machine: the service runs as the user `wyo`; the modes and owners of the program, `wyo.env`, the
data folder, `secret.key` (32 bytes), `certs/` and `backups/`; `https://` answers with a certificate the test authority Pebble issued
through the `http-01` challenge the wyo server answered itself; `http://` is sent to `https://`; a wrong password is refused and the owner
signs in; a second run of the installer changes no file, unit, key or certificate and does not restart the service; a newer release is
installed by the updater after the owner's press (the request file and the path unit), keeping `wyo.previous`, a copy of the database and
the key file; a release that does not come up is put back, the failed version recorded and not retried by the daily run; a release with a
changed program file is refused; `--uninstall` leaves the data.

## Measured (plan section J), 2026-11-18

| Machine           | Memory after start | After 1 000 page and API requests | Released program file |
| ----------------- | ------------------ | --------------------------------- | --------------------- |
| x86_64, 24.04     | 61.7 MB            | 62.8 MB                           | 16.7 MB               |
| x86_64, 22.04     | 61.7 MB            | 62.4 MB                           | 16.7 MB               |
| aarch64, 24.04    | 60.4 MB            | 62.2 MB                           | 15.5 MB               |

The README's requirements carry these numbers with the date. Plan 3 measures again with the synchronisation running.

## Premises (Stage 0), all held

1. Limited machine → focused local runs; the pull request's run is the first whole gate.
2. `modernc.org/sqlite` v1.62.0 with `CGO_ENABLED=0` for amd64 and arm64: write-ahead mode, foreign keys, `VACUUM INTO` during an open
   write — a Go test in `internal/store`.
3. `sqlc` v1.31.1 generates Go for SQLite (pinned in `scripts/tools/versions.sh`; its release has no checksum file, so the sha256 was
   computed from the downloaded archives on 2026-11-17; the same for `shellcheck` v0.11.0).
4. Argon2id with OWASP's parameters read on 2026-11-17: 19 MiB, 2 iterations, 1 degree of parallelism.
5. `net/http.CrossOriginProtection` (Go 1.27) refuses writes from another site.
6. SvelteKit's `csp` in hash mode hashes its own start-up script only; the two scripts of `src/app.html` are hashed by
   `svelte.config.js` from the file itself, and SvelteKit's screen-reader announcer's one style attribute is let through by its hash
   (`'unsafe-hashes'`), read from SvelteKit's own `write_root.js`. zod runs `jitless` (zod's documentation, «AOT compilation»,
   «Content Security Policy») so it never builds a function at run time. A browser test listens for every policy violation.
7. CertMagic v0.25.6 obtains a certificate from Pebble v2.10.3 — proven in the installer job.
8. An Ed25519 signature made by `openssl pkeyutl -sign -rawin` is accepted by Go and the reverse — a Go test with files made by
   `openssl`, and a live check.
9. systemd's `.path` units (`PathExists=`) and `AmbientCapabilities=CAP_NET_BIND_SERVICE` — manual pages of systemd 257, and the
   installer job.
10. The names `wyo`, `/var/lib/wyo`, `/etc/wyo`, `wyo_session` were free.

## The release key

The Ed25519 key pair was made with `openssl` on the development machine on 2026-11-17. The public half is committed as
`internal/release/keys/2026-1.pem` and written into the installer; the private half is the repository's secret `RELEASE_SIGNING_KEY`
and the file `~/.config/wyo-release/signing-key.pem` (mode 600) on the development machine. **The maintainer moves that file to where he
keeps secrets.** The key was never printed, logged or committed.

## Smoke test (locally, the built wyo server in test mode, one browser)

Signing in with a wrong password (refused, the password box emptied) and the right one; «Обзор», «Настройки» and «Мой профиль» at
390 × 844 and 1280 × 800 with screenshots; every section of «Настройки» open; «Сделать копию сейчас» wrote `backups/wyo-<date>.db`; a
waiting version behind the bell; the password changed, signed out, signed in with the new one; an unknown address; the showcase. After
every screen: no console error, no raw text key, no sideways scroll, no request to any address but the wyo server's own (rule R3).
Rule R4: not applicable — no screen shows a key yet. Two findings, both fixed in the plan: an unknown address wrote SvelteKit's
"Not found" into the console (`src/hooks.client.ts` now logs only real faults), and the cards' button read «Перейти в разделе
«Настройки»» (now «Открыть «Настройки»»).

## Rule R1 and the secret scan

The search of rule R1 ran over every file of the working tree and every commit message before each push: nothing found. The pipeline's
secret scan found one made-up test value (`internal/server/app_test.go`, a key a test stores to prove `wyo init` never makes a new key
file over it); it is an exception by its exact value in `.gitleaks.toml`, with its reason.

## Where the plan was not followed to the letter

- **Tests first:** most packages were written test-first with the failing run recorded by the helpers; the commands (`internal/cli`),
  `internal/notices`, `internal/updatestate`, `internal/certstatus` and `config.WithFile` were written by the main agent with their tests
  in the same step, without a separate failing run. Each behaviour has its test; none was changed to pass.
- **Tests changed after the code was shown right:** the help test of `cmd/wyo` (the help now lists every command), the start-up bar's
  test (its height moved from a style text to `src/app.css`, which the page's security policy requires), and the installer job's check of
  the updater's state (it read the state at the moment the new version started; the updater records it only after the version answered
  `/readyz`, which the job now waits for — the job's log showed the two moments 60 ms apart).
- **Extra parts:** `internal/datadir`, `internal/datalock` (the lock that refuses `wyo restore` under a running wyo server),
  `internal/issue`, `internal/testkit/storekit`, `apikit`; `config.WithFile` — a command a person types (`sudo wyo doctor`) reads
  `/etc/wyo/wyo.env` as the service does; `src/hooks.client.ts`; the release job signs before it commits and tags, so a missing secret
  stops a release before anything is pushed.
- **`pnpm dev`** keeps its own data folder `.dev-data/` with the owner `owner` / `development-password`.
- **The library's help buttons** carry the same accessible name as the button they explain (a screen reader hears it twice); it is the
  library's convention as copied, so it is register item 17, not changed here.

## The register

Items 10–17 added (left out of this plan, each with its reason, and the maintainer's checks on a real server, item 16); item 8's shell
parts copied; item 9 (a policy test of rule R7) stays open — this plan added no manifest.

## After the merge

The maintainer, 2026-11-18, seeing the merge's run repeat the pull request's gate and installer job on the same files: «Yes, I want the
change! Why the fuck wait for another 7 minutes?» — pull request #5 (`ci:`, no release): a push to `main` whose merge commit holds
byte for byte the files of a pull-request commit whose run passed skips the gate and the installer job and releases at once.
