import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SlideOverPanel from './SlideOverPanel.svelte';

const bodySnippet = createRawSnippet(() => ({
    render: () => `<div data-test="panel-content">content</div>`
}));

describe('SlideOverPanel', () => {
    afterEach(() => {
        document.body.style.overflow = '';
    });

    it('renders nothing when closed', () => {
        render(SlideOverPanel, { props: { open: false, onClose: vi.fn(), title: 'Settings', children: bodySnippet } });
        expect(screen.queryByTestId('slide-over-panel-backdrop')).not.toBeInTheDocument();
    });

    // One render replacing four, all describing the open panel's layout contract.
    it('renders the open panel right-anchored, full-height, full-width on mobile and 480px from md up', () => {
        render(SlideOverPanel, { props: { open: true, onClose: vi.fn(), title: 'Settings', children: bodySnippet } });

        expect(screen.getByTestId('slide-over-panel-panel')).toBeInTheDocument();
        expect(screen.getByTestId('panel-content')).toBeInTheDocument();

        const panel = screen.getByTestId('slide-over-panel-panel');
        expect(panel).toHaveClass('w-full');
        expect(panel).toHaveClass('md:w-[480px]');
        expect(panel).toHaveStyle({ height: '100dvh' });
        expect(panel).not.toHaveClass('rounded-t-2xl');

        const backdrop = screen.getByTestId('slide-over-panel-backdrop');
        expect(backdrop).toHaveClass('justify-end');
        expect(backdrop).not.toHaveClass('items-end');
    });

    it('uses a custom dataTest prefix for the backdrop, panel and close button', () => {
        render(SlideOverPanel, {
            props: {
                open: true,
                onClose: vi.fn(),
                title: 'Settings',
                dataTest: 'support-settings',
                children: bodySnippet
            }
        });
        expect(screen.getByTestId('support-settings-backdrop')).toBeInTheDocument();
        expect(screen.getByTestId('support-settings-panel')).toBeInTheDocument();
        expect(screen.getByTestId('support-settings-close')).toBeInTheDocument();
    });

    it('renders the title as the dialog aria-label', () => {
        render(SlideOverPanel, {
            props: { open: true, onClose: vi.fn(), title: 'Manage topics', children: bodySnippet }
        });
        expect(screen.getByTestId('slide-over-panel-backdrop')).toHaveAttribute('aria-label', 'Manage topics');
    });

    it('closes from a 44 px close button that shows it was pressed', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, {
            props: { open: true, onClose, title: 'Settings', lang: 'ru', children: bodySnippet }
        });
        const close = screen.getByRole('button', { name: 'Закрыть' });
        expect(close).toBe(screen.getByTestId('slide-over-panel-close'));
        expect(close).toHaveClass('min-h-11', 'min-w-11', 'motion-safe:pressable:active:scale-[0.97]');
        await fireEvent.click(close);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('calls onClose when the backdrop (not the panel) is clicked', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { open: true, onClose, title: 'Settings', children: bodySnippet } });
        await fireEvent.click(screen.getByTestId('slide-over-panel-backdrop'));
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('does not call onClose when clicking inside the panel', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { open: true, onClose, title: 'Settings', children: bodySnippet } });
        await fireEvent.click(screen.getByTestId('slide-over-panel-panel'));
        expect(onClose).not.toHaveBeenCalled();
    });

    it('calls onClose when Escape is pressed while open', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { open: true, onClose, title: 'Settings', children: bodySnippet } });
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('does not call onClose on Escape when closed', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { open: false, onClose, title: 'Settings', children: bodySnippet } });
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).not.toHaveBeenCalled();
    });
});

describe('SlideOverPanel placement="right" (default)', () => {
    afterEach(() => {
        document.body.style.overflow = '';
    });

    it('renders no mobile grab handle', () => {
        const { container } = render(SlideOverPanel, {
            props: { open: true, onClose: vi.fn(), title: 'Settings', children: bodySnippet }
        });
        expect(container.querySelector('[data-test="slide-over-panel-grab-handle"]')).toBeNull();
    });
});

