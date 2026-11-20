import type { TCourierCheck, TOrderExchangeDetails, TOrderParcel, TProblem } from '$lib/components/exchange/types';
import type {
    TEventsSetting,
    TExternalApi,
    TMarketplaceMatches,
    TWarehouseMatches
} from '$lib/components/external-api/types';
import type { THandover, THandoverDetails } from '$lib/components/handovers/types';
import { MARKETPLACE_CODE } from './marketplaces';

/**
 * The data of «Внешний API», of the exchange and of the hand-overs that the tests share: builders of each answer of the wyo server
 * in the shapes of `api/wyo.yaml`. Every test takes them from here; `overrides` change what one test needs. Times are at midday.
 */

/** A fulfilment centre's system that answered the last check: the key writes, two marketplaces are accepted. */
export function connectionOf(overrides: Partial<TExternalApi> = {}): TExternalApi {
    return {
        state: 'working',
        address: 'https://fc.example.com/api',
        keyLast4: 'k9z2',
        sellerName: 'ИП Иванов (склад на Лесной)',
        marketplaces: [
            { code: 'mp-a', name: 'Маркетплейс А' },
            { code: 'mp-b', name: 'Маркетплейс Б' }
        ],
        checkedAt: '2026-11-18T12:00:00',
        eventsMode: 'poll',
        ...overrides
    };
}

/** The setting «Внешний API» before it is filled. */
export function emptyConnection(): TExternalApi {
    return { state: 'empty', eventsMode: 'poll' };
}

/** Two warehouses of one account, the first matched, and two warehouses of the fulfilment centre. */
export function warehouseMatchesOf(overrides: Partial<TWarehouseMatches> = {}): TWarehouseMatches {
    return {
        fcWarehouses: [
            { code: 'MSK-1', name: 'Москва, Лесная', city: 'Москва' },
            { code: 'SPB-1', name: 'Санкт-Петербург, Невский' }
        ],
        rows: [
            {
                accountId: 7,
                accountName: 'Магазин одежды',
                marketplace: MARKETPLACE_CODE,
                warehouseNumber: '1001',
                warehouseName: 'Склад на Ленина',
                fcWarehouse: 'MSK-1',
                autoAssembly: false
            },
            {
                accountId: 7,
                accountName: 'Магазин одежды',
                marketplace: MARKETPLACE_CODE,
                warehouseNumber: '1002',
                warehouseName: 'Склад на Мира',
                autoAssembly: false
            }
        ],
        ...overrides
    };
}

/** One account with a proposed code, and one whose key may only read. */
export function marketplaceMatchesOf(overrides: Partial<TMarketplaceMatches> = {}): TMarketplaceMatches {
    return {
        fcMarketplaces: [
            { code: 'mp-a', name: 'Маркетплейс А' },
            { code: 'mp-b', name: 'Маркетплейс Б' }
        ],
        rows: [
            {
                accountId: 7,
                accountName: 'Магазин одежды',
                marketplace: MARKETPLACE_CODE,
                proposed: 'mp-a',
                readOnly: false,
                handoverSettings: { shipping_type: 'selfShipping', city: 'Москва', office_type: 'sc' }
            },
            {
                accountId: 8,
                accountName: 'Второй магазин',
                marketplace: MARKETPLACE_CODE,
                fcMarketplace: 'mp-b',
                readOnly: true,
                handoverSettings: {}
            }
        ],
        ...overrides
    };
}

/** The events' setting while the wyo server asks once a minute. */
export function eventsOf(overrides: Partial<TEventsSetting> = {}): TEventsSetting {
    return { mode: 'poll', secretSaved: false, ...overrides };
}

/** A problem of the marketplace's refusal, with the marketplace's own words and code. */
export function exchangeProblemOf(overrides: Partial<TProblem> = {}): TProblem {
    return {
        code: 'marketplace_refused',
        params: { step: 'confirm' },
        marketplaceCode: 'FailedToAddSupplyOrder',
        words: 'Не удалось добавить заказ в поставку',
        at: '2026-11-18T12:00:00',
        ...overrides
    };
}

/** The exchange of one order in full: reserved, nothing wrong. */
export function exchangeDetailsOf(overrides: Partial<TOrderExchangeDetails> = {}): TOrderExchangeDetails {
    return {
        state: 'reserved',
        fcNumber: 'FC-5501',
        heldReasons: [],
        verdicts: [],
        history: [
            { at: '2026-11-18T12:00:00', state: 'sent', code: 'sent' },
            { at: '2026-11-18T12:30:00', state: 'reserved', code: 'reserved' }
        ],
        retryable: false,
        parcels: [],
        ...overrides
    };
}

/** One parcel of an order: the marketplace's own number of it, and whether its label was attached at the fulfilment centre. */
export function parcelOf(overrides: Partial<TOrderParcel> = {}): TOrderParcel {
    return { number: 1, marketplaceNumber: '4455667788-1', labelSent: false, ...overrides };
}

/** One code a courier named for an order, awaiting the marketplace's answer unless `overrides` give one. */
export function courierCheckOf(overrides: Partial<TCourierCheck> = {}): TCourierCheck {
    return { at: '2026-11-18T12:30:00', ...overrides };
}

/** A hand-over of the day that waits at its third step. */
export function handoverOf(overrides: Partial<THandover> = {}): THandover {
    return {
        id: 31,
        number: 'HO-2026-0042',
        day: '2026-11-19',
        marketplace: MARKETPLACE_CODE,
        state: 'working',
        step: 'boxes',
        batches: ['GI-1001'],
        orders: 12,
        boxes: 2,
        requestedAt: '2026-11-18T12:00:00',
        updatedAt: '2026-11-18T12:30:00',
        ...overrides
    };
}

/** The hand-over above with the strip of its six steps. */
export function handoverDetailsOf(overrides: Partial<THandoverDetails> = {}): THandoverDetails {
    return {
        handover: handoverOf(),
        steps: [
            { step: 'batch', state: 'done' },
            { step: 'move', state: 'done' },
            { step: 'boxes', state: 'now' },
            { step: 'shipping', state: 'todo' },
            { step: 'close', state: 'todo' },
            { step: 'documents', state: 'todo' }
        ],
        orderList: [
            { id: 101, number: '4455667788' },
            { id: 102, number: '4455667799' }
        ],
        retryable: false,
        needsShippingPoint: false,
        closed: false,
        ...overrides
    };
}
