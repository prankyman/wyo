import { expect, test } from '../fixtures/test.js';
import { openScreen } from '../helpers/open-screen.js';
import { openSection } from '../helpers/sections.js';
import { signInThroughApi } from '../helpers/sign-in.js';
import { ruText } from '../support/texts.js';

/**
 * Signing out, from «Мой профиль»: «Выйти» ends this browser's session; «Выйти на всех устройствах», behind a window that says what
 * it does, ends every session. The wyo server's side (the session is gone from the database, the cookie is cleared) is proved by
 * the Go tests; here a person presses the buttons and the next call of each browser is answered.
 */
test("«Выйти» ends this browser's session and opens the sign-in page", async ({
    ownerPage: page,
    server,
    expectedRefusals
}) => {
    expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' });
    await openScreen(page, '/account');
    await openSection(page, 'account-signout');

    await page.getByTestId('account-sign-out').click();

    await expect(page.getByTestId('login-send')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/auth/login');
    expect((await page.context().request.get(`${server.origin}/api/v1/session`)).status()).toBe(401);
    // Opening a page of the program now meets the sign-in page again.
    await page.goto('/settings');
    await expect(page.getByTestId('login-send')).toBeVisible();
});

test('«Выйти на всех устройствах» asks first, and then ends the session of every browser', async ({
    ownerPage: page,
    server,
    browser,
    expectedRefusals
}) => {
    expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' });
    const other = await browser.newContext();
    try {
        await signInThroughApi(other, server);
        await openScreen(page, '/account');
        await openSection(page, 'account-signout');

        await page.getByTestId('account-sign-out-everywhere').click();
        const confirmWindow = page.getByTestId('account-everywhere-window');
        await expect(confirmWindow).toContainText(ruText('account.everywhere.title'));
        await page.getByTestId('account-everywhere-window-cancel').click();
        await expect(confirmWindow).toHaveCount(0);
        expect((await other.request.get(`${server.origin}/api/v1/session`)).status(), 'cancelling ends nothing').toBe(
            200
        );

        await page.getByTestId('account-sign-out-everywhere').click();
        await page.getByTestId('account-everywhere-window-confirm').click();

        await expect(page.getByTestId('login-send')).toBeVisible();
        expect((await other.request.get(`${server.origin}/api/v1/session`)).status()).toBe(401);
        expect((await page.context().request.get(`${server.origin}/api/v1/session`)).status()).toBe(401);
    } finally {
        await other.close();
    }
});
