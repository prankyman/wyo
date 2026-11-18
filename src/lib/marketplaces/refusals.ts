import { hasMessage } from '$lib/i18n/i18n';
import type { TLocale } from '$lib/i18n/types';

/**
 * The words of an account's refusals and notes. The wyo server answers codes (`key_refused`, `missing.product`), and a code is
 * worded by the ONE lookup here (`accountTextKey`): the marketplace's own text first — `marketplace.<marketplace>.refusal.<code>`,
 * `marketplace.<marketplace>.note.<code>` — and only when it has none the shared one, `accounts.refusal.<code>`,
 * `accounts.note.<code>`. So a sentence that names what only one marketplace has (a kind of key, a category, a level of access)
 * lives in that marketplace's family and the shared family keeps the words that fit every marketplace. The key form, the
 * account's card and its buttons all ask this one function.
 */

/** The family of the shared texts of the accounts' refusals; the page «Обновить», which reads every account, words its refusals from it. */
export const ACCOUNT_REFUSALS = 'accounts.refusal';

/** What an account's text says: why the wyo server refused something (`refusal`), or what an account's card tells the owner (`note`). */
export type TAccountTextKind = 'refusal' | 'note';

/** Whether `key` is a text, or the base of a text written in counted forms (`<key>.one` … `<key>.other`, worded by `tPlural`). */
function holds(key: string, lang?: TLocale): boolean {
    return hasMessage(key, lang) || hasMessage(`${key}.other`, lang);
}

/**
 * The key of the text of `code` for an account of `marketplace` (left out: the shared text only), or null when nobody has one.
 * `code` is what the wyo server answers; a missing category is `missing.<category>`. A text written in counted forms comes back
 * as its base key, for the caller to word with the plural helper.
 */
export function accountTextKey(
    kind: TAccountTextKind,
    code: string,
    marketplace?: string,
    lang?: TLocale
): string | null {
    const own = marketplace === undefined ? null : `marketplace.${marketplace}.${kind}.${code}`;
    if (own !== null && holds(own, lang)) return own;
    const shared = `accounts.${kind}.${code}`;
    return holds(shared, lang) ? shared : null;
}

/**
 * The lookup the forms and the buttons are given for the refusals of one marketplace's account (their `refusals` and `family`
 * options): the key of the text of a refusal's code, or null. It is `accountTextKey` for refusals, with the marketplace fixed.
 */
export function accountRefusalLookup(marketplace?: string): (code: string) => string | null {
    return (code) => accountTextKey('refusal', code, marketplace);
}
