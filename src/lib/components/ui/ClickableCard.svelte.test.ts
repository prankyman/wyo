import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import RowHarness from '$test/harnesses/ui/ClickableRowTestHarness.svelte';
import { resetPage, setNavigating } from '$test/support/page-state.svelte';
import ClickableCard from './ClickableCard.svelte';
import { pageChange } from './page-change.svelte';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

const title = createRawSnippet(() => ({ render: () => '<span>№ 12</span>' }));
const body = createRawSnippet(() => ({
    render: () =>
        '<div><span data-test="card-status">Reserved</span><button data-test="card-action">Release</button></div>'
}));

const renderCard = (props: Record<string, unknown> = {}) =>
    render(ClickableCard, {
        props: {
            as: 'li',
            href: '/items/item-1',
            dataTest: 'order-card',
            linkDataTest: 'order-link',
            title,
            children: body,
            'data-row-id': 'o-1',
            ...props
        } as never
    });

afterEach(() => {
    pageChange.lockExpired = false;
    resetPage();
});

describe('ClickableCard', () => {
    it('is one card whose single link — the record — covers it, with its own controls kept separate', () => {
        renderCard();

        const card = screen.getByTestId('order-card');
        expect(card.tagName).toBe('LI');
        expect(card).toHaveAttribute('data-row-id', 'o-1');
        // The stretched area and the lifted controls come from `.clickable-surface` in app.css;
        // the pressed and "opening" looks are these utilities (jsdom applies no CSS).
        expect(card).toHaveClass(
            'clickable-surface',
            'motion-safe:pressable:has-[[data-stretched-link]:active]:scale-[0.99]',
            'pressable:has-[[data-stretched-link]:active]:bg-ink-100',
            'pressable:hover:border-ink-400',
            'touch-manipulation'
        );

        const link = screen.getByRole('link', { name: '№ 12' });
        expect(link).toBe(screen.getByTestId('order-link'));
        expect(link).toHaveAttribute('href', '/items/item-1');
        expect(link).toHaveAttribute('data-stretched-link');
        expect(screen.getAllByRole('link')).toHaveLength(1);
        // The card's own button is not inside the link, so pressing it opens nothing.
        expect(link).not.toContainElement(screen.getByTestId('card-action'));
        expect(screen.getByTestId('card-status')).toHaveTextContent('Reserved');
    });

    it('while its record is opening says so, and swallows a second press', async () => {
        renderCard();
        const link = screen.getByTestId('order-link');

        setNavigating('/items/item-2');
        await Promise.resolve();
        expect(link).not.toHaveAttribute('aria-busy');

        setNavigating('/items/item-1');
        await Promise.resolve();
        expect(link).toHaveAttribute('aria-busy', 'true');
        expect(link).toHaveAttribute('aria-disabled', 'true');
        expect(screen.getByTestId('stretched-link-spinner')).toBeInTheDocument();

        const second = new MouseEvent('click', { bubbles: true, cancelable: true });
        link.dispatchEvent(second);
        expect(second.defaultPrevented).toBe(true);
    });

    it('stops saying it is opening and lets a press through once the page change is taken as dropped', async () => {
        renderCard();
        const link = screen.getByTestId('order-link');
        setNavigating('/items/item-1');
        await Promise.resolve();
        expect(link).toHaveAttribute('aria-busy', 'true');

        // The page-change guard sets this after 10 s of a change that does not end.
        pageChange.lockExpired = true;
        await Promise.resolve();

        expect(link).not.toHaveAttribute('aria-busy');
        expect(link).not.toHaveAttribute('aria-disabled');
        expect(screen.queryByTestId('stretched-link-spinner')).toBeNull();
        // The test setup cancels every link click that reaches the document, so the press that went
        // through is told by a listener on the link itself, which a swallowed press never reaches.
        const reached = vi.fn();
        link.addEventListener('click', reached);
        link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
        expect(reached).toHaveBeenCalledTimes(1);
    });

    it('lets a label replace the link text as the accessible name', () => {
        renderCard({ ariaLabel: 'Open order 12' });

        expect(screen.getByRole('link', { name: 'Open order 12' })).toHaveTextContent('№ 12');
    });
});

describe('ClickableRow', () => {
    it('is a table row carrying the clickable surface, with the record link in one cell and other controls beside it', async () => {
        render(RowHarness);

        const row = screen.getByTestId('order-row');
        expect(row.tagName).toBe('TR');
        expect(row).toHaveAttribute('data-row-id', 'o-1');
        expect(row).toHaveClass(
            'clickable-surface',
            'pressable:has-[[data-stretched-link]:active]:bg-ink-100',
            'border-b'
        );
        expect(screen.getByTestId('order-link')).toHaveAttribute('data-stretched-link');
        expect(screen.getByTestId('order-shipment')).not.toHaveAttribute('data-stretched-link');

        const checkbox = screen.getByTestId('order-select') as HTMLInputElement;
        await fireEvent.click(checkbox);
        expect(checkbox.checked).toBe(true);
    });
});
