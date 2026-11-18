import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);
vi.mock('$lib/i18n/i18n.js', async () =>
    (await import('$test/support/i18n')).createI18nMock({
        locale: 'ru',
        texts: {
            en: { 'language.nativeName': 'English', 'shell.language.trigger': 'Page language: {name}' },
            zh: { 'language.nativeName': '中文' }
        }
    })
);

import { goto } from '$app/navigation';
import { getActiveLocales } from '$lib/i18n/i18n';
import { resetPage, setPageUrl } from '$test/support/page-state.svelte';
import LanguageSwitcher from './LanguageSwitcher.svelte';

/** The settings page with a query and a fragment in the address. */
function renderOnSettings() {
    setPageUrl('/settings?tab=a#top');
    render(LanguageSwitcher, { props: { lang: 'ru', currentPath: '/settings' } });
    return {
        trigger: screen.getByTestId('language-trigger'),
        list: screen.getByTestId('language-list'),
        option: (code: string) => screen.getByTestId(`language-option-${code}`)
    };
}

afterEach(() => {
    resetPage();
    vi.mocked(goto).mockReset();
    vi.mocked(goto).mockResolvedValue(undefined);
});

describe('LanguageSwitcher', () => {
    it('shows one trigger with the current language, which opens a list of every language under its own name, each keeping the page, its query and its fragment', async () => {
        const { trigger, list, option } = renderOnSettings();

        expect(trigger).toHaveTextContent('ru');
        expect(trigger).toHaveAttribute('aria-expanded', 'false');
        expect(trigger).toHaveAttribute('aria-controls', list.id);
        expect(list).not.toBeVisible();

        await fireEvent.click(trigger);

        expect(trigger).toHaveAttribute('aria-expanded', 'true');
        expect(list).toBeVisible();
        // The default language is unprefixed, every other one prefixed; the query and the fragment stay.
        expect(option('ru')).toHaveAttribute('href', '/settings?tab=a#top');
        expect(option('zh')).toHaveAttribute('href', '/zh/settings?tab=a#top');
        expect(option('ru')).toHaveTextContent('ruРусский');
        expect(option('zh')).toHaveTextContent('zh中文');
        expect(option('en')).toHaveAttribute('hreflang', 'en');
        expect(option('en')).toHaveAttribute('lang', 'en');
        expect(option('ru')).toHaveAttribute('aria-current', 'true');
        expect(option('en')).not.toHaveAttribute('aria-current');
        for (const control of [trigger, option('ru')]) {
            expect(control).toHaveClass('min-h-11', 'touch-manipulation');
        }
    });

    it('moves inside the program on a click, closes the list, gives the focus back to the trigger, and refuses a press of the trigger until the move ends', async () => {
        let finish: () => void = () => undefined;
        vi.mocked(goto).mockImplementation(() => new Promise<void>((resolve) => (finish = resolve)));
        const { trigger, list, option } = renderOnSettings();
        await fireEvent.click(trigger);

        await fireEvent.click(option('en'));

        expect(goto).toHaveBeenCalledExactlyOnceWith('/en/settings?tab=a#top', {
            invalidateAll: true,
            keepFocus: true,
            noScroll: true
        });
        expect(list).not.toBeVisible();
        expect(document.activeElement).toBe(trigger);
        expect(trigger).toHaveAttribute('aria-busy', 'true');
        await fireEvent.click(trigger);
        expect(list).not.toBeVisible();

        finish();
        await vi.waitFor(() => expect(trigger).not.toHaveAttribute('aria-busy'));
        await fireEvent.click(trigger);
        expect(list).toBeVisible();
    });

    it('leaves a click with a modifier key to the browser, and closes on a press of the current language without moving', async () => {
        const { trigger, list, option } = renderOnSettings();
        await fireEvent.click(trigger);

        await fireEvent.click(option('zh'), { ctrlKey: true });
        expect(goto).not.toHaveBeenCalled();
        expect(list).toBeVisible();

        await fireEvent.click(option('ru'));
        expect(goto).not.toHaveBeenCalled();
        expect(list).not.toBeVisible();
        expect(document.activeElement).toBe(trigger);
    });

    it('opens to the first item with Down, moves with Down and Up round the list, and closes on Escape with the focus on the trigger', async () => {
        const { trigger, list, option } = renderOnSettings();
        trigger.focus();

        await fireEvent.keyDown(trigger, { key: 'ArrowDown' });
        expect(list).toBeVisible();
        expect(document.activeElement).toBe(option('ru'));
        await fireEvent.keyDown(option('ru'), { key: 'ArrowDown' });
        expect(document.activeElement).toBe(option('en'));
        await fireEvent.keyDown(option('en'), { key: 'ArrowUp' });
        await fireEvent.keyDown(option('ru'), { key: 'ArrowUp' });
        expect(document.activeElement).toBe(option('zh'));

        await fireEvent.keyDown(option('zh'), { key: 'Escape' });
        expect(list).not.toBeVisible();
        expect(document.activeElement).toBe(trigger);
    });

    it('closes when the focus leaves it with Tab, and on a click outside', async () => {
        const { trigger, list, option } = renderOnSettings();
        const outside = document.createElement('div');
        document.body.append(outside);

        await fireEvent.click(trigger);
        await fireEvent.focusOut(option('zh'), { relatedTarget: outside });
        expect(list).not.toBeVisible();

        await fireEvent.click(trigger);
        await fireEvent.focusOut(option('ru'), { relatedTarget: option('en') });
        expect(list).toBeVisible();
        await fireEvent.click(outside);
        expect(list).not.toBeVisible();
        outside.remove();
    });

    it('draws nothing while Russian is the only language that exists', () => {
        vi.mocked(getActiveLocales).mockReturnValueOnce(['ru']);

        const { container } = render(LanguageSwitcher, { props: { lang: 'ru', currentPath: '/settings' } });

        expect(container.children).toHaveLength(0);
    });
});
