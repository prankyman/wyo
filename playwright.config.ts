import { defineConfig } from '@playwright/test';

/**
 * The browser tests run against the BUILT wyo server (`bin/wyo-test`, made by `scripts/e2e/build.sh` in the global setup), never
 * against the Vite dev server. Every worker starts a wyo server of its own in test mode on its own port with its own data
 * folder (`e2e/fixtures/test.ts`), and the two fakes are started once for the whole run (`e2e/global-setup.ts`), so workers share
 * nothing but the fakes, which keep their state per key. `e2e/README.md` has the details.
 *
 * A failed test is never retried: a retry would hide the failure instead of showing it.
 */
export default defineConfig({
    testDir: './e2e/specs',
    globalSetup: './e2e/global-setup.ts',
    use: {
        testIdAttribute: 'data-test',
        locale: 'ru-RU',
        viewport: { width: 1280, height: 900 },
        // The visitor asks for less motion, as a person can: an animation then needs no pause in a test.
        reducedMotion: 'reduce',
        // A failed test leaves its screenshot and trace in test-results/.
        screenshot: 'only-on-failure',
        trace: 'retain-on-failure'
    },
    // Each worker has its own wyo server, port and data folder, so the number of workers only decides how fast the run is.
    workers: Number(process.env.E2E_WORKERS ?? 2),
    retries: 0,
    reporter: [['list'], ['html', { open: 'never' }]],
    projects: [{ name: 'chromium', use: { browserName: 'chromium' } }]
});