describe('SlideOverPanel placement="responsive"', () => {
    const responsiveProps = {
        open: true,
        onClose: vi.fn(),
        title: 'Size guide',
        placement: 'responsive' as const,
        children: bodySnippet
    };

    afterEach(() => {
        document.body.style.overflow = '';
    });

    it('anchors to the bottom edge below md and to the right edge from md upward', () => {
        render(SlideOverPanel, { props: responsiveProps });
        const backdrop = screen.getByTestId('slide-over-panel-backdrop');
        expect(backdrop).toHaveClass('items-end');
        expect(backdrop).toHaveClass('md:items-stretch');
        expect(backdrop).toHaveClass('md:justify-end');
    });

    it('is a rounded, height-capped sheet below md and a full-height panel from md upward', () => {
        render(SlideOverPanel, { props: responsiveProps });
        const panel = screen.getByTestId('slide-over-panel-panel');
        expect(panel).toHaveClass('rounded-t-2xl');
        expect(panel).toHaveClass('md:rounded-none');
        expect(panel).toHaveClass('max-h-[88dvh]');
        expect(panel).toHaveClass('md:max-h-none');
        expect(panel).toHaveClass('md:h-[100dvh]');
    });

    // One render replacing four, all describing the responsive/bottom-sheet contract.
    it('drops the inline height, shows a mobile-only grab handle, contains overscroll and outranks the header', () => {
        render(SlideOverPanel, { props: responsiveProps });

        expect(screen.getByTestId('slide-over-panel-panel').style.height).toBe('');

        const handle = screen.getByTestId('slide-over-panel-grab-handle');
        expect(handle).toHaveClass('md:hidden');
        expect(handle).toHaveAttribute('aria-hidden', 'true');

        const body = screen.getByTestId('slide-over-panel-body');
        expect(body).toHaveClass('overscroll-contain');
        expect(body).toHaveClass('pb-[env(safe-area-inset-bottom)]');

        // The site header is z-50, so the panel must sit above it.
        expect(screen.getByTestId('slide-over-panel-backdrop')).toHaveClass('z-[60]');
    });

    describe('the grab handle', () => {
        /** The panel drawn 600 px high (jsdom has no layout), and its handle. */
        function renderSheet() {
            const onClose = vi.fn();
            render(SlideOverPanel, { props: { ...responsiveProps, onClose } });
            Object.defineProperty(screen.getByTestId('slide-over-panel-panel'), 'offsetHeight', { value: 600 });
            return { onClose, handle: screen.getByTestId('slide-over-panel-grab-handle') };
        }

        it('is 48 px tall to touch', () => {
            expect(renderSheet().handle).toHaveClass('h-12', 'touch-none');
        });

        it('closes the sheet on a tap', async () => {
            const { onClose, handle } = renderSheet();
            await fireEvent.pointerDown(handle, { pointerId: 1, clientY: 100 });
            await fireEvent.pointerUp(handle, { pointerId: 1, clientY: 100 });
            expect(onClose).toHaveBeenCalledOnce();
        });

        it('pulls the sheet down with the finger and closes it past a quarter of its height', async () => {
            const { onClose, handle } = renderSheet();
            await fireEvent.pointerDown(handle, { pointerId: 1, clientY: 100 });
            await fireEvent.pointerMove(handle, { pointerId: 1, clientY: 260 });
            expect(screen.getByTestId('slide-over-panel-panel').style.transform).toBe('translateY(160px)');
            await fireEvent.pointerUp(handle, { pointerId: 1, clientY: 260 });
            expect(onClose).toHaveBeenCalledOnce();
        });

        it('springs back after a short drag, and a drag up does not lift the sheet', async () => {
            const { onClose, handle } = renderSheet();
            const panel = screen.getByTestId('slide-over-panel-panel');
            await fireEvent.pointerDown(handle, { pointerId: 1, clientY: 100 });
            await fireEvent.pointerMove(handle, { pointerId: 1, clientY: 40 });
            expect(panel.style.transform).toBe('');
            await fireEvent.pointerMove(handle, { pointerId: 1, clientY: 120 });
            expect(panel.style.transform).toBe('translateY(20px)');
            // Held long enough that 20 px is no flick.
            await new Promise((resolve) => setTimeout(resolve, 60));
            await fireEvent.pointerUp(handle, { pointerId: 1, clientY: 120 });
            expect(onClose).not.toHaveBeenCalled();
            expect(panel.style.transform).toBe('');
        });

        it('lets go of a drag the browser cancelled', async () => {
            const { onClose, handle } = renderSheet();
            const panel = screen.getByTestId('slide-over-panel-panel');
            await fireEvent.pointerDown(handle, { pointerId: 1, clientY: 100 });
            await fireEvent.pointerMove(handle, { pointerId: 1, clientY: 300 });
            await fireEvent.pointerCancel(handle, { pointerId: 1 });
            expect(panel.style.transform).toBe('');
            await fireEvent.pointerUp(handle, { pointerId: 1, clientY: 300 });
            expect(onClose).not.toHaveBeenCalled();
        });
    });

    it('honours a custom widthClass', () => {
        render(SlideOverPanel, { props: { ...responsiveProps, widthClass: 'md:w-[560px]' } });
        const panel = screen.getByTestId('slide-over-panel-panel');
        expect(panel).toHaveClass('md:w-[560px]');
        expect(panel).not.toHaveClass('md:w-[480px]');
    });

    it('keeps every dialog behaviour of the right-hand placement', async () => {
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { ...responsiveProps, onClose } });

        const backdrop = screen.getByTestId('slide-over-panel-backdrop');
        expect(backdrop).toHaveAttribute('aria-modal', 'true');
        expect(backdrop).toHaveAttribute('aria-label', 'Size guide');
        expect(document.body.style.overflow).toBe('hidden');

        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('renders without a transition when the visitor prefers reduced motion', () => {
        const originalMatchMedia = globalThis.matchMedia;
        globalThis.matchMedia = vi.fn((query: string) => ({
            matches: query.includes('prefers-reduced-motion'),
            media: query,
            addEventListener: vi.fn(),
            removeEventListener: vi.fn()
        })) as unknown as typeof window.matchMedia;

        render(SlideOverPanel, { props: responsiveProps });
        expect(screen.getByTestId('slide-over-panel-panel')).toBeInTheDocument();
        expect(screen.getByTestId('panel-content')).toBeInTheDocument();

        globalThis.matchMedia = originalMatchMedia;
    });

    it('still renders when the browser has no matchMedia at all', () => {
        const originalMatchMedia = globalThis.matchMedia;
        // @ts-expect-error deliberately removing the API to prove the guard holds
        delete globalThis.matchMedia;

        expect(() => render(SlideOverPanel, { props: responsiveProps })).not.toThrow();
        expect(screen.getByTestId('slide-over-panel-panel')).toBeInTheDocument();

        globalThis.matchMedia = originalMatchMedia;
    });
});

