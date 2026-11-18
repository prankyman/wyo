import { randomBytes, randomUUID } from 'node:crypto';
import { request } from '@playwright/test';
import { FAKE_FULFILMENT_ADDRESS } from './fakes.js';

/**
 * The browser tests' side of the fake fulfilment centre (`internal/fakes/fulfilment`, `docs/development.md`, «The fakes»): a key of
 * a test's own and the fake's control calls, which play the fulfilment centre's part (reserve, pack, reopen, cancel, hand over,
 * ask for the hand-over's documents, send a notification). Fake-only: nothing here ever reaches a real fulfilment centre (rule R3).
 *
 * The fake keeps a state of its own for every key (the `Authorization` header), so every test makes a key of its own with
 * `makeFcKey` and the tests running side by side never share a state. Every call is sent from the test's own process, never from
 * the page, so the page's requests stay the wyo server's only.
 */

/** The address the owner types into «Внешний API» (a test build of the wyo server takes plain http for the fake). */
export const FAKE_FULFILMENT_URL = `http://${FAKE_FULFILMENT_ADDRESS}`;

/** What the fake answers for every key until a test sets another (`newState` of `internal/fakes/fulfilment/model.go`). */
export const FAKE_FC = { warehouse: 'main', marketplace: 'wildberries' } as const;

/** A marking code of 16 or more characters, with the group separator before its last group, as the fake takes it. */
export const MARKING_CODE = '0104600000000017215abcdefghijkl\u001d93Zx9y';

/** A new key of a test's own: each carries a new random id, and the last four characters differ between tests. */
export function makeFcKey(): string {
    return `fc-key-${randomUUID()}`;
}

/**
 * A signing secret in the shape the fulfilment centre's system shows (`whsec_` and the base64 of 32 bytes): a spec types it into
 * the page, and gives the same to the fake with `setFcWebhook`.
 */
export function makeFcSecret(): string {
    return `whsec_${randomBytes(32).toString('base64')}`;
}

/**
 * How the wyo server names the fulfilment centre's key to the screens: its last four characters (`accounts.LastFour`, the same
 * for every key). Never more of the key (rule R4).
 */
export function keyTail(key: string): string {
    return key.slice(-4);
}

/**
 * One control call of the fake for the state of `key`; throws with the fake's answer when it refuses. The control calls carry the
 * key as it is in `Authorization`; the fake's own API takes it as a bearer.
 */
async function call(
    key: string,
    method: 'GET' | 'POST',
    path: string,
    body: unknown,
    authorization: string
): Promise<unknown> {
    const fake = await request.newContext({ baseURL: FAKE_FULFILMENT_URL });
    try {
        const response = await fake.fetch(path, {
            method,
            headers: { Authorization: authorization },
            data: body
        });
        const answer: unknown = await response.json();
        if (!response.ok()) {
            throw new Error(
                `the fake fulfilment centre refused ${method} ${path} for key ${keyTail(key)} with ${response.status()}: ${JSON.stringify(answer)}`
            );
        }
        return answer;
    } finally {
        await fake.dispose();
    }
}

const control = (key: string, path: string, body?: unknown): Promise<unknown> =>
    call(key, 'POST', `/__control/${path}`, body ?? {}, key);

/** What `/__control/setup` may change; only the fields sent change. */
export interface IFcSetup {
    seller_name?: string;
    /** The marketplaces the fulfilment centre takes orders of, by its own codes; the fake knows only `wildberries` until it is told. */
    marketplaces?: { code: string; name: string }[];
    /** `read_only` makes the key refused for every write. */
    access?: 'read_write' | 'read_only';
    api_enabled?: boolean;
    maintenance?: boolean;
    work_stopped?: boolean;
    forbidden?: boolean;
}

export async function setUpFc(key: string, setup: IFcSetup): Promise<void> {
    await control(key, 'setup', setup);
}

/** A product card at the fulfilment centre: whether each unit of the product carries a marking code that must be scanned. */
export interface IFcProductCard {
    article: string;
    barcode: string;
    marking_required: boolean;
}

/**
 * Products the fulfilment centre does not know (`unknown`, by barcode, article or marketplace id), or knows again (`known`), and
 * the cards of products (`catalog`), which an order sent after them takes its marking rule from.
 */
export async function setFcProducts(
    key: string,
    products: { unknown?: string[]; known?: string[]; catalog?: IFcProductCard[] }
): Promise<void> {
    await control(key, 'products', products);
}

/** What the fulfilment centre does with an order: `reserve`, `pack`, `reopen`, `cancel`, `hand-over`. */
export type TFcOrderAction =
    'reserve' | 'wait-stock' | 'start-picking' | 'pick' | 'pack' | 'reopen' | 'cancel' | 'hand-over';

/**
 * A packed unit: the order line it belongs to, its marking code when the line needs one, and the parcel it lies in (from 1; the
 * first parcel when left out). Units in two parcels make the order packed in two parcels.
 */
export interface IFcPackedUnit {
    line: number;
    marking_code?: string;
    parcel?: number;
}

export async function fcOrderStep(
    key: string,
    orderId: string,
    action: TFcOrderAction,
    body?: { units: IFcPackedUnit[] }
): Promise<void> {
    await control(key, `orders/${orderId}/${action}`, body);
}

/** The fulfilment centre's people ask for the documents of a hand-over (`request-documents`), or take the request back. */
export async function fcHandoverStep(
    key: string,
    handoverId: string,
    action: 'request-documents' | 'withdraw-request',
    body?: { transport_boxes: number }
): Promise<void> {
    await control(key, `handovers/${handoverId}/${action}`, body);
}

/** The address (and secret) the fake sends its notifications to; returns the secret, made by the fake when `secret` is left out. */
export async function setFcWebhook(key: string, url: string, secret?: string): Promise<string> {
    const answer = (await control(key, 'webhook', secret === undefined ? { url } : { url, secret })) as {
        secret: string;
    };
    return answer.secret;
}

/** A ping event, which the fake sends signed to the address set by `setFcWebhook`. */
export async function pingFc(key: string): Promise<void> {
    await control(key, 'ping');
}

/** Waits until every notification the fake has queued has been sent. */
export async function flushFc(key: string): Promise<void> {
    await control(key, 'flush');
}

/** An order as the fulfilment centre holds it (the fields a test reads). */
export interface IFcOrder {
    id: string;
    external_id: string;
    status: string;
    handover_id: string | null;
    /** The order's parcels; most orders have one. */
    parcels: { number: number; label_attached: boolean }[];
}

/** Every order the fulfilment centre holds for `key`, newest change first (`GET /api/v1/orders`). */
export async function fcOrders(key: string): Promise<IFcOrder[]> {
    const answer = (await call(key, 'GET', '/api/v1/orders', undefined, `Bearer ${key}`)) as { orders: IFcOrder[] };
    return answer.orders;
}

/** A hand-over as the fulfilment centre holds it (the fields a test reads). */
export interface IFcHandover {
    id: string;
    state: string;
    documents: { kind: string; title: string }[];
}

export async function fcHandover(key: string, handoverId: string): Promise<IFcHandover> {
    const answer = (await call(key, 'GET', `/api/v1/handovers/${handoverId}`, undefined, `Bearer ${key}`)) as {
        handover: IFcHandover;
    };
    return answer.handover;
}
