import { describe, expect, it } from 'vitest';
import { MARKETPLACE, PACKING_MARKETPLACE, SECOND_MARKETPLACE } from '$test/support/marketplaces';
import type { TMarketplace } from '$lib/components/marketplaces/types';
import { exchangeSupported } from './exchange-support';

/**
 * Whether the exchange with a fulfilment centre works for a marketplace: its adapter takes an order into the exchange, by
 * confirming it (`orders.confirm`) or by reporting its packing (`orders.pack`), the way the wyo server decides which marketplaces'
 * orders it sends (`exchange.HandingOver`). The other writing parts alone do not make it take part.
 */
describe('the exchange with a fulfilment centre, by what the adapter can do', () => {
    it.each<[string, TMarketplace | undefined, boolean]>([
        ['an adapter that holds only the reading parts', SECOND_MARKETPLACE, false],
        ['an adapter that confirms orders and holds every other writing part', MARKETPLACE, true],
        ['an adapter that reports the packing and holds every other writing part', PACKING_MARKETPLACE, true],
        [
            'an adapter that only confirms orders',
            { ...MARKETPLACE, capabilities: ['orders.new', 'orders.confirm'] },
            true
        ],
        ['an adapter that only reports the packing', { ...MARKETPLACE, capabilities: ['orders.pack'] }, true],
        [
            'an adapter that fetches labels and closes hand-overs but takes no order into the exchange',
            { ...MARKETPLACE, capabilities: ['orders.new', 'labels.fetch', 'handover.close'] },
            false
        ],
        ['an adapter that holds no part at all', { ...MARKETPLACE, capabilities: [] }, false],
        ['an adapter the interface does not know yet', undefined, true]
    ])('says %s: %s', (_case, adapter, supported) => {
        expect(exchangeSupported(adapter)).toBe(supported);
    });
});
