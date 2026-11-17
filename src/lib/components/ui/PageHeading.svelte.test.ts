import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import PageHeading from './PageHeading.svelte';

const HELP =
    'On this page you see the orders of the shop.\n\nAs the owner of the shop you need this to ship on time.\n\nScenarios\n\n1. Open an order.\n\n2. Pack it.';

const subline = createRawSnippet(() => ({
    render: () => `<span data-test="orders-subline">L1 · Main shop</span>`
}));
const actions = createRawSnippet(() => ({
    render: () => `<a href="/orders/new" data-test="orders-new-link">New order</a>`
}));

describe('PageHeading', () => {
    it('draws the title as the page’s h1 with its one «?» right beside it, and the «?» opens the page’s text in paragraphs with a close cross', async () => {
        const { container } = render(PageHeading, {
            props: {
                title: 'Orders',
                help: HELP,
                headingTestId: 'orders-heading',
                helpTestId: 'orders-help',
                tour: 'orders-title'
            }
        });

        const heading = screen.getByTestId('orders-heading');
        expect(heading.tagName).toBe('H1');
        expect(heading).toHaveClass('text-2xl');
        expect(heading).toHaveTextContent('Orders');
        expect(heading).toHaveAttribute('data-tour', 'orders-title');
        expect(heading.parentElement).toHaveClass('flex', 'items-center', 'gap-1');
        expect(heading.nextElementSibling).toContainElement(screen.getByTestId('orders-help-button'));
        expect(screen.getByTestId('orders-help-button')).toHaveAttribute('aria-label', 'Orders');
        expect(container.querySelectorAll('[data-help-icon]')).toHaveLength(1);
        expect(container.querySelector('a')).toBeNull();

        await fireEvent.click(screen.getByTestId('orders-help-button'));
        const tooltip = screen.getByTestId('orders-help-tooltip');
        expect(tooltip).toHaveTextContent('On this page you see the orders');
        expect(tooltip).toHaveTextContent('Scenarios');
        const paragraphs = Array.from(tooltip.querySelectorAll('p')).map((paragraph) => paragraph.textContent ?? '');
        expect(paragraphs).toHaveLength(5);
        expect(paragraphs.filter((text) => /^\d\. /.test(text))).toHaveLength(2);
        expect(screen.getByTestId('orders-help-close')).toBeInTheDocument();
    });

    it('draws the back link above, the buttons at the right, the grey line under the title, and the names it is given', () => {
        const { container } = render(PageHeading, {
            props: {
                title: 'Move stock',
                help: 'What the page does.',
                ariaLabel: 'About moving stock',
                headingTestId: 'stock-move-heading',
                helpTestId: 'stock-move-help',
                helpPosition: 'left',
                back: { href: '/stock', label: 'Stock', dataTest: 'stock-move-back', dataTour: 'stock-move-back' },
                rowTour: 'stock-move-hint',
                level: 'h2',
                dataTest: 'stock-move-head',
                class: 'mb-6',
                subline,
                actions
            }
        });

        const block = screen.getByTestId('stock-move-head');
        expect(block).toHaveClass('mb-6');
        const back = screen.getByTestId('stock-move-back');
        expect(back).toHaveAttribute('href', '/stock');
        expect(back).toHaveTextContent('← Stock');
        expect(back).toHaveAttribute('data-tour', 'stock-move-back');
        expect(block.firstElementChild).toBe(back);
        const heading = screen.getByTestId('stock-move-heading');
        expect(heading.tagName).toBe('H2');
        expect(heading.parentElement).toHaveAttribute('data-tour', 'stock-move-hint');
        expect(heading.parentElement?.parentElement).toHaveClass('mt-2');
        expect(screen.getByTestId('stock-move-help-button')).toHaveAttribute('aria-label', 'About moving stock');
        expect(screen.getByTestId('orders-new-link')).toBeInTheDocument();
        expect(screen.getByTestId('orders-subline').parentElement).toHaveClass('mt-1', 'text-sm', 'text-ink-500');
        expect(container.querySelectorAll('h1')).toHaveLength(0);
    });
});
