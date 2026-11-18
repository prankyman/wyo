import { signInThroughPage } from '../helpers/sign-in.js';
import { openScreen } from '../helpers/open-screen.js';
import { sweepScreen } from '../helpers/sweep.js';
import { expect, test } from '../fixtures/test.js';
import { WRONG_TEST_PASSWORD } from '../support/owner.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * The sign-in page (`/auth/login`): the sweep, the right login and password, a visit with no session, the forgotten-password text,
 * a wrong password, and the limit of attempts. The sign-in CALL's rules (the same answer for a wrong login and a wrong password, the
 * limit's count and window, the cookie's properties) are proved by the Go tests; here the page is walked as a person walks it.
 */
test('the sign-in page fits the phone and stands level at every width', async ({ page }) => {
    await sweepScreen(page, screenOf('/auth/login'));
});

test('the right login and password open «Обзор» with a session cookie no script can read', async ({
    page,
    server,
    context
}) => {
    await openScreen(page, '/auth/login');

    expect(await signInThroughPage(page, server.owner.login, server.owner.password)).toBe(200);

    await expect(page.getByTestId('overview-cards')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/');
    await expect(page.getByTestId('app-user-login')).toHaveAttribute('aria-label', server.owner.login);
    const cookie = (await context.cookies(server.origin)).find((candidate) => candidate.name === 'wyo_session');
    expect(cookie, 'the wyo server set the session cookie').toBeDefined();
    expect(cookie).toMatchObject({ httpOnly: true, sameSite: 'Strict', path: '/', secure: false });
});

test('a visit to a page with no session goes to the sign-in page', async ({ page, expectedRefusals }) => {
    expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' });

    await page.goto('/settings');

    await expect(page.getByTestId('login-send')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/auth/login');
    await expect(page.getByTestId('app-main')).toHaveCount(0);
});

test('«Забыли пароль?» says that the password is set again on the server', async ({ page }) => {
    await openScreen(page, '/auth/login');
    await expect(page.getByTestId('login-forgot-text')).toHaveCount(0);

    await page.getByTestId('login-forgot').click();

    await expect(page.getByTestId('login-forgot-text')).toHaveText(ruText('login.forgotText'));
});

test.describe('on a wyo server of its own', () => {
    test.use({ serverKind: 'own' });

    test('a wrong password and an unknown login get the same answer, the login stays and the password box empties', async ({
        page,
        server,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' });
        await openScreen(page, '/auth/login');

        for (const login of [server.owner.login, 'nobody-by-this-name']) {
            expect(await signInThroughPage(page, login, WRONG_TEST_PASSWORD)).toBe(401);

            await expect(page.getByTestId('login-answer-error')).toHaveText(ruText('errors.sign_in_refused'));
            await expect(page.getByTestId('login-password')).toHaveValue('');
            await expect(page.getByTestId('login-login')).toHaveValue(login);
        }
        expect(new URL(page.url()).pathname).toBe('/auth/login');
    });

    test('after 5 wrong passwords even the right one is refused, with the minutes to wait', async ({
        page,
        server,
        expectedRefusals
    }) => {
        expectedRefusals.push({ status: 401, pathEnd: '/api/v1/session' }, { status: 429, pathEnd: '/api/v1/session' });
        await openScreen(page, '/auth/login');
        const password = page.getByTestId('login-password');

        for (let attempt = 1; attempt <= 5; attempt++) {
            expect(await signInThroughPage(page, server.owner.login, WRONG_TEST_PASSWORD), `attempt ${attempt}`).toBe(
                401
            );
            await expect(password).toHaveValue('');
        }
        expect(await signInThroughPage(page, server.owner.login, server.owner.password)).toBe(429);

        // The five failures were made seconds ago, so the oldest ages out in a little under 15 minutes, counted up to 15.
        await expect(page.getByTestId('login-answer-error')).toHaveText(
            ruText('errors.too_many_attempts', { minutes: 15 })
        );
        expect(new URL(page.url()).pathname).toBe('/auth/login');
    });
});
