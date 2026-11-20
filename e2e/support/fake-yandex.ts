import { randomUUID } from 'node:crypto';
import { request } from '@playwright/test';
import { FAKE_MARKETPLACES_ADDRESS } from './fakes.js';

/**
 * The browser tests' side of the fake Яндекс Маркет (`internal/fakes/yandex`, `docs/development.md`, «The fakes»): made-up keys and
 * the fake's control calls. Fake-only: nothing here ever reaches the real Яндекс Маркет (rule R3).
 *
 * The fake shares an address with the other fake marketplaces and is told apart by the first part of the path, the host of Яндекс
 * Маркет's API. It keeps a state of its own for every key, so every test makes a key of its own with `makeYandexKey`. The field
 * names below are the control calls' own (`internal/fakes/yandex/control.go`, `control_orders.go`).
 */

/** The host of Яндекс Маркет's API: the first part of every path the fake answers. */
export const FAKE_YANDEX_HOST = 'api.partner.market.yandex.ru';

/** A made-up Api-Key token of no real cabinet, new on every call. */
export function makeYandexKey(): string {
    return `y0_${randomUUID().replaceAll('-', '')}`;
}

export interface IFakeYandexSetup {
    business?: { id: number; name: string };
    scopes?: string[];
    key_state?: '' | 'invalid' | 'revoked';
    campaigns?: { id: number; name: string; placement_type?: string; api_off?: boolean }[];
    warehouses?: { id: number; name: string; models: { placement_type: string; campaign_id: number }[] }[];
    offers?: { offer_id: string; barcodes: string[] }[];
}

export interface IFakeYandexOrder {
    order_id?: number;
    campaign_id: number;
    program_type?: 'FBS' | 'EXPRESS' | 'FBY';
    fake?: boolean;
    status?: string;
    substatus?: string;
    shipment_date?: string;
    shipment_time?: string;
    items: { offer_id: string; offer_name: string; count: number; required_instance_types?: string[] }[];
}

/**
 * A made-up cabinet: an FBS shop and an Express shop (both placed FBS) with their warehouses, an FBY shop (Яндекс Маркет's own
 * warehouse, no warehouse of the seller's), a DBS warehouse of the FBS shop (the seller delivers: never read), and two products.
 */
export const FAKE_YANDEX_SHOP_FBS = 5001;
export const FAKE_YANDEX_SHOP_EXPRESS = 5002;
export const FAKE_YANDEX_SHOP_FBY = 5003;
export const FAKE_YANDEX_WAREHOUSE_FBS = 301;
export const FAKE_YANDEX_WAREHOUSE_EXPRESS = 302;
export const FAKE_YANDEX_WAREHOUSE_DBS = 303;

/** What `setUpFakeYandex` is given for that cabinet. */
export const FAKE_YANDEX_CABINET: IFakeYandexSetup = {
    business: { id: 7001, name: 'Тестовый кабинет' },
    scopes: ['ALL_METHODS'],
    campaigns: [
        { id: FAKE_YANDEX_SHOP_FBS, name: 'fbs.shop.test', placement_type: 'FBS' },
        { id: FAKE_YANDEX_SHOP_EXPRESS, name: 'express.shop.test', placement_type: 'FBS' },
        { id: FAKE_YANDEX_SHOP_FBY, name: 'fby.shop.test', placement_type: 'FBY' }
    ],
    warehouses: [
        {
            id: FAKE_YANDEX_WAREHOUSE_FBS,
            name: 'Склад FBS',
            models: [{ placement_type: 'FBS', campaign_id: FAKE_YANDEX_SHOP_FBS }]
        },
        {
            id: FAKE_YANDEX_WAREHOUSE_EXPRESS,
            name: 'Склад Экспресс',
            models: [{ placement_type: 'EXPRESS', campaign_id: FAKE_YANDEX_SHOP_EXPRESS }]
        },
        {
            id: FAKE_YANDEX_WAREHOUSE_DBS,
            name: 'Склад DBS',
            models: [{ placement_type: 'DBS', campaign_id: FAKE_YANDEX_SHOP_FBS }]
        }
    ],
    offers: [
        { offer_id: 'SHIRT', barcodes: ['4600000007001'] },
        { offer_id: 'SOCKS', barcodes: ['4600000007002'] }
    ]
};

async function control(key: string, path: string, body: unknown): Promise<unknown> {
    const fake = await request.newContext({ baseURL: `http://${FAKE_MARKETPLACES_ADDRESS}` });
    try {
        const response = await fake.post(`/${FAKE_YANDEX_HOST}/__control/${path}`, {
            headers: { Authorization: key },
            data: body
        });
        if (!response.ok()) {
            throw new Error(
                `the fake Яндекс Маркет refused /__control/${path} with ${response.status()}: ${await response.text()}`
            );
        }
        return await response.json();
    } finally {
        await fake.dispose();
    }
}

