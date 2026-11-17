import { readFileSync } from 'node:fs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/**
 * The start-up bar in `src/app.html`: until the interface has started, the page looks ready but nothing on it reacts, and a press
 * on a link makes the browser load a new page by itself. The script runs the bar from the start and keeps it for a press. It is read out of the page template and run here, exactly as the browser
 * runs it; jsdom has read the page already, so the start-up bar is counted from the moment the script runs.
 */

function barScript(): string {
    const html = readFileSync('src/app.html', 'utf8');
    const script = /<script data-startup="page-change-bar">([\s\S]*?)<\/script>/.exec(html);
    if (!script) throw new Error('src/app.html has no start-up page-change bar');
    return script[1];
}

function release(): void {
    (window as unknown as { __startupPageChangeBar: { release(): void } }).__startupPageChangeBar.release();
}

const bar = (): HTMLElement | null => document.querySelector('[data-test="startup-page-change-bar"]');

/** Presses the element as a person would (the test setup stops the page load that jsdom cannot follow). */
function press(element: Element, init: MouseEventInit = {}): void {
    element.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init }));
}

function submit(form: HTMLFormElement): void {
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
}

/** Runs the script on a page that holds `html`. */
function load(html = ''): void {
    document.body.innerHTML = html;
    new Function(barScript())();
}

beforeEach(() => {
    vi.useFakeTimers();
});

afterEach(() => {
    release();
    bar()?.remove();
    document.body.innerHTML = '';
    vi.useRealTimers();
});

describe('start-up bar', () => {
    it('runs a 6 px bar along the top from 150 ms after the page was read until the interface has started', () => {
        load();
        vi.advanceTimersByTime(149);
        expect(bar()).toBeNull();
        vi.advanceTimersByTime(1);
        expect(bar()).not.toBeNull();
        expect(bar()?.getAttribute('role')).toBe('progressbar');
        expect(bar()?.style.height).toBe('6px');
        expect(bar()?.style.top).toBe('0px');

        release();
        expect(bar()).toBeNull();
    });

    it('shows nothing when the interface starts within 150 ms', () => {
        load();
        vi.advanceTimersByTime(100);
        release();
        vi.advanceTimersByTime(500);
        expect(bar()).toBeNull();
    });

    it('waits for the page to be read when the script runs while it is still loading', () => {
        Object.defineProperty(document, 'readyState', { configurable: true, get: () => 'loading' });
        try {
            load();
        } finally {
            Reflect.deleteProperty(document, 'readyState');
        }
        vi.advanceTimersByTime(500);
        expect(bar()).toBeNull();

        document.dispatchEvent(new Event('DOMContentLoaded'));
        vi.advanceTimersByTime(150);
        expect(bar()).not.toBeNull();
    });

    it('runs the bar along the lower edge of the pinned header', () => {
        document.body.innerHTML = '<header></header>';
        const header = document.querySelector('header') as HTMLElement;
        header.getBoundingClientRect = () => ({ bottom: 72.4 }) as DOMRect;
        new Function(barScript())();
        vi.advanceTimersByTime(150);
        expect(bar()?.style.top).toBe('72px');
    });
});

describe('the bar of a press before the interface has started', () => {
    it('keeps the bar after the interface has started when a link of this site was pressed', () => {
        load('<a href="/settings"><span>Настройки</span></a>');
        vi.advanceTimersByTime(150);
        press(document.querySelector('span') as Element);
        release();
        expect(bar()).not.toBeNull();
    });

    it('still draws the bar of a press made in the first 150 ms when the interface starts in between', () => {
        load('<a href="/settings">x</a>');
        press(document.querySelector('a') as Element);
        vi.advanceTimersByTime(100);
        release();
        vi.advanceTimersByTime(50);
        expect(bar()).not.toBeNull();
    });

    it.each([
        ['another site', '<a href="https://example.org/">x</a>', {}],
        ['a new window', '<a href="/settings" target="_blank">x</a>', {}],
        ['a download', '<a href="/file.pdf" download>x</a>', {}],
        ['a jump within this page', '<a href="#top">x</a>', {}],
        ['a press with Ctrl held', '<a href="/settings">x</a>', { ctrlKey: true }]
    ])('keeps no bar for %s', (_name, html, init) => {
        load(html);
        press(document.querySelector('a') as Element, init);
        vi.advanceTimersByTime(500);
        release();
        expect(bar()).toBeNull();
    });

    it('keeps no bar for a form: a form sent before the interface starts is stopped by the submit guard, nothing loads', () => {
        load('<form></form>');
        submit(document.querySelector('form') as HTMLFormElement);
        vi.advanceTimersByTime(500);
        release();
        expect(bar()).toBeNull();
    });

    it('watches no press once the interface has started', () => {
        load('<a href="/settings">x</a>');
        release();
        press(document.querySelector('a') as Element);
        vi.advanceTimersByTime(500);
        expect(bar()).toBeNull();
    });

    it('takes the bar away when the page comes back from the back/forward cache', () => {
        load('<a href="/settings">x</a>');
        press(document.querySelector('a') as Element);
        vi.advanceTimersByTime(150);
        expect(bar()).not.toBeNull();

        window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
        expect(bar()).toBeNull();
    });
});
