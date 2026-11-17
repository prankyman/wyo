import { getLocale, hasMessage, t } from './i18n';
import type { TLocale } from './types';

/**
 * Renders a counted phrase in the grammatical form the reader's language actually uses.
 *
 * Russian has three forms where English has two — «1 день», «2 дня», «5 дней» — and the
 * choice is not "one or more than one": 21 takes the same form as 1, 22 the same as 2, and
 * 11 through 14 take the same form as 5. A `count === 1 ? a : b` ternary is therefore wrong
 * for 0, 5, 11 and 25 every single time, which is how a page comes to read «0 дня» and «2 дней».
 *
 * The bundle carries one key per CLDR plural category — `<base>.one`, `.few`, `.many`,
 * `.other` — and `Intl.PluralRules` decides which one this count needs. English only ever
 * selects `one` and `other`; its `few` and `many` repeat `other`, because
 * `message-bundles.test.ts` requires every bundle to hold the same key set.
 *
 * `{count}` is substituted for you. Anything else the sentence needs goes in `params`, and
 * `lang` follows the same rule as `t()`'s second argument: omit it where the page's address
 * names the language, pass it where it does not.
 */
export function tPlural(
    baseKey: string,
    count: number,
    lang?: TLocale,
    params?: Record<string, string | number>
): string {
    const locale = getLocale(lang);
    const categoryKey = `${baseKey}.${new Intl.PluralRules(locale).select(count)}`;

    // A missing category degrades to the general form rather than printing
    // `days.count.few` at the reader. `t()` returns the key itself on a miss, so
    // `hasMessage()` is the only way to ask this question.
    const key = hasMessage(categoryKey, locale) ? categoryKey : `${baseKey}.other`;

    return t(key, locale, { count, ...params });
}
