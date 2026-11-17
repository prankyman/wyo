/**
 * Decides which error messages a form shows after a field loses focus.
 *
 * The problem this exists to solve: leaving a field used to update only that field's message.
 * `validate('city', { update: 'errors' })` runs the whole schema but writes the result into the
 * `city` slot alone (`superForm.js`, the `validate` method), so every other message on screen
 * stayed exactly as it was — including messages that were already untrue. Three ways that
 * happened daily:
 *
 * - The browser fills an address. It types into five fields, but focus never leaves the one the
 *   visitor picked, so the other four keep their old red messages. Which events autofill fires
 *   is still not defined by the HTML standard (whatwg/html#3016), so no handler can rely on one.
 * - Fixing one field fixes another. «Passwords do not match» hangs under *Confirm password*, the
 *   visitor corrects *Password* instead, and the message under *Confirm* has no reason to move.
 *   Same for a postal code rule that only applies to one country: change the country and the
 *   postal-code message stays.
 * - A saved draft of a long form is restored and the whole form is checked at once, which
 *   marks fields on steps nobody has reached yet.
 *
 * Stale messages are not only noise: a form whose submit button is disabled while any message is
 * on screen simply stops working, with nothing on screen that is actually wrong.
 *
 * So: leaving a field re-checks the whole form, and this module decides what that check may
 * change. Three rules, in order of precedence:
 *
 * 1. An error the browser cannot reproduce — one the server sent, like «this code is
 *    зарегистрирован» — stays until its own field's value changes. The browser has no way to
 *    re-ask the question, so clearing it would be a guess, and the guess would be "you fixed it".
 * 2. A field the check still fails shows the message, if it is allowed to show one at all: it is
 *    the field just left, it already shows one, or its value differs from the one the page
 *    started with (typed, pasted or autofilled).
 * 3. Everything else shows nothing. An empty field the visitor has not reached yet stays quiet
 *    until they submit — the form does not go red in front of them as they walk down it.
 */

/** A path as the error tree spells it, e.g. `legalAddressCity` or `items.0._errors`. */
type TPathKey = string;

export interface IErrorRefreshInput {
    /** The messages on screen right now (the `errors` store). */
    current: unknown;
    /** The result of one whole-form check, run without writing anything. */
    fresh: unknown;
    /** Fields just left, spelled as they were passed to `validate()`. */
    justLeft: readonly string[];
    /** The form's tainted state: which values differ from the ones the page started with. */
    tainted: unknown;
    /** The form's current values, for comparing a server error against the value it was raised on. */
    data: unknown;
    /** Server errors, by path, each with the value it was raised on. */
    serverOnly: ReadonlyMap<TPathKey, unknown>;
}

export interface IErrorRefreshResult {
    /**
     * Every message the form should now show. Paths left out are cleared: superforms' own
     * `errors.set` blanks the previous messages before applying these.
     */
    errors: Record<string, unknown>;
    /** The surviving server errors — the ones whose field still holds the value they came with. */
    serverOnly: Map<TPathKey, unknown>;
}

/**
 * Splits a path the way the form library does (`lines[0].qty` and `lines.0.qty` are the same field; the library's own
 * `splitPath` cuts at `[`, `]` and `.`), then joins it back into one comparable key. It is written out here, not imported,
 * because `super-form.ts` is the only file that imports the form library.
 */
export function pathKey(path: string): TPathKey {
    return path
        .split(/[[\].]+/)
        .filter((part) => part)
        .join('.');
}

/** The field a message belongs to. `items.0._errors` is the array's own message, so: `items.0`. */
function fieldKey(key: TPathKey): TPathKey {
    const segments = key.split('.');
    if (segments.length > 1 && segments[segments.length - 1] === '_errors') segments.pop();
    return segments.join('.');
}

/**
 * Every message in an error tree, by path. A leaf is a non-empty array of strings; superforms
 * leaves `undefined` behind where a message used to be, and those are not messages.
 */
export function collectErrors(errors: unknown): Map<TPathKey, string[]> {
    const found = new Map<TPathKey, string[]>();

    const walk = (node: unknown, path: string[]): void => {
        if (Array.isArray(node)) {
            const messages = node.filter((entry): entry is string => typeof entry === 'string');
            if (messages.length > 0 && path.length > 0) found.set(path.join('.'), messages);
            return;
        }
        if (!node || typeof node !== 'object') return;
        for (const [key, value] of Object.entries(node)) walk(value, [...path, key]);
    };

    walk(errors, []);
    return found;
}

/**
 * The first message of every field of an error tree, by the field's path (`owner.title`, `items.2.weight`, `lines`
 * for an array's own `_errors`); the form's own top-level messages are not a field's and are left out.
 */
