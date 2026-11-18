import { randomUUID } from 'node:crypto';
import { request } from '@playwright/test';
import { FAKE_MARKETPLACES_ADDRESS } from './fakes.js';

/**
 * The browser tests' side of the fake Ozon (`internal/fakes/ozon`, `docs/development.md`, «The fakes»): made-up keys and the fake's
 * control calls. Fake-only: nothing here ever reaches the real Ozon (rule R3).
 *
 * The fake Ozon shares an address with the fake Wildberries and is told apart by the first part of the path, the host of Ozon's
 * Seller API. It keeps a state of its own for every API key (the `Authorization` header of a control call, the `Api-Key` header of
 * the wyo server's calls), so every test makes a key of its own with `makeOzonKey` and the tests running side by side never share a
 * state. The field names below are the control calls' own, as `internal/fakes/ozon/control.go` lists them.
 */

/** The host of Ozon's Seller API: the first part of every path the fake Ozon answers (`marketplace.Origin`). */
export const FAKE_OZON_HOST = 'api-seller.ozon.ru';

/** A made-up Ozon key: the two fields the seller types, each new on every call. */
export interface IFakeOzonKey {
    clientId: string;
    apiKey: string;
}

/** A key and Client ID of no real account, new on every call, so no two tests share the state the fake keeps per key. */
export function makeOzonKey(): IFakeOzonKey {
    return { clientId: String(100000 + Math.floor(Math.random() * 900000)), apiKey: randomUUID() };
}

/** What makes the fake refuse every method but `/v1/roles` (`key_state` of the setup); `''` lifts it. */
export type FakeOzonKeyState = '' | 'deactivated' | 'ip_restricted' | 'offer_not_signed';

export interface IFakeOzonRole {
    name: string;
    /** The methods the role opens: whole paths, or beginnings of paths that end at a `/`, as `/v1/roles` lists them. */
    methods: string[];
}

export interface IFakeOzonWarehouse {
    id: number;
    name: string;
    is_rfbs?: boolean;
    /** The label type of the warehouse (`carriage_label_type`); `UNSPECIFIED` when left out. */
    label_type?: 'UNSPECIFIED' | 'BIG' | 'SMALL';
    /** Ozon's automatic assembly (`is_auto_assembly`). */
    auto_assembly?: boolean;
}

/**
 * A product of the seller's catalogue. What the product asks of the seller before a posting of it may be packed
 * (`internal/fakes/ozon/needs.go`): `mark` says whether a marking code is needed (every unit must carry one) or only possible; a
 * product with none takes no code, and sending one is Ozon's refusal.
 */
export interface IFakeOzonProduct {
    sku: number;
    offer_id: string;
    name: string;
    barcodes?: string[];
    mark?: 'needed' | 'possible';
}

/**
 * What `/__control/setup` may change; each part left out stays as it is. The key is refused as an invalid one until a setup gives
 * it the `client_id` its calls carry. A key set up with nothing else is an `Admin` for three months of a seller named
 * «Test seller», with no warehouses and no products.
 */
export interface IFakeOzonSetup {
    client_id?: string;
    seller_name?: string;
    /** When the key ends (RFC 3339); from that moment the fake answers every method but `/v1/roles` as a deactivated key. */
    expires_at?: string;
    roles?: IFakeOzonRole[];
    key_state?: FakeOzonKeyState;
    warehouses?: IFakeOzonWarehouse[];
    products?: IFakeOzonProduct[];
}

/** A product line of a posting: units of one product. */
export interface IFakeOzonLine {
    sku: number;
    offer_id: string;
    name: string;
    quantity: number;
}

/** The statuses of an FBS posting (OZC section 6: `.agents/research/ozon-official-seller-api-check-2026-11-18.md`). */
export type FakeOzonFbsStatus =
    | 'acceptance_in_progress'
    | 'arbitration'
    | 'awaiting_approve'
    | 'awaiting_deliver'
    | 'awaiting_packaging'
    | 'awaiting_registration'
    | 'awaiting_verification'
    | 'cancelled'
    | 'cancelled_from_split_pending'
    | 'client_arbitration'
    | 'delivered'
    | 'delivering'
    | 'driver_pickup'
    | 'not_accepted';

/** The statuses of an FBO posting. */
export type FakeOzonFboStatus = 'awaiting_packaging' | 'awaiting_deliver' | 'delivering' | 'delivered' | 'cancelled';

/** Who cancelled a posting (`cancellation_type` of Ozon's file). */
export type FakeOzonCancellationType = 'seller' | 'client' | 'customer' | 'ozon' | 'system' | 'delivery';

/** A new FBS posting; the fake adds the made-up buyer to every answer that carries it. */
export interface IFakeOzonPosting {
    posting_number: string;
    order_number: string;
    /** One of the warehouses the key was set up with. */
    warehouse_id: number;
    /** `awaiting_packaging` when left out. */
    status?: FakeOzonFbsStatus;
    /** RFC 3339; now when left out. */
    in_process_at?: string;
    /** The deadline of assembly, RFC 3339. */
    shipment_date: string;
    products: IFakeOzonLine[];
}

