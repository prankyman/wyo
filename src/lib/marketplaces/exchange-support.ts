import type { TMarketplace } from '$lib/components/marketplaces/types';

/**
 * The parts of the adapter contract that write to a marketplace for the exchange with a fulfilment centre: taking an order into
 * assembly, fetching its label, reporting and checking the marking codes, and grouping, closing and documenting a hand-over. They
 * are written as a group and its parts, so that a part's name is not taken for the key of a text (`message-bundles.test.ts`).
 * An adapter that holds none of them only reads: the wyo server has nothing to exchange for its orders.
 */
const EXCHANGE_PARTS: readonly string[] = Object.entries({
    orders: ['confirm'],
    labels: ['fetch'],
    codes: ['report', 'verdict'],
    handover: ['group', 'close', 'documents']
}).flatMap(([group, parts]) => parts.map((part) => `${group}.${part}`));

/**
 * Whether the exchange with a fulfilment centre works for orders of `adapter`: it holds at least one writing part. An adapter the
 * interface has not read (`undefined`) is taken to work, so no screen claims «не поддерживается» about what it does not know.
 */
export function exchangeSupported(adapter: TMarketplace | undefined): boolean {
    return adapter === undefined || adapter.capabilities.some((part) => EXCHANGE_PARTS.includes(part));
}
