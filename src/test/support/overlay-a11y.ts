import { fireEvent } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { settle } from './superform-answer';

/**
 * The keyboard contract every `role="dialog"` overlay owes, generated rather than
 * hand-written — the same reasoning as `describeTenantGuards`: six dialogs each had a
 * DIFFERENT subset of it, and a hand-written matrix is exactly where a subset comes from.
 *
 * What it proves, per dialog:
 *
 * - opening moves focus into the dialog, so a keyboard user is not left on the page behind;
 * - Tab and Shift+Tab cycle inside it and never escape into that page;
 * - the rest of the document is `inert` while it is open, and gets scrolling, its inert
 *   state and the focus back when the dialog goes away — including when it is destroyed
 *   while still open, which is what a route change does;
 * - Escape closes it, or, for a gate that must be answered, deliberately does not.
 *
 * jsdom implements none of `inert`, so the background probe gives its own sibling a plain
 * `inert` own property — enough for `setBackgroundInert`'s `'inert' in child` check to see
 * it, which is what the production code branches on.
 */
export interface IOverlayA11yOptions {
    /** Mounts the dialog already open. Returns testing-library's `unmount`. */
    open: () => { unmount: () => void };
    /** `data-test` of the dialog's root element — the box focus is trapped inside. */
    rootTestId: string;
    /**
     * Whether Escape dismisses it. `false` for a one-time initialization gate, which has no
     * dismiss affordance while unresolved.
     */
    dismissible: boolean;
    /**
     * How to ask whether the dialog is still showing. Defaults to "its root is in the
     * document", which is right for a dialog behind `{#if open}`. A dialog that stays
     * mounted so it can animate — the catalogue filter drawer slides — overrides this,
     * because its root never leaves the DOM.
     */
    stillShowing?: () => boolean;
}

function dialogRoot(rootTestId: string): HTMLElement {
    const root = document.body.querySelector<HTMLElement>(`[data-test="${rootTestId}"]`);
    if (!root) throw new Error(`No dialog with data-test="${rootTestId}" on <body> — is it in a Portal?`);
    return root;
}

/** Mirrors `FOCUSABLE_SELECTOR` in `$lib/components/ui/overlay-a11y.ts`, hidden inputs left out. */
const FOCUSABLE =
    'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusables(rootTestId: string): HTMLElement[] {
    return Array.from(dialogRoot(rootTestId).querySelectorAll<HTMLElement>(FOCUSABLE));
}

export function describeOverlayA11y(name: string, options: IOverlayA11yOptions): void {
    const { open, rootTestId, dismissible } = options;
    const stillShowing =
        options.stillShowing ?? (() => Boolean(document.body.querySelector(`[data-test="${rootTestId}"]`)));

    describe(`${name} — overlay accessibility`, () => {
        afterEach(() => {
            document.body.style.overflow = '';
        });

        it('moves focus onto the first control inside the dialog when it opens', async () => {
            open();
            await settle();

            expect(document.activeElement).toBe(focusables(rootTestId)[0]);
        });

        it('wraps Tab from the last control back to the first', async () => {
            open();
            await settle();
            const controls = focusables(rootTestId);
            controls[controls.length - 1].focus();

            await fireEvent.keyDown(window, { key: 'Tab' });

            expect(document.activeElement).toBe(controls[0]);
        });

        it('wraps Shift+Tab from the first control back to the last', async () => {
            open();
            await settle();
            const controls = focusables(rootTestId);
            controls[0].focus();

            await fireEvent.keyDown(window, { key: 'Tab', shiftKey: true });

            expect(document.activeElement).toBe(controls[controls.length - 1]);
        });

        it('makes the rest of the page inert and unscrollable, and gives all of it back on teardown', async () => {
            const opener = document.createElement('button');
            const sibling = document.createElement('div');
            Object.defineProperty(sibling, 'inert', { value: false, writable: true, configurable: true });
            document.body.append(opener, sibling);
            opener.focus();

            const { unmount } = open();
            await settle();
            expect((sibling as unknown as { inert: boolean }).inert).toBe(true);
            expect(document.body.style.overflow).toBe('hidden');

            // Unmount rather than closing: destroying a dialog that is still open is what a
            // route change does, and it used to leave the page locked until a reload.
            unmount();
            await settle();

            expect((sibling as unknown as { inert: boolean }).inert).toBe(false);
            expect(document.body.style.overflow).toBe('');
            expect(document.activeElement).toBe(opener);

            opener.remove();
            sibling.remove();
        });

        it(dismissible ? 'closes when Escape is pressed' : 'stays open when Escape is pressed', async () => {
            open();
            await settle();

            await fireEvent.keyDown(window, { key: 'Escape' });
            await settle();

            expect(stillShowing()).toBe(!dismissible);
        });
    });
}
