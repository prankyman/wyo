import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import { DEFAULT_LOCALE, getActiveLocales, isLocale, messages } from './messages';
import { getLocale, getLocaleFromPath, hasMessage, localizePath, stripLocale, t } from './i18n';

/**
 * The text system. Russian is the only file in `messages/`; a second language exists in these tests only, put into the
 * table of texts for the length of one test and taken out again.
 */
const SECOND = 'xx';

function withSecondLanguage(texts: Record<string, string>): void {
    messages[SECOND] = texts;
}

afterEach(() => {
    delete messages[SECOND];
    resetPage();
});

describe('the languages', () => {
    it('knows Russian as the only language and writes no other code', () => {
        expect(DEFAULT_LOCALE).toBe('ru');
        expect(getActiveLocales()).toEqual(['ru']);
        expect(isLocale('ru')).toBe(true);
        expect(isLocale(SECOND)).toBe(false);
    });

    it('knows a second language the moment its file exists, listed after the default one', () => {
        withSecondLanguage({});
        expect(getActiveLocales()).toEqual(['ru', SECOND]);
        expect(isLocale(SECOND)).toBe(true);
    });
});

describe('t', () => {
    it('gives the Russian text of a key', () => {
        expect(t('nav.settings')).toBe('Настройки');
        expect(t('nav.overview', 'ru')).toBe(messages.ru['nav.overview']);
    });

    it('fills {name} placeholders, every occurrence, with numbers and text', () => {
        messages.ru['test.twice'] = '{name} and {name}: {count}';
        expect(t('test.twice', 'ru', { name: 'wyo', count: 3 })).toBe('wyo and wyo: 3');
        delete messages.ru['test.twice'];
    });

    it('writes a menu token out from the menu, in the language of the text', () => {
        messages.ru['test.place'] = 'Нажмите в меню {menu:nav-settings}.';
        expect(t('test.place', 'ru')).toBe('Нажмите в меню «Настройки».');
        delete messages.ru['test.place'];
    });

    it('gives back the key itself when no language has it, so a missing text is visible', () => {
        expect(t('no.such.key')).toBe('no.such.key');
        expect(hasMessage('no.such.key')).toBe(false);
        expect(hasMessage('nav.settings')).toBe(true);
    });

    it('reads the text in the language the page address names and falls back to Russian for a key it lacks', () => {
        withSecondLanguage({ 'nav.settings': 'second title' });
        setPageUrl('/xx/showcase');

        expect(getLocale()).toBe(SECOND);
        expect(t('nav.settings')).toBe('second title');
        expect(t('nav.overview')).toBe(messages.ru['nav.overview']);
        expect(hasMessage('nav.overview')).toBe(true);
    });

    it('prefers the language it is given over the address', () => {
        withSecondLanguage({ 'nav.settings': 'second title' });
        setPageUrl('/showcase');

        expect(t('nav.settings', SECOND)).toBe('second title');
        expect(getLocale(SECOND)).toBe(SECOND);
    });
});

describe('addresses', () => {
    it('cuts a language prefix off only when that language exists', () => {
        expect(stripLocale('/xx/showcase')).toEqual({ locale: 'ru', path: '/xx/showcase' });

        withSecondLanguage({});
        expect(stripLocale('/xx/showcase')).toEqual({ locale: SECOND, path: '/showcase' });
        expect(stripLocale('/xx')).toEqual({ locale: SECOND, path: '/' });
        expect(stripLocale('/showcase')).toEqual({ locale: 'ru', path: '/showcase' });
        expect(getLocaleFromPath('/xx/a/b')).toBe(SECOND);
    });

    it('never treats the default language as a prefix', () => {
        expect(stripLocale('/ru/showcase')).toEqual({ locale: 'ru', path: '/ru/showcase' });
    });

    it('writes an address in a language: unprefixed for Russian, prefixed for the others, whatever it started as', () => {
        withSecondLanguage({});
        expect(localizePath('/showcase', SECOND)).toBe('/xx/showcase');
        expect(localizePath('/xx/showcase', 'ru')).toBe('/showcase');
        expect(localizePath('/xx/showcase', SECOND)).toBe('/xx/showcase');
        expect(localizePath('/', SECOND)).toBe('/xx');
    });
});