/**
 * What the next sets of marking codes of a posting end in (`POST /__control/postings/exemplar-check`): `error_sets` of them end
 * with every mark failed and the error codes `error_codes`, so Ozon refuses the codes with its own words.
 */
export interface IFakeOzonCodeCheck {
    posting_number: string;
    error_sets: number;
    error_codes: string[];
}

export interface IFakeOzonStatusChange {
    posting_number: string;
    status: FakeOzonFbsStatus;
    /** Replaces the substatus: left out, it becomes empty. */
    substatus?: string;
    /** Fills the cancellation; only with the status `cancelled`. */
    cancellation_type?: FakeOzonCancellationType;
    /** Whether the cancellation came after the posting was packed. */
    cancelled_after_ship?: boolean;
}

/** The split made in the seller's cabinet: the parts hold exactly the products and units the posting had. */
export interface IFakeOzonSplit {
    posting_number: string;
    parts: { posting_number: string; products: { sku: number; quantity: number }[] }[];
}

/** A new FBO posting. */
export interface IFakeOzonFboPosting {
    posting_number: string;
    order_number: string;
    status: FakeOzonFboStatus;
    /** RFC 3339. */
    created_at: string;
    /** RFC 3339; the time of creation when left out. */
    in_process_at?: string;
    products: IFakeOzonLine[];
}

const HOUR_MS = 60 * 60 * 1000;

/** A moment `hours` hours from now, written as the fake reads it (RFC 3339), for a deadline or a time of creation. */
export function hoursFromNow(hours: number): string {
    return new Date(Date.now() + hours * HOUR_MS).toISOString();
}

/**
 * One control call of the fake Ozon for the state of `apiKey`, sent from the test's own process (never from the page, so the
 * page's requests stay the wyo server's only); throws with the fake's answer when it refuses.
 */
async function control(apiKey: string, path: string, body?: unknown): Promise<void> {
    const fake = await request.newContext({ baseURL: `http://${FAKE_MARKETPLACES_ADDRESS}` });
    try {
        const response = await fake.post(`/${FAKE_OZON_HOST}/__control/${path}`, {
            headers: { Authorization: apiKey },
            data: body
        });
        if (!response.ok()) {
            throw new Error(
                `the fake Ozon refused /__control/${path} with ${response.status()}: ${JSON.stringify(await response.json())}`
            );
        }
    } finally {
        await fake.dispose();
    }
}

/** Sets the key up (`/__control/setup`). */
export async function setUpFakeOzon(apiKey: string, setup: IFakeOzonSetup): Promise<void> {
    await control(apiKey, 'setup', setup);
}

/** A new FBS posting of the key's seller. */
export async function makeFakeOzonPosting(apiKey: string, posting: IFakeOzonPosting): Promise<void> {
    await control(apiKey, 'postings', posting);
}

/** What Ozon has done with a posting: a new status, and for a cancellation who cancelled. */
export async function setFakeOzonPostingStatus(apiKey: string, change: IFakeOzonStatusChange): Promise<void> {
    await control(apiKey, 'postings/status', change);
}

/** The split of a posting made in the seller's cabinet: the original is cancelled, each part is a new posting with its parent. */
export async function splitFakeOzonPosting(apiKey: string, split: IFakeOzonSplit): Promise<void> {
    await control(apiKey, 'postings/split', split);
}

/** A new deadline of assembly (`shipment_date`, RFC 3339) for a posting. */
export async function setFakeOzonDeadline(apiKey: string, postingNumber: string, shipmentDate: string): Promise<void> {
    await control(apiKey, 'postings/deadline', { posting_number: postingNumber, shipment_date: shipmentDate });
}

/** Makes the next sets of marking codes of a posting end in a refusal with Ozon's error codes. */
export async function planFakeOzonCodeCheck(apiKey: string, check: IFakeOzonCodeCheck): Promise<void> {
    await control(apiKey, 'postings/exemplar-check', check);
}

/** One call the fake Ozon received, without its key (`GET /__control/calls`). */
export interface IFakeOzonCall {
    method: string;
    path: string;
    status: number;
}

/** Every call the fake Ozon received for the state of `apiKey`, oldest first, so a test can tell what the wyo server wrote and how often. */
export async function fakeOzonCalls(apiKey: string): Promise<IFakeOzonCall[]> {
    const fake = await request.newContext({ baseURL: `http://${FAKE_MARKETPLACES_ADDRESS}` });
    try {
        const response = await fake.get(`/${FAKE_OZON_HOST}/__control/calls`, { headers: { Authorization: apiKey } });
        if (!response.ok()) throw new Error(`the fake Ozon refused /__control/calls with ${response.status()}`);
        return ((await response.json()) as { calls: IFakeOzonCall[] }).calls;
    } finally {
        await fake.dispose();
    }
}

/** A new FBO posting of the key's seller. */
export async function addFakeOzonFboPosting(apiKey: string, posting: IFakeOzonFboPosting): Promise<void> {
    await control(apiKey, 'fbo-postings', posting);
}

/** Empties the key's state, call list and faults: the key is refused as an invalid one until it is set up again. */
export async function resetFakeOzon(apiKey: string): Promise<void> {
    await control(apiKey, 'reset');
}
