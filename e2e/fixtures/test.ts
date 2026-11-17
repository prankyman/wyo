import path from 'node:path';
import { expect, test as base } from '@playwright/test';
import { recordRequests, type IRequestLog } from '../helpers/requests.js';
import { FIRST_WORKER_PORT, startWyoServer, type IWyoServer } from '../support/wyo-server.js';
import { attachConsoleCollector, reportConsoleErrors, type IConsoleError } from './console.js';

interface ITestFixtures {
    consoleErrors: IConsoleError[];
    /** Every request the test's browser context makes, from the start of the test (`e2e/helpers/requests.ts`). */
    requests: IRequestLog;
}

interface IWorkerFixtures {
    /** The worker's own wyo server, in test mode, on its own port and in its own empty data folder. */
    wyo: IWyoServer;
}

/**
 * The test every browser spec imports.
 *
 * - `wyo`: one wyo server per WORKER, started the first time a test needs it and stopped when the worker ends. It listens on port
 *   8800 + the worker's parallel index and runs in a data folder under the worker's output folder (`test-results/`, or the folder
 *   `--output` names), so workers share nothing. The parallel index is unique among the workers running at the same moment, and a
 *   restarted worker keeps it (https://playwright.dev/docs/test-parallel, "Worker index and parallel index"), so a port is never
 *   used by two servers.
 * - `baseURL`: the worker's wyo server, so `page.goto('/showcase')` opens the page of the worker's own server.
 * - `page`: every error the browser console prints, and every uncaught error of the page, FAILS the test that opened it.
 * - `requests`: where the test's pages went.
 *
 * A spec that needs no browser (a plain Node test) imports `test` from `@playwright/test` instead and so starts no server.
 */
export const test = base.extend<ITestFixtures, IWorkerFixtures>({
    wyo: [
        // eslint-disable-next-line no-empty-pattern
        async ({}, use, workerInfo) => {
            const dataDir = path.join(workerInfo.project.outputDir, 'wyo-data', `worker-${workerInfo.parallelIndex}`);
            const server = await startWyoServer(FIRST_WORKER_PORT + workerInfo.parallelIndex, dataDir);
            try {
                await use(server);
            } finally {
                await server.program.stop();
            }
        },
        { scope: 'worker' }
    ],
    baseURL: async ({ wyo }, use) => {
        await use(wyo.origin);
    },
    consoleErrors: [
        // eslint-disable-next-line no-empty-pattern
        async ({}, use) => {
            await use([]);
        },
        { auto: true }
    ],
    page: async ({ page, consoleErrors }, use, testInfo) => {
        attachConsoleCollector(page, consoleErrors);
        await use(page);
        reportConsoleErrors(consoleErrors, testInfo);
        expect(
            consoleErrors.map((error) => `[${error.type}] ${error.text}`),
            'the browser console printed errors'
        ).toEqual([]);
    },
    requests: async ({ context }, use) => {
        await use(recordRequests(context));
    }
});

export { expect };
