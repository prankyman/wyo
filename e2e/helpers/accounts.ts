import { expect, type BrowserContext } from '@playwright/test';
import type { IWyoServer } from '../support/wyo-server.js';

/**
 * Marketplace accounts and readings made through the wyo server's own API, for the specs that need an account or orders but are
 * not about adding one: the form «Добавить аккаунт» is walked once, by `e2e/specs/marketplaces.spec.ts`. The calls are made by the
 * context's own request object, which carries the signed-in owner's session (`signInThroughApi`). The key goes only to the wyo
 * server, never anywhere else.
 */

export interface IAddedAccount {
    id: number;
    name: string;
    keyLast4: string;
}

/** Adds a Wildberries account with `token` (`POST /api/v1/accounts`); throws with the wyo server's answer when it refuses. */
export async function addAccountThroughApi(
    context: BrowserContext,
    server: IWyoServer,
    name: string,
    token: string
): Promise<IAddedAccount> {
    const response = await context.request.post(`${server.origin}/api/v1/accounts`, {
        data: { marketplace: 'wildberries', name, key: { token } }
    });
    if (response.status() !== 201) {
        throw new Error(`the account "${name}" was refused with status ${response.status()}: ${await response.text()}`);
    }
    return (await response.json()) as IAddedAccount;
}

/** The number of orders the wyo server holds (`GET /api/v1/orders`). */
async function orderTotal(context: BrowserContext, server: IWyoServer): Promise<number> {
    const response = await context.request.get(`${server.origin}/api/v1/orders`);
    return ((await response.json()) as { total: number }).total;
}

/**
 * «Обновить» through the API (`POST /api/v1/sync/refresh`), then waits until the wyo server holds `total` orders: the readings go
 * on after the answer, so the condition is asked again until it holds, never after a fixed time.
 */
export async function readOrdersThroughApi(context: BrowserContext, server: IWyoServer, total: number): Promise<void> {
    const response = await context.request.post(`${server.origin}/api/v1/sync/refresh`, { data: {} });
    expect(response.status(), 'the wyo server started the readings').toBe(202);
    await expect
        .poll(() => orderTotal(context, server), {
            message: `the wyo server never held ${total} orders`,
            timeout: 20_000
        })
        .toBe(total);
}
