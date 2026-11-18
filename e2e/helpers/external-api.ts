import { expect, type BrowserContext } from '@playwright/test';
import {
    FAKE_FC,
    FAKE_FULFILMENT_URL,
    makeFcKey,
    makeFcSecret,
    setFcWebhook,
    setUpFc
} from '../support/fake-fulfilment.js';
import { makeToken } from '../support/fake-wildberries.js';
import {
    hoursFromNow,
    makeOzonKey,
    setUpFakeOzon,
    type IFakeOzonKey,
    type IFakeOzonLine,
    type IFakeOzonProduct,
    type IFakeOzonSetup
} from '../support/fake-ozon.js';
import { addAccountThroughApi, addOzonAccountThroughApi } from './accounts.js';
import type { IWyoServer } from '../support/wyo-server.js';

/**
 * «Внешний API» and the exchange made through the wyo server's own API and its test address, for the specs that need them but are
 * not about the page: the page «Внешний API» is walked once, by `e2e/specs/external-api.spec.ts`. The calls are made by the
 * context's own request object, which carries the signed-in owner's session (`signInThroughApi`). The key goes only to the wyo
 * server, never anywhere else.
 */

/** The hand-over settings of a Wildberries account that the fake Wildberries offers the point `100` for. */
export const WILDBERRIES_HANDOVER = { shipping_type: 'selfShipping', city: 'Moscow', office_type: 'pp' } as const;

async function put(context: BrowserContext, server: IWyoServer, path: string, data: unknown): Promise<void> {
    const response = await context.request.put(`${server.origin}${path}`, { data });
    if (response.status() !== 200) {
        throw new Error(`PUT ${path} was refused with status ${response.status()}: ${await response.text()}`);
    }
}

/** «Проверить и сохранить» through the API: the address of the fake fulfilment centre and `key`. */
export async function connectExternalApiThroughApi(
    context: BrowserContext,
    server: IWyoServer,
    key: string
): Promise<void> {
    await put(context, server, '/api/v1/external-api', { address: FAKE_FULFILMENT_URL, key });
}

export interface IMatchOptions {
    /** Also save the hand-over settings of the account (`WILDBERRIES_HANDOVER`). */
    handoverSettings?: boolean;
    /** The marketplace's code in the fulfilment centre's list; the fake fulfilment centre's `FAKE_FC.marketplace` when left out. */
    marketplace?: string;
}

/**
 * «Склады» and «Маркетплейсы» through the API for the account `accountId`: its warehouse 1001 at the fake Wildberries stands for
 * the fake fulfilment centre's warehouse `main`, and its marketplace code is `wildberries` (or `options.marketplace`).
 */
export async function matchExternalApiThroughApi(
    context: BrowserContext,
    server: IWyoServer,
    accountId: number,
    options: IMatchOptions = {}
): Promise<void> {
    await put(context, server, '/api/v1/external-api/warehouses', {
        rows: [{ accountId, warehouseNumber: '1001', fcWarehouse: FAKE_FC.warehouse }]
    });
    await put(context, server, '/api/v1/external-api/marketplaces', {
        rows: [
            {
                accountId,
                fcMarketplace: options.marketplace ?? FAKE_FC.marketplace,
                ...(options.handoverSettings ? { handoverSettings: WILDBERRIES_HANDOVER } : {})
            }
        ]
    });
}

/**
 * Runs every job of the exchange once, at once (`POST /__test/exchange/run`, which only a test build answers, with no sign-in):
 * the new orders are read and sent, the fulfilment centre's events are applied, the marketplace's verdicts are asked. A spec calls
 * it after each control call of a fake instead of waiting for the jobs' minute.
 */
export async function runExchange(context: BrowserContext, server: IWyoServer): Promise<void> {
    const response = await context.request.post(`${server.origin}/__test/exchange/run`);
    expect(response.status(), 'the wyo server ran the exchange').toBe(204);
}

/** What `GET /api/v1/orders` tells of an order's exchange. */
interface IOrderRow {
    id: number;
    number: string;
    exchange?: { state: string; attention?: string; problem?: { code: string } };
}

/** The wyo server's own rows of the orders (`GET /api/v1/orders`), newest first. */
export async function readOrderRows(context: BrowserContext, server: IWyoServer): Promise<IOrderRow[]> {
    const response = await context.request.get(`${server.origin}/api/v1/orders`);
    return ((await response.json()) as { orders: IOrderRow[] }).orders;
}

