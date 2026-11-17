# The marketplace connector as a program a seller can install, part 1 of 3 of Stage 1: the wyo server with its database, the owner's sign-in, the interface's top bar, menu and first pages, one installer script that asks for the domain and the owner's login and password, HTTPS obtained by the program itself, signed releases, updates that install themselves, the daily copy of the database, and the step-by-step instructions

> **Plan 2** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 1 of the three plans of **Stage 1** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md` (the maintainer, 2026-11-17: «3»).
> Written 2026-11-17 from `origin/main` at `ab8838f`; **it starts only after Plan 1 is merged and released**
> (`.agents/plans/wyo-repository-set-up-agent-rules-skills-constitution-tools-component-library-theme-fakes-fulfilment-centre-api.md`),
> and its section «Today» is checked again on that day.
> Research: the high-level plan's «MC» §6 (installers, HTTPS, signed releases — kept by the maintainer outside this repository); the
> official documents each premise names.
>
> **The plans beside it.** Plan 3 `.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`
> and Plan 4 `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`
> build on this plan's database, sign-in, menu, settings, notices, secret store, scheduler and outbound-call gate.
>
> **Branch** `feat/installable-program-sign-in-updates`. One plan = one branch = one squashed commit = one pull request. The commit is
> `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: nobody uses the program yet. **Database migrations: the first one.**
> Constitution: the database principle and the outbound-call principle gain their tests and details, **MINOR**.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused".

## Terms used below

- **The wyo server** — the Go program, one file named `wyo`. **The wyo interface** — its pages in a browser, carried inside it.
  **The installer** — the script `install.sh` of section G. **The updater** — the wyo server's own command `wyo update`, run by the
  system on a timer and on the owner's press.
- **The seller** — the company that installs wyo on a server it owns or rents. **The owner** — the one person who signs in.
- **A release** — one published version: the wyo server for two processor families, the installer, a list of checksums and the
  list's signature.
- **A checksum** — a short text computed from a file (SHA-256); a changed file gives another text. **A signature** — a text only the
  holder of the project's private key can make for a file, and which anybody can check with the public key.
- **The key file** — a file on the seller's server, readable only by the wyo server's own system user, with which stored secrets are
  encrypted (high-level plan, «Keys at rest»).
- **A certificate** — what lets a browser open the wyo interface by `https://`. **A certificate authority** — the organisation that
  issues it (Let's Encrypt by default).
- **A notice** — one line behind the bell in the top bar that tells the owner something needs attention.
- **A migration** — a numbered file of database changes, applied once to every database, in order.

## The maintainer's request and answers — nothing is open

- 2026-11-16, the high-level plan: «easily installed on any linux OS (some VPS|VDS or similar) with some script that just installs
  the whole thing, and only, probably asks the domain name and couple other settings (such as, I guess, the username|password of the
  owner). The website (software) itself upon the first log-in just asks for the marketplaces … seller API keys and then just works.»
- 2026-11-17, the people who sign in: «What more people? … Is this a sought-after functionality?» — **one owner only.** The
  high-level plan's "more people added by the owner" stays in the register until sellers ask (section «Not in this plan»).
- 2026-11-17, testing the installer and HTTPS on clean machines of GitHub Actions with a test certificate authority, the maintainer's
  own check on a real server after the release: «If the GH Actions passes fine for you - not a problem!»
- 2026-11-17: «Be very thorough with instructions on how to install it and use it, most probably - in README.md, unless you have a
  better proposition.» — section K: the README carries the whole path from an empty server to the first sign-in; matters a seller
  needs rarely are pages of `docs/` linked from it; the same guidance stands in the interface.
- 2026-11-17: «Russian only. But do create the languages capability.» — every text of this plan is a key of `messages/ru.json`.

## Today (to be checked again on the day the plan starts)

- After Plan 1 the wyo server serves the interface's start page and `GET /api/v1/meta`; it has no database, no sign-in, no settings.
- A release (Plan 1, section L) is two program files and `checksums.txt`, unsigned. Nothing installs it.
- The component library, the theme, the text system, the form wrapper, the one API module `src/lib/api/` and the policy tests exist.
  The top bar, the menu, the sign-in parts, the bell and the language switch wait in the other repository (Plan 1, section E).
- The repository's rule R3 (alone by default) and R4 (keys never leave the server) have no test yet.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **SQLite without a C compiler:** the driver `modernc.org/sqlite` (v1.62.0 on 2026-11-17) builds with `CGO_ENABLED=0` for
   `linux/amd64` and `linux/arm64`, opens a database in the write-ahead mode with foreign keys on, and `VACUUM INTO` writes a whole
   copy while the database is in use (`https://www.sqlite.org/lang_vacuum.html`). Proven by a Go test. If not, stop and ask.
3. **Typed queries:** `sqlc` (`https://docs.sqlc.dev`, v1.31.1 on 2026-11-17) generates Go for SQLite from this plan's first query
   file. If it cannot, queries are written by hand in one package and the constitution says so.
4. **Passwords:** the parameters of Argon2id are taken from OWASP's password storage page read on the day
   (`https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html`); the function is
   `golang.org/x/crypto/argon2`.
5. **Requests from another site are refused by Go's own protection** (`net/http`'s cross-origin protection, in Go since 1.25 — its
   documentation read on the day). If it does not fit, the same test is met by a check of the `Origin` header written once.
