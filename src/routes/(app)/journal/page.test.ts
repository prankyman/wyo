import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountOf, journalEntryOf, MARKETPLACE } from '$test/support/marketplaces';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import { load } from './+page';

const get = vi.mocked(api.get);
const ENTRIES = [journalEntryOf(), journalEntryOf({ id: 502, outcome: 'server_error', status: 503 })];
const LIST = { entries: ENTRIES, total: 2, page: 1, pageSize: 50 };

function answers(list: unknown = { ok: true, data: LIST }): void {
    get.mockImplementation(async (path: string) => {
        if (path.startsWith('/api/v1/journal')) return list as never;
        if (path === '/api/v1/accounts') return { ok: true, data: { accounts: [accountOf()] } } as never;
        return { ok: true, data: { marketplaces: [MARKETPLACE] } } as never;
    });
}

const loadAt = (address: string) =>
    load({ url: new URL(address, 'http://localhost') } as Parameters<typeof load>[0]) as Promise<
        Record<string, unknown>
    >;

afterEach(() => get.mockReset());

describe('the load of «Журнал запросов»', () => {
    it('reads the page of the journal the address names, with its filters, the accounts and the adapters', async () => {
        answers();

        const data = await loadAt('/journal?outcome=server_error&from=2026-11-01&to=2026-11-18');

        expect(get).toHaveBeenCalledWith('/api/v1/journal?outcome=server_error&from=2026-11-01&to=2026-11-18');
        expect(data).toEqual({
            filter: { account: '', outcome: 'server_error', target: '', from: '2026-11-01', to: '2026-11-18' },
            list: LIST,
            refusal: null,
            accounts: [accountOf()],
            marketplaces: [MARKETPLACE],
            opened: null
        });
    });

    it('asks for the requests that went to one place when the address names it', async () => {
        answers();

        const data = await loadAt('/journal?target=fulfilment');

        expect(get).toHaveBeenCalledWith('/api/v1/journal?target=fulfilment');
        expect(data.filter).toMatchObject({ target: 'fulfilment' });
    });

    it('opens the row the address names from the page it read, and nothing for a row not on it', async () => {
        answers();

        expect((await loadAt('/journal?entry=502')).opened).toEqual(ENTRIES[1]);
        expect((await loadAt('/journal?entry=999')).opened).toBeNull();
    });

    it('keeps the refusal’s code when the journal cannot be read', async () => {
        answers({ ok: false, status: 500, refusal: { code: 'internal' } });

        const data = await loadAt('/journal');

        expect(data.list).toBeNull();
        expect(data.refusal).toBe('internal');
    });
});