/** The exchange state, or the code of the problem, of the order whose marketplace number is `number`. */
async function exchangeOf(
    context: BrowserContext,
    server: IWyoServer,
    number: string,
    state: string | { problem: string }
): Promise<string | undefined> {
    const row = (await readOrderRows(context, server)).find((one) => one.number === number);
    return typeof state === 'string' ? row?.exchange?.state : row?.exchange?.problem?.code;
}

const wanted = (state: string | { problem: string }): string => (typeof state === 'string' ? state : state.problem);

/**
 * Runs the exchange again and again until the order whose marketplace number is `number` has the exchange state `state`
 * (or, with `problem`, a problem), so the next look at the page is at a settled state. Waits for the condition, never a fixed time.
 */
export async function runExchangeUntil(
    context: BrowserContext,
    server: IWyoServer,
    number: string,
    state: string | { problem: string }
): Promise<void> {
    await expect
        .poll(
            async () => {
                await runExchange(context, server);
                return exchangeOf(context, server, number, state);
            },
            { message: `the exchange of order ${number} never reached ${JSON.stringify(state)}`, timeout: 30_000 }
        )
        .toBe(wanted(state));
}

/**
 * Waits, without running the jobs, until the order has the state: for an exchange the wyo server's own listener drives, such as
 * the events a notification announced (`receiveNotificationsThroughApi`).
 */
export async function waitForExchange(
    context: BrowserContext,
    server: IWyoServer,
    number: string,
    state: string | { problem: string }
): Promise<void> {
    await expect
        .poll(() => exchangeOf(context, server, number, state), {
            message: `order ${number} never reached ${JSON.stringify(state)} by itself`,
            timeout: 30_000
        })
        .toBe(wanted(state));
}

/**
 * Switches «События» to «Принимать уведомления» through the API and gives the fake fulfilment centre the wyo server's address and
 * the secret, so that what the fulfilment centre does is announced to the wyo server and applied by it at once, with none of the
 * jobs that ask the marketplace for verdicts running. Returns nothing; the fake sends after each `flushFc`.
 */
export async function receiveNotificationsThroughApi(
    context: BrowserContext,
    server: IWyoServer,
    key: string
): Promise<void> {
    const secret = makeFcSecret();
    await put(context, server, '/api/v1/external-api/events', { mode: 'notify', secret });
    const events = await context.request.get(`${server.origin}/api/v1/external-api/events`);
    await setFcWebhook(key, ((await events.json()) as { hookUrl: string }).hookUrl, secret);
}

/** What `GET /api/v1/handovers` tells of a hand-over. */
export interface IHandoverRow {
    id: number;
    number: string;
    state: string;
}

/** The wyo server's own rows of the hand-overs (`GET /api/v1/handovers`), newest first. */
export async function readHandoverRows(context: BrowserContext, server: IWyoServer): Promise<IHandoverRow[]> {
    const response = await context.request.get(`${server.origin}/api/v1/handovers`);
    return ((await response.json()) as { handovers: IHandoverRow[] }).handovers;
}

/**
 * Runs the exchange again and again until the newest hand-over has `state`, and returns it. Waits for the condition, never a
 * fixed time.
 */
export async function runExchangeUntilHandover(
    context: BrowserContext,
    server: IWyoServer,
    state: string
): Promise<IHandoverRow> {
    await expect
        .poll(
            async () => {
                await runExchange(context, server);
                return (await readHandoverRows(context, server))[0]?.state;
            },
            { message: `the newest hand-over never reached ${state}`, timeout: 30_000 }
        )
        .toBe(state);
    return (await readHandoverRows(context, server))[0];
}

export interface IExchangeSetUp {
    /** The Wildberries token of the account; the fake Wildberries keeps its orders under it. */
    token: string;
    /** The key of the fake fulfilment centre; it keeps its orders under it. */
    key: string;
    accountId: number;
}

/**
 * Everything the exchange needs, through the API: a Wildberries account on a token of its own, «Внешний API» filled with a key of
 * its own, and the account's warehouse and marketplace code matched (`matchExternalApiThroughApi`).
 */
