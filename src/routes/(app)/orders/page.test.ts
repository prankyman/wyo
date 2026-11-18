import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf, MARKETPLACE, orderOf } from '$test/support/marketplaces';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import { load } from './+page';

const get = vi.mocked(api.get);
const LIST = { orders: [orderOf()], total: 1, page: 2, pageSize: 50 };
const DETAILS = { order: orderOf(), events: [] };

/** The wyo server's answer to each call of the page's load. */
function answers(list: unknown = { ok: true, data: LIST }): void {
    get.mockImplementation(async (path: string) => {
        if (path.startsWith('/api/v1/orders/')) return { ok: true, data: DETAILS } as never;
        if (path.startsWith('/api/v1/orders')) return list as never;
        if (path === '/api/v1/accounts') return { ok: true, data: { accounts: [accountOf()] } } as never;
        if (path === '/api/v1/marketplaces') return { ok: true, data: { marketplaces: [MARKETPLACE] } } as never;
        return { ok: true, data: { nextAllowedAt: '2026-11-18T12:05:00' } } as never;
    });
}

const loadAt = (address: string) =>
    load({ url: new URL(address, 'http://localhost') } as Parameters<typeof load>[0]) as Promise<
        Record<string, unknown>
    >;

afterEach(() => get.mockReset());

describe('the load of «Заказы»', () => {
    it('reads the page of orders the address names, with its filters, and what the page draws beside them', async () => {
        answers();

        const data = await loadAt('/orders?account=7&state=new&q=DRESS&page=2');

        expect(get).toHaveBeenCalledWith('/api/v1/orders?account=7&state=new&q=DRESS&page=2');
        expect(data).toEqual({
            filter: { account: '7', scheme: '', state: 'new', from: '', to: '', q: 'DRESS', attention: '' },
            list: LIST,
            refusal: null,
            accounts: [accountOf()],
            marketplaces: [MARKETPLACE],
            refresh: { nextAllowedAt: '2026-11-18T12:05:00' },
            opened: null
        });
        expect(get).toHaveBeenCalledTimes(4);
    });

    it('asks only for the orders that need the owner when the address says so', async () => {
        answers();

        const data = await loadAt('/orders?attention=true');

        expect(get).toHaveBeenCalledWith('/api/v1/orders?attention=true');
        expect(data.filter).toMatchObject({ attention: 'true' });
    });

    it('reads the order the address opens, for its details beside the list', async () => {
        answers();

        const data = await loadAt('/orders?order=101');

        expect(get).toHaveBeenCalledWith('/api/v1/orders/101');
        expect(data.opened).toEqual(DETAILS);
    });

    it('keeps the refusal’s code when the list cannot be read', async () => {
        answers({ ok: false, status: 500, refusal: { code: 'internal' } });

        const data = await loadAt('/orders');

        expect(data.list).toBeNull();
        expect(data.refusal).toBe('internal');
    });
});
