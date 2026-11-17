import type { Snippet } from 'svelte';
import type { HTMLAttributes, HTMLFormAttributes } from 'svelte/elements';
import type { Readable } from 'svelte/store';

/** One choice of `SelectBox`'s `options`. */
export interface ISelectOption<T extends string | number = string> {
    value: T;
    label: string;
    disabled?: boolean;
}

/** A link under the paragraphs of a «?» popup; it opens in a new tab (`HelpTooltip`'s `links`). */
export interface IHelpLink {
    label: string;
    href: string;
}

/**
 * One filter in use, shown as a chip under the «Фильтр» heading (`FilterSection`). `label` is the chip's words,
 * already translated ("Статус: Активен"). A filter that lives in the address gives `href` (the address without it); a
 * filter held by the screen gives `onRemove`.
 */
export interface IFilterChip {
    key: string;
    label: string;
    href?: string;
    onRemove?: () => void;
}

// ------------------------------------------------------------------ press feedback & single submission
//
// The shared pressable controls (constitution, press feedback and single submission). How to use each one is written down in `docs/library-layout.md`.

/** The looks a `Button` comes in — the colour families the screens already used before it existed. */
export type TButtonVariantEnum =
    'primary' | 'secondary' | 'danger' | 'danger-outline' | 'success' | 'warning' | 'ghost' | 'ghost-inverse' | 'link';

/** `md` is the 44 px default; `lg` (56 px) and `xl` (64 px) are for work done on a phone. */
export type TButtonSizeEnum = 'md' | 'lg' | 'xl';

export interface IButtonProps extends Omit<HTMLAttributes<HTMLElement>, 'class' | 'children' | 'onclick'> {
    variant?: TButtonVariantEnum;
    size?: TButtonSizeEnum;
    /** Stretch to the width of the container. */
    fullWidth?: boolean;
    type?: 'button' | 'submit';
    name?: string;
    value?: string;
    /** The id of a form outside this button that it submits (the `form` attribute). */
    form?: string;
    formaction?: string;
    formmethod?: 'get' | 'post';
    formnovalidate?: boolean;
    /** A real `disabled`: the control cannot be used for a reason the screen explains. */
    disabled?: boolean;
    /**
     * The action this button started is running (a `fetch` the screen started itself): spinner,
     * `aria-busy`, and it cannot fire again. A button that submits a `GuardedForm` does NOT need
     * this — the form tells it.
     */
    pending?: boolean;
    /** Renders an `<a>` styled as a button. Already localized by the caller. */
    href?: string;
    target?: string;
    rel?: string;
    /** A download link. A link with `download` or `target` never shows the working state. */
    download?: boolean | string;
    hreflang?: string;
    dataTest?: string;
    ariaLabel?: string;
    id?: string;
    /** Extra classes, merged with `cn()`: a conflicting utility here wins over the variant's. */
    class?: string;
    onclick?: (event: MouseEvent) => void;
    /** An icon before the text; replaced by the spinner while working. Alone, it makes a square button. */
    icon?: Snippet;
    children?: Snippet;
    /** The rendered element, for focusing it or calling `requestSubmit(button)`. */
    element?: HTMLElement;
}

/** What a `GuardedForm` needs from a `superForm(...)` result — pass `{ enhance, submitting }`. */
export interface IGuardedSuperForm {
    enhance(formElement: HTMLFormElement): { destroy(): void };
    submitting: Readable<boolean>;
}

export interface IGuardSubmitOptions {
    /** The state object to drive; `GuardedForm` passes its own so it can render from it. */
    guard?: IFormGuard;
    /**
     * True when JavaScript sends the form (a superForm) and will say when
     * the answer is in; the guard then waits for that instead of for a page load.
     */
    enhanced?: boolean;
    /** A superForm's `submitting` store: its return to `false` ends the submission. */
    submitting?: Readable<boolean>;
}

/** The live state of one guarded form, read by every `Button` that submits it. */
export interface IFormGuard {
    readonly submitting: boolean;
    /** The button the submission came from; `null` for `form.requestSubmit()` with no submitter. */
    readonly submitter: HTMLElement | null;
    start(submitter: HTMLElement | null): void;
    release(): void;
}

/** The words of a confirmation window: the thing named, what happens and what stays, the action's word. */
export interface IFormConfirm {
    title: string;
    message: string;
    confirmText: string;
}

export interface IGuardedFormProps extends Omit<HTMLFormAttributes, 'class' | 'children' | 'method' | 'action'> {
    method?: 'GET' | 'POST' | 'get' | 'post';
    id?: string;
    class?: string;
    dataTest?: string;
    /** A superForm — `{ enhance, submitting }` from `superForm(...)` (`src/lib/forms/super-form.ts`). */
    superform?: IGuardedSuperForm;
    /** The `<form>` element, for `requestSubmit()`. */
    element?: HTMLFormElement;
    /**
     * The confirmation window of a destructive press: a press of the form's submit button opens
     * `ConfirmationModal` instead of sending, and the window's red button sends the form. Left out, the form sends at once.
     * The words are already worded by the page, so a text may name the thing and repeat the reason typed.
     */
    confirm?: IFormConfirm;
    /** The form's boxes and buttons; none for a form that is filled and sent from code. */
    children?: Snippet;
}

interface IStretchedLinkBase {
    href: string;
    dataTest?: string;
    /** Overrides the accessible name when the visible text alone does not say what opens. */
    ariaLabel?: string;
    class?: string;
    hreflang?: string;
}

/** The link that opens a record; its hit area covers the whole `ClickableCard` / `ClickableRow`. */
export interface IStretchedLinkProps extends IStretchedLinkBase {
    children: Snippet;
}

export interface IClickableCardProps
    extends IStretchedLinkBase, Omit<HTMLAttributes<HTMLElement>, 'class' | 'children' | 'title'> {
    /** The card's element: `li` inside a list, `article` for a standalone record, `div` otherwise. */
    as?: 'div' | 'li' | 'article';
    /** `data-test` of the link itself (the card's own id goes in `dataTest`). */
    linkDataTest?: string;
    /** Extra classes for the link (the card heading). */
    linkClass?: string;
    /** The card heading: the text of the link that opens the record. */
    title: Snippet;
    /** Everything else on the card. Buttons, links and checkboxes in here keep working on their own. */
    children?: Snippet;
}

export interface IClickableRowProps extends Omit<HTMLAttributes<HTMLTableRowElement>, 'class' | 'children'> {
    dataTest?: string;
    class?: string;
    /** The cells. Exactly one of them holds a `StretchedLink`. */
    children: Snippet;
}