/** Sets the key up: the parts named replace what the fake keeps. */
export async function setUpFakeYandex(key: string, setup: IFakeYandexSetup): Promise<void> {
    await control(key, 'setup', setup);
}

/** Makes an order at the fake; the fake notifies the address it was told, if any, before it answers. */
export async function makeFakeYandexOrder(key: string, order: IFakeYandexOrder): Promise<void> {
    await control(key, 'orders', order);
}

/** Cancels an order with a reason (Яндекс Маркет's substatus), notified as above. */
export async function cancelFakeYandexOrder(key: string, orderId: number, substatus: string): Promise<void> {
    await control(key, 'orders/cancel', { order_id: orderId, substatus });
}

/** Tells the fake where to send its notifications; it sends its check (PING) at once. */
export async function setFakeYandexNotifications(key: string, address: string): Promise<void> {
    await control(key, 'notifications', { address });
}

/**
 * What the fake answers when the wyo server asks for the verdict on the marketplace codes of an order: `OK`, `IN_PROGRESS`,
 * `FAILED`, `INVALID` or `NOT_ON_VALIDATION`; `''` gives back the fake's own course (in progress at the first reading, accepted after).
 */
export type TFakeYandexVerdict = 'OK' | 'IN_PROGRESS' | 'FAILED' | 'INVALID' | 'NOT_ON_VALIDATION' | '';

export async function setFakeYandexVerdict(key: string, orderId: number, status: TFakeYandexVerdict): Promise<void> {
    await control(key, 'orders/verdict', { order_id: orderId, status });
}

/** The code the Express courier names for the order, and how many tries the fake allows; `need_update` makes the next check ask for a new code. */
export async function setFakeYandexCourierCode(
    key: string,
    courier: { order_id: number; code: string; attempts_left?: number; need_update?: boolean }
): Promise<void> {
    await control(key, 'orders/courier-code', courier);
}

/** A daily shipment of a warehouse at the fake: the orders in it, whether its cutoff has passed, and which orders are being taken out. */
export interface IFakeYandexShipment {
    warehouse_id: number;
    /** The day, `YYYY-MM-DD`. */
    date: string;
    kind: 'WITHDRAW' | 'IMPORT';
    order_ids: number[];
    cutoff_passed?: boolean;
    excluding_order_ids?: number[];
}

/** Makes the day's shipment and returns its number. */
export async function makeFakeYandexShipment(key: string, shipment: IFakeYandexShipment): Promise<number> {
    const answer = (await control(key, 'shipments', shipment)) as { shipment_id: number };
    return answer.shipment_id;
}

/** Changes a shipment: any of the fields named; `order_ids` replaces the list. */
export async function updateFakeYandexShipment(
    key: string,
    shipmentId: number,
    change: Partial<Pick<IFakeYandexShipment, 'cutoff_passed' | 'excluding_order_ids' | 'order_ids'>>
): Promise<void> {
    await control(key, 'shipments/update', { shipment_id: shipmentId, ...change });
}

/** A call the fake received: its method, path and the status it answered. */
export interface IFakeYandexCall {
    method: string;
    path: string;
    status: number;
}

/** Every call the fake received for the state of `key`, oldest first, so a test can tell what the wyo server wrote and how often. */
export async function fakeYandexCalls(key: string): Promise<IFakeYandexCall[]> {
    const fake = await request.newContext({ baseURL: `http://${FAKE_MARKETPLACES_ADDRESS}` });
    try {
        const response = await fake.get(`/${FAKE_YANDEX_HOST}/__control/calls`, { headers: { Authorization: key } });
        if (!response.ok())
            throw new Error(`the fake Яндекс Маркет refused /__control/calls with ${response.status()}`);
        return ((await response.json()) as { calls: IFakeYandexCall[] }).calls;
    } finally {
        await fake.dispose();
    }
}

/** The ends of the paths of Яндекс Маркет's methods that change something: the boxes, the status, the courier's code, the confirmation. */
const WRITE_ENDS = ['/boxes', '/status', '/verifyEac', '/confirm'];

/**
 * What the wyo server wrote to the fake and the fake accepted, oldest first, as `METHOD /last-part-of-the-path`. The reading of the
 * verdict on the codes (`.../identifiers/status`) is not a write, though its path ends as the status's does.
 */
export async function fakeYandexWrites(key: string): Promise<string[]> {
    return (await fakeYandexCalls(key))
        .filter(
            (call) =>
                call.status < 300 &&
                !call.path.endsWith('/identifiers/status') &&
                WRITE_ENDS.some((end) => call.path.endsWith(end))
        )
        .map((call) => `${call.method} ${call.path.slice(call.path.lastIndexOf('/'))}`);
}
