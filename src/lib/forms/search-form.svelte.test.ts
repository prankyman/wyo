import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Harness from '$test/harnesses/forms/SearchFormTestHarness.svelte';
import { resetPage } from '$test/support/page-state.svelte';
import { searchAddress } from './search-form';

vi.mock('$app/state', async () => {
    const state = await import('$test/support/page-state.svelte');
    return { page: state.page, navigating: state.navigating };
});

const navigation = vi.hoisted(() => {
    const state = { settle: (): void => {} };
    return { state, goto: vi.fn(() => new Promise<void>((resolve) => (state.settle = resolve))) };
});
const { goto } = navigation;
vi.mock('$app/navigation', () => ({
    goto: navigation.goto,
    invalidateAll: vi.fn(),
    beforeNavigate: vi.fn(),
    afterNavigate: vi.fn()
}));

afterEach(() => {
    goto.mockClear();
    resetPage();
});

/** A search form never posts and never reloads: a valid submit is a router page change. */
describe('searchForm', () => {
    it('turns a valid submit into a page change with the values in the address, keeps them and guards the press until it settles', async () => {
        render(Harness);
        await fireEvent.input(screen.getByTestId('search-q'), { target: { value: 'abc' } });

        await fireEvent.click(screen.getByTestId('search-submit'));

        await waitFor(() => expect(goto).toHaveBeenCalledTimes(1));
        expect(goto).toHaveBeenCalledWith('/app/orders?sort=date&q=abc&status=open', {
            keepFocus: true,
            noScroll: true
        });
        expect(screen.getByTestId('search-submit')).toHaveAttribute('aria-busy', 'true');
        navigation.state.settle();
        await waitFor(() => expect(screen.getByTestId('search-submit')).not.toHaveAttribute('aria-busy'));
        expect(screen.getByTestId('search-q')).toHaveValue('abc');
    });

    it('shows a refused value under its box and does not navigate', async () => {
        render(Harness);
        await fireEvent.input(screen.getByTestId('search-q'), { target: { value: 'too long' } });

        await fireEvent.click(screen.getByTestId('search-submit'));

        expect(await screen.findByTestId('search-q-error')).toHaveTextContent('q.tooLong');
        expect(goto).not.toHaveBeenCalled();
        expect(screen.getByTestId('search-q')).toHaveValue('too long');
    });
});

describe('searchAddress', () => {
    it('sets the values, drops the empty ones and the page number, and repeats a list', () => {
        const current = new URL('http://localhost/app/stock?page=2&q=old#top');

        expect(searchAddress(current, { q: '', zone: ['A', 'B'], active: true, archived: false })).toBe(
            '/app/stock?zone=A&zone=B&active=true#top'
        );
    });
});
