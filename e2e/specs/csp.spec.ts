import { expect, test } from '../fixtures/test.js';
import { openAllSections } from '../helpers/sections.js';
import { visitScreen } from '../helpers/open-screen.js';
import { policyViolations, watchPolicyViolations } from '../helpers/policy-violations.js';
import { needsSignIn, SCREENS } from '../support/screens.js';

/**
 * The page's security policy (`svelte.config.js`, `kit.csp`): the wyo interface runs its own scripts and loads nothing from any other
 * address. On every page of the screens table — once drawn, with its folded sections open and the bell's panel open — the browser
 * reported no refusal by the policy. The console fixture already fails a test for a refusal the browser prints; this spec also
 * listens to the refusal event itself, which catches the ones the browser keeps out of its console.
 */
test('the policy is in force: an inline script added to a page is refused, and the listener sees it', async ({
    browser,
    server
}) => {
    // A page of its own, outside the fixtures: the browser prints this refusal to its console, which would fail the test.
    const probe = await browser.newPage({ baseURL: server.origin });
    try {
        await watchPolicyViolations(probe);
        await probe.goto('/auth/login');
        await expect(probe.getByTestId('login-send')).toBeVisible();
        expect(await policyViolations(probe), 'the page itself draws without a refusal').toEqual([]);

        await probe.evaluate(() => {
            const script = document.createElement('script');
            script.textContent = 'window.__ran = true';
            document.head.appendChild(script);
        });

        expect((await policyViolations(probe)).map((violation) => violation.directive)).toEqual(['script-src-elem']);
        expect(await probe.evaluate(() => (window as unknown as { __ran?: boolean }).__ran)).toBeUndefined();
    } finally {
        await probe.close();
    }
});

for (const row of SCREENS) {
    test(`${row.route}: the policy refuses nothing the page does`, async ({ page, server }) => {
        await watchPolicyViolations(page);
        await visitScreen(page, server, row.route);
        await openAllSections(page);
        if (needsSignIn(row.route)) {
            await page.getByTestId('notification-bell').click();
            await expect(page.getByTestId('notification-panel-body')).toBeVisible();
        }

        expect(await policyViolations(page)).toEqual([]);
    });
}
