import { describe, expect, it } from 'vitest';
import { addressWith, apiListPath, filterChips, pageNumber } from './list-address';

const LIST = new URL('http://localhost/orders?account=7&state=new&page=3&order=101');

describe('the address of a list', () => {
    it('sets and removes parameters, dropping the page number and the opened row, so the list starts again', () => {
        expect(addressWith(LIST, { state: null })).toBe('/orders?account=7');
        expect(addressWith(LIST, { q: 'платье' })).toBe(
            '/orders?account=7&state=new&q=%D0%BF%D0%BB%D0%B0%D1%82%D1%8C%D0%B5'
        );
    });

    it('keeps the filters and the page when it is told to, for the pages and the opened row', () => {
        expect(addressWith(LIST, { page: '4' }, { keep: ['page'] })).toBe('/orders?account=7&state=new&page=4');
        expect(addressWith(LIST, { order: '102' }, { keep: ['page'] })).toBe(
            '/orders?account=7&state=new&page=3&order=102'
        );
        expect(addressWith(LIST, { page: '1' }, { keep: ['page'] })).toBe('/orders?account=7&state=new');
    });

    it.each([
        ['/orders?page=3', 3],
        ['/orders', 1],
        ['/orders?page=0', 1],
        ['/orders?page=two', 1],
        ['/orders?page=2.5', 1]
    ])('reads the page number of %s as %i, the first page when it cannot', (address, number) => {
        expect(pageNumber(new URL(address, 'http://localhost'))).toBe(number);
    });

    it('makes the call of the wyo server from the filters in use, leaving out the empty ones', () => {
        expect(apiListPath('/api/v1/orders', { account: '7', scheme: '', q: 'A 1', page: 2 })).toBe(
            '/api/v1/orders?account=7&q=A+1&page=2'
        );
        expect(apiListPath('/api/v1/journal', { account: '', page: 1 })).toBe('/api/v1/journal');
    });

    it('draws a chip for each filter in use, linked to the address without it, in the order given', () => {
        const chips = filterChips(LIST, [
            { key: 'state', label: 'Состояние: Новый' },
            { key: 'scheme', label: null },
            { key: 'account', label: 'Аккаунт: Магазин' }
        ]);

        expect(chips).toEqual([
            { key: 'state', label: 'Состояние: Новый', href: '/orders?account=7' },
            { key: 'account', label: 'Аккаунт: Магазин', href: '/orders?state=new' }
        ]);
    });
});
