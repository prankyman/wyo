import { expect, test } from '../fixtures/test.js';
import { sweepScreen } from '../helpers/sweep.js';
import { openScreen } from '../helpers/open-screen.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * «Обзор» (`/`): the strip of steps and the three cards of the installation. What each card says in each state is proved once by
 * the unit tests of `installationCards`; here the page is read from the real wyo server's answer (`GET /api/v1/installation`).
 */
test('«Обзор» fits the phone and stands level at every width', async ({ ownerPage: page }) => {
    await sweepScreen(page, screenOf('/'));
});

test('«Обзор» shows where the seller stands and the three cards of a fresh installation', async ({
    ownerPage: page,
    request
}) => {
    const meta = (await (await request.get('/api/v1/meta')).json()) as { version: string; mode: string };
    expect(meta.mode, 'the worker runs the wyo server in test mode').toBe('test');

    await openScreen(page, '/');

    await expect(page.getByTestId('overview-heading')).toHaveText(ruText('overview.title'));
    await expect(page.getByTestId('overview-steps-now')).toHaveText(ruText('overview.now.keys'));
    await expect(page.getByTestId('overview-next')).toHaveText(ruText('overview.next'));

    const version = page.getByTestId('overview-card-version');
    await expect(version.getByTestId('overview-card-version-state')).toHaveText(ruText('overview.version.ok'));
    await expect(version.getByTestId('overview-card-version-sentence')).toContainText(`версия ${meta.version}`);
    // A server in test mode has no domain: no certificate is asked for. It has made no copy yet.
    await expect(page.getByTestId('overview-card-certificate-state')).toHaveText(ruText('overview.certificate.none'));
    await expect(page.getByTestId('overview-card-backups-state')).toHaveText(ruText('overview.backups.none'));
    await expect(page.getByTestId('nav-version')).toHaveText(ruText('shell.nav.version', { version: meta.version }));
});

test('the action of a card opens «Настройки»', async ({ ownerPage: page }) => {
    await openScreen(page, '/');

    await page.getByTestId('overview-card-version-action').click();

    await expect(page.getByTestId('settings-sections')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/settings');
});
