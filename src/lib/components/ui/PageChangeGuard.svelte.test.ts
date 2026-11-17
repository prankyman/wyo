import fs from 'node:fs';
import path from 'node:path';
import { render, screen } from '@testing-library/svelte';
import { createRawSnippet, tick } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetPage, setNavigating } from '$test/support/page-state.svelte';
import { pageChange } from './page-change.svelte';
import PageChangeGuard from './PageChangeGuard.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

const children = createRawSnippet(() => ({ render: () => '<a href="/orders" data-test="page-link">Orders</a>' }));

const content = () => screen.getByTestId('page-content');
/**
 * jsdom implements no `inert`, so Svelte's `inert={…}` lands as a plain property there; a browser
 * reflects that same property to the attribute (checked in Chromium and WebKit).
 */
const inert = () => (content() as unknown as { inert?: boolean }).inert === true;
const bar = () => screen.queryByTestId('page-change-bar');

/** Moves the clock and lets Svelte apply what the timers changed. */
async function wait(ms: number): Promise<void> {
    vi.advanceTimersByTime(ms);
    await tick();
}

beforeEach(() => vi.useFakeTimers());

afterEach(() => {
    vi.useRealTimers();
    pageChange.nativeSubmission = false;
    pageChange.lockExpired = false;
    resetPage();
});

describe('PageChangeGuard', () => {
    it('locks the page the moment a navigation starts, shows the bar only after 150 ms, and unlocks when it ends', async () => {
        render(PageChangeGuard, { lang: 'ru', children });
        expect(inert()).toBe(false);
        expect(content()).not.toHaveAttribute('aria-busy');

        setNavigating('/orders');
        await tick();
        expect(inert()).toBe(true);
        expect(content()).toHaveAttribute('aria-busy', 'true');
        expect(bar()).toBeNull();

        await wait(149);
        expect(bar()).toBeNull();
        await wait(1);
        expect(screen.getByRole('progressbar', { name: 'Открываем страницу…' })).toBe(bar());

        setNavigating(null);
        await tick();
        expect(inert()).toBe(false);
        expect(bar()).toBeNull();
    });

    it.each([
        ['link', true],
        ['form', true],
        // Code navigating — a search box refreshing its list on every keystroke, a page's own
        // redirect — must leave the page usable: the person is still typing into it.
        ['goto', false],
        ['popstate', false]
    ] as const)('locks the page and runs the bar for a %s navigation: %s', async (type, locks) => {
        render(PageChangeGuard, { lang: 'ru', children });

        setNavigating('/items?q=ab', type);
        await tick();
        await wait(150);

        expect(inert()).toBe(locks);
        expect(content().getAttribute('aria-busy')).toBe(locks ? 'true' : null);
        expect(bar() !== null).toBe(locks);
    });

    it('runs a 6 px bar in the brand colour along the lower edge of the pinned header, the window’s top without one', async () => {
        render(PageChangeGuard, { lang: 'ru', children });
        setNavigating('/orders');
        await tick();
        await wait(150);

        // jsdom applies no stylesheet: the classes say where Tailwind places it — `--header-height` is the header's drawn height.
        expect(bar()).toHaveClass('fixed', 'top-[var(--header-height,0px)]', 'h-1.5', 'bg-brand-100');
        expect(bar()?.firstElementChild).toHaveClass('bg-brand-500');
    });

    it('stops the start-up bar from watching presses once the app has started', () => {
        const release = vi.fn();
        window.__startupPageChangeBar = { release };
        try {
            render(PageChangeGuard, { lang: 'ru', children });
            expect(release).toHaveBeenCalledOnce();
        } finally {
            delete window.__startupPageChangeBar;
        }
    });

    it('never shows the bar for a navigation that ends within 150 ms', async () => {
        render(PageChangeGuard, { lang: 'ru', children });

        setNavigating('/orders');
        await tick();
        await wait(100);
        setNavigating(null);
        await wait(200);

        expect(bar()).toBeNull();
        expect(inert()).toBe(false);
    });

    it('gives the page back after 10 s of a navigation that does not end, while the bar keeps running', async () => {
        render(PageChangeGuard, { lang: 'ru', children });

        setNavigating('/orders');
        await tick();
        await wait(9_999);
        expect(inert()).toBe(true);
        await wait(1);

        expect(inert()).toBe(false);
        expect(bar()).not.toBeNull();
    });

    it('tells the links that the change is dropped after 10 s, clears that when it ends, and counts a newer change afresh', async () => {
        render(PageChangeGuard, { lang: 'ru', children });

        setNavigating('/items/item-1');
        await tick();
        await wait(9_999);
        expect(pageChange.lockExpired).toBe(false);
        await wait(1);
        expect(pageChange.lockExpired).toBe(true);

        // A press on the same dead link starts a new change to the same address: its lock counts from now.
        setNavigating('/items/item-1');
        await tick();
        expect(pageChange.lockExpired).toBe(false);
        expect(inert()).toBe(true);
        await wait(10_000);
        expect(pageChange.lockExpired).toBe(true);

        setNavigating(null);
        await tick();
        expect(pageChange.lockExpired).toBe(false);
    });

    it('locks the page while a plain form loads its answer, and unlocks when the page comes back from the cache', async () => {
        render(PageChangeGuard, { lang: 'ru', children });

        pageChange.nativeSubmission = true;
        await tick();
        expect(inert()).toBe(true);

        const pageshow = new Event('pageshow');
        Object.defineProperty(pageshow, 'persisted', { value: true });
        window.dispatchEvent(pageshow);
        await tick();

        expect(pageChange.nativeSubmission).toBe(false);
        expect(inert()).toBe(false);
    });
});

