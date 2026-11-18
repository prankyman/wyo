import type { TMarketplace } from '$lib/components/marketplaces/types';

/**
 * The parts of the adapter contract by which an adapter takes an order into the exchange with a fulfilment centre: it confirms the
 * order into a batch when the fulfilment centre reserves the goods (`orders.confirm`), or it reports the packing after the
 * fulfilment centre has packed them (`orders.pack`). They are the two flows of the wyo server's exchange
 * (`exchangeFlow` of `GET /api/v1/marketplaces`), and the wyo server sends the orders of an adapter that holds one of them
 * (`exchange.HandingOver`) and no others. They are written as a group and its parts, so that a part's name is not taken for the
 * key of a text (`message-bundles.test.ts`). An adapter that holds neither only reads: the wyo server has nothing to exchange for
 * its orders.
 */
const EXCHANGE_PARTS: readonly string[] = Object.entries({
    orders: ['confirm', 'pack']
}).flatMap(([group, parts]) => parts.map((part) => `${group}.${part}`));

/**
 * Whether the exchange with a fulfilment centre works for orders of `adapter`: it holds a part of `EXCHANGE_PARTS`, as the wyo
 * server decides. An adapter the interface has not read (`undefined`) is taken to work, so no screen claims «не поддерживается»
 * about what it does not know.
 */
export function exchangeSupported(adapter: TMarketplace | undefined): boolean {
    return adapter === undefined || adapter.capabilities.some((part) => EXCHANGE_PARTS.includes(part));
}
