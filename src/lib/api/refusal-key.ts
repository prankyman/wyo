import { hasMessage } from '$lib/i18n/i18n';

/**
 * Which text of `messages/` words a refusal. The wyo server answers codes, never sentences, so a refusal is worded here, from the
 * interface's own texts: a code with no text of its own gets a general one.
 *
 * A place whose refusals have texts of their own (the marketplace accounts: `accounts.refusal.<code>`) names that family; a code
 * the family has no text for is worded from the general texts `errors.<code>`, so a lost connection reads the same everywhere.
 */

/** One family of texts (a prefix), or a lookup that answers the key of the text of a code (or null): an account's, marketplace first. */
export type TFamily = string | ((code: string) => string | null);

/** The family's own text of `code`, when a family is named and holds one. */
function familyKey(code: string, family: TFamily | undefined): string | null {
    if (family === undefined) return null;
    if (typeof family === 'function') return family(code);
    return hasMessage(`${family}.${code}`) ? `${family}.${code}` : null;
}

/** The key of the text that words a refusal `code`: `<family>.<code>`, `errors.<code>`, or `errors.unknown`. */
export function refusalKey(code: string, family?: TFamily): string {
    return familyKey(code, family) ?? (hasMessage(`errors.${code}`) ? `errors.${code}` : 'errors.unknown');
}

/** The key of the text that words the refusal of one field: `<family>.<code>`, `errors.<code>`, or `errors.invalid`. */
export function fieldRefusalKey(code: string, family?: TFamily): string {
    return familyKey(code, family) ?? (hasMessage(`errors.${code}`) ? `errors.${code}` : 'errors.invalid');
}
