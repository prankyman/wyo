# Stock, part 3 of 3: wyo's own count for a seller who works without a fulfilment centre — «Мои склады» (one made at the update, more added by the owner), each seller's warehouse at a marketplace linked to one of them, wyo's own products made from the marketplaces' catalogues by the one matching rule, the first count typed, loaded from a file or taken from a marketplace, lowered by orders and raised by cancellations with a record of every change, and written to every linked warehouse the owner set to «Передавать свой счёт wyo», so a sale on one marketplace lowers what the others offer

> **Plan 11** of the marketplace connector (product name **wyo**), repository `https://github.com/prankyman/wyo` (`~/projects/wyo`; paths
> without a prefix are from its root). Part 3 of the three stock plans of **Stage 2** of the high-level plan
> `.agents/plans/Marketplace_connector_plan.md`. Written 2026-11-21 from `origin/main` at `eed4607`. **It starts only after part 2**
> (Plan 10,
> `.agents/plans/wyo-stock-part-2-of-3-the-choice-per-warehouse-and-sending-the-fulfilment-centres-free-stock-to-the-marketplaces.md`)
> **is merged and released**, which itself follows part 1 (Plan 9,
> `.agents/plans/wyo-stock-part-1-of-3-products-and-stock-read-from-the-marketplaces-and-the-fulfilment-centre-stock-page-wildberries-product-names.md`);
> its section «Today» is written against what parts 1 and 2 build and is checked again on that day.
> Research: `.agents/research/marketplaces-stock-official-documents-check-2026-11-21.md` (cited «STK»: part W Wildberries, part O Ozon,
> part Y Яндекс Маркет).
>
> **Branch** `feat/stock-own-count-without-a-fulfilment-centre`. One plan = one branch = one squashed commit = one pull request. The
> commit is `feat:` — never `feat!:` and never a `BREAKING CHANGE:` footer: the own count writes nothing until the owner chooses
> «Передавать свой счёт wyo» for a warehouse. **One database migration**, the next free number at the start. Constitution: principle
> XIII «Stock» widened, **MINOR**, the next free number at merge time.

**Every AI agent implementing this plan follows the `AGENTS.md` principles, global (`~/.agents/AGENTS.md`) and project (`AGENTS.md`),
reads `.specify/memory/constitution.md` and `.agents/backlog.md` first, and uses the repository's testing skills for all test work:
`go-testing` (`.agents/skills/go-testing/SKILL.md`) for Go, `sveltekit-testing` (`.agents/skills/sveltekit-testing/SKILL.md`) for the
interface, `smoke-testing` (`.agents/skills/smoke-testing/SKILL.md`) for the smoke test.** Only the main agent runs `pnpm precommit`,
whole-codebase checks and full test suites; a subagent runs only its own test files and lints only its own changed files; at most
3 subagents at once; every subagent prompt repeats these two rules and the rule "everything is a component and is reused". **An issue
found on the way is fixed in this plan at once** (constitution, «Plan Implementation Workflow», step 2), and the report lists it under
«Found while implementing».

## Terms used below

- **The wyo server**, **the wyo interface**, **the owner**, **an account**, **an adapter**, **the budget**, **the journal** — as in
  Plans 2 and 3. **A seller's warehouse at a marketplace**, **a marketplace product**, **the catalogue**, **a match** — as in part 1.
  **The choice**, **the writer**, **the warning** — as in part 2.
- **An own warehouse** («Мой склад») — a place the seller keeps goods in and ships from itself, as wyo knows it: a name and a count of
  each product. It is wyo's record only; no marketplace sees it.
- **A link** — which own warehouse a seller's warehouse at a marketplace ships from. Several marketplace warehouses may link to one own
  warehouse; that is how a sale on one marketplace lowers what the others offer.
- **An own product** — one product in wyo's own count: an article, a name, a size and barcodes, with the marketplace products it
  covers (its **members**): for example a Wildberries size, an Ozon product and a Яндекс Маркет offer with the same barcode.
- **The own count** — how many units of an own product an own warehouse holds that orders may still take: the same meaning as the
  fulfilment centre's free number.
- **A movement** — one change of the own count, kept for ever with its cause.

## The maintainer's request and answers (2026-11-21) — nothing is open

- **Stand-alone:** «if not - then it is wyo that takes care of it. Makes sense? YOu did not forget that this software may not have the FO
  plugged in and function purely stand-alone so the business logic must take it into consideration.» — and the second mode: «WYO reads
  and writes the products because it itself takes care of product counts, not the WMS, which is not integrated with WYO or marketplaces».
