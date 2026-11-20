import { expect, type BrowserContext } from '@playwright/test';
import { fcCourierChecks, FAKE_FC, makeFcKey, setUpFc, type IFcCourierAnswer } from '../support/fake-fulfilment.js';
import {
    FAKE_YANDEX_CABINET,
    FAKE_YANDEX_WAREHOUSE_EXPRESS,
    FAKE_YANDEX_WAREHOUSE_FBS,
    makeYandexKey,
    setUpFakeYandex
} from '../support/fake-yandex.js';
import type { IWyoServer } from '../support/wyo-server.js';
import { addYandexAccountThroughApi } from './accounts.js';
import { connectExternalApiThroughApi, matchExternalApiThroughApi, runExchange } from './external-api.js';

/**
 * The exchange of Яндекс Маркет orders with the fake fulfilment centre, set up through the wyo server's own API for the specs that
 * walk it (`e2e/specs/exchange-yandex.spec.ts`). The walks themselves use the helpers of `external-api.ts` (`runExchangeUntil`,
 * `runExchangeUntilHandover`): the test build's «run every job at once» reads Яндекс Маркет's new and changed orders as well.
 */

/** The marketplace's code the fake fulfilment centre is told to take orders of, for the Яндекс Маркет account of `setUpYandexExchange`. */
export const FAKE_FC_YANDEX = 'yandex_market';

/** The day after today as `YYYY-MM-DD`: the shipment day the made-up orders carry, so a hand-over is for a day that has not passed. */
export function tomorrow(): string {
    const day = new Date(Date.now() + 24 * 60 * 60 * 1000);
    return day.toISOString().slice(0, 10);
}

export interface IYandexExchangeSetUp {
    /** The made-up Api-Key; the fake Яндекс Маркет keeps its orders under it. */
    yandexKey: string;
    /** The key of the fake fulfilment centre. */
    key: string;
    accountId: number;
}

/**
 * Everything the exchange of Яндекс Маркет orders needs, through the API: a cabinet at the fake on a key of its own
 * (`FAKE_YANDEX_CABINET`), a Яндекс Маркет account on that key, a fake fulfilment centre on a key of its own that takes orders of
 * `FAKE_FC_YANDEX`, «Внешний API» filled with it, and both warehouses of the cabinet and the marketplace's code matched.
 */
export async function setUpYandexExchange(context: BrowserContext, server: IWyoServer): Promise<IYandexExchangeSetUp> {
    const yandexKey = makeYandexKey();
    await setUpFakeYandex(yandexKey, FAKE_YANDEX_CABINET);
    const key = makeFcKey();
    await setUpFc(key, {
        marketplaces: [
            { code: FAKE_FC.marketplace, name: 'Wildberries' },
            { code: FAKE_FC_YANDEX, name: 'Яндекс Маркет' }
        ]
    });
    const account = await addYandexAccountThroughApi(context, server, 'Кабинет на Маркете', yandexKey);
    await connectExternalApiThroughApi(context, server, key);
    await matchExternalApiThroughApi(context, server, account.id, {
        marketplace: FAKE_FC_YANDEX,
        warehouseNumbers: [String(FAKE_YANDEX_WAREHOUSE_FBS), String(FAKE_YANDEX_WAREHOUSE_EXPRESS)]
    });
    return { yandexKey, key, accountId: account.id };
}

/**
 * Runs the exchange again and again until the fake fulfilment centre holds the wyo server's answer `result` to the check `checkId` of
 * the order `fcOrderId`, and returns the answer. Waits for the condition, never a fixed time.
 */
export async function runExchangeUntilCourierAnswer(
    context: BrowserContext,
    server: IWyoServer,
    key: string,
    fcOrderId: string,
    checkId: string,
    result: IFcCourierAnswer['result']
): Promise<IFcCourierAnswer> {
    const answerOf = async (): Promise<IFcCourierAnswer | null> =>
        (await fcCourierChecks(key, fcOrderId)).find((check) => check.check_id === checkId)?.answer ?? null;
    await expect
        .poll(
            async () => {
                await runExchange(context, server);
                return (await answerOf())?.result;
            },
            { message: `the check ${checkId} was never answered ${result}`, timeout: 30_000 }
        )
        .toBe(result);
    return (await answerOf())!;
}