6. **The page's security policy allows the interface's own scripts and nothing else:** SvelteKit's `csp` setting in its hash mode
   works for static files (`https://svelte.dev/docs/kit/configuration#csp`), the two start-up scripts of `src/app.html` included.
   Proven by a browser test whose console shows no refusal.
7. **Certificates by the program itself:** the library CertMagic (`https://github.com/caddyserver/certmagic`, v0.25.6 on
   2026-11-17) obtains and renews a certificate, keeps it in a folder, and can be pointed at another certificate authority — which
   the tests need: Pebble, the small test authority Let's Encrypt publishes (`https://github.com/letsencrypt/pebble`). Proven in the
   pipeline's installer job (section L).
8. **A signature both sides can check:** an Ed25519 signature made with `openssl pkeyutl -sign -rawin` is accepted by Go's
   `crypto/ed25519`, and one made by Go is accepted by `openssl pkeyutl -verify -rawin` (OpenSSL's manual page of `pkeyutl`, read on
   the day). It needs OpenSSL 3 on the seller's server; Debian 12 and 13, Ubuntu 22.04 and later and the Red Hat family from 9 have
   it. Proven by a Go test with a pair of files made by `openssl`. If the two do not agree, the second choice is a signature checked
   with `ssh-keygen -Y verify`; stop and ask before taking it.
