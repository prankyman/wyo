import type { TLocale } from './types';

/**
 * A decimal number with the comma a Russian reader and a Russian keyboard use. The number boxes read a comma and a dot alike,
 * so a form fills its boxes with the comma; a text shown to the reader follows the reader's language with `decimalText`.
 */

/** The decimal text (or number) with a comma instead of the dot: `'2.5'` → `'2,5'`, `''` stays `''`. */
export function decimalComma(value: string | number): string {
    return String(value).replace('.', ',');
}

/** A decimal number as the language writes it: «58,5» in Russian, "58.5" in the other languages. */
export function decimalText(value: string | number, lang: TLocale): string {
    return lang === 'ru' ? decimalComma(value) : String(value);
}
