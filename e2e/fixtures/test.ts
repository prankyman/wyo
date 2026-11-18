import path from 'node:path';
import { expect, test as base, type Page } from '@playwright/test';
import { signInThroughApi } from '../helpers/sign-in.js';
import { recordRequests, type IRequestLog } from '../helpers/requests.js';
import { FIRST_OWN_PORT, FIRST_WORKER_PORT, startWyoServer, type IWyoServer } from '../support/wyo-server.js';
import { attachConsoleCollector, reportConsoleErrors, type IConsoleError, type IExpectedRefusal } from './console.js';

/** Which wyo server a test runs against: the worker's shared one, or one started for this test alone. */
export type TServerKind = 'shared' | 'own';

interface ITestOptions {
    /**
     * `'shared'` (the default): the worker's own wyo server, started the first time a test needs it and used by every test of the
     * worker one after another. `'own'`: a wyo server started for the test and stopped at its end, in a data folder of its own. A
     * test that changes what other tests would find — the owner's password, a setting, the sign-in limit's count, the updater's
     * request — uses `'own'`, set for a file or a block with `test.use({ serverKind: 'own' })`.
     */
    serverKind: TServerKind;
}

interface ITestFixtures {
    consoleErrors: IConsoleError[];
    /** The refusals of the wyo server this test provokes on purpose; the browser's console line for each is not an error. */
    expectedRefusals: IExpectedRefusal[];
    /** Every request the test's browser context makes, from the start of the test (`e2e/helpers/requests.ts`). */
    requests: IRequestLog;
    /** The wyo server this test runs against (see `serverKind`): its address, its data folder and its owner. */
    server: IWyoServer;
    /** `page`, with the owner already signed in (`signInThroughApi`): for every spec that needs a signed-in page and is not about signing in. */
    ownerPage: Page;
}

interface IWorkerFixtures {
    /** Starts the worker's shared wyo server the first time it is called, and gives the same one afterwards. */
    sharedWyo: () => Promise<IWyoServer>;
}

/**
 * The test every browser spec imports.
 *
 * - `server`: the wyo server of the test, in test mode, with the owner made (`e2e/support/owner.ts`). The shared one of the worker
 *   listens on port 8800 + the worker's parallel index; one a test starts for itself (`serverKind: 'own'`) on 8900 + the index. The
 *   parallel index is unique among the workers running at the same moment, and a restarted worker keeps it
 *   (https://playwright.dev/docs/test-parallel, "Worker index and parallel index"), so a port is never used by two servers. Every
 *   server has a data folder of its own, so servers share nothing.
 * - `baseURL`: the test's server, so `page.goto('/settings')` opens the page of the test's own server.
 * - `page`: every error the browser console prints, and every uncaught error of the page, FAILS the test that opened it — except
 *   the console line of a refusal the test declared in `expectedRefusals`.
 * - `ownerPage`: `page` with the owner signed in.
 * - `requests`: where the test's pages went.
 *
 * A spec that needs no browser (a plain Node test) imports `test` from `@playwright/test` instead and so starts no server.
 */
export const test = base.extend<ITestOptions & ITestFixtures, IWorkerFixtures>({
    serverKind: ['shared', { option: true }],
    sharedWyo: [
        // eslint-disable-next-line no-empty-pattern
        async ({}, use, workerInfo) => {
            const dataDir = path.join(workerInfo.project.outputDir, 'wyo-data', `worker-${workerInfo.parallelIndex}`);
            let started: Promise<IWyoServer> | undefined;
            try {
                await use(() => (started ??= startWyoServer(FIRST_WORKER_PORT + workerInfo.parallelIndex, dataDir)));
            } finally {
                const server = await started?.catch(() => undefined);
                await server?.program.stop();
            }
        },
        { scope: 'worker' }
    ],
    server: async ({ serverKind, sharedWyo }, use, testInfo) => {
        if (serverKind === 'shared') {
            await use(await sharedWyo());
            return;
        }
        const own = await startWyoServer(
            FIRST_OWN_PORT + testInfo.parallelIndex,
            path.join(testInfo.outputDir, 'wyo-data')
        );
        try {
            await use(own);
        } finally {
            await own.program.stop();
        }
    },
    baseURL: async ({ server }, use) => {
        await use(server.origin);
    },
    consoleErrors: [
        // eslint-disable-next-line no-empty-pattern
        async ({}, use) => {
            await use([]);
        },
        { auto: true }
    ],
    expectedRefusals: [
        // eslint-disable-next-line no-empty-pattern
        async ({}, use) => {
            await use([]);
        },
        { auto: true }
    ],
    page: async ({ page, consoleErrors, expectedRefusals }, use, testInfo) => {
        attachConsoleCollector(page, consoleErrors, expectedRefusals);
        await use(page);
        reportConsoleErrors(consoleErrors, testInfo);
        expect(
            consoleErrors.map((error) => `[${error.type}] ${error.text}`),
            'the browser console printed errors'
        ).toEqual([]);
    },
    ownerPage: async ({ page, server }, use) => {
        await signInThroughApi(page.context(), server);
        await use(page);
    },
    requests: async ({ context }, use) => {
        await use(recordRequests(context));
    }
});

export { expect };