9. **A system unit may start another when a file appears** (systemd's `.path` units) and a program run by systemd as an ordinary
   user may listen on ports 80 and 443 (`AmbientCapabilities=CAP_NET_BIND_SERVICE`) — systemd's manual pages, read on the day, and
   the installer job.
10. The names this plan takes are free: the system user `wyo`, the folders `/var/lib/wyo` and `/etc/wyo`, the cookie `wyo_session`.

## A. The wyo server's commands, folders and modes

One program, several commands (texts of every command come from the same `messages/ru.json`, embedded in the program, through one
small Go function — so a second language later covers the commands too):

| Command                                  | What it does                                                                                                                         |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `wyo serve`                              | Runs the wyo server.                                                                                                                 |
| `wyo version`                            | Prints the version.                                                                                                                  |
| `wyo init`                               | Creates the data folder, the key file and the database if they are missing; changes nothing that exists.                             |
| `wyo owner create`, `wyo owner reset-password` | Creates the owner (once) or sets a new password; the password is read from the keyboard without being shown, or from a file. |
| `wyo update`, `wyo update --check`       | Looks for a newer release and installs it; or only looks.                                                                            |
| `wyo backup`, `wyo restore <file>`       | Writes a copy of the database now; puts a copy back (only while the wyo server is stopped).                                          |
| `wyo doctor`                             | Prints, line by line, what is right and what is wrong: the service, the domain's address, ports 80 and 443, the certificate, the disk, the last copy, the last update. |

| Place on the seller's server      | What it holds                                                                                                               |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `/usr/local/bin/wyo`              | The program. `wyo.previous` beside it is the version before the last update.                                                |
| `/etc/wyo/wyo.env`                | What the installer was told: the domain, the mode. Readable by the system user `wyo`, written only by root.                  |
| `/var/lib/wyo/wyo.db`             | The database (SQLite).                                                                                                      |
| `/var/lib/wyo/secret.key`         | The key file: 32 random bytes, mode 600, owner `wyo`.                                                                       |
| `/var/lib/wyo/certs/`             | The certificate and its key.                                                                                                |
| `/var/lib/wyo/backups/`           | The daily copies of the database, 7 kept, and the copy made before each update.                                             |

**A released wyo server has one way of running — installed.** The development and test ways (plain `http` on `127.0.0.1`, the
showcase page, the fakes' addresses, another release address, another certificate authority, another accepted signing key) exist
only in a wyo server compiled with the Go build tag `wyotest` (Plan 1, section E), which `pnpm dev`, the browser tests and the
installer job use. A released wyo server does not contain that code, and a Go test of the release build proves it.

## B. The database — written once, upgraded in place for ever

A seller's database is never rebuilt: every later release upgrades it where it lies. So:

- **Migrations** are numbered SQL files inside the program (`internal/store/migrations/`). At start the wyo server applies the ones
  a database lacks, **all of one start in one transaction**: either every pending migration is applied or none.
- **An applied migration is never edited.** The table `schema_migrations` stores each one's checksum; a wyo server whose own copy of
  an applied migration differs refuses to start and says which. The committed file `internal/store/migrations.sum` holds the same
  checksums, and a unit test fails when a listed file changes.
- **The upgrade is tested:** a small database file of every released version is kept under `internal/store/testdata/` and a test
  upgrades each to the newest.
- **The wyo server is the only writer.** One connection writes; reads use their own. Nobody edits the file by hand; an agent looks
  into a database only through a copy opened read-only.
- **Tables of this plan:** `schema_migrations`; `people` (one row, the owner: login, password hash, whether the password must be
  changed); `sessions` (the SHA-256 of the session's text, the person, created, active until, the browser's name); `settings` (key,
  value, when changed); `secrets` (a name, the encrypted value, its last four characters) — filled first by Plan 3.
- **The secret store** `internal/secretbox`, written once for Plans 3 and 4: AES-256-GCM with the key file, the row's name bound
  into the encryption so that a value copied to another row cannot be read, a random nonce per value. A wyo server without its key
  file refuses to start and says how to restore it; it never makes a new one silently over existing secrets.

## C. The owner's sign-in

- **The installer creates the owner**; the wyo interface never offers to create an account. Login: 3–64 characters. Password: at
  least 10 characters, stored only as an Argon2id hash (premise 4).
- **Signing in** (`/auth/login`): the login and the password; a wrong login and a wrong password give the same answer in the same
  time. After 5 failures in 15 minutes for one login or one address the wyo server answers «слишком много попыток» with the minutes
  left.
- **The session** is 32 random bytes in the cookie `wyo_session` (not readable by scripts, sent only over `https`, not sent with
  another site's requests); the database keeps only its SHA-256.
- **A person who does nothing is signed out** after the setting «Выход при бездействии», 60 minutes by default (5 to 720). Only the
  person's own actions prolong the session: the one API module marks a call a person caused; the pages' own periodic re-reads do not
  count, or a screen left open would never sign out.
- **«Мой профиль»** (`/account`): change the password (asks for the current one; ends every other session), «Выйти» (this
  browser), «Выйти на всех устройствах» behind a window that says what it does.
- **A forgotten password** is set again on the server: `sudo wyo owner reset-password`. The sign-in page says so behind «Забыли
  пароль?»; wyo sends no e-mail and has no other way in.
- **Every write is refused when it comes from another site** (premise 5), and the page's security policy lets the browser load
  nothing from any other address (premise 6) — which is also rule R3 seen from the browser.
- **Who may call what is one table** in `internal/httpapi`, from which the router is built: open to everybody — `/healthz`,
  `/readyz`, `GET /api/v1/meta`, `POST /api/v1/session`; everything else — the signed-in owner. A test walks every path with every
  method.

## D. The interface's shell and first pages

- **Copied from the other repository now** (Plan 1, section E: "copied by the plan that first needs one"), each with its test file,
  re-coloured by the theme's tokens: `ShellHeader`, `ShellUserBlock`, `AppNav` with its group state and `CounterPill`,
  `NotificationBell` with `NotificationPanel`, `AuthShell`, `NewPasswordFields`, `LanguageSwitcher` and `DocumentLanguage` (the
  switch draws nothing while one language exists), the error page.
- **The top bar** (filled with the brand colour): the name wyo at the left; at the right the bell and the owner's login, which opens
  «Мой профиль». **The menu** — at the left on a computer, a strip at the bottom on a phone — is drawn from ONE list,
  `src/lib/nav/nav-items.ts`; this plan's items are «Обзор», «Настройки»; Plans 3 and 4 add theirs to the same list.
- **«Обзор»** (`/`): a strip of steps that says where the seller is — «1 Установка — 2 Ключи маркетплейсов — 3 Заказы — 4 Внешний
  API (по желанию)» — with «Сейчас: …» and one sentence on what to do next; then the state of the installation as cards: the
  version and whether a newer one waits, the certificate and the day it ends, the last copy of the database. Plans 3 and 4 add
  their cards.
- **«Настройки»** (`/settings`): folded sections, each one form with its own «Сохранить» (`SectionForm`): «Обновления» (the switch
  «Обновлять автоматически», the version, the last look, «Проверить сейчас», «Обновить сейчас»), «Резервные копии» (the last
  seven copies with their sizes, «Сделать копию сейчас», the sentence about the key file), «Вход и безопасность» (the idle
  minutes). The sections are drawn from the settings registry (`internal/settings`: one list of settings with their limits, defaults
  and help keys), so a later plan adds a setting in one place.
- **The bell** shows notices from **notice sources** — one Go interface, each package that can need the owner's attention
  implements it: the updater (a newer version waits; an update failed and the earlier version was put back), the certificate (could
  not be obtained or renewed, with what to check), the copies (the last copy is older than two days or failed). Plans 3 and 4 add
  sources, never another bell.
- **Help in the interface, by the constitution's help-text rules:** a two-sentence introduction on every page; a «?» beside every
  field, switch and state, saying what it does and what changes when it is switched; every empty state says what to do next; every
  refusal says why and what to do.
- Every page is a row of the screens table and is measured at 390 × 844.

## E. HTTPS by the program itself

- **The default:** the wyo server listens on ports 443 and 80, obtains a certificate for the installer's domain from Let's Encrypt
  with CertMagic, renews it by itself, and sends every `http://` visit to `https://`. One program, one service — no web server to
  set up beside it.
- **When ports 80 and 443 belong to a web server the seller already runs:** the installer's flag `--behind-proxy` makes the wyo
  server listen on `127.0.0.1:8484` in plain `http`; `docs/behind-a-proxy.md` gives the complete settings for Caddy and for nginx.
- **When a certificate cannot be obtained** the wyo server keeps trying as CertMagic does, and the bell and `wyo doctor` say what to
  check: that the domain's address is this server's, that ports 80 and 443 are open.
- The high-level plan left the choice "CertMagic, or Caddy in front" to a test on a bare server; the test is the pipeline's
  installer job (section L), by the maintainer's answer of 2026-11-17.

## F. Signed releases

- **The key.** The implementing agent makes one Ed25519 key pair with `openssl` on the development machine, stores the private key
  as the repository's secret `RELEASE_SIGNING_KEY` (with `gh secret set`), and leaves the only other copy in
  `~/.config/wyo-release/signing-key.pem` (mode 600) on the development machine. **The maintainer moves that file to where he keeps
  secrets;** the report names the path. The private key is never printed, logged or committed.
- **The public key** is committed (`internal/release/keys/2026-1.pem`) and is inside the installer. The wyo server accepts a release
  signed by any key of that folder, so a key is replaced by a release that is signed with the old key and brings the new one.
- **A release** now holds: `wyo_linux_amd64`, `wyo_linux_arm64`, `install.sh`, `VERSION`, `checksums.txt` (the SHA-256 of each of
  these) and `checksums.txt.sig`. The file names never change, so the address
  `https://github.com/prankyman/wyo/releases/latest/download/<name>` always gives the newest.
- **The one order of checking, written once in Go (`internal/release`) and once in the installer:** the signature of
  `checksums.txt` first; then each downloaded file's checksum against it. A file that fails is deleted and nothing is installed. No
  flag skips the check.

## G. The installer — `install.sh`

Two commands on the seller's server (the README explains each):

```
curl -fsSL -o install.sh https://github.com/prankyman/wyo/releases/latest/download/install.sh
sudo bash install.sh
```

1. **It looks before it asks:** it runs as root; the system uses systemd; the processor is `x86_64` or `aarch64`; OpenSSL is 3 or
   newer; ports 80 and 443 are free (or `--behind-proxy`). Each refusal says what is wrong and what to do.
2. **It asks four things**, each with a sentence of explanation: the domain name (and it shows this server's address and whether the
   domain already points to it); the owner's login; the owner's password, twice, not shown; whether updates install themselves
   («да» by default). The same answers can be given as flags and a password file, for a run without questions.
3. **It downloads** the program for this processor, `VERSION`, `checksums.txt` and its signature; checks them in the order of
   section F.
4. **It sets up:** the system user `wyo`; the folders of section A; `wyo init`; the units `wyo.service` (the wyo server as the user
   `wyo`, allowed ports 80 and 443, restarted when it stops), `wyo-update.service` with `wyo-update.timer` (once a day at a random
   minute) and `wyo-update.path` (section H); `wyo owner create`.
5. **It starts the wyo server, waits until `https://<domain>/healthz` answers,** and prints the address to open, the login, and the
   three things to remember: the forgotten-password command, where the copies are, and that the key file is worth copying away.
6. **Run again on a server that has wyo, it repairs and never destroys:** it replaces the program and the units, keeps the
   database, the key file, the certificate and the owner, and asks nothing it already knows. `--uninstall` removes the program and
   the units and leaves `/var/lib/wyo`; removing the data too needs `--purge` and the domain typed as confirmation.

Its sentences are in one table at the top of the script. It sends nothing anywhere but its downloads.

## H. Updates

- **The updater runs as root in its own unit, apart from the wyo server**, so the wyo server — which faces the internet — cannot
  rewrite its own program. The timer starts it once a day; the owner's «Обновить сейчас» makes the wyo server write the file
  `/var/lib/wyo/update-request`, which `wyo-update.path` answers by starting the same unit.
- **What it does:** downloads `checksums.txt` and its signature and checks them; reads `VERSION`; stops when the version is not
  newer (it never goes back to an older one); with «Обновлять автоматически» off it only records that a version waits. Otherwise:
  downloads the program, checks it, stops the wyo server, copies the database into `backups/`, puts the new program in place
  keeping the old as `wyo.previous`, starts the wyo server and waits for `/readyz`.
- **When the new version does not come up within 90 seconds, the earlier one is put back** — the program and the database copy made
  a moment before — and started, and the bell says that the update to that version failed. The next daily run does not try the same
  version again; «Обновить сейчас» does.
- **The look for a newer version sends nothing about the installation** (rule R3): it is a plain download of two files from the
  release address, naming neither the domain nor the installed version.
- **Every address the wyo server or the updater may call is listed in one place,** `internal/outbound`, by purpose: the release
  address and the certificate authority in this plan; the marketplaces in Plan 3; the fulfilment centre's address in Plan 4, only
  while «External API» is filled. No other package may make an HTTP client (the linter forbids it), and a test proves that a call
  to an address outside the list is refused before it leaves.

## I. The daily copy of the database

- Once a day, at a random minute of the night, the wyo server writes `backups/wyo-<date>.db` with `VACUUM INTO` and deletes all but
  the newest seven. `wyo backup` and «Сделать копию сейчас» do the same at once.
- **The key file is not in the copy.** The page and the README say it plainly: without `secret.key` the stored marketplace keys
  cannot be read from a restored copy and are typed again; copy the key file away once.
- `wyo restore <file>` puts a copy back while the wyo server is stopped; `docs/backup-and-moving.md` walks through moving wyo to
  another server.
- **The scheduler** that runs this daily job (`internal/schedule`: a job, its period, a random shift, an injected clock) is written
  once here; the synchronisation engine of Plan 3 is built on it.

## J. Measured, not assumed

The high-level plan promises a server of 1 processor core and 1 GB of memory and says the real use is measured in Stage 1. The
installer job (section L) records the wyo server's memory after its start and after 1 000 page and API requests, and the size of
the program file; the numbers go into the README's requirements with the date. Plan 3 measures again with the synchronisation
running.

## K. The instructions

- **`README.md`, in Russian, written for a seller who has never administered a server**, by the constitution's help-text rules (every
  term explained where it first appears, every step with what the person will see):
    1. Что такое wyo и чего он никогда не делает. 2. Что понадобится: a server (the measured requirements; the tested systems), a
       domain name, 15 minutes. 3. Как направить домен на сервер — what an A record is, with an example. 4. Как подключиться к
       серверу — the `ssh` command with an example. 5. Установка — the two commands, each of the four questions with a screen-like
       example of the dialogue, what the last lines mean. 6. Первый вход. 7. Обновления — by themselves; by the button; how to
       switch them off; how to see the version. 8. Резервные копии и ключевой файл. 9. Если забыли пароль. 10. Если что-то не
       работает — `sudo wyo doctor` and the frequent cases with their remedies (the domain does not point here; a port is taken;
       the certificate is not issued; the service does not start, and how to read its log). 11. Удаление.
- **Pages of `docs/` linked from it:** `behind-a-proxy.md`, `backup-and-moving.md`, `release-signature.md` (how to check a
  release by hand), `development.md` (Plan 1).
- **The interface repeats the guidance where it is needed:** the strip of steps on «Обзор», the «?» of every setting.
- **`readme-policy.test.ts`** fails when the README names a `wyo` command that the program does not have, an installer flag the
  script does not read, a menu item or a button that is not in `messages/ru.json`, or the install address in another spelling than
  the installer's own.

## L. The pipeline's new jobs

- **The release job** (Plan 1, section L) now also publishes `install.sh` and `VERSION`, signs `checksums.txt` with the repository's
  secret, and checks its own signature with the committed public key before publishing.
- **The installer job**, on `ubuntu-24.04`, `ubuntu-22.04` and `ubuntu-24.04-arm` — clean machines with systemd: it starts Pebble
  and a local server of release files made from the commit under test and signed with a key made for the run. The job makes a
  test copy of the release: the wyo server compiled with the tag `wyotest`, and `install.sh` with its two constants — the release
  address and the public key — replaced by the run's own. **The released installer and the released wyo server have no setting
  that changes either.** Then the job runs that installer without questions and proves: the service runs as the user `wyo`; the
  files have the modes of section A; `https://` answers with the test authority's certificate; the owner signs in; a second run of
  the installer changes nothing; a newer release is installed by the updater and the data is still there; a release that does not
  come up is put back; a release with a changed file is refused; `--uninstall` leaves the data. It records the numbers of section J.
- `shellcheck` over `install.sh` joins `pnpm lint`.

## M. Data, code, tests

**Data:** one migration, `0001_people_sessions_settings_secrets` (section B).

**Code — where things live:** `internal/store` (migrations, generated queries), `internal/secretbox`, `internal/auth` (passwords,
sessions, the sign-in limit), `internal/httpapi` (the access table, the handlers; `api/wyo.yaml` gains this plan's calls),
`internal/settings`, `internal/notices`, `internal/schedule`, `internal/backup`, `internal/release` (the checking order, the
updater), `internal/outbound`, `internal/tlsauto` (CertMagic), `internal/cli` (the commands and their texts function),
`scripts/install.sh`; in the interface `src/routes/auth/login`, `src/routes/(app)/` with its layout and the pages of section D,
`src/lib/nav/`, `src/lib/components/shell/`, `src/lib/components/auth/`.

**Components created:** `InstallationStateCard` (one card of «Обзор»: a title, a state with its tone, a sentence, an action) and
`SettingsSection` (one registry group drawn as a `SectionForm`). **Components reused:** everything of the library — `PageHeading`,
`Stack`, `StepStrip`, `SectionForm`, `Field`, `TextBox`, `NumberField`, `Button`, `NoteBox`, `EmptyState`, `ModalWindow` with
`WindowButtons`, `StatusBadge` — and the shell parts copied in section D.

**Tests** (written before the code they check, failing first):

- **Go:** the migrations (a new database; a second start changes nothing; a changed applied migration is refused; a failing migration
  leaves nothing behind) and `migrations.sum`; the secret store (a value comes back; a value moved to another row does not; a missing
  key file stops the start); passwords and sign-in (right, wrong, unknown login in the same shape and time, the limit, the cookie's
  three properties); the idle sign-out with the injected clock (a periodic re-read does not prolong); sign-out here and everywhere;
  the change of password; a write from another site refused; the access table walked path by path; the settings registry; each
  notice source; the scheduler with its random shift; the copies (written, seven kept, put back); the checking order of a release
  (a right signature, a wrong one, a changed file, an older version, an unknown key); the updater against a made-up release server
  and a made-up service manager (the new version comes up; it does not and the earlier one is put back; automatic updates off);
  the outbound gate; each command.
- **The canary test of rule R4** starts here: a distinctive secret is stored, the wyo server is driven through this plan's calls,
  and the secret must appear in no log line and no answer. Plans 3 and 4 widen it.
- **Interface unit tests:** each new component once in its own file, in every state; each page tested only for what it adds; the
  menu drawn from the one list; the policy tests green; `readme-policy`.
- **Browser tests** (against the built wyo server in test mode, each worker with its own data folder): signing in and out; a wrong
  password and the limit's message; the idle sign-out; the change of password; saving a setting; a notice appearing behind the bell;
  no request to any other address; every page of the screens table swept at 390 × 844 and the wider widths.
- **The installer job** of section L.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration, the store, the secret store. 3. Passwords, sessions, the access table, the sign-in
   calls. 4. The shell parts copied; the sign-in page; «Мой профиль». 5. The settings registry and «Настройки»; notices and the
   bell; «Обзор». 6. The scheduler and the daily copy. 7. The outbound gate; HTTPS. 8. The release key, the checking order, the
   updater. 9. The commands; the installer. 10. The pipeline's jobs. 11. The README and the `docs/` pages, the constitution.
   12. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server in development mode): sign in; a wrong password; the
  three pages and the bell at a computer's width and at 390 × 844 with screenshots; change the password and sign in again; make a
  copy; the console and the outside-request check after every screen.
- **Gate:** the test flow of `AGENTS.md`; the check of rule R1; squash; push; the pull request; the installer job green on the three
  machines; the merge by the agent; the release run watched to its end; then **the released installer and the released wyo
  server, unchanged, are run once in a pipeline job against the real release address** with `--behind-proxy` (a clean machine of
  GitHub has no domain for a real certificate): the download, the signature, the checksums, the units, the owner's sign-in. That
  path exists only after the release. `main` pulled; the report with the numbers of section J.

## After the release — the maintainer's checks on a real server (register)

A real server of 1 core and 1 GB with Debian 12 or 13 (the pipeline has only Ubuntu) and a real domain: the installer from the
README, a real Let's Encrypt certificate, the memory after a day, an update from this release to the next one, a copy restored on a
second server. What they find becomes the next plan's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **More people who sign in** — the maintainer, 2026-11-17: «What more people? … Is this a sought-after functionality?» Until
  sellers ask.
- **E-mail from wyo** (a forgotten password by mail, notices by mail): a seller's server has no mail set up; notices live behind
  the bell.
- **A server without a domain name** (a certificate for a bare address): the maintainer's request names a domain.
- **A second sign-in step** (a code from a phone), **a list of allowed addresses:** later, if asked.
- **Systems without systemd or without OpenSSL 3, a Docker image, Windows:** the installer refuses them and says why.
- **Postgres** (high-level plan: "not offered in the first stages"). **An offline page and installing the interface as an
  application on a phone:** not asked for.
- **Marketplaces and «External API»** — Plans 3 and 4.

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no password, session text, key-file byte or private key in a log, an answer, an
   error, a commit or the pipeline's output.
5. Everything is a component and is reused: search before writing; the shell parts are copied, not rewritten; the secret store, the
   scheduler, the settings registry, the notice sources and the outbound gate are written once here and used by Plans 3 and 4. Every
   subagent prompt repeats this rule.
6. Every visible text — the interface's, the commands' — is a key of `messages/ru.json`, written by the help-text rules; no
   sentence in Go; the installer's sentences in its one table.
7. No test, no development run and no pipeline run calls a real marketplace, a real fulfilment centre or the real Let's Encrypt.
8. Rules R1–R7 of the high-level plan bind every file; the check of R1 before every push.
9. Name the program every time: the wyo server, the wyo interface, the installer, the updater.
10. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage;
    the final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

Four tables, about fifteen calls of the wyo server's own API, four pages, seven commands, one installer of about 400 lines, three
system units and two pipeline jobs. The parts with the most unknowns are the installer job (systemd, ports and a test certificate
authority on GitHub's machines) and the updater's putting-back; both are proven in the pipeline before the merge.
