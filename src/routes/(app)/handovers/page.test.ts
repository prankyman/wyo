import { afterEach, describe, expect, it, vi } from 'vitest';
import { handoverDetailsOf, handoverOf } from '$test/support/external-api';

vi.mock('$lib/api/api', async (original) => ({
    ...(await original<typeof import('$lib/api/api')>()),
    api: { get: vi.fn() }
}));

import { api } from '$lib/api/api';
import { load } from './+page';

const get = vi.mocked(api.get);
const LIST = { handovers: [handoverOf()] };
const DETAILS = handoverDetailsOf();

function answers(list: unknown = { ok: true, data: LIST }): void {
    get.mockImplementation(async (path: string) => {
        if (path.startsWith('/api/v1/handovers/')) return { ok: true, data: DETAILS } as never;
        return list as never;
    });
}

const loadAt = (address: string) =>
    load({ url: new URL(address, 'http://localhost') } as Parameters<typeof load>[0]) as Promise<
        Record<string, unknown>
    >;

afterEach(() => get.mockReset());

describe('the load of «Передачи»', () => {
    it('reads the list of hand-overs', async () => {
        answers();

        const data = await loadAt('/handovers');

        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/handovers');
        expect(data).toEqual({ list: LIST, refusal: null, opened: null });
    });

    it('reads the hand-over the address opens, for its details beside the list', async () => {
        answers();

        const data = await loadAt('/handovers?handover=31');

        expect(get).toHaveBeenCalledWith('/api/v1/handovers/31');
        expect(data.opened).toEqual(DETAILS);
    });

    it('does not ask for a hand-over whose number is not a number', async () => {
        answers();

        await loadAt('/handovers?handover=../x');

        expect(get).toHaveBeenCalledExactlyOnceWith('/api/v1/handovers');
    });

    it('keeps the refusal’s code when the list cannot be read', async () => {
        answers({ ok: false, status: 500, refusal: { code: 'internal' } });

        const data = await loadAt('/handovers');

        expect(data.list).toBeNull();
        expect(data.refusal).toBe('internal');
    });
});
