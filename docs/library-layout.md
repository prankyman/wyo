# The component library

Everything is a component and is reused (`.specify/memory/constitution.md`, principle III): **no screen of any plan writes a piece this
library already has.** This page says what is in the library, what waits to be copied, and how a piece is added.

The library is `src/lib/components/ui/`. It was carried over from the maintainer's other repository by Plan 1, component by component, each
with its test file beside it; names, props and test files are kept, so a copied piece behaves as it did there. Only three things changed:
the colours (they are theme tokens now, `src/app.css`, constitution «The theme»), the few pieces that touched a server (below), and every
word, comment or example that belonged to the other repository's subject.

## What is in the library

**Copied as they are** (only the colours changed), each with its `.svelte.test.ts` or `.test.ts`:

- Buttons and press feedback: `Button` with `press.ts`, `IconButton`, `ButtonWithHelp`, `BlockedButton`, `CopyButton`.
- Help: `IconTooltip`, `HelpTooltip`, `HelpLinkList`, `DisclosureWithHelp`, `HeadingWithHelp`, `PageHeading`.
- Layout and notes: `Stack`, `NoteBox`, `WarningBanner`, `EmptyState`, `BackLink`, `StatusBadge`, `StatusBadgeWithHelp` with
  `status-tones.ts`, `PrevNextLinks`.
- Clickable cards and rows: `ClickableCard`, `ClickableRow`, `StretchedLink`.
- Windows: `Portal`, `ModalWindow`, `WindowButtons`.
- Fields and forms: `Field`, `FieldLabel`, `FieldRefusal`, `TextBox`, `SelectBox`, `FormButtons`, `FormAnswer`.
- Messages: the toast store with `Toaster`.
- Helpers: `field-classes.ts`, `overlay-a11y.ts`, `dismissable.ts`, `tooltip-position.ts`, `sheet-drag.ts`, `reduced-motion.svelte.ts`,
  `form-guard.svelte.ts`, `page-change.svelte.ts`, `shallow-details.ts` (the row a side panel opened, kept in the page's history
  state), `collapsible-section.ts`, `step-strip.ts`, `number-field.ts`, `disclosure-form.ts`, `types.ts`, `cn.ts`, `clipboard.ts`.
  `field-classes.ts` also holds `COPY_BOX_CLASS`, the box `ShowOnceSecret` draws its secret in.

**Copied with a named change:**

- `GuardedForm` — its branch for SvelteKit form actions is removed; it keeps the form library's branch, the single submission, the busy
  state of its buttons and the confirmation window.
- `src/lib/forms/super-form.ts`, `answer.ts`, `section-form.ts` — the form library in its single-page mode: a form is checked in the
  browser by its `zod` schema and sent as JSON through `src/lib/api/`; the wyo server checks it again and is the authority. Its refusal
  `{ "error": { "code", "params"?, "issues"?: [{ "path", "code" }] } }` is laid onto the form's fields by one function.
- `ConfirmationModal`, `SlideOverPanel`, `RowDetailsPanel`, `CollapsibleSection`, `StepStrip`, `Tabs`, `Pagination`, `FilterSection`,
  `RemovableChip`, `FilteredListNote`, `NumberField` with `number-input.ts`, `Textarea`, `FieldMessage`, `SubmitWithAnswer`,
  `SectionForm`, `DisclosureForm`, `DisclosureClose`, `PageChangeGuard`, `PopupCloseButton`, `AttentionIcon`, `ShowOnceSecret` (its one
  import from outside the library is brought inside) — the language comes from the page's address in the browser (constitution,
  principle I), no longer from a server.

