import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import HelpTooltip from './HelpTooltip.svelte';

describe('HelpTooltip', () => {
    const defaultProps = {
        tooltip: 'This is a help tooltip',
        ariaLabel: 'Help tooltip'
    };

    describe('Rendering', () => {
        it('renders a ? button of type button carrying the aria-label', () => {
            render(HelpTooltip, { props: defaultProps });

            const button = screen.getByTestId('helptooltip-button');
            expect(button).toBeInTheDocument();
            expect(button).toHaveAttribute('type', 'button');
            expect(button).toHaveAttribute('aria-label', 'Help tooltip');
        });

        it('renders tooltip with role="tooltip" when visible, portaled outside the render container', async () => {
            const { container } = render(HelpTooltip, { props: { ...defaultProps, testId: 'test-help' } });
            const button = screen.getByTestId('test-help-button');
            await fireEvent.click(button);
            const tooltip = screen.getByTestId('test-help-tooltip');
            expect(tooltip).toHaveAttribute('role', 'tooltip');
            expect(container).not.toContainElement(tooltip);
        });
    });

    describe('A glyph of its own', () => {
        it('draws the given glyph in place of the «?» and still opens and closes by a click or a tap', async () => {
            const icon = createRawSnippet(() => ({ render: () => '<svg data-test="own-glyph"></svg>' }));
            const { container } = render(HelpTooltip, {
                props: { ...defaultProps, testId: 'own', icon, class: 'text-ok-600' }
            });
            const button = screen.getByTestId('own-button');

            expect(screen.getByTestId('own-glyph')).toBeInTheDocument();
            expect(button).toHaveClass('text-ok-600');
            // The «?» is gone: the button holds the given glyph alone.
            expect(button.querySelectorAll('svg')).toHaveLength(1);
            expect(container.querySelector('[data-test="own-tooltip"]')).toBeNull();

            await fireEvent.click(button);
            expect(screen.getByTestId('own-tooltip')).toHaveTextContent('This is a help tooltip');
            await fireEvent.click(button);
            expect(document.body.querySelector('[data-test="own-tooltip"]')).toBeNull();
        });
    });

    describe('Hover behavior', () => {
        it('does not open on hover, and a hover does not close an open one', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-hover' } });
            const button = screen.getByTestId('test-hover-button');
            const tooltip = () => document.body.querySelector('[data-test="test-hover-tooltip"]');

            await fireEvent.pointerEnter(button, { pointerType: 'mouse' });
            await fireEvent.mouseEnter(button);
            expect(tooltip()).not.toBeInTheDocument();

            await fireEvent.click(button);
            expect(tooltip()).toBeInTheDocument();

            await fireEvent.mouseLeave(button);
            expect(tooltip()).toBeInTheDocument();
        });

        it('stays open while the pointer moves on and off the popup itself', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-popup-hover' } });
            await fireEvent.click(screen.getByTestId('test-popup-hover-button'));
            const tooltip = screen.getByTestId('test-popup-hover-tooltip');

            await fireEvent.mouseEnter(tooltip);
            await fireEvent.mouseLeave(tooltip);
            expect(screen.getByTestId('test-popup-hover-tooltip')).toBeInTheDocument();
        });
    });

    describe('Click behavior (mobile/touch)', () => {
        it('toggles tooltip visibility on click', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-click' } });
            const button = screen.getByTestId('test-click-button');

            await fireEvent.click(button);
            expect(document.body.querySelector('[data-test="test-click-tooltip"]')).toBeInTheDocument();

            await fireEvent.click(button);
            expect(document.body.querySelector('[data-test="test-click-tooltip"]')).not.toBeInTheDocument();
        });

        // A tap sends the pointer events and then the mouse events a mouse would, `mouseenter` before
        // `click` (W3C Pointer Events, "Mapping for devices that do not support hover"). The first tap
        // has to show the text and the second has to hide it.
        it.each(['touch', 'pen'])('opens on the first %s tap and closes on the second', async (pointerType) => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-tap' } });
            const button = screen.getByTestId('test-tap-button');
            const tooltip = () => document.body.querySelector('[data-test="test-tap-tooltip"]');

            await fireEvent.pointerEnter(button, { pointerType });
            await fireEvent.pointerDown(button, { pointerType });
            await fireEvent.pointerUp(button, { pointerType });
            await fireEvent.mouseEnter(button);
            await fireEvent.click(button);
            expect(tooltip()).toBeInTheDocument();

            // The finger is still "over" the icon, so the second tap brings no mouseenter.
            await fireEvent.pointerDown(button, { pointerType });
            await fireEvent.pointerUp(button, { pointerType });
            await fireEvent.click(button);
            expect(tooltip()).not.toBeInTheDocument();
        });
    });

    describe('Closing', () => {
        const tooltipOf = (testId: string) => document.body.querySelector(`[data-test="${testId}-tooltip"]`);

        async function open(testId: string) {
            render(HelpTooltip, { props: { ...defaultProps, testId } });
            await fireEvent.click(screen.getByTestId(`${testId}-button`));
            expect(tooltipOf(testId)).toBeInTheDocument();
        }

        it('closes on Escape and puts the focus back on the ? button', async () => {
            await open('test-escape');

            await fireEvent.keyDown(document, { key: 'Escape' });

            expect(tooltipOf('test-escape')).not.toBeInTheDocument();
            expect(screen.getByTestId('test-escape-button')).toHaveFocus();
        });

        it('closes on a click outside the popup', async () => {
            await open('test-outside');

            await fireEvent.click(document.body);

            expect(tooltipOf('test-outside')).not.toBeInTheDocument();
        });

        it('stays open on a click inside the popup text', async () => {
            await open('test-inside');

            await fireEvent.click(screen.getByText('This is a help tooltip'));

            expect(tooltipOf('test-inside')).toBeInTheDocument();
        });

        it('closes on the cross in the popup corner and puts the focus back on the ? button', async () => {
            await open('test-cross');
            const cross = screen.getByTestId('test-cross-close');
            expect(cross).toHaveAttribute('aria-label');
            expect(screen.getByTestId('test-cross-tooltip')).toContainElement(cross);

            await fireEvent.click(cross);

            expect(tooltipOf('test-cross')).not.toBeInTheDocument();
            expect(screen.getByTestId('test-cross-button')).toHaveFocus();
        });

        it('opening a second ? closes the first', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'first' } });
            render(HelpTooltip, { props: { ...defaultProps, testId: 'second' } });

            await fireEvent.click(screen.getByTestId('first-button'));
            await fireEvent.click(screen.getByTestId('second-button'));

            expect(tooltipOf('first')).not.toBeInTheDocument();
            expect(tooltipOf('second')).toBeInTheDocument();
        });
    });

    describe('Custom styling', () => {
        it('applies custom class to button', () => {
            render(HelpTooltip, {
                props: { ...defaultProps, class: 'text-wait-500 hover:text-wait-600' }
            });
            const button = screen.getByTestId('helptooltip-button');
            expect(button).toHaveClass('text-wait-500', 'hover:text-wait-600');
        });

        it('applies custom tooltipClass', async () => {
            render(HelpTooltip, {
                props: {
                    ...defaultProps,
                    testId: 'test-class',
                    tooltipClass: 'bg-wait-500 text-wait-50'
                }
            });
            const button = screen.getByTestId('test-class-button');

            await fireEvent.click(button);
            const tooltip = document.body.querySelector('[data-test="test-class-tooltip"]');
            expect(tooltip).toHaveClass('bg-wait-500', 'text-wait-50');
        });

        it('uses default button styling when no class provided', () => {
            render(HelpTooltip, { props: defaultProps });
            const button = screen.getByTestId('helptooltip-button');
            expect(button).toHaveClass('text-ink-400', 'pressable:hover:text-ink-600');
            // A press dims the icon, so a tap on a phone visibly registered.
            expect(button).toHaveClass('pressable:active:opacity-60', 'touch-manipulation', 'focus-visible:outline-2');
        });
    });

    describe('Icon size', () => {
        it('uses default icon size', () => {
            const { container } = render(HelpTooltip, { props: defaultProps });
            const icon = container.querySelector('svg');
            expect(icon).toHaveAttribute('width', '0.875rem');
            expect(icon).toHaveAttribute('height', '0.875rem');
        });

        it('uses custom icon size', () => {
            const { container } = render(HelpTooltip, {
                props: { ...defaultProps, iconSize: '1rem' }
            });
            const icon = container.querySelector('svg');
            expect(icon).toHaveAttribute('width', '1rem');
            expect(icon).toHaveAttribute('height', '1rem');
        });
    });

    // The «?» is the size of its icon, with a 0.5 rem margin to the word before it. It was a
    // 44 × 44 px button, which made every label that held one 44 px tall. Nothing here may give it
    // a size, a padding or a negative margin: a bigger box would again make the label row taller than its text, and one
    // pulled back over its neighbours would take the taps of the field below the label. jsdom lays nothing out, so this
    // proves the markup that draws it; the heights are measured by `e2e/specs/form-box-alignment.spec.ts`.
    describe('Tap box', () => {
        const classesOf = (element: Element): string[] =>
            (element.getAttribute('class') ?? '').split(/\s+/).filter(Boolean);

        it('is a button the height of the 14 px icon, 0.5 rem of left padding from the word before it, with no size, vertical padding or margin', () => {
            const { container } = render(HelpTooltip, { props: defaultProps });
            const button = screen.getByTestId('helptooltip-button');

            // The 0.5 rem is the button's own left padding, so a press on the strip beside the icon opens the popup:
            // as a margin it was a strip that took the press and did nothing, or folded the section the «?» stands in.
            const classes = classesOf(button);
            expect(classes).toEqual(expect.arrayContaining(['pl-2', 'flex', 'items-center', 'justify-center']));
            expect(classes.filter((token) => /(^|:)(size|w|h|min-w|min-h)-/.test(token))).toEqual([]);
            expect(classes.filter((token) => /(^|:)-?m[lrtbxy]?-/.test(token))).toEqual([]);
            // Horizontal only, so a label row is as tall as its text.
            expect(classes.filter((token) => /(^|:)p(-|[tby]-)/.test(token))).toEqual([]);
            expect(classesOf(button.parentElement!).filter((token) => /(^|:)-?m[lrtbxy]?-/.test(token))).toEqual([]);
            expect(container.querySelector('svg')).toHaveAttribute('width', '0.875rem');
        });

        it('takes the left padding away when it stands flush, in a box of its own that sets the distance', () => {
            render(HelpTooltip, { props: { ...defaultProps, flush: true } });

            expect(classesOf(screen.getByTestId('helptooltip-button'))).not.toContain('pl-2');
        });

        it('sits in a wrapper that stays relative and inline-block, and marks itself as a help icon for the phone sweep', () => {
            render(HelpTooltip, { props: defaultProps });

            const wrapper = screen.getByTestId('helptooltip-button').parentElement as HTMLElement;
            expect(wrapper).toHaveClass('relative', 'inline-block');
            expect(wrapper).toHaveAttribute('data-help-icon');
            expect(wrapper.children).toHaveLength(1);
        });
    });

    describe('Tooltip position', () => {
        const rect = {
            top: 100,
            left: 50,
            right: 70,
            bottom: 120,
            width: 20,
            height: 20,
            x: 50,
            y: 100,
            toJSON: () => ({})
        } as DOMRect;

        beforeEach(() => {
            vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(rect);
        });

        afterEach(() => {
            vi.restoreAllMocks();
        });

        it('centers tooltip on the trigger by default (portaled, fixed position)', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-default' } });
            const button = screen.getByTestId('test-default-button');

            await fireEvent.click(button);
            const tooltip = document.body.querySelector('[data-test="test-default-tooltip"]');
            expect(tooltip).toHaveClass('fixed');
            expect(tooltip).toHaveStyle({
                bottom: `${window.innerHeight - rect.top + 4}px`,
                left: `${rect.left + rect.width / 2}px`,
                transform: 'translateX(-50%)'
            });
        });

        it('right-anchors tooltip to the trigger when position="left"', async () => {
            render(HelpTooltip, {
                props: { ...defaultProps, testId: 'test-left', position: 'left' }
            });
            const button = screen.getByTestId('test-left-button');

            await fireEvent.click(button);
            const tooltip = document.body.querySelector('[data-test="test-left-tooltip"]');
            expect(tooltip).toHaveStyle({
                bottom: `${window.innerHeight - rect.top + 4}px`,
                right: `${window.innerWidth - rect.right}px`
            });
        });
    });

    describe('Staying inside the viewport', () => {
        // jsdom lays nothing out, so the browser measurements this component reads are stubbed
        // (SKILL.md 10.1) and the assertions below check the placement DECISION — pushed inside
        // the screen, flipped under the icon, or left anchored — never a real pixel position.
        const ICON_SIZE = 14;
        const POPUP_WIDTH = 256;
        const POPUP_HEIGHT = 158;

        const box = (left: number, top: number, width: number, height: number) =>
            ({
                left,
                top,
                right: left + width,
                bottom: top + height,
                width,
                height,
                x: left,
                y: top,
                toJSON: () => ({})
            }) as DOMRect;

        const originalWidth = window.innerWidth;
        const originalHeight = window.innerHeight;

        function stubScreen(iconLeft: number, iconTop: number, screenWidth: number, screenHeight: number) {
            Object.defineProperty(window, 'innerWidth', { value: screenWidth, configurable: true, writable: true });
            Object.defineProperty(window, 'innerHeight', { value: screenHeight, configurable: true, writable: true });
            vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
                return this.getAttribute('role') === 'tooltip'
                    ? box(0, 0, POPUP_WIDTH, POPUP_HEIGHT)
                    : box(iconLeft, iconTop, ICON_SIZE, ICON_SIZE);
            });
        }

        async function openTooltip(testId: string) {
            render(HelpTooltip, { props: { ...defaultProps, testId } });
            await fireEvent.click(screen.getByTestId(`${testId}-button`));
            return screen.getByTestId(`${testId}-tooltip`) as HTMLElement;
        }

        afterEach(() => {
            vi.restoreAllMocks();
            Object.defineProperty(window, 'innerWidth', { value: originalWidth, configurable: true, writable: true });
            Object.defineProperty(window, 'innerHeight', {
                value: originalHeight,
                configurable: true,
                writable: true
            });
        });

        it('pushes the popup back on screen when the icon sits near the right edge of a phone', async () => {
            stubScreen(336, 300, 400, 800);

            const tooltip = await openTooltip('test-clamp');

            expect(tooltip.style.transform).toBe('');
            expect(parseFloat(tooltip.style.left)).toBeGreaterThanOrEqual(0);
            expect(parseFloat(tooltip.style.left) + POPUP_WIDTH).toBeLessThanOrEqual(window.innerWidth);
        });

        it('leaves the popup centred on its icon when the whole width already fits', async () => {
            stubScreen(500, 300, 1280, 800);

            const tooltip = await openTooltip('test-no-clamp');

            expect(tooltip.style.transform).toBe('translateX(-50%)');
        });

        it.each([
            { room: 'no room above the icon', iconTop: 60, anchored: 'top', absent: 'bottom' },
            { room: 'room above the icon', iconTop: 400, anchored: 'bottom', absent: 'top' }
        ])('anchors the popup by its $anchored edge when there is $room', async ({ iconTop, anchored, absent }) => {
            stubScreen(100, iconTop, 400, 800);

            const tooltip = await openTooltip(`test-flip-${anchored}`);

            expect(tooltip.style.getPropertyValue(anchored)).not.toBe('');
            expect(tooltip.style.getPropertyValue(absent)).toBe('');
        });

        it('keeps the popup on screen when it fits neither above nor below the icon', async () => {
            stubScreen(100, 60, 400, 200);

            const tooltip = await openTooltip('test-squeeze');

            expect(parseFloat(tooltip.style.top)).toBeGreaterThanOrEqual(0);
            expect(parseFloat(tooltip.style.top) + POPUP_HEIGHT).toBeLessThanOrEqual(window.innerHeight);
        });

        it('caps the popup at the screen height and lets it scroll, so a long text is never cut off', async () => {
            // At 14px the longest help text (the shop logo's, 1,196 characters in Russian) runs to
            // roughly 800px. Clamping the top edge on screen is not enough when the popup is taller
            // than the screen itself: its bottom simply ran off, with nothing to scroll.
            stubScreen(100, 60, 400, 200);

            const tooltip = await openTooltip('test-tall');

            expect(tooltip.getAttribute('style')).toContain('max-height: calc(100vh - 16px)');
            expect(tooltip.getAttribute('style')).toContain('overflow-y: auto');
        });

        it('fits a short text and lets a long one grow sideways to 500px, never past the screen', async () => {
            // A fixed 256px column made the longest texts a tall, narrow strip. Sized to its text and
            // capped at 500px — and at the screen minus its margins — a long help text grows wider
            // on a wide screen, and still fits a 320px phone.
            stubScreen(100, 300, 400, 800);

            const tooltip = await openTooltip('test-width');

            expect(tooltip.className).toContain('w-max');
            expect(tooltip.className).not.toContain('w-64');
            expect(tooltip.getAttribute('style')).toContain('max-width: min(500px, calc(100vw - 16px))');
        });
    });

    describe('Content', () => {
        // Help texts are several paragraphs long; a blank line in the message starts a paragraph.
        it('draws each blank-line separated part of a long text as its own paragraph, and a short text as one', async () => {
            const { unmount } = render(HelpTooltip, {
                props: {
                    ...defaultProps,
                    tooltip: 'First part.\n\nSecond part.\n\n\nThird part.',
                    testId: 'test-paragraphs'
                }
            });
            await fireEvent.click(screen.getByTestId('test-paragraphs-button'));

            const paragraphs = screen.getByTestId('test-paragraphs-tooltip').querySelectorAll('p');
            expect([...paragraphs].map((paragraph) => paragraph.textContent)).toEqual([
                'First part.',
                'Second part.',
                'Third part.'
            ]);
            unmount();

            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-one' } });
            await fireEvent.click(screen.getByTestId('test-one-button'));
            expect(screen.getByTestId('test-one-tooltip').querySelectorAll('p')).toHaveLength(1);
        });

        it('displays tooltip text', async () => {
            render(HelpTooltip, {
                props: { ...defaultProps, tooltip: 'Custom tooltip text' }
            });
            const button = screen.getByTestId('helptooltip-button');

            await fireEvent.click(button);
            expect(screen.getByText('Custom tooltip text')).toBeInTheDocument();
        });
    });

    describe('Accessibility', () => {
        it('exposes the button with an explicit type and an aria-label', () => {
            render(HelpTooltip, { props: defaultProps });

            const button = screen.getByTestId('helptooltip-button');
            expect(button).toHaveAttribute('type', 'button');
            expect(button).toHaveAttribute('aria-label');
        });

        it('tooltip has role="tooltip"', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-a11y' } });
            const button = screen.getByTestId('test-a11y-button');

            await fireEvent.click(button);
            const tooltip = screen.getByTestId('test-a11y-tooltip');
            expect(tooltip).toHaveAttribute('role', 'tooltip');
        });
    });
    // A «?» may carry links under its paragraphs; the list itself is tested in HelpLinkList.svelte.test.ts.
    describe('Links', () => {
        const LINKS = [
            { label: 'Первая статья', href: 'https://example.org/first-article' },
            { label: 'Вторая статья', href: 'https://example.org/second-article' }
        ];

        it('draws no link list when no links are given', async () => {
            render(HelpTooltip, { props: { ...defaultProps, testId: 'test-no-links' } });
            await fireEvent.click(screen.getByTestId('test-no-links-button'));

            expect(screen.queryByTestId('test-no-links-links')).not.toBeInTheDocument();
            expect(screen.getByTestId('test-no-links-tooltip').querySelectorAll('a')).toHaveLength(0);
        });

        it('draws the links inside the popup under its paragraphs, which stay as they were', async () => {
            render(HelpTooltip, {
                props: { ...defaultProps, tooltip: 'Первый.\n\nВторой.', links: LINKS, testId: 'test-links' }
            });
            await fireEvent.click(screen.getByTestId('test-links-button'));

            const tooltip = screen.getByTestId('test-links-tooltip');
            const list = screen.getByTestId('test-links-links');
            expect(tooltip).toContainElement(list);
            expect(screen.getAllByTestId('test-links-link')).toHaveLength(2);
            const paragraphs = [...tooltip.querySelectorAll('p')];
            expect(paragraphs.map((paragraph) => paragraph.textContent)).toEqual(['Первый.', 'Второй.']);
            expect(paragraphs[1].compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
        });

        it('still closes on its close cross when it carries links', async () => {
            render(HelpTooltip, { props: { ...defaultProps, links: LINKS, testId: 'test-links-close' } });
            await fireEvent.click(screen.getByTestId('test-links-close-button'));
            await fireEvent.click(screen.getByTestId('test-links-close-close'));

            expect(screen.queryByTestId('test-links-close-tooltip')).not.toBeInTheDocument();
        });
    });
});
