import type { TJournalEntry } from '$lib/components/journal/types';
import type { TAccount, TMarketplace } from '$lib/components/marketplaces/types';
import type { TOrder } from '$lib/components/orders/types';

/**
 * The marketplace data the tests of the wyo interface share: one adapter as the wyo server describes it, and builders of an
 * account, an order and a journal row in the shapes of `api/wyo.yaml`. It is the one test file that holds a marketplace's code and
 * name (`marketplace-name-policy.test.ts` names it with its reason): every other test takes them from here. Times are at midday.
 */

/** The adapter's code, as the wyo server sends it. */
export const MARKETPLACE_CODE = 'wildberries';
/** The adapter's name, as `marketplace.<code>.name` words it. */
export const MARKETPLACE_NAME = 'Wildberries';

export const MARKETPLACE: TMarketplace = {
    code: MARKETPLACE_CODE,
    mark: 'plum',
    keyFields: [{ name: 'token', secret: true }],
    capabilities: ['key.check', 'warehouses.read', 'orders.new', 'orders.states', 'orders.feed'],
    handoverSettings: [
        { name: 'shipping_type', choices: ['selfShipping', 'transportCompany'] },
        { name: 'city', choices: [] },
        { name: 'office_type', choices: ['sc', 'sw', 'pp', 'pfc', 'gpp'] }
    ]
};

/** A working account with a Personal key that ends in 180 days; `overrides` change what a test needs. */
export function accountOf(overrides: Partial<TAccount> = {}): TAccount {
    return {
        id: 7,
        marketplace: MARKETPLACE_CODE,
        name: 'Магазин одежды',
        state: 'working',
        test: false,
        sellerName: 'ИП Иванов',
        keyLast4: 'a1b2',
        keyType: 'personal',
        keyExpiresAt: '2027-05-16T12:00:00',
        missingCategories: [],
        readOnly: false,
        deadlineHours: 48,
        lastCheckAt: '2026-11-18T12:00:00',
        lastSuccessAt: '2026-11-18T12:30:00',
        failuresInRow: 0,
        hasOrders: true,
        createdAt: '2026-11-17T12:00:00',
        ...overrides
    };
}

/** A new FBS order of the account above, created at midday, due 48 hours later. */
export function orderOf(overrides: Partial<TOrder> = {}): TOrder {
    return {
        id: 101,
        accountId: 7,
        accountName: 'Магазин одежды',
        marketplace: MARKETPLACE_CODE,
        scheme: 'fbs',
        number: '4455667788',
        basket: '1234567',
        warehouse: 'Склад на Ленина',
        article: 'DRESS-RED-42',
        barcodes: ['2040000000017'],
        productId: '987654',
        kind: 'Платья',
        brand: 'Северный ветер',
        size: '42',
        createdAt: '2026-11-17T12:00:00',
        deadlineAt: '2026-11-19T12:00:00',
        state: 'new',
        marketplaceStatus: { seller: 'new', marketplace: 'waiting' },
        updatedAt: '2026-11-17T12:30:00',
        ...overrides
    };
}

/** A journal row of a request that was answered with success. */
export function journalEntryOf(overrides: Partial<TJournalEntry> = {}): TJournalEntry {
    return {
        id: 501,
        at: '2026-11-18T12:00:00',
        accountId: 7,
        accountName: 'Магазин одежды',
        marketplace: MARKETPLACE_CODE,
        method: 'GET',
        path: '/api/v3/orders/new',
        status: 200,
        outcome: 'done',
        durationMs: 312,
        job: 'orders.new',
        ...overrides
    };
}
