import { expect, type Page } from '@playwright/test';
import type { IWyoServer } from '../support/wyo-server.js';
import { needsSignIn, screenOf } from '../support/screens.js';
import { signInThroughApi } from './sign-in.js';

/**
 * Opens the page of `route` and waits until it is drawn: the element its row of the screens table names (`ready`) is on the
 * screen. The wyo interface draws everything in the browser, so a drawn page is a page whose script has started and which has read
 * what it shows. The one way a spec waits for a page; it never waits a fixed time.
 */
export async function openScreen(page: Page, route: string): Promise<void> {
    const row = screenOf(route);
    await page.goto(route);
    await expect(page.getByTestId(row.ready), `${route} never drew its ready element ${row.ready}`).toBeVisible();
}

/**
 * `openScreen` as the page's own visitor: the owner is signed in first when the page needs it (`needsSignIn`), and nobody is when
 * it does not (the sign-in page). For the specs that walk every row of the screens table.
 */
export async function visitScreen(page: Page, server: IWyoServer, route: string): Promise<void> {
    if (needsSignIn(route)) await signInThroughApi(page.context(), server);
    await openScreen(page, route);
}
