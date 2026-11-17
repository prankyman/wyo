import type { TLocale } from './types';

/**
 * Splits a language code off the front of `pathname` when the first segment is one of `locales`: `/en/showcase` is the
 * page `/showcase` in `en`. The default language is never written in an address, so it is not a prefix; a path without
 * a prefix is in `fallback`. The one place an address is cut into language and page (`src/hooks.ts` and `i18n.ts` both
 * use it), so they can never disagree.
 */
export function splitLocale(
    pathname: string,
    locales: readonly TLocale[],
    fallback: TLocale
): { locale: TLocale; path: string } {
    const segments = pathname.split('/').filter(Boolean);
    const first = segments[0];
    if (first !== undefined && first !== fallback && locales.includes(first)) {
        return { locale: first, path: '/' + segments.slice(1).join('/') };
    }
    return { locale: fallback, path: pathname };
}

/** `pathname` (without a language prefix) as `locale` writes it: the default language is unprefixed. */
export function withLocale(pathname: string, locale: TLocale, fallback: TLocale): string {
    if (locale === fallback) return pathname;
    return '/' + [locale, ...pathname.split('/').filter(Boolean)].join('/');
}