describe('SlideOverPanel — modal={false}', () => {
    const originalWidth = window.innerWidth;
    const props = {
        open: true,
        onClose: vi.fn(),
        title: 'Assistant',
        placement: 'responsive' as const,
        modal: false,
        children: bodySnippet
    };

    afterEach(() => {
        window.innerWidth = originalWidth;
        document.body.style.overflow = '';
        document.body.removeAttribute('inert');
    });

    it('from md up: no backdrop colour, the page is not inert or locked, a click outside does not close it', async () => {
        window.innerWidth = 1024;
        const onClose = vi.fn();
        const sibling = document.createElement('div');
        document.body.appendChild(sibling);
        render(SlideOverPanel, { props: { ...props, onClose } });

        const backdrop = screen.getByTestId('slide-over-panel-backdrop');
        expect(backdrop).toHaveClass('pointer-events-none');
        expect(backdrop).not.toHaveClass('bg-ink-900/20');
        expect(backdrop).toHaveAttribute('aria-modal', 'false');
        expect(screen.getByTestId('slide-over-panel-panel')).toHaveClass('pointer-events-auto');
        expect(sibling).not.toHaveAttribute('inert');
        expect(document.body.style.overflow).not.toBe('hidden');

        await fireEvent.click(backdrop);
        expect(onClose).not.toHaveBeenCalled();
        sibling.remove();
    });

    it('from md up: Escape still closes it', async () => {
        window.innerWidth = 1024;
        const onClose = vi.fn();
        render(SlideOverPanel, { props: { ...props, onClose } });
        await fireEvent.keyDown(window, { key: 'Escape' });
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('below md: the bottom sheet stays modal, with its backdrop and scroll lock', () => {
        window.innerWidth = 500;
        render(SlideOverPanel, { props });
        const backdrop = screen.getByTestId('slide-over-panel-backdrop');
        expect(backdrop).toHaveClass('bg-ink-900/20');
        expect(backdrop).toHaveAttribute('aria-modal', 'true');
        expect(document.body.style.overflow).toBe('hidden');
    });
});
