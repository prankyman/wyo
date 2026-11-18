import { spawn, type ChildProcess } from 'node:child_process';
import http from 'node:http';

/**
 * The one way the browser tests start, wait for and stop a program of this repository (the wyo server of a worker, the fakes of
 * the run). A program is waited for by asking it, never by waiting a fixed time: `waitForAnswer` asks the address again until the
 * program answers, and gives up at once, with everything the program printed, when the program has already ended.
 */

/** The most output of a program that is kept (the end of it), characters. */
const MAX_OUTPUT = 20_000;

/** A program started for a test run. */
export interface IProgram {
    name: string;
    /** What the program printed so far (standard output and error together), the end of it. */
    output(): string;
    /** True once the program has ended. */
    ended(): boolean;
    /** Resolves with the program's exit code once it has ended (null when it was ended by a signal or could not start). */
    finished(): Promise<number | null>;
    /** Asks the program to stop, and waits until it has; ends it by force when it does not stop in time. */
    stop(): Promise<void>;
}

export interface IStartOptions {
    cwd?: string;
    env?: NodeJS.ProcessEnv;
}

/** Starts `command` and keeps what it prints. */
export function startProgram(
    name: string,
    command: string,
    args: readonly string[],
    options: IStartOptions = {}
): IProgram {
    const child: ChildProcess = spawn(command, [...args], {
        cwd: options.cwd,
        env: options.env ?? process.env,
        stdio: ['ignore', 'pipe', 'pipe']
    });
    let printed = '';
    let ended = false;
    let exitCode: number | null = null;
    const keep = (chunk: Buffer): void => {
        printed = (printed + chunk.toString('utf8')).slice(-MAX_OUTPUT);
    };
    child.stdout?.on('data', keep);
    child.stderr?.on('data', keep);
    const exited = new Promise<void>((resolve) => {
        child.once('exit', (code) => {
            ended = true;
            exitCode = code;
            resolve();
        });
        child.once('error', (error) => {
            printed += `\n${name} could not be started: ${error.message}`;
            ended = true;
            resolve();
        });
    });

    return {
        name,
        output: () => printed,
        ended: () => ended,
        finished: () => exited.then(() => exitCode),
        async stop() {
            if (ended) return;
            child.kill('SIGTERM');
            const stopped = await Promise.race([exited.then(() => true), delay(5_000).then(() => false)]);
            if (!stopped) {
                child.kill('SIGKILL');
                await exited;
            }
        }
    };
}

/** What a program that was run to its end printed, and how it ended. */
export interface IProgramResult {
    code: number | null;
    output: string;
}

/**
 * Runs `command` to its end and returns its exit code and what it printed. For a command that does one job and stops (creating the
 * owner of a wyo server), as `startProgram` is for one that keeps running. Gives up, ending the command, after `timeoutMs`.
 */
export async function runProgram(
    name: string,
    command: string,
    args: readonly string[],
    options: IStartOptions = {},
    timeoutMs = 30_000
): Promise<IProgramResult> {
    const program = startProgram(name, command, args, options);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const timedOut = new Promise<null>((resolve) => {
        timer = setTimeout(() => resolve(null), timeoutMs);
    });
    const outcome = await Promise.race([program.finished().then((code) => ({ code })), timedOut]);
    clearTimeout(timer);
    if (outcome === null) {
        await program.stop();
        return { code: null, output: `${name} did not end within ${timeoutMs} ms.\n${program.output()}` };
    }
    return { code: outcome.code, output: program.output() };
}

const delay = (milliseconds: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, milliseconds));

/** The status of a GET to `url`, or null when nothing answers there. */
export function statusOf(url: string): Promise<number | null> {
    return new Promise((resolve) => {
        const request = http.get(url, { timeout: 2_000 }, (response) => {
            response.resume();
            resolve(response.statusCode ?? null);
        });
        request.once('timeout', () => request.destroy());
        request.once('error', () => resolve(null));
    });
}

/** How long to rest between two questions to a program that has not answered yet, milliseconds. */
const ASK_AGAIN_AFTER = 50;

/**
 * Asks `url` until it answers 200 (the program is up). Throws, with the program's own output, when the program ends first or
 * when `timeoutMs` pass.
 */
export async function waitForAnswer(program: IProgram, url: string, timeoutMs = 20_000): Promise<void> {
    const deadline = Date.now() + timeoutMs;
    for (;;) {
        if ((await statusOf(url)) === 200) return;
        if (program.ended()) {
            throw new Error(`${program.name} ended before it answered ${url}.\n${program.output()}`);
        }
        if (Date.now() > deadline) {
            await program.stop();
            throw new Error(`${program.name} did not answer ${url} within ${timeoutMs} ms.\n${program.output()}`);
        }
        await delay(ASK_AGAIN_AFTER);
    }
}
