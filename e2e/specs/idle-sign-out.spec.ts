import type { Request } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { openScreen } from '../helpers/open-screen.js';
import { signInThroughApi } from '../helpers/sign-in.js';

/**
 * The idle sign-out, at the edge a browser can see. The wyo server's clock is not controllable from a browser, so what happens
 * after the idle minutes (a session past its time is ended; only a call that carries `X-Wyo-Person: 1` prolongs it; the idle
 * length is the setting `security.idleMinutes`) is proved by the Go tests with an injected clock (`internal/auth`,
 * `internal/httpapi`). Here two things are proved in a real browser:
 *
 * 1. a person's own calls carry the mark and the bell's periodic read does not, so a screen left open does not keep the session
 *    alive (`src/lib/api/api.ts`, `src/lib/notifications/notices-feed.svelte.ts`);
 * 2. when the wyo server answers `signed_out` — which is how an ended session, by idleness or by another browser, shows itself —
 *    the next thing the person presses sends them to the sign-in page.
 */
const PERSON_HEADER = 'x-wyo-person';

test("a person's calls carry the mark of activity and the bell's periodic read does not", async ({
    ownerPage: page
}) => {
    const asked: Request[] = [];
    page.on('request', (request) => {
        if (new URL(request.url()).pathname.startsWith('/api/')) asked.push(request);
    });

    await openScreen(page, '/');

    const pathsAsked = () => asked.map((request) => new URL(request.url()).pathname);
    await expect
        .poll(pathsAsked)
        .toEqual(expect.arrayContaining(['/api/v1/session', '/api/v1/notices', '/api/v1/installation']));
    const markOf = async (path: string): Promise<string | undefined> => {
        const request = asked.find((candidate) => new URL(candidate.url()).pathname === path);
        return request ? (await request.allHeaders())[PERSON_HEADER] : undefined;
    };
    expect(await markOf('/api/v1/session'), "reading who is signed in is the layout's own call").toBe('1');
    expect(await markOf('/api/v1/installation'), "the page's read").toBe('1');
    expect(await markOf('/api/v1/notices'), "the bell's read has no mark").toBeUndefined();
});

test('a call answered «signed_out» sends the person to the sign-in page', async ({
    ownerPage: page,
    server,
    browser,
    expectedRefusals
}) => {
    for (const pathEnd of ['/api/v1/settings', '/api/v1/installation', '/api/v1/backups']) {
        expectedRefusals.push({ status: 401, pathEnd });
    }
    await openScreen(page, '/');
    // The session ends from outside, as it ends by idleness: another browser signs in and ends every session.
    const outside = await browser.newContext();
    try {
        await signInThroughApi(outside, server);
        const ended = await outside.request.delete(`${server.origin}/api/v1/sessions`, {
            headers: { 'X-Wyo-Person': '1' }
        });
        expect(ended.status()).toBe(204);
    } finally {
        await outside.close();
    }

    await page.getByTestId('nav-settings').click();

    await expect(page.getByTestId('login-send')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/auth/login');
    await expect(page.getByTestId('app-main')).toHaveCount(0);
});
