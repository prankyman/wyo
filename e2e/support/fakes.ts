import path from 'node:path';
import { startProgram, statusOf, waitForAnswer, type IProgram } from './process.js';

/**
 * Where the fakes listen (`cmd/wyofakes`, `docs/development.md`). The fake Wildberries and the fake Ozon share the first address:
 * a path that begins with the host of the fake Ozon, `api-seller.ozon.ru`, goes to it, and every other path to the fake Wildberries.
 */
export const FAKE_MARKETPLACES_ADDRESS = '127.0.0.1:8791';
export const FAKE_FULFILMENT_ADDRESS = '127.0.0.1:8792';

const WYOFAKES_BINARY = path.resolve(import.meta.dirname, '../../bin/wyofakes');

const healthOf = (address: string): string => `http://${address}/__control/health`;

/**
 * Starts the fake marketplaces (the fake Wildberries and the fake Ozon) and the fake fulfilment centre, one program for all, and
 * returns when both addresses answer `/__control/health`. When both already answer (a `pnpm dev` is running), they are used as they are and null is returned: the
 * fakes keep their state per key, so another run's keys are never touched, and stopping them is not this run's to do.
 */
export async function startFakes(): Promise<IProgram | null> {
    const running = await Promise.all(
        [healthOf(FAKE_MARKETPLACES_ADDRESS), healthOf(FAKE_FULFILMENT_ADDRESS)].map(statusOf)
    );
    if (running.every((status) => status === 200)) return null;

    const program = startProgram('the fakes', WYOFAKES_BINARY, [
        '--marketplaces',
        FAKE_MARKETPLACES_ADDRESS,
        '--fulfilment',
        FAKE_FULFILMENT_ADDRESS
    ]);
    await waitForAnswer(program, healthOf(FAKE_MARKETPLACES_ADDRESS));
    await waitForAnswer(program, healthOf(FAKE_FULFILMENT_ADDRESS));
    return program;
}
