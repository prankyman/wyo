import { hasMessage } from '$lib/i18n/i18n';

/**
 * Which text of `messages/` words a refusal. The wyo server answers codes, never sentences, so a refusal is worded here, from the
 * interface's own texts: a code with no text of its own gets a general one.
 */

/** The key of the text that words a refusal `code`: `errors.<code>`, or `errors.unknown` when no text exists for the code. */
export function refusalKey(code: string): string {
    return hasMessage(`errors.${code}`) ? `errors.${code}` : 'errors.unknown';
}

/** The key of the text that words the refusal of one field: `errors.<code>`, or `errors.invalid` when no text exists. */
export function fieldRefusalKey(code: string): string {
    return hasMessage(`errors.${code}`) ? `errors.${code}` : 'errors.invalid';
}
