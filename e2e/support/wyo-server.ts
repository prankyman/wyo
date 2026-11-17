import { mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { startProgram, waitForAnswer, type IProgram } from './process.js';

/** The first port of the workers' wyo servers: worker 0 listens on 8800, worker 1 on 8801. */
export const FIRST_WORKER_PORT = 8800;

/** The wyo server built with the wyotest tag (`scripts/e2e/build.sh`). */
const WYO_TEST_BINARY = path.resolve(import.meta.dirname, '../../bin/wyo-test');

export interface IWyoServer {
    /** Where the browser opens it: `http://127.0.0.1:<port>`. */
    origin: string;
    port: number;
    /** The folder the wyo server runs in; it is empty when the server starts. */
    dataDir: string;
    program: IProgram;
}

/**
 * Starts one wyo server in test mode on `port`, running in the empty folder `dataDir`, and returns when `GET /api/v1/meta`
 * answers. The server is given no settings but its flags: nothing a developer has set in the environment reaches it.
 */
export async function startWyoServer(port: number, dataDir: string): Promise<IWyoServer> {
    rmSync(dataDir, { recursive: true, force: true });
    mkdirSync(dataDir, { recursive: true });
    const origin = `http://127.0.0.1:${port}`;
    const environment = { ...process.env };
    delete environment.WYO_LISTEN;
    delete environment.WYO_MODE;
    const program = startProgram(
        `the wyo server of port ${port}`,
        WYO_TEST_BINARY,
        ['--listen', `127.0.0.1:${port}`, '--mode', 'test'],
        { cwd: dataDir, env: environment }
    );
    await waitForAnswer(program, `${origin}/api/v1/meta`);
    return { origin, port, dataDir, program };
}
