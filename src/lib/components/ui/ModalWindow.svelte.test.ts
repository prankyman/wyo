import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { describeOverlayA11y } from '$test/support/overlay-a11y';
import ModalWindow from './ModalWindow.svelte';

const controls = createRawSnippet(() => ({
    render: () =>
        `<div data-test="mw-content"><input data-test="mw-first" /><button type="button" data-test="mw-last">Go</button></div>`
}));
const footer = createRawSnippet(() => ({
    render: () => `<button type="button" data-test="mw-footer-button">Yes</button>`
}));
const help = createRawSnippet(() => ({ render: () => `<span data-test="mw-help">?</span>` }));

const dialog = () => document.body.querySelector<HTMLElement>('[role="dialog"]');
const panel = () => screen.getByTestId('mw-panel');

describe('ModalWindow', () => {
    afterEach(() => {
        document.body.style.overflow = '';
    });

    it('renders nothing while closed, and takes the window down again when `open` goes back to false', async () => {
        const { rerender } = render(ModalWindow, { props: { open: false, title: 'Cancel?', children: controls } });
        expect(dialog()).toBeNull();

        await rerender({ open: true });
        expect(dialog()).not.toBeNull();

        await rerender({ open: false });
        expect(dialog()).toBeNull();
    });

    it('is a labelled modal dialog in a portal on <body>: the title is its heading, the content sits in a centred white card of the default width', () => {
        const { container } = render(ModalWindow, {
            props: {
                open: true,
                title: 'Cancel the order?',
                dataTest: 'mw',
                panelTestId: 'mw-panel',
                tour: 'mw-tour',
                children: controls
            }
        });

        const root = dialog()!;
        expect(container.contains(root)).toBe(false);
        expect(root.parentElement).toHaveAttribute('data-test', 'portal-root');
        expect(root).toHaveAttribute('aria-modal', 'true');
        expect(root).toHaveAttribute('tabindex', '-1');
        expect(root).toHaveAttribute('data-test', 'mw');
        expect(root).toHaveClass('fixed', 'inset-0', 'z-50', 'bg-black/50', 'items-center', 'justify-center');
        const heading = root.querySelector('h2')!;
        expect(heading).toHaveTextContent('Cancel the order?');
        expect(heading).toHaveClass('font-display', 'text-lg', 'text-ink-900');
        expect(root).toHaveAttribute('aria-labelledby', heading.id);
        expect(root).not.toHaveAttribute('aria-label');
        expect(panel()).toHaveClass('max-w-md', 'mx-4', 'rounded', 'bg-white', 'shadow-lg', 'max-h-[90vh]');
        expect(panel()).toHaveAttribute('data-tour', 'mw-tour');
        expect(panel().children).toHaveLength(2);
        expect(panel().children[0]).toHaveClass('px-6', 'py-3.75');
        expect(screen.getByTestId('mw-content').parentElement).toHaveClass('overflow-y-auto', 'px-6', 'pt-1', 'pb-6');
        expect(root.querySelector('button[aria-label]')).toBeNull();
    });

    it.each([
        ['md', 'max-w-md'],
        ['lg', 'max-w-lg'],
        ['xl', 'max-w-xl'],
        ['2xl', 'max-w-2xl']
    ] as const)('draws the %s card at its own greatest width', (size, widthClass) => {
        render(ModalWindow, { props: { open: true, size, panelTestId: 'mw-panel', children: controls } });

        expect(panel()).toHaveClass(widthClass);
    });

    it.each([
        ['base', 'z-40'],
        ['top', 'z-50'],
        ['bar', 'z-[60]'],
        ['screen', 'z-[70]']
    ] as const)('sits on the %s layer', (layer, zClass) => {
        render(ModalWindow, { props: { open: true, layer, children: controls } });

        expect(dialog()).toHaveClass(zClass);
    });

    it('is a bottom sheet on a phone that becomes the centred card from sm: up when placed as a sheet', () => {
        render(ModalWindow, { props: { open: true, placement: 'sheet', panelTestId: 'mw-panel', children: controls } });

        expect(dialog()).toHaveClass('items-end', 'sm:items-center', 'justify-center');
        expect(panel()).toHaveClass('rounded-t', 'sm:rounded', 'sm:mx-4');
        expect(panel()).not.toHaveClass('mx-4');
    });

    it('draws the heading row with the «?» after the title, a ✕ named and tested after the window, and the footer in a row of its own', async () => {
        const onClose = vi.fn();
        render(ModalWindow, {
            props: {
                open: true,
                title: 'Print',
                heading: help,
                closeLabel: 'Close the window',
                dataTest: 'mw',
                panelTestId: 'mw-panel',
                onClose,
                footer,
                children: controls
            }
        });

        const headingRow = panel().children[0];
        expect(headingRow.querySelector('h2')).toHaveTextContent('Print');
        expect(headingRow.querySelector('h2')!.nextElementSibling).toBe(screen.getByTestId('mw-help'));
        const close = screen.getByTestId('mw-close');
        expect(headingRow).toContainElement(close);
        expect(close).toHaveAttribute('aria-label', 'Close the window');
        expect(panel().children).toHaveLength(3);
        expect(panel().children[2]).toContainElement(screen.getByTestId('mw-footer-button'));
        expect(panel().children[2]).toHaveClass('px-6', 'py-3.75');
        expect(headingRow).toHaveClass('px-6', 'py-3.75');
        expect(screen.getByTestId('mw-content').parentElement).toHaveClass('pb-1');

        await fireEvent.click(close);
        expect(onClose).toHaveBeenCalledTimes(1);
        expect(dialog()).toBeNull();
    });

    it('draws no empty content band when the title asks the question and the footer holds the answers', () => {
        render(ModalWindow, {
            props: { open: true, title: 'Нажать «Упаковка закончена»?', panelTestId: 'mw-panel', footer }
        });

        expect(panel().children).toHaveLength(2);
        expect(panel().children[0].querySelector('h2')).toHaveTextContent('Нажать «Упаковка закончена»?');
        expect(panel().children[1]).toContainElement(screen.getByTestId('mw-footer-button'));
    });

    it('with modal={false} leaves the page usable: no backdrop colour, no inert page, no scroll lock, no outside-click close, no Tab trap; Escape still closes', async () => {
        const onClose = vi.fn();
        const sibling = document.createElement('div');
        document.body.appendChild(sibling);
        render(ModalWindow, {
            props: {
                open: true,
                title: 'Side',
                dataTest: 'mw',
                modal: false,
                closeOnBackdrop: true,
                onClose,
                children: controls
            }
        });
        const root = screen.getByTestId('mw');
        expect(root).toHaveAttribute('aria-modal', 'false');
        expect(root).toHaveClass('pointer-events-none');
        expect(root).not.toHaveClass('bg-black/50');
        expect(sibling).not.toHaveAttribute('inert');
        expect(document.body.style.overflow).not.toBe('hidden');
        await fireEvent.click(root);
        expect(onClose).not.toHaveBeenCalled();
        const last = screen.getByTestId('mw-last');
        last.focus();
        const tab = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
        window.dispatchEvent(tab);
        expect(tab.defaultPrevented).toBe(false);
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);
        sibling.remove();
    });

    it('names the ✕ as it is told and draws no title when there is none', () => {
        render(ModalWindow, {
            props: { open: true, closeLabel: 'Close', closeTestId: 'camera-close', children: controls }
        });

        expect(screen.getByTestId('camera-close')).toBeInTheDocument();
        expect(dialog()!.querySelector('h2')).toBeNull();
    });

    it('takes an accessible name instead of a heading, and the light backdrop, for a window that draws its own panel', () => {
        render(ModalWindow, {
            props: {
                open: true,
                ariaLabel: 'Settings',
                frame: 'none',
                backdrop: 'light',
                class: 'justify-end',
                dataTest: 'mw',
                children: controls
            }
        });

        const root = dialog()!;
        expect(root).toHaveAttribute('aria-label', 'Settings');
        expect(root).not.toHaveAttribute('aria-labelledby');
        expect(root).toHaveClass('bg-ink-900/20', 'justify-end');
        expect(root).not.toHaveClass('bg-black/50', 'items-center');
        // No card: the children are the panel.
        expect(screen.getByTestId('mw-content').parentElement).toBe(root);
        expect(root.querySelector('h2')).toBeNull();
    });

    it('is the one black full-screen window for the camera picture: no card, no footer, the ✕ in white', () => {
        render(ModalWindow, {
            props: {
                open: true,
                size: 'full',
                layer: 'screen',
                title: 'Camera',
                closeLabel: 'Close the camera',
                dataTest: 'mw',
                panelTestId: 'mw-panel',
                footer,
                children: controls
            }
        });

        expect(dialog()).toHaveClass('z-[70]');
        expect(dialog()).not.toHaveClass('items-center');
        expect(panel()).toHaveClass('h-full', 'w-full', 'bg-black', 'text-white');
        expect(panel()).not.toHaveClass('rounded', 'shadow-lg', 'max-w-md');
        expect(panel().children).toHaveLength(2);
        expect(screen.queryByTestId('mw-footer-button')).toBeNull();
        expect(screen.getByTestId('mw-content').parentElement).toHaveClass('flex', 'flex-col');
        expect(screen.getByTestId('mw-content').parentElement).not.toHaveClass('px-6');
        expect(screen.getByTestId('mw-close')).toHaveClass('text-white');
    });

    it('calls onClose and closes on Escape, and stays open when Escape is switched off or the window may not close yet', async () => {
        const onClose = vi.fn();
        const { rerender } = render(ModalWindow, {
            props: { open: true, closeOnEscape: false, onClose, children: controls }
        });

        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).not.toHaveBeenCalled();
        expect(dialog()).not.toBeNull();

        await rerender({ closeOnEscape: true, canClose: () => false });
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).not.toHaveBeenCalled();
        expect(dialog()).not.toBeNull();

        await rerender({ canClose: () => true });
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);
        expect(dialog()).toBeNull();
    });

    it('closes on a click on the backdrop only when asked to, never on a click inside the card, and not while it may not close', async () => {
        const onClose = vi.fn();
        const { rerender } = render(ModalWindow, {
            props: { open: true, onClose, panelTestId: 'mw-panel', children: controls }
        });

        await fireEvent.click(dialog()!);
        expect(onClose).not.toHaveBeenCalled();

        await rerender({ closeOnBackdrop: true, canClose: () => false });
        await fireEvent.click(dialog()!);
        expect(onClose).not.toHaveBeenCalled();

        await rerender({ canClose: () => true });
        await fireEvent.click(panel());
        expect(onClose).not.toHaveBeenCalled();
        expect(dialog()).not.toBeNull();

        await fireEvent.click(dialog()!);
        expect(onClose).toHaveBeenCalledTimes(1);
        expect(dialog()).toBeNull();
    });

    it('does not keep the ✕ working while the window may not close', async () => {
        const onClose = vi.fn();
        render(ModalWindow, {
            props: {
                open: true,
                closeLabel: 'Close',
                dataTest: 'mw',
                canClose: () => false,
                onClose,
                children: controls
            }
        });

        await fireEvent.click(screen.getByTestId('mw-close'));
        expect(onClose).not.toHaveBeenCalled();
        expect(dialog()).not.toBeNull();
    });

    it('leaves the focus where it is when it goes away without restoring it (a gate whose answer re-reads the page)', async () => {
        const opener = document.createElement('button');
        document.body.append(opener);
        opener.focus();
        const { unmount } = render(ModalWindow, { props: { open: true, restoreFocus: false, children: controls } });
        await new Promise((resolve) => setTimeout(resolve, 0));
        expect(document.activeElement).toBe(screen.getByTestId('mw-first'));

        unmount();
        await new Promise((resolve) => setTimeout(resolve, 0));
        expect(document.activeElement).not.toBe(opener);
        expect(document.body.style.overflow).toBe('');
        opener.remove();
    });
});

describeOverlayA11y('ModalWindow', {
    open: () => render(ModalWindow, { props: { open: true, title: 'Cancel?', dataTest: 'mw', children: controls } }),
    rootTestId: 'mw',
    dismissible: true
});

describeOverlayA11y('ModalWindow without Escape', {
    open: () =>
        render(ModalWindow, {
            props: { open: true, title: 'Choose', dataTest: 'mw', closeOnEscape: false, children: controls }
        }),
    rootTestId: 'mw',
    dismissible: false
});
