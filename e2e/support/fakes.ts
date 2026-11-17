import path from 'node:path';
import { startProgram, statusOf, waitForAnswer, type IProgram } from './process.js';

/** Where the fakes listen (`cmd/wyofakes`, `docs/development.md`). */
export const FAKE_WILDBERRIES_ADDRESS = '127.0.0.1:8791';
export const FAKE_FULFILMENT_ADDRESS = '127.0.0.1:8792';

const WYOFAKES_BINARY = path.resolve(import.meta.dirname, '../../bin/wyofakes');

const healthOf = (address: string): string => `http://${address}/__control/health`;

/**
 * Starts the fake Wildberries and the fake fulfilment centre, one program for both, and returns when both answer
 * `/__control/health`. When both already answer (a `pnpm dev` is running), they are used as they are and null is returned: the
 * fakes keep their state per key, so another run's keys are never touched, and stopping them is not this run's to do.
 */
export async function startFakes(): Promise<IProgram | null> {
    const running = await Promise.all(
        [healthOf(FAKE_WILDBERRIES_ADDRESS), healthOf(FAKE_FULFILMENT_ADDRESS)].map(statusOf)
    );
    if (running.every((status) => status === 200)) return null;

    const program = startProgram('the fakes', WYOFAKES_BINARY, [
        '--wildberries',
        FAKE_WILDBERRIES_ADDRESS,
        '--fulfilment',
        FAKE_FULFILMENT_ADDRESS
    ]);
    await waitForAnswer(program, healthOf(FAKE_WILDBERRIES_ADDRESS));
    await waitForAnswer(program, healthOf(FAKE_FULFILMENT_ADDRESS));
    return program;
}
