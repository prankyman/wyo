import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import PrevNextLinks from './PrevNextLinks.svelte';

/** «Назад» and «Дальше» of a list without a total: the lists test only which page they are on. */
const props = (page: number, hasMore: boolean) => ({
    page,
    hasMore,
    hrefFor: (to: number) => `/items?page=${to}`,
    prevLabel: 'Назад',
    nextLabel: 'Дальше',
    dataTest: 'items'
});

describe('PrevNextLinks', () => {
    it.each([
        [2, true, '/items?page=1', '/items?page=3'],
        [1, true, null, '/items?page=2'],
        [3, false, '/items?page=2', null]
    ])('on page %i with more = %s leads back to %s and on to %s', (page, hasMore, prev, next) => {
        render(PrevNextLinks, { props: props(page, hasMore) });

        expect(screen.queryByTestId('items-prev')?.getAttribute('href') ?? null).toBe(prev);
        expect(screen.queryByTestId('items-next')?.getAttribute('href') ?? null).toBe(next);
        if (prev) expect(screen.getByTestId('items-prev')).toHaveTextContent('Назад');
        if (next) expect(screen.getByTestId('items-next')).toHaveTextContent('Дальше');
    });

    it('draws nothing on a list of one page', () => {
        render(PrevNextLinks, { props: props(1, false) });

        expect(screen.queryByTestId('items-pages')).toBeNull();
    });
});