- **The value of the choice:** «Передавать свой счёт wyo» for a warehouse linked to one of wyo's own warehouses — «Yes».
- **Own warehouses:** a list «Мои склады» with one made by default, or one fixed warehouse — «A list».
- **The first count:** typed, loaded from a file (article and count), or the button «Взять числа маркетплейса» — «All three».
- **What the own count is lowered and raised by** — orders lower it, cancellations raise it, every change recorded — as proposed with
  the three plans («3 plans») and not objected to.
- **A product the fulfilment centre does not know gets 0** («Writing 0», part 2); the same rule holds here for a product the own count
  does not know: wyo cannot know such goods are there.
- 2026-11-17, still true: no real account — built and tested only against the fakes; the checks on real ones are in «After the release».

## Today (written against parts 1 and 2; checked again on the day the plan starts)

- After part 1: every catalogue and every warehouse's stock read; one matching function `MatchProducts` with the side `own` reserved in
  `product_matches`.
- After part 2: the choice on each seller's warehouse with «Только показывать» and «Передавать числа фулфилмент-центра»; the column of
  `stock_modes` already takes `own`; the one writer (read first, only what differs, the limits, stored outcomes) and the warning.
- **Orders are read every 30 minutes for the owner's own screens** and every minute (new) and 5 minutes (states) only for accounts in
  the exchange (`internal/sync/engine.go`, `ordersPeriod`; the adapters' `OrdersPeriod` and `StatesPeriod`). A count kept by orders
  read every 30 minutes would sell the same goods twice in that time, so section E changes it for a counted warehouse.
- **No file is read anywhere in the wyo interface**; `FilePicker` and `FileFormatButtons` wait in register item 8 to be copied from the
  maintainer's other repository.

## Stage 0 — Premises (before any code; each failure stops the stage and asks the maintainer)

1. `nproc` and `free -g` decide the test flow (`AGENTS.md`, "The test flow of every plan").
2. **Part 2 is released** and its «After the release» checks, if made by then, are read: whether an Ozon order lowers the number by
   itself and what `free_stock` reads after a write replace the cautious choices of part 2's warning.
3. **The file reader is the one the maintainer's other repository uses:** `read-excel-file` at its version there (9.3.10 on the day of
   writing), checked to be under a licence rule R7 allows; `FilePicker` and `FileFormatButtons` copied with their tests
   (`docs/library-layout.md`), never rewritten.

## A. «Мои склады»

1. **The list** is a fourth group on «Остатки», «Мои склады». The migration makes one own warehouse, «Мой склад», for every
   installation, so the owner starts with one and adds more only for a second storeroom.
