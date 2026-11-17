import { vi } from 'vitest';
import { DEFAULT_LOCALE, messages } from '$lib/i18n/messages';
import { splitLocale, withLocale } from '$lib/i18n/locale-path';

/**
 * The one canonical mock for `$lib/i18n/i18n`.
 *
 * Every component test that needs the text system through a mock takes it from here, so the mock cannot drift from the real
 * module: `t` keeps the real signature (`params` placeholders, the fall back to the default language, the key itself for a
 * text nobody has) and `localizePath` leaves the default language unprefixed.
 *
 * Always mock through a lazy async factory so the call site is immune to `vi.mock` hoisting:
 *
 * ```ts
 * vi.mock('$lib/i18n/i18n', async () => (await import('$test/support/i18n')).createI18nMock());
 * ```
 *
 * The texts are the real `messages/*.json`. A language that exists only for a test (`texts`) is passed in, never written to a
 * file; a test that asserts a rendered sentence asserts it in one language only.
 */

export interface II18nMockOptions {
    /** What `getLocale()`/`getLocaleFromPath()` return, and the language `t(key)` reads when given no explicit one. */
    locale?: string;
    /** The texts of a language that exists only for this test, by language code. */
    texts?: Record<string, Record<string, string>>;
}

export function createI18nMock(options: II18nMockOptions = {}) {
    const locale = options.locale ?? DEFAULT_LOCALE;
    const bundles: Record<string, Record<string, string>> = { ...messages, ...options.texts };
    const activeLocales = [
        DEFAULT_LOCALE,
        ...Object.keys(bundles)
            .filter((code) => code !== DEFAULT_LOCALE)
            .sort()
    ];

    const bundle = (requested?: string) => bundles[requested ?? locale] ?? bundles[locale] ?? bundles[DEFAULT_LOCALE];

    /** Mirrors `t(key, lang?, params?)` in src/lib/i18n/i18n.ts. */
    const t = (key: string, lang?: string, params?: Record<string, string | number>) => {
        let message = bundle(lang)[key] ?? bundles[DEFAULT_LOCALE]?.[key] ?? key;
        if (params) {
            for (const [param, value] of Object.entries(params)) {
                message = message.replaceAll('{' + param + '}', String(value));
            }
        }
        return message;
    };

    // A path with no prefix is in the language the mock stands for (the page's own); a prefixed one is in its prefix's.
    const stripLocale = (pathname: string) => {
        const split = splitLocale(pathname, activeLocales, DEFAULT_LOCALE);
        const prefixed = split.path !== pathname;
        return { locale: prefixed ? split.locale : locale, path: split.path };
    };

    return {
        DEFAULT_LOCALE,
        t,
        // With no argument it resolves the page's language; the divergence between a page and a module outside one lives
        // inside i18n.ts, below this boundary, and is asserted against the real module in `i18n.test.ts`.
        getLocale: vi.fn((lang?: string) => lang ?? locale),
        getActiveLocales: vi.fn(() => activeLocales),
        isLocale: vi.fn((value: string) => activeLocales.includes(value)),
        getLocaleFromPath: vi.fn((pathname: string) => stripLocale(pathname).locale),
        localizePath: vi.fn((pathname: string, target: string) =>
            withLocale(stripLocale(pathname).path, target, DEFAULT_LOCALE)
        ),
        stripLocale: vi.fn(stripLocale),
        messages: bundles,
        hasMessage: vi.fn((key: string, lang?: string) => key in bundle(lang) || key in bundles[DEFAULT_LOCALE])
    };
}
