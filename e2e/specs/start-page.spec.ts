import { sweepScreen } from '../helpers/sweep.js';
import { expect, test } from '../fixtures/test.js';
import { screenOf } from '../support/screens.js';

test('the start page shows the name and the version the wyo server reports', async ({ page, request }) => {
    const meta = (await (await request.get('/api/v1/meta')).json()) as { name: string; version: string; mode: string };
    expect(meta.mode, 'the worker runs the wyo server in test mode').toBe('test');

    await page.goto('/');

    await expect(page.getByTestId('start-name')).toHaveText(meta.name);
    await expect(page.getByTestId('start-version')).toContainText(meta.version);
});

test('the start page fits the phone and stands level at every width', async ({ page }) => {
    await sweepScreen(page, screenOf('/'));
});
