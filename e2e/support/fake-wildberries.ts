import { randomUUID } from 'node:crypto';
import { request } from '@playwright/test';
import { FAKE_WILDBERRIES_ADDRESS } from './fakes.js';

/**
 * The browser tests' side of the fake Wildberries (`internal/fakes/wildberries`, `docs/development.md`, «The fakes»): made-up
 * tokens in Wildberries' own shape and the fake's control calls. Fake-only: nothing here ever reaches a real marketplace (rule R3).
 *
 * The fake keeps a state of its own for every token (the `Authorization` header), so every test makes a token of its own with
 * `makeToken` — each one carries a new random id — and the tests running side by side never share a state.
 */

/** The seller the fake answers for every token until a test sets another (`newState` of `internal/fakes/wildberries/state.go`). */
export const FAKE_SELLER = { id: '00000000-0000-0000-0000-000000000001', name: 'Test seller' } as const;

/** The field `acc` of a token: its type (`TokenType` of `internal/fakes/wildberries/token.go`). */
export const TOKEN_TYPE = { base: 1, test: 2, personal: 3, service: 4 } as const;

/** The bits of the field `s` the connector reads: the categories «Маркетплейс» and «Статистика», and «только на чтение». */
export const CATEGORY = { marketplace: 1 << 4, statistics: 1 << 5 } as const;
const READ_ONLY_BIT = 1 << 30;

const DAY_MS = 24 * 60 * 60 * 1000;

export interface ITokenClaims {
    /** The token's type; a Personal token when left out. */
    type?: (typeof TOKEN_TYPE)[keyof typeof TOKEN_TYPE];
    /** The categories it opens; «Маркетплейс» and «Статистика» when left out. */
    categories?: number;
    /** When it stops working; 180 days from now when left out. */
    expiresAt?: Date;
    readOnly?: boolean;
    /** The seller profile it belongs to; the fake's own seller when left out. */
    seller?: string;
}

const base64url = (text: string): string => Buffer.from(text, 'utf8').toString('base64url');

/**
 * A made-up token in the shape of Wildberries' own, as `MakeToken` of `internal/fakes/wildberries/token.go` writes it: a JWT whose
 * middle part holds id, sid, acc, s, t, exp and, for a Personal or a Service token, for. The signature is made up too: neither the
 * fake nor the wyo server checks it.
 */
export function makeToken(claims: ITokenClaims = {}): string {
    const type = claims.type ?? TOKEN_TYPE.personal;
    const categories =
        (claims.categories ?? CATEGORY.marketplace | CATEGORY.statistics) | (claims.readOnly ? READ_ONLY_BIT : 0);
    const expiresAt = claims.expiresAt ?? new Date(Date.now() + 180 * DAY_MS);
    const payload: Record<string, string | number | boolean> = {
        id: randomUUID(),
        sid: claims.seller ?? FAKE_SELLER.id,
        acc: type,
        s: categories,
        t: type === TOKEN_TYPE.test,
        exp: Math.floor(expiresAt.getTime() / 1000)
    };
    if (type === TOKEN_TYPE.personal) payload.for = 'self';
    if (type === TOKEN_TYPE.service) payload.for = 'asid:fake-service';
    return [
        base64url(JSON.stringify({ alg: 'ES256', typ: 'JWT' })),
        base64url(JSON.stringify(payload)),
        base64url('fake-signature')
    ].join('.');
}

/** A moment `days` days from now, less an hour, so the count of days left, rounded up, is exactly `days`. */
export function daysFromNow(days: number): Date {
    return new Date(Date.now() + days * DAY_MS - 60 * 60 * 1000);
}

/**
 * One control call of the fake for the state of `token`, sent from the test's own process (never from the page, so the page's
 * requests stay the wyo server's only); throws with the fake's answer when it refuses.
 */
async function control(token: string, path: string, body: unknown): Promise<unknown> {
    const fake = await request.newContext({ baseURL: `http://${FAKE_WILDBERRIES_ADDRESS}` });
    try {
        const response = await fake.post(`/__control/${path}`, { headers: { Authorization: token }, data: body });
        const answer: unknown = await response.json();
        if (!response.ok()) {
            throw new Error(
                `the fake Wildberries refused /__control/${path} with ${response.status()}: ${JSON.stringify(answer)}`
            );
        }
        return answer;
    } finally {
        await fake.dispose();
    }
}

/** What `/__control/setup` may change; `token_refused` makes every call of the token answer 401 (`''` lifts it). */
export interface IFakeSetup {
    token_refused?: 'expired' | 'invalid' | '';
}

export async function setUpFake(token: string, setup: IFakeSetup): Promise<void> {
    await control(token, 'setup', setup);
}

/** A new assembly order (FBS) of the token's seller; returns the fake's number of it. */
export async function makeFakeOrder(token: string, order: { article: string; skus: string[] }): Promise<number> {
    const answer = (await control(token, 'orders', order)) as { id: number };
    return answer.id;
}

/** What Wildberries has done with an order: `declined_by_client` is the buyer's cancellation. */
export async function setFakeWbStatus(token: string, orderId: number, wbStatus: string): Promise<void> {
    await control(token, `orders/${orderId}/wb-status`, { wb_status: wbStatus });
}

/** A row of the orders feed of an order Wildberries assembles from its own warehouse (FBO). */
export interface IFakeFeedRow {
    srid: string;
    supplier_article: string;
    barcode: string;
    warehouse_name: string;
}

export async function addFakeFboOrder(token: string, row: IFakeFeedRow): Promise<void> {
    await control(token, 'feed', { rows: [{ ...row, warehouse_type: 'Склад WB', nm_id: 55555555 }] });
}
