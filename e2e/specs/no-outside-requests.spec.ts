import { expect, test } from '../fixtures/test.js';
import { SCREENS } from '../support/screens.js';

/**
 * Rule R3 (alone by default): on every page, no request leaves for any address but the wyo server's own. The pages come from the
 * screens table (`e2e/support/screens.ts`), so a new page is covered by adding its row. The recorder sees the page's requests, its
 * frames', its workers' and its web sockets (`e2e/helpers/requests.ts`).
 */
for (const row of SCREENS) {
    test(`${row.route}: every request goes to the wyo server`, async ({ page, requests, baseURL }) => {
        await page.goto(row.route);
        await expect(page.getByTestId(row.ready)).toBeVisible();

        const asked = requests.all();
        expect(asked.length, 'the page asked for at least its own files').toBeGreaterThan(0);
        expect(requests.outside([baseURL!])).toEqual([]);
    });
}

test('the recorder sees a request that leaves for another address', async ({ page, requests, baseURL }) => {
    // The address is never reached: the test answers it itself. The recorder must still have seen the request leave the page.
    await page.route('http://outside.invalid/**', (route) =>
        route.fulfill({ status: 200, contentType: 'image/png', body: '' })
    );
    await page.setContent('<img src="http://outside.invalid/pixel.png" alt="">');

    await expect
        .poll(() => requests.outside([baseURL!]).map((item) => item.url))
        .toEqual(['http://outside.invalid/pixel.png']);
});