export function firstErrorByField(errors: unknown): Record<string, string> {
    const flat: Record<string, string> = {};
    for (const [path, messages] of collectErrors(errors)) {
        const field = path === '_errors' ? '' : path.replace(/\._errors$/, '');
        if (field !== '' && !(field in flat)) flat[field] = messages[0];
    }
    return flat;
}

/** The value at a path, or `undefined` when the path does not lead anywhere. */
export function valueAtPath(data: unknown, key: TPathKey): unknown {
    if (key === '') return data;
    let node: unknown = data;
    for (const segment of key.split('.')) {
        if (!node || typeof node !== 'object') return undefined;
        node = (node as Record<string, unknown>)[segment];
    }
    return node;
}

/**
 * Whether the visitor changed anything at or below this field. Superforms marks a changed value
 * `true` in a tree shaped like the form itself, and marks a value returned to what it started as
 * back to `undefined` — so this answers "differs from what the page started with", which is
 * exactly what earns a field the right to show a new message.
 */
export function hasChanged(tainted: unknown, key: TPathKey): boolean {
    const node = valueAtPath(tainted, key);
    const anyTrue = (value: unknown): boolean => {
        if (value === true) return true;
        if (!value || typeof value !== 'object') return false;
        return Object.values(value).some(anyTrue);
    };
    return anyTrue(node);
}

/** Writes a message array into a nested tree, creating the objects on the way. */
function setAtPath(target: Record<string, unknown>, key: TPathKey, messages: string[]): void {
    const segments = key.split('.');
    let node = target;
    for (const segment of segments.slice(0, -1)) {
        const next = node[segment];
        if (!next || typeof next !== 'object') node[segment] = {};
        node = node[segment] as Record<string, unknown>;
    }
    node[segments[segments.length - 1]] = messages;
}

/** Compares two form values. Values here are what a form holds: primitives, arrays, plain objects. */
export function sameValue(left: unknown, right: unknown): boolean {
    if (left === right) return true;
    if (left instanceof Date && right instanceof Date) return left.getTime() === right.getTime();
    if (Array.isArray(left) && Array.isArray(right)) {
        return left.length === right.length && left.every((entry, index) => sameValue(entry, right[index]));
    }
    if (left && right && typeof left === 'object' && typeof right === 'object') {
        const leftEntries = Object.entries(left as Record<string, unknown>);
        const rightObject = right as Record<string, unknown>;
        if (leftEntries.length !== Object.keys(rightObject).length) return false;
        return leftEntries.every(([key, value]) => key in rightObject && sameValue(value, rightObject[key]));
    }
    return false;
}

/**
 * Takes a copy of a value, so a later comparison is against what the value WAS.
 *
 * A form holding an object writes into that same object when the visitor edits it, so keeping
 * the reference would compare the value with itself and every server error would look unchanged
 * forever. Anything that cannot be copied (a File, most likely) is kept by reference: a file
 * input is never the field a server error sticks to.
 */
export function snapshotValue(value: unknown): unknown {
    if (!value || typeof value !== 'object') return value;
    try {
        return structuredClone(value);
    } catch {
        return value;
    }
}

/**
 * The errors on screen that the browser's own check does not produce — so, the ones that came
 * from the server (a duplicate code, an e-mail already registered) — each remembered with the
 * value its field held when it arrived.
 */
export function serverOnlyErrors(current: unknown, fresh: unknown, data: unknown): Map<TPathKey, unknown> {
    const freshErrors = collectErrors(fresh);
    const records = new Map<TPathKey, unknown>();
    for (const key of collectErrors(current).keys()) {
        if (freshErrors.has(key)) continue;
        records.set(key, snapshotValue(valueAtPath(data, fieldKey(key))));
    }
    return records;
}

/** Applies the three rules in this module's comment to one whole-form check. */
export function refreshedErrors(input: IErrorRefreshInput): IErrorRefreshResult {
    const current = collectErrors(input.current);
    const fresh = collectErrors(input.fresh);
    const justLeft = input.justLeft.map(pathKey);

    const errors: Record<string, unknown> = {};
    const serverOnly = new Map<TPathKey, unknown>();

    // Rule 1: a server error owns its field until that field's value changes.
    for (const [key, messages] of current) {
        if (!input.serverOnly.has(key)) continue;
        const raisedOn = input.serverOnly.get(key);
        if (!sameValue(valueAtPath(input.data, fieldKey(key)), raisedOn)) continue;
        setAtPath(errors, key, messages);
        serverOnly.set(key, raisedOn);
    }

    // Rules 2 and 3: the check's own messages, for the fields allowed to show one.
    for (const [key, messages] of fresh) {
        if (serverOnly.has(key)) continue;
        const field = fieldKey(key);
        const allowed =
            justLeft.some((left) => key === left || field === left || key.startsWith(`${left}.`)) ||
            current.has(key) ||
            hasChanged(input.tainted, field);
        if (allowed) setAtPath(errors, key, messages);
    }

    return { errors, serverOnly };
}
