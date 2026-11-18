import type { BrowserContext, Page } from '@playwright/test';
import type { IOwner } from '../support/owner.js';
import type { IWyoServer } from '../support/wyo-server.js';

/** The name of the cookie that holds the session (`api/wyo.yaml`, «Sessions and the idle sign-out»). */
export const SESSION_COOKIE = 'wyo_session';

/**
 * Signs `owner` in to `server` the way the sign-in page does — `POST /api/v1/session` — but without the page, and leaves the
 * session cookie in the browser context, so every page the context opens afterwards is a signed-in one. The sign-in PAGE is walked
 * once, by `e2e/specs/sign-in.spec.ts`; every other spec needs only to be signed in, and does not repeat the form.
 *
 * The call is made by the context's own request object, which shares the context's cookies
 * (https://playwright.dev/docs/api/class-browsercontext#browser-context-request). Throws, without the password, when the
 * wyo server refuses.
 */
export async function signInThroughApi(
    context: BrowserContext,
    server: IWyoServer,
    owner: IOwner = server.owner
): Promise<void> {
    const response = await context.request.post(`${server.origin}/api/v1/session`, {
        headers: { 'X-Wyo-Person': '1' },
        data: { login: owner.login, password: owner.password }
    });
    if (!response.ok()) {
        throw new Error(
            `signing in as "${owner.login}" was refused with status ${response.status()}: ${await response.text()}`
        );
    }
}

/** The value of the session cookie the context holds for `server`, or undefined when it holds none. */
export async function sessionCookieOf(context: BrowserContext, server: IWyoServer) {
    const cookies = await context.cookies(server.origin);
    return cookies.find((cookie) => cookie.name === SESSION_COOKIE);
}

/**
 * Fills the sign-in page (already open) with `login` and `password`, presses «Войти» and waits for the wyo server's answer to the
 * call, never for a fixed time. Returns the answer's status. The page is the one place this form is walked
 * (`e2e/specs/sign-in.spec.ts`); a spec that only needs to be signed in uses `signInThroughApi`.
 */
export async function signInThroughPage(page: Page, login: string, password: string): Promise<number> {
    await page.getByTestId('login-login').fill(login);
    await page.getByTestId('login-password').fill(password);
    const [response] = await Promise.all([
        page.waitForResponse(
            (candidate) => candidate.url().endsWith('/api/v1/session') && candidate.request().method() === 'POST'
        ),
        page.getByTestId('login-send').click()
    ]);
    return response.status();
}