**The shell and the sign-in parts** (copied by Plan 2, `src/lib/components/shell/` and `src/lib/components/auth/`, each with its test file; what
belonged to the other repository's subject is dropped, and the names, props and behaviour that still apply are kept):

- `ShellHeader` (the bar of the top: the left side and the buttons in one row that never wraps), `ShellUserBlock` (the owner's login that opens
  «Мой профиль», an avatar below 768 px), `AppNav` with its group state `NavGroupsState` (`src/lib/nav/nav-groups.svelte.ts`) and `CounterPill` — the
  menu as a column at the left from 1024 px and a strip along the bottom of a phone, drawn from the ONE list `src/lib/nav/nav-items.ts`; the
  menu has no group yet, the group machinery is kept for the plans that add items.
- `NotificationBell` with `NotificationPanel` and `panel-rules.ts` — the bell and its panel. The wyo server's notices are lines only
  (`src/lib/notifications/`): the other repository's short-lived events and its filter by place are not copied.
- `AuthShell` (the frame of the pages before sign-in) and `NewPasswordFields` (the current, new and repeated password boxes).
- `LanguageSwitcher` and `DocumentLanguage` — the switch draws nothing while one language exists; a language's name and writing direction
  are texts of its own file (`language.nativeName`, `language.direction`), so a second file is a second language with no change in code.
- The error page `src/routes/+error.svelte`, drawn with theme tokens.
- **Menu places** (`src/lib/i18n/menu-places.ts`, `menu-place-policy.test.ts`): a text that says where something is in the menu writes
  `{menu:<id>}` (for example `{menu:nav-settings}`), and the text system writes it out from the menu's own list. Constitution, help-text rule 9.

**New in this repository, built on the pieces above** (Plan 2): `InstallationStateCard` (one card of «Обзор»: a title, a state with its tone, a
sentence, an action), `SettingsSection` (one registry group drawn as a `SectionForm`), `UpdatesStatus`, `BackupsSection` and `ApiAction` (a press
that is one call of the wyo server with its answer under the button).

**Policy tests copied with the components** (each allow-list emptied of the other repository's entries; a policy test reads the source files
and fails the build when a rule is broken): `modal-window-policy`, `window-buttons-policy`, `note-box-policy`, `empty-state-policy`,
`stack-policy`, `form-control-policy`, `field-label-policy`, `number-field-policy`, `form-buttons-policy`, `page-width-policy`,
`disclosure-form-policy`, `section-form-policy`, `collapsible-section-policy`, `press-feedback-policy`, `status-tones-policy`,
`select-arrow-policy`, `page-help-policy`, `help-placement-policy`, `helper-duplication` (one home file per helper) and
`test-helper-duplication` (one home per test fixture). New in this repository: `theme-policy`, `message-bundles`,
`locale-argument-policy`, `reference-policy`, `menu-place-policy` (Plan 2).

**The test set-up copied:** the two Vitest projects chosen by file name (`*.svelte.test.ts` in a browser-like environment, everything else
without one), the stand-ins for SvelteKit's `$app/*` modules, the `$test` alias, and the shared fixtures that are not about the other
repository's subject (the texts fixture, the form fixtures for the single-page mode, `renderPageWith`) — all in `src/test/support/`.

**`src/app.html`** keeps the two start-up scripts that are plain browser code: the guard that refuses a second submission before the
interface's code runs, and the page-change bar (in the theme's colour). A form sent before the code runs is stopped, not posted: static
files have nothing to post to.

**The showcase page** `/showcase` draws every copied component in each of its states under the theme. It is how the maintainer judges the
theme, and the page the browser tests measure at 390 pixels. It exists only in a wyo server built with the Go build tag `wyotest`; a
released wyo server answers "not found" at its address, and a Go test of the release build proves it. The page is drawn by two sections, `src/routes/showcase/ShowcaseBasics.svelte` (buttons, help, notes, statuses, navigation) and
`ShowcaseForms.svelte` (fields, windows, forms); `ShowcaseRealForm.svelte` is one real single-page form, sent to a stub
(`showcase-stub.ts`) instead of a server.

## What waits

**Not copied yet — copied by the plan that first needs one, with its test, never rewritten:**

- the searchable picker `Combobox` (the only user of the package `bits-ui`), `FilePicker`, `CodeNameSelect`, `DateRangeFields`,
  `TickAllBox`, `FileFormatButtons`, `StepsCard`, `RowButtonForm`, `ReasonDialog`;
- the list component `DataList` with `ColumnHeading` and `ColumnLegend` — Plan 3 (the order list);

The same list is item 8 of the register (`.agents/backlog.md`). When a plan copies one, it moves the name from this section to the list
above, with its test and its policy test if the other repository has one, and closes nothing else.

## How a piece is added

1. Search the library for the component or function that already does it, and use it.
2. If it is not here, look for it in the maintainer's other repository and copy it with its test file — never invent it. Remove every plan
   number, name and comment that points at that repository (rule R1), replace colours with theme tokens and examples with neutral ones, and
   keep names, props and behaviour. A copied test changes only in the ways the plan names; if it fails under the new stack, find the cause
   first — never change what it checks to make it pass.
3. If a piece is about to be written a second time, turn the first one into a component and use it in both places, in the same plan.
4. A component is tested once, in its own test file, in every state and with every prop. A page's test checks only what the page adds.
5. Every plan names the components it creates and the ones it reuses.
