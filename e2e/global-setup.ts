import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { startFakes } from './support/fakes.js';

/**
 * Before the run: the interface, the wyo server and the fakes are built (`scripts/e2e/build.sh`; the interface only when it is
 * out of date), and the fake Wildberries and the fake fulfilment centre are started once for the whole run. Playwright keeps this
 * process alive for the whole run, so the fakes live as long as it does; the function this returns stops them.
 *
 * The workers' wyo servers are not started here: each worker starts its own (`e2e/fixtures/test.ts`).
 */
export default async function globalSetup(): Promise<() => Promise<void>> {
    execFileSync('bash', [path.resolve(import.meta.dirname, '../scripts/e2e/build.sh')], { stdio: 'inherit' });
    const fakes = await startFakes();
    return async () => {
        await fakes?.stop();
    };
}
