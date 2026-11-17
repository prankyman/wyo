import { beforeEach, describe, expect, it, vi } from 'vitest';
import { focusableElements, isBackdropClick, lockBodyScroll, setBackgroundInert, trapTabKey } from './overlay-a11y';

function mount(html: string): HTMLElement {
    document.body.innerHTML = html;
    return document.body.firstElementChild as HTMLElement;
}

beforeEach(() => {
    document.body.innerHTML = '';
    document.body.style.overflow = '';
});

describe('focusableElements', () => {
    it('collects focusable descendants in DOM order and skips disabled, hidden inputs and tabindex="-1"', () => {
        const root = mount(`
            <div>
                <input type="hidden" name="formId" value="k-1" />
                <a href="/a">a</a>
                <button disabled>skipped</button>
                <input />
                <div tabindex="-1">skipped</div>
                <button>b</button>
                <input type="hidden" name="id" value="row-1" />
            </div>
        `);

        expect(focusableElements(root).map((el) => el.tagName)).toEqual(['A', 'INPUT', 'BUTTON']);
    });

    it('returns an empty list rather than throwing when there is no root', () => {
        expect(focusableElements(undefined)).toEqual([]);
    });
});

describe('trapTabKey', () => {
    it('wraps focus from last to first and back again, leaving other keys alone', () => {
        const root = mount('<div><a href="/1">1</a><a href="/2">2</a></div>');
        const [first, last] = focusableElements(root);

        last.focus();
        const forward = new KeyboardEvent('keydown', { key: 'Tab' });
        const preventForward = vi.spyOn(forward, 'preventDefault');
        expect(trapTabKey(forward, root)).toBe(true);
        expect(preventForward).toHaveBeenCalled();
        expect(document.activeElement).toBe(first);

        const backward = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true });
        expect(trapTabKey(backward, root)).toBe(true);
        expect(document.activeElement).toBe(last);

        expect(trapTabKey(new KeyboardEvent('keydown', { key: 'Escape' }), root)).toBe(false);
    });
});

describe('setBackgroundInert', () => {
    it('inerts siblings but never the overlay itself, and lifts it again', () => {
        document.body.innerHTML = '<main id="page"></main><div id="overlay"></div>';
        const overlay = document.getElementById('overlay') as HTMLElement;
        const page = document.getElementById('page') as HTMLElement & { inert: boolean };

        // jsdom does not implement `inert`, and the helper deliberately skips elements that
        // lack it. Seeding the property is what puts the real branch under test rather than
        // the no-op guard.
        page.inert = false;
        (overlay as HTMLElement & { inert: boolean }).inert = false;

        setBackgroundInert(overlay, true);
        expect(page.inert).toBe(true);
        expect((overlay as HTMLElement & { inert: boolean }).inert).toBe(false);

        setBackgroundInert(overlay, false);
        expect(page.inert).toBe(false);
    });
});

describe('isBackdropClick', () => {
    it('is true only when the click landed on the backdrop rather than the panel', () => {
        const backdrop = mount('<div><span>panel</span></div>');
        const panel = backdrop.firstElementChild as HTMLElement;

        expect(isBackdropClick({ target: backdrop, currentTarget: backdrop } as unknown as MouseEvent)).toBe(true);
        expect(isBackdropClick({ target: panel, currentTarget: backdrop } as unknown as MouseEvent)).toBe(false);
    });
});

describe('lockBodyScroll', () => {
    it('restores the exact previous overflow, not a hard-coded empty string', () => {
        document.body.style.overflow = 'scroll';

        const restore = lockBodyScroll();
        expect(document.body.style.overflow).toBe('hidden');

        restore();
        expect(document.body.style.overflow).toBe('scroll');
    });
});