2. **Add, rename, archive** (`OwnWarehouseForm`, a superform with its zod schema, no reload): a name of 1 to 100 characters, unique
   among the own warehouses. An own warehouse with no movement may be removed; one with movements is archived — it keeps its count and
   movements, takes no link, and every warehouse linked to it falls back to «Только показывать» (part 2's fall-back, with its notice).
3. **The intro:** «Мой склад — это ваше помещение с товаром. wyo ведёт его счёт: заказы маркетплейсов уменьшают его, отмены до отгрузки
   возвращают, вы поправляете его вручную или файлом. Этот счёт wyo сообщает маркетплейсам — для тех складов, где вы это выбрали».

## B. The link

1. **A seller's warehouse at a marketplace is linked to one own warehouse** on the own warehouse's page («Склады маркетплейсов этого
   склада», `WarehouseLinks`) and shows it on its own place on «Остатки». Offered for an FBS or Express warehouse; a DBS or rFBS
   warehouse is not offered, with the reason «wyo не читает заказы этого склада, поэтому не может вести его счёт» (register item 19).
2. **A link only counts, it never writes:** from the moment of the link, that warehouse's orders lower the own count (section E). Writing
   needs the choice «Передавать свой счёт wyo» (section G).
3. **Unlinking** stops the counting of that warehouse's orders from then on; movements made stay; a warehouse in «Передавать свой счёт
   wyo» falls back to «Только показывать» with part 2's notice.
4. A warehouse matched in «Внешний API» may be linked too: the choice decides which number is written, and the link alone only counts.

## C. Own products — made from the catalogues by the one matching function

1. **After each reading of a catalogue (part 1) and at each new link,** every marketplace product of the accounts whose warehouses are
   linked is looked up among the own products by part 1's `MatchProducts` with the side `own`: by barcode, then by a marketplace number
   one of the own product's members already has, then by article — each only one-to-one. Found: it becomes a member. Nothing found: a
   **new own product is made from it** (its article, name, size and barcodes). So a Wildberries size, an Ozon product and a Яндекс Маркет
   offer sharing a barcode are one own product, and one count serves all three.
2. **«Неоднозначно»** — the marketplace product matches two own products: it joins none and is shown with the candidates; a warehouse
   written from the own count gets 0 for it (as part 2's ambiguous product).
3. An own product's barcodes are its members' together; its article, name and size are those of the member it was made from. Its
   members are listed in its row's details.
4. **An own product is never deleted:** one whose members are all gone keeps its count and movements and is marked «нет на
   маркетплейсах».

## D. The count and its movements

1. **One whole number per own product and own warehouse.** It **starts** with its first count (section F); before that the product is
   «не считали», nothing lowers it, and a write gives the marketplace 0 for it.
2. **Below zero is kept as it is** (more was ordered than counted), shown «продано больше, чем было» in red with a notice behind the bell,
   and written as 0 (part 2's rule for a number below 0).
3. **Every change is a movement** (`own_movements`), never changed or deleted (a store test proves no code updates or deletes one):
   when, the own warehouse, the own product, the change, the count after, and the cause — «введено вручную», «загружено из файла
   <имя>», «взято у маркетплейса <аккаунт>, <склад>», «заказ <маркетплейс> № <номер>», «отмена заказа <маркетплейс> № <номер>» — with
   the order and its line, the file's name or the marketplace warehouse it came from. The count and its movement are saved in one
   transaction.

## E. Orders lower the count, cancellations raise it

1. **A new order** (FBS or Express) of a linked warehouse lowers each line's own product by the line's quantity, **once per order line**
   (a movement unique by the order and the line, so an order read twice lowers once) — only for an own product whose count has started,
   only for an order that arrived after the warehouse was linked. **Never counted:** test orders (Plan 7: counted nowhere), FBO and FBY
   orders (the marketplace's own warehouses ship them), orders of a warehouse not linked.
2. **A line whose product is no own product** (a new card not read yet, a product «неоднозначно») is matched again by its own barcodes
   and article when the catalogue is next read; until then the own warehouse shows «Заказы на товары, которых нет в счёте: N», each
   order named.
3. **A cancellation before the order was handed over** — its state was «Новый» or «На сборке» when the cancellation was read — raises the
   count back by exactly what that order lowered. **After the hand-over** («В доставке», «Получен покупателем») nothing is raised: the
   goods are not in the storeroom; the movements show «Отмена после отгрузки: когда товар вернётся, исправьте счёт вручную». Returns are
   Stage 3 (register item 2).
4. **Orders are read soon enough to count by:** an account with a warehouse in «Передавать свой счёт wyo» has its new orders read at
   the exchange's periods (`OrdersPeriod`, `StatesPeriod` of its adapter: every minute and every 5 minutes; Яндекс Маркет at once by
   its notifications), with «Внешний API» empty too, inside each marketplace's budget; otherwise the periods of Plan 3 stay.

## F. The first count — three ways, and any time later

1. **Typed** («Изменить счёт», `OwnCountEditor`): the own warehouse's table turns its count into a `NumberField` per row (a whole number
   from 0); «Сохранить» saves every changed row as a movement «введено вручную». The same is a stock-taking later.
2. **From a file** («Загрузить из файла», `StockImportForm`): CSV or Excel (`.xlsx`), its first row the headings, one column «Артикул» or
   «Штрихкод» and one «Количество»; at most 10 000 rows and 5 MB. The wyo interface reads the file in the browser — Excel with
   `read-excel-file`, CSV with one small function (the separators `;` and `,`, quotes, a leading byte-order mark) — and sends the wyo
   server only the rows. A row finds its own product by barcode, or by article only when exactly one own product has it: two sizes of
   one Wildberries card share an article, and such a row says «У этого артикула несколько товаров — укажите штрихкод».
   `FileFormatButtons` gives a sample file of both kinds.
3. **«Взять числа маркетплейса»** (`TakeFromMarketplaceForm`): the owner picks one of the own warehouse's linked marketplace warehouses;
   each own product with a member there gets that marketplace's «Доступно» as last read (part 1) — the units buyers may still order, the
   own count's meaning. A product the marketplace gives no number for is left as it is.
4. **A preview before anything is saved,** the same for the file and «Взять числа маркетплейса» (`StockImportPreview`): each row's own
   product, the count now and after, rows not found, rows ambiguous; «Сохранить» saves the found rows as movements with their cause; the
   rest are only shown.

## G. Writing the own count — the choice's third value

1. **«Передавать свой счёт wyo»** joins `StockModeChoice`: offered for a warehouse linked to an own warehouse whose key may write stock
   (part 2, section B). «?»: «wyo будет сообщать маркетплейсу свой счёт: число из «Моих складов», уменьшенное заказами всех маркетплейсов
   этого склада. Товар, которого нет в счёте, получит 0. Никакая другая программа не должна менять остатки этого склада». It works with
   «Внешний API» empty and filled.
2. **The confirmation** is part 2's, with this warehouse's preview: products, changed, 0 because not counted, ambiguous.
3. **The number to write:** the own count of the product's own product at the linked own warehouse; «не считали», no own product or
   «неоднозначно» — 0; below 0 — 0.
4. **When:** after every movement, to every linked warehouse in this value, and in part 2's full comparison every 30 minutes — through
   part 2's one writer: read first, only what differs, the batches and limits, Ozon's 30 seconds, the stored outcomes, Яндекс Маркет once
   a day, the answers, the warning. Nothing of it is written again.
5. **Falls back to «Только показывать»** when the link is removed or the own warehouse archived, with part 2's notice.

## H. The owner's view and the instructions

- **«Остатки» → «Мои склады»** (`StockPlaceList`'s fourth group, `StockPlaceCard` with what an own warehouse adds): each own warehouse's
  linked marketplace warehouses, products counted, units in all, the last movement.
- **An own warehouse's page:** its table (`StockTable` with «В моём счёте», «Последнее изменение» and, for each linked warehouse, that
  marketplace's «Доступно»), «Изменить счёт», «Загрузить из файла», «Взять числа маркетплейса», «Склады маркетплейсов этого склада»
  (link and unlink, each linked warehouse with its choice), «Движения» (`MovementList`: the product, the cause, the dates with
  `DateRangeFields`, `Pagination`), the products «не считали» and the orders for products not in the count.
- **Empty states that teach:** an own warehouse with no link — «Привяжите склады маркетплейсов, с которых вы отгружаете товар из этого
  помещения»; no count yet — the three ways of section F, each in one sentence.
- **Notices behind the bell:** a count below zero; orders for products not in the count; the fall-back.
- **`README.md`, «Остатки»,** gains «Мои склады», step by step: the own warehouse; the link; the first count three ways, with a sample
  file; what orders and cancellations do and what they never do (a return is typed by hand); a stock-taking; switching the writing on;
  the same rule of one program per warehouse. Section 1 «Что такое wyo…» names that wyo keeps the count itself for a seller without a
  fulfilment centre.
- **The constitution (MINOR):** principle XIII widened — the own count: movements never changed or deleted, an order lowers once per
  line and only from the link and the first count, a cancellation before the hand-over raises by what it lowered, own products made by
  the one matching function, the count written only through the one writer, 0 for what is not counted; the tests named.

## I. Rules R3, R4 and R5, proven here

- **R3, alone by default — the stand-alone walk:** «Внешний API» empty, one own warehouse linked to a Wildberries, an Ozon and a Яндекс
  Маркет warehouse, all three in «Передавать свой счёт wyo», a count typed; an order on the fake Ozon lowers the count, and the fake
  Wildberries and the fake Яндекс Маркет receive the lower number within their periods; a cancellation before the hand-over raises all
  three back; nothing reaches the fake fulfilment centre (`TestTheOwnCountAloneSendsASaleOnOneMarketplaceToTheOthersAndNothingElsewhere`).
- **R4, the keys stay:** the canary test widened to the own count's walk.
- **R5, the limits:** part 2's hour test with orders on every marketplace every minute and every linked warehouse written from the own
  count.

## J. Data, code, tests

**Data — one migration, `<next>_own_warehouses_and_count`:** `own_warehouses` (number, name, archived, made when; one row «Мой склад»
inserted); `own_warehouse_links` (the account, the marketplace warehouse's number, the own warehouse, linked when); `own_products`
(number, article, name, size, barcodes, made when; the members in part 1's `product_matches` with the side `own`); `own_counts` (the own
warehouse, the own product, the count, started when, changed when); `own_movements` (number, when, the own warehouse, the own product,
the change, the count after, the cause, the order and its line or the file's name or the marketplace warehouse; unique by the order,
the line and the cause). The upgrade test runs from the snapshot of part 2's release, written first.

**The wyo server's own API** (`api/wyo.yaml`): own warehouses (`GET`, `POST`, `PATCH`, `DELETE /api/v1/stock/own-warehouses…`), links
(`PUT`, `DELETE …/links/{account}/{warehouse}`), the count (`PATCH …/{id}/counts` with the typed rows), the preview and the saving of a
file's rows or of a marketplace's numbers (`POST …/{id}/count-preview`, `POST …/{id}/count-import`), the movements
(`GET …/{id}/movements`); part 2's `PUT …/mode` takes `own`.

**Code — where things live:** `internal/stock` (own warehouses, links, own products through the matching function, the count, the
movements, the orders' and cancellations' effect, the number to write from the own count), `internal/sync` (the periods of section E),
`internal/fakes/*` (orders cancelled before and after the hand-over on command where a fake lacks it), the interface's
`src/lib/components/stock/` and `src/lib/components/ui/` (the copied `FilePicker`, `FileFormatButtons`), `src/lib/stock/` (the file
reading).

**Components created** (each once, with its test file in every state): `OwnWarehouseForm`, `WarehouseLinks`, `OwnCountEditor`,
`StockImportForm`, `TakeFromMarketplaceForm`, `StockImportPreview` (shared by the file and «Взять числа маркетплейса»),
`MovementList`. **Copied** with their tests (register item 8): `FilePicker`, `FileFormatButtons`. **Components reused:**
`StockPlaceList`, `StockPlaceCard`, `StockTable`, `MatchBadge` (part 1), `StockModeChoice`, `StockModeConfirmation`,
`StockWriteState`, `StockWriteSummary`, `ForeignWriterWarning` (part 2), `ProductCell`, `NumberField`, `SectionForm`, `FormButtons`,
`SubmitWithAnswer`, `ConfirmationModal`, `DataList`, `DateRangeFields`, `Pagination`, `FilterSection`, `EmptyState`, `NoteBox`,
`StatusBadge` with the tones, `WarningBanner`, the bell; in Go the matching function, the writer, the one caller, the one budget, the one
journal.

**Tests** (written before the code they check, failing first):

- **Go:** the stand-alone walk of section I; own products made from three catalogues sharing barcodes, one ambiguous, one without
  barcodes found by article, two Wildberries sizes of one article kept apart; an order lowering once though read three times; an order
  before the link and before the first count lowering nothing; a test order and an FBO order lowering nothing; a cancellation before the
  hand-over raising exactly what was lowered and one after raising nothing; a count below zero written 0 with its notice; the file's rows
  matched by barcode and by article and the ambiguous article; «Взять числа маркетплейса» leaving a product without a number as it is;
  an archived own warehouse falling its links back; the periods of section E with «Внешний API» empty; the writer interrupted after each
  batch with no write made twice; no code updating or deleting a movement.
- **Interface unit tests:** each new component once in every state; the file-reading function with a table of every case (both
  separators, quotes, a byte-order mark, headings in another order, an empty count, a count that is not a whole number, too many rows,
  an Excel file through `read-excel-file`); the pages only for what they add; the policy tests green; `readme-policy`.
- **Browser tests** (the built wyo server in test mode with the fakes, each worker with its own data): the first count each of the three
  ways through its preview; an order made in the fake Ozon lowering the count and the fake Wildberries receiving the new number; a
  cancellation raising it; the movements list filtered; a second own warehouse added, renamed and archived; every new state swept at
  390 × 844 and the wider widths.

## Stages, smoke test, gate

1. Stage 0, the premises. 2. The migration. 3. Own warehouses and links. 4. Own products through the matching function. 5. The count and
   its movements. 6. Orders and cancellations, and the periods. 7. The first count three ways (the copied components and the file reading
   among them). 8. The choice's third value through part 2's writer. 9. The owner's view and the notices. 10. The tests of section I and
   the interrupted walks. 11. The README, `docs/`, the help texts, the constitution. 12. The smoke test, then the gate.

- A progress checklist naming each stage's work is printed after every stage.
- **Smoke test** (the `smoke-testing` skill; locally, the built wyo server with the fakes, «Внешний API» empty): «Мой склад» is there;
  link a Wildberries, an Ozon and a Яндекс Маркет warehouse; make the first count from a sample file, then correct one row by hand, then
  take the numbers of one marketplace; switch all three to «Передавать свой счёт wyo»; make an order in the fake Ozon and see the count
  and the two other fakes go down; cancel it before the hand-over and see them go back; see every movement; add, rename and archive a
  second own warehouse; every page at a computer's width and at 390 × 844 with screenshots; the console, the raw-key check and the
  outside-request check after every screen — no request to anything but the fake marketplaces.
- **Gate:** the test flow of `AGENTS.md` (`pnpm remote:gate` on the Mac mini); the check of rule R1; squash; push; the pull request; the
  merge by the agent; the release run watched to its end; `main` pulled; the report, with «Found while implementing». At the merge the
  register is updated as «Not in this plan» and «After the release» say; register items 29 and 1 (its stock half) close.

## After the release — the checks with the first real accounts (register)

On a seller without a fulfilment centre: how soon an order made on one marketplace shows as a lower number on the two others; whether a
marketplace's own lowering at an order and wyo's write after it ever disagree (part 2's warning would show it); a real file from a
seller's spreadsheet program in both kinds. What they find becomes the next plan's premises.

## Not in this plan (each goes to the register at the merge, with its reason)

- **Returns raising the count** — Stage 3 (register item 2); typed by hand meanwhile.
- **Moving goods between own warehouses** — not asked; a movement typed on each.
- **Joining or splitting own products by hand** — not asked; a wrong group is fixed by the barcode at the marketplace.
- **Supplies to marketplace warehouses lowering the own count** — Stage 5 (register item 3).
- **DBS and rFBS warehouses** — wyo reads none of their orders (register item 19).
- **A new register item, «The checks of Plan 11 on the first real accounts»,** holds «After the release».

## Rules for every AI agent implementing this plan

1. Follow `~/.agents/AGENTS.md` and `AGENTS.md`; read `.specify/memory/constitution.md` and `.agents/backlog.md` before the first
   edit.
2. Use `go-testing`, `sveltekit-testing` and `smoke-testing`; tests first, failing first; never change what a test checks to make it
   pass — the existing tests stay as they are.
3. `pnpm` and the scripts of `package.json` only; no command over 2 minutes except a run in GitHub Actions; heavy checks only by the
   main agent; at most 3 subagents, each told these rules.
4. ONE new migration; no applied migration edited; no key and no buyer datum in a log, a journal row, an answer or an error.
5. Everything is a component and is reused: one matching function, one writer, one preview for the file and the marketplace's numbers,
   one table of a place's products, one choice component; `FilePicker` and `FileFormatButtons` copied, never rewritten; search before
   writing; the components section J names are created once. Every subagent prompt repeats this rule.
6. Nothing marketplace-specific outside `internal/marketplace/<code>`, its fake and the message keys `marketplace.<code>.*`; no
   fulfilment product named anywhere (rule R1) — the check before every push.
7. **A movement is never changed or deleted; an order lowers the count once per line; nothing is written to a warehouse the owner did
   not choose; what is not counted is written as 0.** With «Внешний API» empty nothing is sent anywhere but to the marketplaces.
8. No test, no development run and no pipeline run calls a real marketplace or a real fulfilment centre. A fact about a marketplace is
   written only from its official documents (STK), with the address and the date; a gap is marked "not sourced", gets the cautious
   choice, and goes to «After the release».
9. An issue found on the way is fixed in this plan at once, or goes to the register with its reason when it cannot be; the report lists
   every finding under «Found while implementing».
10. Every visible text is a key of `messages/ru.json`, written by the help-text rules; no sentence in Go. Every form is a superform with
    its zod schema, kept without a reload. Name the program every time. Code is written mobile-first (390 × 844 first).
11. One branch, one commit, no `!` and no `BREAKING CHANGE:` without the maintainer's yes; a progress checklist after each stage; the
    final report starts with Done / Not Done.

## Estimate (inference, not a sourced fact)

Five tables, about ten addresses of the wyo server's own API, seven components created and two copied, the file reading in the browser,
and no new marketplace method — the writing is part 2's. The stand-alone walk, the once-per-line rule and the cancellation's raise are
the largest test work, and the parts that protect a seller without a fulfilment centre from selling the same unit on two marketplaces.
