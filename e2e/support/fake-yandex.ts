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
    items: { offer_id: string; offer_name: string; count: number }[];
}

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
