# The marketplace connector: a free, open-source web program a marketplace seller installs on its own server, which shows the seller's orders, stock and sales across marketplaces with the seller's own keys and, only when its setting «External API» is filled, exchanges orders, labels, statuses, marking codes and supplies with an external fulfilment centre through a neutral fulfilment-centre API — the high-level plan

> A high-level plan, without a plan number. Written 2026-11-17; the first plan of this repository (`https://github.com/prankyman/wyo`).
> The research it cites by short names — «MC» (similar software, marketplace terms, stack), «FA» (fulfilment systems' public APIs),
> «WBA» (Wildberries tokens), «WBF» (Wildberries FBS orders, supplies, stickers, codes), «WBP» (Wildberries products, stock, returns) —
> is kept by the maintainer outside this repository; the marketplaces' own pages it relies on are in `docs/marketplaces/`.
>
> **This is not an implementation plan.** Each stage below becomes a detailed plan of this repository, written here, with its own
> research check, before any code of that stage.

**Every AI agent that writes a stage's plan or implements it follows the `AGENTS.md` principles — global (`~/.agents/AGENTS.md`) and
this repository's own (Stage 0 creates it) — and uses this repository's testing skills for all test work.** Only the main agent runs
whole-codebase checks and full test suites; a subagent runs only its own test files; at most 3 subagents at once; every subagent prompt
repeats these rules and the rule "everything is a component and is reused".

## Terms used below

- **The marketplace connector** (short: the connector) — the program this plan describes. Its product name is **wyo**.
- **The seller** — the company that sells on marketplaces and installs the connector. **The owner** — the person who signs in to it.
- **The maintainer** — the person who runs this project and decides its questions.
- **A marketplace key** — the seller's own key or token of a marketplace, typed by the owner into the connector.
- **A fulfilment centre** — an outside warehouse company that stores, packs and hands over the seller's goods.
- **The fulfilment-centre API** — the neutral data format the connector speaks to a fulfilment centre's system: orders in, labels in,
  statuses and marking codes back, hand-over documents, stock, supplies.
- **«External API»** — the connector's setting holding a fulfilment centre's API address and key. Empty by default.
- **FBS** — the seller-warehouse scheme (the order is packed outside the marketplace and handed to it). **FBO** — the
  marketplace-warehouse scheme (the goods lie in the marketplace's warehouse).

## The maintainer's request and answers (2026-11-16 and 2026-11-17)

- "Let's call the 'intermediary system' … a 'marketplace connector'."
- "It must be some free OpenSource web software written in probably GO and SvelteKit, that could be easily installed on any linux OS
  (some VPS|VDS or similar) with some script that just installs the whole thing, and only, probably asks the domain name and couple other
  settings (such as, I guess, the username|password of the owner). The website (software) itself upon the first log-in just asks for the
  marketplaces (WB, OZON, YM, Megamarket, Avito (don't know if Avito is needed, you advise), Lemana Pro (not sure, either)) seller API
  keys and then just works. It periodically, like once in half hour pulls the data for the software itself (or can do it on demand by
  button click … but allow it not too often), and then it serves as a channel between the marketplaces … and [a fulfilment centre's
  system]."
- "The connector must not communicate with [a fulfilment centre's system] unless another setting is selected inside it - 'external
  API', which the user of the connector must be instructed to point to [that system's] API with the key created inside [it]."
- The connector names no fulfilment product, "but must say in its description what it does, and mention, among other things, that it
  can work with external fulfillment centers." Its documentation "must have that neutral 'fulfillment center API'".
- The feature list of an earlier conversation is what matters; its technical and legal claims are not taken for granted ("take only
  the gist … think for yourself, and research the available similar software").
- Licence: "Apache-2.0 for now". First version: as recommended (section «Stages»). "Don't forget the step where a separate repo must be
  created and set up for development. The agents rules and skills must come (or at least be based on) from [the maintainer's other]
  repo."
- The name: «wyo - is the name». No domain: "I am not planning to have a website". The trade mark: "Let's forget about the trademark,
  so far". No marketplace keys yet: "Right now you'll have to do all work pretty much blindly".

## The rules that bind every stage

- **R1. No fulfilment product is named.** No file, screen, text, example, test, commit message or package name of the connector
  names a fulfilment product, its website, its domains or its owner. The connector's description says what it does and that it "can
  work with external fulfilment centres". The list of words searched for before every push is kept by the maintainer outside this
  repository.
- **R2. The fulfilment-centre API is neutral.** The connector's documents describe the format in their own words, as a specification
  any fulfilment centre's system may implement. «External API» takes any address and any key.
- **R3. Alone by default.** With «External API» empty the connector talks only to the marketplaces. It never sends anything anywhere
  else — no usage statistics, no update check that carries data about the installation.
- **R4. The seller installs it and types the keys.** The connector runs on a server the seller owns or rents, and only the seller's
  own people sign in. A marketplace key never leaves that server: it is not sent to a fulfilment centre and not written to a log. The
  documents tell a fulfilment centre's staff never to install the connector for a seller or hold a seller's key — Wildberries' rule
  reads «Персональный токен … нельзя передавать третьим лицам или использовать в облачных сервисах» and allows the Personal token for
  the seller's own programs «развёрнутым на собственной или арендованной инфраструктуре» (WBA §1.1, §1.5 and its addendum; MC §4.1).
- **R5. Nothing is hidden from a marketplace.** The connector names itself and its version in every request, keeps inside each
  marketplace's documented request limits, and waits as the marketplace's answer tells it. No request disguising, no proxies, no
  rotation of addresses. Polling moments are spread at random only so that many installations do not ask in the same second.
- **R6. One contract per side.** One marketplace-adapter contract that every marketplace implements, one synchronisation engine, one
  fulfilment-centre client; nothing marketplace-specific outside its adapter; every piece of interface used twice is one component.
- **R7. Free and open:** Apache-2.0, no paid edition, no licence key.

## What the research found (2026-11-17)

- **Nothing like it can be installed today.** The nearest, BormoStats (GPL-3.0, Python, six services under Docker Compose), shows
  profit, money and advertising for three marketplaces and has no joint order list, no hand-off to a warehouse and no simple installer
  (MC §1). The cloud products it would stand beside cost a small seller from about 2 200 to 13 000 ₽ a month (MC §2).
- **No fulfilment system offers a neutral intake.** МПФИТ and the others pull from the marketplace with the seller's key themselves;
  the general ones (ShipBob, Linnworks, Shopify) treat a marketplace as a tag on an ordinary order (FA §3).
- **The marketplaces change their interfaces about every 2–3 days:** 88 official change notices from Wildberries, 74 from Ozon and 23
  from Яндекс Маркет in six months (MC §8). A program that is not updated breaks.
- **Keys expire:** a Wildberries token after 180 days, an Ozon key after 3 months since 2026-09-03; a Яндекс Маркет key does not (MC §4).
- **Ready clients in Go exist** for Wildberries (generated nightly from the official description files) and Ozon; Яндекс Маркет
  publishes its own description file; none was found for Мегамаркет and Авито (MC §5).
- **The marketplaces' own terms allow the connector** (read 2026-11-17 from the pages saved in `docs/marketplaces/`):
  - **Wildberries** — «Облачный сервис или локальное решение: как отличить и какой токен выбрать» (updated 16.09.2026) names «Программа
    на арендованном VPS/VDS — вы арендуете сервер и сами устанавливаете ПО» a local version that takes the **Personal token**. «Как
    подключиться по API через токен» (updated 28.05.2026) forbids only passing that token to third parties or using it in cloud
    services, which R4 already excludes. Its **test token** works against the WB API sandbox with generated data — the first safe check
    once the maintainer has a Wildberries account. The platform agreement concerns services of WB's catalogue, which the connector is
    not; the saved copy holds only its contents.
  - **Ozon** — the seller contract («Договор для продавцов», 111 pages) has no clause against the seller's own program using Seller API;
    it names Seller API as an ordinary way of sending data.
  - **Яндекс Маркет** — the seller contract (in force from 2026-10-04, 63 pages) has no such clause; 6.42 makes the seller answer for
    every action under its own access. «Передача информации третьим лицам запрещена» is on the page for IT partners, companies listed
    in Yandex's catalogue; the connector does not apply to be one. That page also asks integrations to identify themselves — R5 does.

## What it is built of

- **One program.** A Go server with the interface built by SvelteKit as static files and embedded in the binary (`adapter-static`,
  Go `embed` with the `all:` prefix for the `_app` folder — MC §6). One file to install, one service to run.
- **One database file** (SQLite) beside the program, with a daily dump kept for 7 days. Postgres is not offered in the first stages.
- **The marketplace-adapter contract:** check a key and say when it expires; read orders since a mark; read an order's state; confirm
  an order for packing and fetch its label; report marking codes and read the marketplace's verdict; group orders for hand-over, close
  the group and fetch its document; push stock; read stock in the marketplace's warehouses, sales, money and advertising rows; read
  product cards; create and read supplies to the marketplace's warehouses. Each adapter says which of these it supports; the screens
  ask the adapter, never the marketplace's name. Clients are generated from the marketplaces' description files where they exist, so a
  change arrives as a difference in a file.
- **The synchronisation engine:** a scheduler with one request budget per marketplace and seller account read from the marketplace's
  own limit headers, marks of what was read, retries with growing pauses, and a journal of every request (method, path, status,
  duration — never a key or a body).
- **The fulfilment-centre client:** speaks the neutral API of R2 — it sends orders, labels, hand-over documents and supply requests,
  and receives events either as signed notifications at the connector's own address or by asking for them every minute. It works only
  while «External API» is filled and its check passes.
- **Keys at rest:** every marketplace key and the fulfilment centre's key encrypted with a key file only the service's user can read;
  the screens show the last four characters and the expiry date.
- **Sign-in:** one owner created by the installer, more people added by the owner; passwords hashed; sessions in the database.
- **HTTPS by itself:** the program obtains and renews its certificate for the domain the installer was given (the CertMagic library, or
  Caddy in front — chosen in Stage 1 by a test on a bare server).
- **The installer:** one script that downloads the signed release, checks its signature and checksum, asks for the domain and the
  owner's login and password, creates the service and starts it. **Updates:** the program checks for a new signed release once a day
  and installs it by itself (the owner can turn this off and update by a button); the check sends nothing about the installation.
- **Size:** a server of 1 processor core and 1 GB of memory is the target (about 1 920 ₽ a month at the cheapest provider read, MC §6);
  the real use is measured in Stage 1, not assumed.

## The marketplaces, in order

1. **Wildberries, Ozon, Яндекс Маркет** — the three with the most sellers and with usable description files.
2. **Мегамаркет** — after its documents are read for the FBS steps (`docs/marketplaces/` holds its API descriptions).
3. **Авито and Лемана ПРО** — last, and only on demand: no ready client exists, and Лемана ПРО issues keys only after an application.
   The advice given to the maintainer is to leave both out of the first stages.

## What the seller sees — the features, by stage

| Stage                        | What it adds                                                                                                                                                                                                                                                                                 |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0                            | The repository and its development set-up (below).                                                                                                                                                                                                                                           |
| 1 — the first usable version | Installation; the owner's sign-in; the keys of Wildberries with their health and expiry dates; one order list (FBS orders with their state; FBO orders read-only); «External API» and the whole Wildberries FBS exchange with a fulfilment centre (below); the journal of requests; updates. |
| 2                            | Ozon and Яндекс Маркет: keys, the order list, the same exchange with a fulfilment centre. Stock: what lies in each marketplace's warehouses, and the fulfilment centre's free stock sent to every marketplace that sells from it, so a sale on one lowers what the others offer.             |
| 3                            | Sales and money: sales and returns, the marketplaces' commissions, logistics and advertising spend read in; the cost price typed per product; profit per product and per marketplace; the fulfilment centre's charges where its API gives them.                                              |
| 4                            | Planning: how many days each product's stock will last at the present speed of sales, what to buy and when, a purchase list as a file.                                                                                                                                                       |
| 5                            | Supplies to marketplace warehouses through the fulfilment centre: a supply request sent, the packed boxes' contents received, the marketplace's box file and labels produced.                                                                                                                |
| 6                            | Мегамаркет; Авито and Лемана ПРО on demand.                                                                                                                                                                                                                                                  |

Left out on purpose (other products do them, each adds a risky write to a marketplace): automatic price changes, advertising bid
management, automatic answers to reviews, competitor tracking (MC §9).

## The exchange with a fulfilment centre — Wildberries FBS, step by step

Each step names the connector's call to the fulfilment-centre API and to Wildberries. The Wildberries facts are in WBF.

1. **A new order.** The connector reads Wildberries' new assembly orders every 1–2 minutes and sends each as an order: its number, the
   buyer's basket number, the fulfilment warehouse that stands for the seller's Wildberries warehouse (a setting), one line with the
   barcode, the article and Wildberries' size number, the quantity 1, and the hand-over deadline — the order's creation time plus the
   seller's setting, 48 hours by default, the limit after which Wildberries fines (WBF §3.4). No buyer data is sent.
2. **The sticker.** When the fulfilment centre reports the goods reserved, the connector puts the order into its open Wildberries
   supply — which is what makes Wildberries issue the sticker — fetches the sticker at 58 × 40 mm and attaches it to the order as its
   label. It waits for the reservation because putting an order into a supply ends the buyer's free hour of cancellation and a later
   cancel by the seller is fined (WBF §3.3). An order whose goods are not there is shown to the owner to decide.
3. **The marking codes.** When the fulfilment centre reports packing finished, the connector sends each unit's code to Wildberries and
   reads Wildberries' verdict until it is final. A refusal becomes a problem on the order at the fulfilment centre, in Wildberries' own
   words; it is cleared when corrected codes pass. If the fulfilment centre reports the parcel reopened, the connector removes the
   codes and waits for the next "packing finished".
4. **The hand-over.** When the fulfilment centre asks for the hand-over's documents, the connector makes a new Wildberries supply named
   after the hand-over, moves exactly its orders into it, adds transport boxes when asked, sets the shipping point, type and date from
   the seller's settings (Wildberries requires them, WBF §4.2 step 7), closes the supply, fetches its QR code and the boxes' stickers and
   sends them as the hand-over's documents. One supply per cargo type, as Wildberries requires. What Wildberries refuses becomes a
   problem on the hand-over.
5. **Cancellations.** The connector asks Wildberries for the state of every open order every few minutes and passes a cancellation on.
   A cancellation by the warehouse is shown to the owner; the connector never cancels in Wildberries by itself (it costs a fine;
   МПФИТ does not either, FA §3).
6. **Stock** (Stage 2). The fulfilment centre's free stock goes to the seller's Wildberries warehouses when it changes.

Ozon and Яндекс Маркет follow the same six steps through their adapters; there the label exists only after packing is reported, which
the neutral API's separate label call already covers.

To check on the first real account (Stage 1's premises): whether a sticker stays valid when its order is moved between two open
supplies (WBF §1.5 states a new sticker only for a move back from delivery); which text the sticker's code gives to a scanner (WBF §6.1).

## How often it asks

- **For its own screens:** every 30 minutes — orders, sales, stock in the marketplaces' warehouses; money rows once a day; «Обновить»
  by a button, answered at most once in 5 minutes.
- **While «External API» is filled:** Wildberries' new FBS orders every 1–2 minutes and open orders' states every 5 minutes, inside its
  budget of 300 requests a minute (an answer with an error counts as 10 — WBF §8.1); Ozon every 5 minutes; Яндекс Маркет by its own
  notifications where the connector's address is public, every 5 minutes otherwise (MC §9).
- A few seconds of random shift on every timer (R5).

## Stage 0 — the repository and its development set-up

1. **Decided (2026-11-17):** the name **wyo**; the repository `https://github.com/prankyman/wyo`. No domain: the connector has no
   website of its own — each seller opens it at the address of its own server, and development runs on `localhost`. The trade-mark
   check is set aside; a web search on 2026-11-17 found no software product named «WYO».
2. **The premises — done 2026-11-17** (research section). The marketplaces' saved pages are committed in `docs/marketplaces/`, to be
   read again whenever a rule is in doubt.
3. **The repository** is set up with: `LICENSE` (Apache-2.0); a `README` saying what the program does and that it can work with
   external fulfilment centres; `AGENTS.md` based on the maintainer's existing rules for AI agents (the maintainer points to them) — the
   register of requests, the rule to name the program, the rule about reading long files, one plan = one branch = one commit, tests
   first and never rigged, everything is a component, heavy checks only by the main agent — rewritten for a Go server with a SvelteKit
   interface and stripped of everything R1 forbids; `.agents/skills/` with a testing skill for the interface, a smoke-testing skill
   (both based on the maintainer's existing skills) and a new skill for Go tests; `.agents/plans/`, `.agents/research/`,
   `.agents/backlog.md`; a short constitution (R2–R7, the adapter contract, where code lives, the test rules).
4. **The tools:** Go and Node versions pinned; linting and formatting for both; unit tests for both; a build that embeds the
   interface; fake marketplaces and a fake fulfilment centre for tests (no test ever calls a real marketplace); continuous integration
   that runs all of it and, on a tag, builds and signs the release.
5. **The neutral specification** `docs/fulfilment-centre-api.md` and its description file: written in the connector's own words from
   the format the maintainer provides, naming no product. The fake fulfilment centre implements it, and the connector is tested only
   against the fake.
6. **The check of R1 before every push:** a search of the whole tree and of the commit messages for the maintainer's list of words,
   run by the agent from outside this repository.

## Stages 1–6

Each is one plan of this repository, written when the stage before it is released (terms, the request and answers, today, premises,
lettered sections, data, code, tests, stages and gate, not in the plan, rules for agents). Stage 1 needs the fulfilment-centre API's
order, label, status and hand-over parts; Stage 2's stock and Stage 5's supplies need its stock and supply parts. Each is developed and
tested against the fake fulfilment centre.

**The detailed plans written on 2026-11-17, and what the maintainer decided that day:**

- Stage 0 is **Plan 1**,
  `.agents/plans/wyo-repository-set-up-agent-rules-skills-constitution-tools-component-library-theme-fakes-fulfilment-centre-api.md`.
- Stage 1 is **three plans** (the maintainer: «3»), each releasable alone: **Plan 2**, the program itself,
  `.agents/plans/wyo-program-go-server-embedded-interface-database-owner-sign-in-installer-https-updates-signed-release.md`;
  **Plan 3**, Wildberries read-only, `.agents/plans/wyo-wildberries-keys-adapter-synchronisation-engine-order-list-request-journal.md`;
  **Plan 4**, «External API» and the exchange,
  `.agents/plans/wyo-external-api-wildberries-fbs-exchange-with-a-fulfilment-centre-orders-stickers-codes-hand-over.md`.
- The interface is in Russian only, with the machinery for more languages kept («Russian only. But do create the languages
  capability.»).
- One owner signs in; "more people added by the owner" of «What it is built of» waits in the register («What more people? … Is this
  a sought-after functionality?»).
- The whole set of checks runs in GitHub Actions, the installer's and the certificate's tests included («If the GH Actions passes
  fine for you - not a problem!»); the choice between CertMagic and Caddy in front is made there, not on a bare server.
- The signature of a release is made by Plan 2, together with the installer and the updater that check it, not by Stage 0.

## Not in this plan

- A cloud edition hosted for many sellers: Wildberries' Personal token may not be used in a cloud service (R4), so it would need
  registration with the marketplaces — a different product.
- A desktop edition for a seller's own computer: the fulfilment exchange needs a program that is always on.
- Packing orders without a fulfilment centre (printing stickers and confirming from the connector itself): later, if sellers ask.
- Anything of R5's list, in any stage.

## Developed without marketplace keys

The maintainer holds no Wildberries, Ozon or Яндекс Маркет key yet. Every stage is built and tested only against the fake marketplaces
and the fake fulfilment centre of Stage 0, step 4, made from the marketplaces' official description files and documents. The
maintainer tests each release with real keys once they are available; what that test finds becomes the next plan's premises.

## Rules for every AI agent working on the connector

1. Follow `~/.agents/AGENTS.md` and this repository's `AGENTS.md`; read its constitution and register before the first edit.
2. R1–R7 bind every file. Before every push run the check of Stage 0, step 6.
3. Use the repository's testing skills; tests first, failing first; never change what a test checks to make it pass; no test calls a
   real marketplace or a real fulfilment centre.
4. Everything is a component and is reused: one adapter contract, one engine, one client, one component per repeated piece of
   interface; every subagent prompt repeats this rule.
5. Only the main agent runs whole-codebase checks; at most 3 subagents at once.
6. One plan = one branch = one commit = one pull request; never `!` or `BREAKING CHANGE:` without the maintainer's yes.
7. A fact about a marketplace is written only from its official documents, with the address and the date read; a gap is marked "not
   sourced" and goes to the first real account's checks.
8. Every report starts with Done / Not Done.
