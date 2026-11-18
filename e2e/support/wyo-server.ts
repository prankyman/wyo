import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { FAKE_WILDBERRIES_ADDRESS } from './fakes.js';
import { TEST_OWNER, type IOwner } from './owner.js';
import { runProgram, startProgram, waitForAnswer, type IProgram } from './process.js';

/** The first port of the workers' shared wyo servers: worker 0 listens on 8800, worker 1 on 8801. */
export const FIRST_WORKER_PORT = 8800;

/** The first port of the servers a single test starts for itself (`serverKind: 'own'`): worker 0 listens on 8900, worker 1 on 8901. */
export const FIRST_OWN_PORT = 8900;

/** The wyo server built with the wyotest tag (`scripts/e2e/build.sh`). */
const WYO_TEST_BINARY = path.resolve(import.meta.dirname, '../../bin/wyo-test');

export interface IWyoServer {
    /** Where the browser opens it: `http://127.0.0.1:<port>`. */
    origin: string;
    port: number;
    /** The folder the wyo server runs in: its database, key file and copies. It holds only the owner when the server starts. */
    dataDir: string;
    /** The person who can sign in to it. */
    owner: IOwner;
    program: IProgram;
}

/** The wyo server's own environment: nothing a developer has set reaches it, so a run is the same on every machine. */
function cleanEnvironment(): NodeJS.ProcessEnv {
    const environment = { ...process.env };
    for (const name of Object.keys(environment)) {
        if (name.startsWith('WYO_')) delete environment[name];
    }
    return environment;
}

/**
 * Makes the owner in the empty folder `dataDir` with the wyo server's own command (`wyo owner create`), before the server runs
 * in it. The password goes in a file that exists only for the command, as the command asks for it, never on the command line.
 */
async function createOwner(dataDir: string, owner: IOwner): Promise<void> {
    const passwordFile = `${dataDir}.owner-password`;
    writeFileSync(passwordFile, `${owner.password}\n`, { mode: 0o600 });
    try {
        const result = await runProgram(
            'the creation of the owner',
            WYO_TEST_BINARY,
            ['owner', 'create', '--login', owner.login, '--password-file', passwordFile, '--data', dataDir],
            { env: cleanEnvironment() }
        );
        if (result.code !== 0) {
            throw new Error(`the owner could not be created (exit code ${result.code}).\n${result.output}`);
        }
    } finally {
        rmSync(passwordFile, { force: true });
    }
}

/**
 * Starts one wyo server in test mode on `port`, in the empty folder `dataDir`, with the owner already made, and returns when
 * `GET /api/v1/meta` answers. The server is given no settings but its flags and the address of the fake Wildberries
 * (`WYO_MARKETPLACE_URL`), so it asks the fake and never a real marketplace.
 */
export async function startWyoServer(port: number, dataDir: string, owner: IOwner = TEST_OWNER): Promise<IWyoServer> {
    rmSync(dataDir, { recursive: true, force: true });
    mkdirSync(dataDir, { recursive: true });
    await createOwner(dataDir, owner);
    const origin = `http://${HOST}:${port}`;
    return { origin, port, dataDir, owner, program: await runWyoServer(port, dataDir) };
}

const HOST = '127.0.0.1';

/** Runs the wyo server on `port` in `dataDir` as the folder stands, and returns when `GET /api/v1/meta` answers. */
async function runWyoServer(port: number, dataDir: string): Promise<IProgram> {
    const program = startProgram(
        `the wyo server of port ${port}`,
        WYO_TEST_BINARY,
        ['--listen', `${HOST}:${port}`, '--mode', 'test', '--data', dataDir],
        { cwd: dataDir, env: { ...cleanEnvironment(), WYO_MARKETPLACE_URL: `http://${FAKE_WILDBERRIES_ADDRESS}` } }
    );
    await waitForAnswer(program, `http://${HOST}:${port}/api/v1/meta`);
    return program;
}

/**
 * Stops `server` and runs it again on the same port in the same data folder: everything it stored stays — the owner, the session,
 * the accounts, the orders — and what a wyo server holds only while it runs starts afresh, such as the moment «Обновить» was last
 * answered (the 5-minute rule). For a test that needs a second reading without waiting five minutes. `server.program` is replaced
 * in place, so whoever stops the server at the end of the test stops the new program.
 */
export async function restartWyoServer(server: IWyoServer): Promise<void> {
    await server.program.stop();
    server.program = await runWyoServer(server.port, server.dataDir);
}