describe('start-up submission guard (the inline script in src/app.html)', () => {
    /** Runs the script in this document, as the browser does while the page loads and before the app starts. */
    function runStartupScript(): void {
        const html = fs.readFileSync(path.join(process.cwd(), 'src/app.html'), 'utf8');
        const code = html.match(/<script>([\s\S]*?)<\/script>/)?.[1];
        expect(code, 'src/app.html carries the start-up submission guard').toBeDefined();
        new Function(code!)();
    }

    /** A form with a submit button, and how many submissions reached it. */
    function form(attributes: Record<string, string> = {}): { submit: () => void; received: () => number } {
        const element = document.createElement('form');
        element.method = 'post';
        for (const [name, value] of Object.entries(attributes)) element.setAttribute(name, value);
        const button = document.createElement('button');
        button.type = 'submit';
        element.append(button);
        document.body.append(element);
        let count = 0;
        element.addEventListener('submit', () => (count += 1));
        return { submit: () => element.requestSubmit(button), received: () => count };
    }

    afterEach(() => {
        window.__startupSubmitGuard?.release();
        delete window.__startupSubmitGuard;
        document.body.replaceChildren();
    });

    it('before the interface starts, stops every submission, the first one too and the one into another window too', () => {
        runStartupScript();
        const first = form();
        const other = form();
        const download = form({ target: '_blank' });

        first.submit();
        first.submit();
        other.submit();
        download.submit();

        expect(first.received()).toBe(0);
        expect(other.received()).toBe(0);
        expect(download.received()).toBe(0);
    });

    it.each([1, 0])(
        'hands over to the interface when the page-change guard mounts: after %i stopped submission(s) the page is not locked and the next submission goes through',
        async (early) => {
            runStartupScript();
            const plain = form();
            for (let i = 0; i < early; i += 1) plain.submit();

            render(PageChangeGuard, { lang: 'ru', children });
            await tick();

            // A stopped submission was not posted: nothing is loading, so the page is not replaced.
            expect(pageChange.nativeSubmission).toBe(false);
            expect(inert()).toBe(false);
            // The start-up guard is gone: from here on the interface's own guards decide.
            plain.submit();
            expect(plain.received()).toBe(1);
        }
    );
});
