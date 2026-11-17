import { getContext, setContext } from 'svelte';
import type { TLocale } from '$lib/i18n/types';

/**
 * What a `DisclosureForm` tells the form inside it, and what it reads back (the constitution's forms rule on forms opened
 * by a press). The form's «Закрыть» (`DisclosureClose`) stands beside the form's own main button, so it reaches the
 * `DisclosureForm` around it through this context; every superform made inside one reports its answer here (`superForm`,
 * `src/lib/forms/super-form.ts`), so an edit form closes after a successful save without a line of code of its own. A form
 * that is not a superform (a browser-only one) calls `answered` itself.
 */
export interface IDisclosureForm {
    /** «Закрыть»: closes at once when nothing was changed, asks first when something was. */
    close(): void;
    /** The form's answer: a success closes an edit form (`after="close"`) or readies an add form for the next row. */
    answered(ok: boolean): void;
    readonly lang: TLocale;
    /** `<testId>-close` names the «Закрыть» button. */
    readonly testId: string | undefined;
}

const KEY = Symbol('disclosure-form');

export function setDisclosureForm(form: IDisclosureForm): void {
    setContext(KEY, form);
}

/**
 * The `DisclosureForm` around the component being made, or nothing. Safe outside a component's start (a form helper
 * called from a plain module), where Svelte has no context to read.
 */
export function disclosureFormOf(): IDisclosureForm | undefined {
    try {
        return getContext<IDisclosureForm | undefined>(KEY);
    } catch {
        return undefined;
    }
}

type TControl = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;

/** What a person can change: every named box, tick box and list, not the hidden values a screen sets itself. */
function controlsOf(root: HTMLElement): TControl[] {
    return [...root.querySelectorAll<TControl>('input[name], select[name], textarea[name]')].filter(
        (control) =>
            !(control instanceof HTMLInputElement && ['hidden', 'submit', 'button', 'reset'].includes(control.type))
    );
}

function valueOf(control: TControl): string {
    if (control instanceof HTMLInputElement) {
        if (control.type === 'checkbox' || control.type === 'radio') return control.checked ? 'on' : '';
        if (control.type === 'file') return [...(control.files ?? [])].map((file) => file.name).join('\n');
    }
    if (control instanceof HTMLSelectElement && control.multiple)
        return [...control.selectedOptions].map((option) => option.value).join('\n');
    return control.value;
}

/** The values of a form's fields as they are now, read from the page itself. */
export type TFieldValues = Map<TControl, string>;

export function fieldValues(root: HTMLElement): TFieldValues {
    return new Map(controlsOf(root).map((control) => [control, valueOf(control)]));
}

/** What a field held when it was drawn: an added line or a new step starts from it. */
function initialValueOf(control: TControl): string {
    if (control instanceof HTMLInputElement) {
        if (control.type === 'checkbox' || control.type === 'radio') return control.defaultChecked ? 'on' : '';
        if (control.type === 'file') return '';
        return control.defaultValue;
    }
    if (control instanceof HTMLSelectElement) {
        const chosen = [...control.options].filter((option) => option.defaultSelected).map((option) => option.value);
        return control.multiple ? chosen.join('\n') : (chosen[0] ?? control.options[0]?.value ?? '');
    }
    return control.defaultValue;
}

/**
 * Whether anything was changed since `before`: a field whose value differs, or a field added since (a line put into the
 * form) that holds something. A field that left the page is not a change by itself — a form of several steps (the card
 * sign-in) replaces its fields as it goes. It reads the page, not the form's data, so it sees a change also in a form that
 * binds nothing to its data (the camera form, the video mode) and in a station or browser-only form.
 */
export function changedSince(before: TFieldValues, root: HTMLElement): boolean {
    for (const [control, value] of fieldValues(root)) {
        const was = before.get(control);
        if (was === undefined ? value !== initialValueOf(control) : was !== value) return true;
    }
    return false;
}

/** The first box a person types into, which takes the cursor when the form opens. */
export function firstField(root: HTMLElement): HTMLElement | undefined {
    return controlsOf(root).find((control) => !control.disabled && !(control as HTMLInputElement).readOnly);
}
