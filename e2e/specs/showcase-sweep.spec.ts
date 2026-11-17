import { sweepScreen } from '../helpers/sweep.js';
import { test } from '../fixtures/test.js';
import { screenOf } from '../support/screens.js';

/**
 * The showcase page draws every component of the library in each of its states; this is the page the browser tests measure at
 * 390 x 844 and again at 640, 768, 1024 and 1280 px (`e2e/helpers/sweep.ts` lists what is measured).
 */
test('the showcase page fits the phone and stands level at every width', async ({ page }) => {
    await sweepScreen(page, screenOf('/showcase'));
});