export async function setUpExchange(
    context: BrowserContext,
    server: IWyoServer,
    options: IMatchOptions = {}
): Promise<IExchangeSetUp> {
    const token = makeToken();
    const key = makeFcKey();
    const account = await addAccountThroughApi(context, server, 'Магазин для обмена', token);
    await connectExternalApiThroughApi(context, server, key);
    await matchExternalApiThroughApi(context, server, account.id, options);
    return { token, key, accountId: account.id };
}

/** The marketplace's code the fake fulfilment centre is told to take orders of, for the Ozon account of `setUpOzonExchange`. */
export const FAKE_FC_OZON = 'ozon';

/** The warehouse of the made-up Ozon seller that stands for the fulfilment centre's `main`, and the number the fake Ozon gives it. */
export const OZON_WAREHOUSE = { id: 1001, name: 'Склад FBS' } as const;

/**
 * The made-up Ozon seller's two products, none of which asks for a marking code or another identifier (a test that needs a code
 * changes a product with `mark`).
 */
export const OZON_PRODUCTS: IFakeOzonProduct[] = [
    { sku: 2001, offer_id: 'SHIRT-RED', name: 'Рубашка', barcodes: ['4600000002001'] },
    { sku: 2002, offer_id: 'SOCKS-BLUE', name: 'Носки', barcodes: ['4600000002002'] }
];

/**
 * The methods of the Ozon level «Posting FBS read-only» and the others the wyo server reads with, as `/v1/roles` lists them: a
 * key that holds exactly these may read and may not write, so the wyo server marks it «only reads» and names the level to add
 * (`Posting FBS`). Whole paths: a beginning of a path would open the methods that write under it too.
 */
export const OZON_READING_METHODS: string[] = [
    '/v1/seller/info',
    '/v2/warehouse/list',
    '/v4/posting/fbs/unfulfilled/list',
    '/v3/posting/fbs/get',
    '/v3/posting/fbo/list',
    '/v3/product/info/list'
];

/** `quantity` units of `product` as a line of a posting. */
export function ozonLine(product: IFakeOzonProduct, quantity: number): IFakeOzonLine {
    return { sku: product.sku, offer_id: product.offer_id, name: product.name, quantity };
}

/** A key that writes (`Admin`), alive for 60 days, of a seller with the one FBS warehouse `OZON_WAREHOUSE` and `OZON_PRODUCTS`. */
export function ozonSellerSetUp(key: IFakeOzonKey, change: IFakeOzonSetup = {}): IFakeOzonSetup {
    return {
        client_id: key.clientId,
        seller_name: 'ООО «Тестовый продавец»',
        expires_at: hoursFromNow(60 * 24),
        roles: [{ name: 'Admin', methods: [] }],
        warehouses: [{ ...OZON_WAREHOUSE }],
        products: OZON_PRODUCTS,
        ...change
    };
}

export interface IOzonExchangeSetUp {
    /** The made-up Ozon key; the fake Ozon keeps its postings under it. */
    ozon: IFakeOzonKey;
    /** The key of the fake fulfilment centre. */
    key: string;
    accountId: number;
}

/**
 * Everything an exchange of an Ozon posting needs, through the API: a seller at the fake Ozon on a key of its own (`change` alters
 * what `ozonSellerSetUp` gives), an Ozon account on that key, a fake fulfilment centre on a key of its own that takes orders of
 * Ozon, «Внешний API» filled with it, and the account's warehouse `OZON_WAREHOUSE` and marketplace code matched.
 */
export async function setUpOzonExchange(
    context: BrowserContext,
    server: IWyoServer,
    change: IFakeOzonSetup = {}
): Promise<IOzonExchangeSetUp> {
    const ozon = makeOzonKey();
    await setUpFakeOzon(ozon.apiKey, ozonSellerSetUp(ozon, change));
    const key = makeFcKey();
    await setUpFc(key, {
        marketplaces: [
            { code: FAKE_FC.marketplace, name: 'Wildberries' },
            { code: FAKE_FC_OZON, name: 'Ozon' }
        ]
    });
    const account = await addOzonAccountThroughApi(context, server, 'Магазин на Ozon', ozon);
    await connectExternalApiThroughApi(context, server, key);
    await matchExternalApiThroughApi(context, server, account.id, { marketplace: FAKE_FC_OZON });
    return { ozon, key, accountId: account.id };
}
