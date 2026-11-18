import { page } from '$app/state';
import { DEFAULT_LOCALE, getActiveLocales, isLocale, messages } from './messages';
import { splitLocale, withLocale } from './locale-path';
import { resolveMenuPlaces } from './menu-places';
import type { TLocale } from './types';
export type { TLocale } from './types';
export { DEFAULT_LOCALE, getActiveLocales, isLocale, messages };

/**
 * Splits a language prefix off `pathname`, if present. Every `pathname.startsWith('/settings')`-style check goes through
 * this rather than splitting by hand, since the default language is unprefixed and every other language is prefixed.
 */
export function stripLocale(pathname: string): { locale: TLocale; path: string } {
    return splitLocale(pathname, getActiveLocales(), DEFAULT_LOCALE);
}

export function getLocaleFromPath(pathname: string): TLocale {
    return stripLocale(pathname).locale;
}

/**
 * The language of the page being shown: the one in its address. Outside a page (a test, a module loading) there is no address
 * to read and the default language answers. The wyo interface has no server side, so the address is the only source.
 */
export function getLocale(explicit?: TLocale): TLocale {
    if (explicit) return explicit;
    try {
        return getLocaleFromPath(page.url.pathname);
    } catch {
        return DEFAULT_LOCALE;
    }
}

/**
 * The text of `key` in the language of the page (or in `lang`), with `{name}` placeholders filled from `params`. A key the
 * language lacks falls back to the default language; a key nobody has comes back as itself, so a missing text is visible.
 */
export function t(key: string, lang?: TLocale, params?: Record<string, string | number>): string {
    const locale = getLocale(lang);
    let message = messages[locale]?.[key] ?? messages[DEFAULT_LOCALE]?.[key] ?? key;
    if (params) {
        for (const [name, value] of Object.entries(params)) {
            message = message.replaceAll(`{${name}}`, String(value));
        }
    }
    // A menu place is written out from the menu itself, in the same language.
    return resolveMenuPlaces(message, (label) => messages[locale]?.[label] ?? messages[DEFAULT_LOCALE]?.[label]);
}

/**
 * Whether `key` has a text in the requested language or in the default one. `t()` returns the key itself for a miss, so
 * `t(k) ?? fallback` never falls back; a caller that builds a key out of data asks this first.
 */
export function hasMessage(key: string, lang?: TLocale): boolean {
    const locale = getLocale(lang);
    return key in (messages[locale] ?? {}) || key in (messages[DEFAULT_LOCALE] ?? {});
}

/** The address of `pathname` in `locale`: unprefixed for the default language, prefixed for every other. */
export function localizePath(pathname: string, locale: TLocale): string {
    return withLocale(stripLocale(pathname).path, locale, DEFAULT_LOCALE);
}
