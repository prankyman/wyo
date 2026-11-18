import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { openScreen } from '../helpers/open-screen.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { sessionCookieOf, signInThroughApi, signInThroughPage } from '../helpers/sign-in.js';
import { sweepScreen } from '../helpers/sweep.js';
import { NEW_TEST_PASSWORD, WRONG_TEST_PASSWORD } from '../support/owner.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * «Мой профиль» (`/account`): the sweep, who is signed in, and the change of the password. Signing out from this page has its own
 * spec (`sign-out.spec.ts`). The change of the password changes the owner, so its tests run on a wyo server of their own.
 */
test('«Мой профиль» fits the phone and stands level at every width', async ({ ownerPage: page }) => {
    await sweepScreen(page, screenOf('/account'));
});

test('«Мой профиль» with its sections open fits the phone and stands level at every width', async ({
    ownerPage: page
}) => {
    await sweepScreen(page, screenOf('/account'), { before: openAllSections });
});

test('«Мой профиль» says who is signed in', async ({ ownerPage: page, server }) => {
    await openScreen(page, '/account');

    await expect(page.getByTestId('account-heading')).toHaveText(ruText('account.title'));
    await expect(page.getByTestId('account-login')).toHaveText(
        ruText('account.signedInAs', { login: server.owner.login })
    );
    await expect(page.getByTestId('account-must-change')).toHaveCount(0);
});

/** Types the three boxes of the password form, replacing what they hold, and presses «Сохранить». */
async function sendPasswordForm(page: Page, current: string, next: string, confirm: string): Promise<void> {
    await page.getByTestId('account-current').fill(current);
    await page.getByTestId('account-new').fill(next);
    await page.getByTestId('account-confirm').fill(confirm);
    await page.getByTestId('account-password-save').click();
}

test.describe('on a wyo server of its own', () => {
    test.use({ serverKind: 'own' });

    test('each refusal of the password form is worded on its own box', async ({
        ownerPage: page,
        server,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 422, pathEnd: '/api/v1/account/password' });
        await openScreen(page, '/account');
        await openSection(page, 'account-password');

        // The current password is wrong: only the wyo server can know it.
        await sendPasswordForm(page, WRONG_TEST_PASSWORD, NEW_TEST_PASSWORD, NEW_TEST_PASSWORD);
        await expect(page.getByTestId('account-current-error')).toHaveText(ruText('errors.wrong'));

        // The rest are refused by the form itself, before anything is sent.
        await sendPasswordForm(page, server.owner.password, 'short', 'short');
        await expect(page.getByTestId('account-new-error')).toHaveText(ruText('errors.too_short'));

        await sendPasswordForm(page, server.owner.password, NEW_TEST_PASSWORD, `${NEW_TEST_PASSWORD}-x`);
        await expect(page.getByTestId('account-confirm-error')).toHaveText(ruText('errors.passwords_differ'));

        await sendPasswordForm(page, server.owner.password, server.owner.password, server.owner.password);
        await expect(page.getByTestId('account-new-error')).toHaveText(ruText('errors.same_as_current'));
    });

    test('a new password ends the other sessions, keeps this one, and is the one that signs in afterwards', async ({
        ownerPage: page,
        server,
        browser,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' });
        const other = await browser.newContext();
        try {
            await signInThroughApi(other, server);
            expect((await other.request.get(`${server.origin}/api/v1/session`)).status()).toBe(200);
            await openScreen(page, '/account');
            await openSection(page, 'account-password');

            await sendPasswordForm(page, server.owner.password, NEW_TEST_PASSWORD, NEW_TEST_PASSWORD);

            await expect(page.getByTestId('account-password-saved')).toHaveText(ruText('common.disclosure.saved'));
            expect((await page.context().request.get(`${server.origin}/api/v1/session`)).status()).toBe(200);
            expect((await other.request.get(`${server.origin}/api/v1/session`)).status()).toBe(401);
        } finally {
            await other.close();
        }

        await openSection(page, 'account-signout');
        await page.getByTestId('account-sign-out').click();
        await expect(page.getByTestId('login-send')).toBeVisible();
        expect(await signInThroughPage(page, server.owner.login, server.owner.password)).toBe(401);
        await expect(page.getByTestId('login-answer-error')).toHaveText(ruText('errors.sign_in_refused'));

        expect(await signInThroughPage(page, server.owner.login, NEW_TEST_PASSWORD)).toBe(200);
        await expect(page.getByTestId('overview-cards')).toBeVisible();
        expect(await sessionCookieOf(page.context(), server)).toBeDefined();
    });
});
