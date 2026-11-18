import { render } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$lib/i18n/i18n.js', async () =>
    (await import('$test/support/i18n')).createI18nMock({
        locale: 'ru',
        texts: { en: { 'language.direction': 'ltr' }, ar: { 'language.direction': 'rtl' }, xx: {} }
    })
);

import DocumentLanguage from './DocumentLanguage.svelte';

afterEach(() => {
    document.documentElement.lang = '';
    document.documentElement.dir = '';
});

describe('DocumentLanguage', () => {
    it('points the page language and its writing direction at the language of the address, on every switch', async () => {
        const { rerender } = render(DocumentLanguage, { props: { locale: 'ru' } });
        const html = document.documentElement;

        expect([html.lang, html.dir]).toEqual(['ru', 'ltr']);

        await rerender({ locale: 'ar' });
        expect([html.lang, html.dir]).toEqual(['ar', 'rtl']);

        await rerender({ locale: 'en' });
        expect([html.lang, html.dir]).toEqual(['en', 'ltr']);
    });

    it('reads a language that does not say its direction as left to right', () => {
        render(DocumentLanguage, { props: { locale: 'xx' } });

        expect(document.documentElement.dir).toBe('ltr');
    });
});
