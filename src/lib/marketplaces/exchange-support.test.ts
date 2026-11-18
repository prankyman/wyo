import { describe, expect, it } from 'vitest';
import { MARKETPLACE, SECOND_MARKETPLACE } from '$test/support/marketplaces';
import type { TMarketplace } from '$lib/components/marketplaces/types';
import { exchangeSupported } from './exchange-support';

/** Whether the exchange with a fulfilment centre works for a marketplace: its adapter holds at least one part that writes to it. */
describe('the exchange with a fulfilment centre, by what the adapter can do', () => {
    it.each<[string, TMarketplace | undefined, boolean]>([
        ['an adapter that holds only the reading parts', SECOND_MARKETPLACE, false],
        ['an adapter that holds every writing part', MARKETPLACE, true],
        [
            'an adapter that only confirms orders',
            { ...MARKETPLACE, capabilities: ['orders.new', 'orders.confirm'] },
            true
        ],
        ['an adapter that only fetches labels', { ...MARKETPLACE, capabilities: ['orders.new', 'labels.fetch'] }, true],
        ['an adapter that only closes hand-overs', { ...MARKETPLACE, capabilities: ['handover.close'] }, true],
        ['an adapter that holds no part at all', { ...MARKETPLACE, capabilities: [] }, false],
        ['an adapter the interface does not know yet', undefined, true]
    ])('says %s: %s', (_case, adapter, supported) => {
        expect(exchangeSupported(adapter)).toBe(supported);
    });
});
