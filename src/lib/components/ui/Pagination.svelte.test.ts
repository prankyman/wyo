import { render, screen, waitFor } from '@testing-library/svelte';
import { fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import Pagination from './Pagination.svelte';

describe('Pagination', () => {
    it('renders nothing when total fits on one page', () => {
        const { container } = render(Pagination, {
            props: { page: 1, perPage: 20, total: 10, lang: 'ru', onNavigate: vi.fn() }
        });

        expect(container.querySelector('[data-test="pagination"]')).not.toBeInTheDocument();
    });

    it('renders the showing summary and page buttons', () => {
        render(Pagination, { props: { page: 2, perPage: 20, total: 137, lang: 'ru', onNavigate: vi.fn() } });

        expect(screen.getByTestId('pagination-summary')).toHaveTextContent('21–40');
        expect(screen.getByTestId('pagination-summary')).toHaveTextContent('137');
        // The current page is marked, and every page is a 44 px target with the pressed look.
        const current = screen.getByTestId('pagination-page-2');
        expect(current).toHaveAttribute('aria-current', 'page');
        expect(current).toHaveClass('min-h-11', 'min-w-11', 'motion-safe:pressable:active:scale-[0.97]');
        expect(screen.getByTestId('pagination-page-3')).not.toHaveAttribute('aria-current');
    });

    it('disables prev/first on the first page', () => {
        render(Pagination, { props: { page: 1, perPage: 20, total: 137, lang: 'ru', onNavigate: vi.fn() } });

        expect(screen.getByTestId('pagination-prev')).toBeDisabled();
        expect(screen.getByTestId('pagination-first')).toBeDisabled();
        expect(screen.getByTestId('pagination-next')).not.toBeDisabled();
    });

    it('disables next/last on the last page', () => {
        render(Pagination, { props: { page: 7, perPage: 20, total: 137, lang: 'ru', onNavigate: vi.fn() } });

        expect(screen.getByTestId('pagination-next')).toBeDisabled();
        expect(screen.getByTestId('pagination-last')).toBeDisabled();
        expect(screen.getByTestId('pagination-prev')).not.toBeDisabled();
    });

    it('calls onNavigate with the clicked page number', async () => {
        const onNavigate = vi.fn();
        render(Pagination, { props: { page: 1, perPage: 20, total: 137, lang: 'ru', onNavigate } });

        await fireEvent.click(screen.getByTestId('pagination-next'));

        expect(onNavigate).toHaveBeenCalledTimes(1);
        expect(onNavigate).toHaveBeenCalledWith(2);
    });

    it('works while the page change it started runs, and takes no second press until it ends', async () => {
        // Pagination moves by `goto`, which the page-change guard does not lock: the buttons
        // guard themselves, so a second tap on "next" cannot start a second page change.
        let finish = (): void => {};
        const onNavigate = vi.fn(() => new Promise<void>((resolve) => (finish = resolve)));
        render(Pagination, { props: { page: 1, perPage: 20, total: 137, lang: 'ru', onNavigate } });
        const next = screen.getByTestId('pagination-next');

        const last = screen.getByTestId('pagination-last');

        await fireEvent.click(next);
        await fireEvent.click(next);
        await fireEvent.click(last);

        expect(onNavigate).toHaveBeenCalledTimes(1);
        expect(onNavigate).toHaveBeenCalledWith(2);
        expect(next).toHaveAttribute('aria-busy', 'true');
        expect(last).toHaveAttribute('aria-disabled', 'true');
        expect(last).not.toHaveAttribute('aria-busy');

        finish();
        await waitFor(() => expect(next).not.toHaveAttribute('aria-busy'));
        await fireEvent.click(last);
        expect(onNavigate).toHaveBeenCalledTimes(2);
        expect(onNavigate).toHaveBeenLastCalledWith(7);
    });

    it('renders an ellipsis for large page counts', () => {
        render(Pagination, { props: { page: 1, perPage: 10, total: 500, lang: 'ru', onNavigate: vi.fn() } });

        expect(screen.getAllByText('…').length).toBeGreaterThanOrEqual(1);
    });

    it('applies the dataTest prefix', () => {
        render(Pagination, {
            props: { page: 1, perPage: 20, total: 137, lang: 'ru', onNavigate: vi.fn(), dataTest: 'ticket' }
        });

        expect(screen.getByTestId('ticket-prev')).toBeInTheDocument();
        expect(screen.getByTestId('ticket-next')).toBeInTheDocument();
    });
});
