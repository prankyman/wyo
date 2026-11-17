import { goto, preloadData, pushState } from '$app/navigation';
import { page, resetPage, setPageUrl } from '$test/support/page-state.svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { stubHistoryBack } from '$test/support/navigation';
import { closeShallowDetails, openShallowLink } from './shallow-details';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});
vi.mock('$app/navigation', async () => (await import('$test/support/navigation')).navigationModule);

const OPTIONS = {
    param: 'item',
    pick: (data: Record<string, unknown>) => data.opened ?? undefined
} as const;

const ITEM = { id: 'item-1' };

/** A click on a link inside a list, as the browser would deliver it to the window in the capture phase. */
function press(href: string, init: MouseEventInit = {}, attributes: Record<string, string> = {}) {
    const anchor = document.createElement('a');
    anchor.setAttribute('href', href);
    for (const [name, value] of Object.entries(attributes)) anchor.setAttribute(name, value);
    const inner = document.createElement('span');
    anchor.append(inner);
    document.body.append(anchor);
    const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init });
    Object.defineProperty(event, 'target', { value: inner });
    return event;
}

const loaded = (data: Record<string, unknown>, status = 200) => ({ type: 'loaded' as const, status, data });

describe('openShallowLink', () => {
    beforeEach(() => {
        setPageUrl('/items?status=failed');
        vi.mocked(preloadData).mockResolvedValue(loaded({ opened: ITEM }));
    });

    afterEach(() => {
        document.body.innerHTML = '';
        resetPage();
        vi.clearAllMocks();
    });

    it('opens the record in a shallow history entry without leaving the page', async () => {
        const event = press('/items?status=failed&item=item-1');
        await openShallowLink(event, OPTIONS);

        expect(event.defaultPrevented).toBe(true);
        expect(preloadData).toHaveBeenCalledWith('/items?status=failed&item=item-1');
        expect(pushState).toHaveBeenCalledWith('/items?status=failed&item=item-1', {
            details: { param: 'item', id: 'item-1', value: ITEM }
        });
        expect(goto).not.toHaveBeenCalled();
    });

    it('follows the link as an ordinary navigation, without moving the page, when the record cannot be loaded', async () => {
        vi.mocked(preloadData).mockResolvedValue(loaded({ opened: null }));
        await openShallowLink(press('/items?item=gone'), OPTIONS);

        expect(pushState).not.toHaveBeenCalled();
        expect(goto).toHaveBeenCalledWith('/items?item=gone', { noScroll: true });
    });

    it('follows the link when the page data could not be loaded at all', async () => {
        vi.mocked(preloadData).mockResolvedValue({ type: 'redirect', location: '/sign-in' });
        await openShallowLink(press('/items?item=item-1'), OPTIONS);

        expect(pushState).not.toHaveBeenCalled();
        expect(goto).toHaveBeenCalledOnce();
    });

    it('follows the link when the load answered with an error status', async () => {
        vi.mocked(preloadData).mockResolvedValue(loaded({ opened: ITEM }, 500));
        await openShallowLink(press('/items?item=item-1'), OPTIONS);

        expect(pushState).not.toHaveBeenCalled();
        expect(goto).toHaveBeenCalledOnce();
    });

    it.each([
        ['a press with a modifier key', { ctrlKey: true }, {}],
        ['a press with the meta key', { metaKey: true }, {}],
        ['a press with shift', { shiftKey: true }, {}],
        ['a press with alt', { altKey: true }, {}],
        ['a middle click', { button: 1 }, {}],
        ['a link that opens another window', {}, { target: '_blank' }],
        ['a link that downloads', {}, { download: '' }]
    ])('leaves %s to the browser', async (_name, init, attributes) => {
        const event = press('/items?item=item-1', init, attributes);
        await openShallowLink(event, OPTIONS);

        expect(event.defaultPrevented).toBe(false);
        expect(preloadData).not.toHaveBeenCalled();
    });

    it.each([
        ['another page', '/other?item=item-1'],
        ['another site', 'https://example.com/items?item=item-1'],
        ['a link without the record parameter', '/items?status=failed']
    ])('leaves a link to %s alone', async (_name, href) => {
        const event = press(href);
        await openShallowLink(event, OPTIONS);

        expect(event.defaultPrevented).toBe(false);
        expect(preloadData).not.toHaveBeenCalled();
    });

    it('leaves a press that is not on a link, or was already handled, alone', async () => {
        const plain = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 });
        await openShallowLink(plain, OPTIONS);

        const handled = press('/items?item=item-1');
        handled.preventDefault();
        await openShallowLink(handled, OPTIONS);

        expect(preloadData).not.toHaveBeenCalled();
    });

    it('swallows a second press while the first record is still being opened', async () => {
        let finish: (value: ReturnType<typeof loaded>) => void = () => undefined;
        vi.mocked(preloadData).mockReturnValue(new Promise((resolve) => (finish = resolve)));

        const first = openShallowLink(press('/items?item=item-1'), OPTIONS);
        const second = press('/items?item=item-1');
        await openShallowLink(second, OPTIONS);
        finish(loaded({ opened: ITEM }));
        await first;

        expect(second.defaultPrevented).toBe(true);
        expect(preloadData).toHaveBeenCalledOnce();
        expect(pushState).toHaveBeenCalledOnce();
    });
});

describe('closeShallowDetails', () => {
    afterEach(() => {
        resetPage();
        vi.clearAllMocks();
    });

    it('goes back in the history when the record was opened by a press, so the list stays where it was', () => {
        const back = stubHistoryBack();
        page.state = { details: { param: 'item', id: 'item-1', value: ITEM } };

        closeShallowDetails('item', '/items?status=failed');

        expect(back).toHaveBeenCalledOnce();
        expect(goto).not.toHaveBeenCalled();
        back.mockRestore();
    });

    it.each([
        ['by a direct address', {}],
        ["by a press on another panel's row", { details: { param: 'edit', id: 'kg', value: {} } }]
    ])('replaces the address by the list without scrolling when the record was opened %s', (_name, state) => {
        const back = vi.spyOn(history, 'back');
        page.state = state;

        closeShallowDetails('item', '/items?status=failed');

        expect(back).not.toHaveBeenCalled();
        expect(goto).toHaveBeenCalledWith('/items?status=failed', {
            noScroll: true,
            keepFocus: true,
            replaceState: true
        });
        back.mockRestore();
    });
});
