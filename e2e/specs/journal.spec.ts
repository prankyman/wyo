import { expect, test } from '../fixtures/test.js';
import { addAccountThroughApi } from '../helpers/accounts.js';
import { openScreen } from '../helpers/open-screen.js';
import { expectInsidePhoneGutter, PHONE } from '../helpers/phone.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { sweepScreen } from '../helpers/sweep.js';
import { makeToken, setUpFake } from '../support/fake-wildberries.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * «Журнал запросов» (`/journal`): every request the wyo server sent to the fake Wildberries is a row, the filters by account and
 * by outcome narrow the list, a row opens the whole request beside the list, and nothing in it holds a key (rule R4). What each
 * outcome's badge says is proved by the unit test of `JournalOutcomeBadge`; here the rows come from the real wyo server's
 * requests. The rows of a test are its own, so every test runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

test('«Журнал запросов» with rows fits the phone and stands level at every width, with its filter open too', async ({
    ownerPage: page,
    server
}) => {
    await addAccountThroughApi(page.context(), server, 'Магазин для замера', makeToken());

    await sweepScreen(page, screenOf('/journal'));
    await sweepScreen(page, screenOf('/journal'), { before: openAllSections });
});

test('with no request sent yet the journal says so', async ({ ownerPage: page }) => {
    await openScreen(page, '/journal');

    await expect(page.getByTestId('journal-empty')).toHaveText(ruText('journal.empty.none'));
});

test('the journal lists the requests, narrows them by outcome and by account, and opens one beside the list without its key', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const working = makeToken();
    const refused = makeToken();
    const first = await addAccountThroughApi(page.context(), server, 'Магазин А', working);
    const second = await addAccountThroughApi(page.context(), server, 'Магазин Б', refused);
    // Wildberries stops accepting the second key: the check «Проверить ключ сейчас» meets one 401, written as «key_refused».
    await setUpFake(refused, { token_refused: 'invalid' });
    await page.context().request.post(`${server.origin}/api/v1/accounts/${second.id}/check`, { data: {} });
    await openScreen(page, '/journal');
    const rows = page.getByTestId('journal-list-row');
    const refusedRows = page.locator('[data-test="journal-list-row"][data-outcome="key_refused"]');
    await expect(refusedRows).toHaveCount(1);
    const all = await rows.count();
    // The requests of the check before an account is stored name no account; once stored, the wyo server read its warehouses.
    const ofFirst = rows.filter({ hasText: 'Магазин А' });
    await expect(ofFirst).toHaveCount(1);
    await expect(ofFirst).toContainText('GET /api/v3/warehouses');

    await openSection(page, 'journal-filter');
    await page.getByTestId('journal-filter-outcome').selectOption('key_refused');
    await page.getByTestId('journal-filter-apply').click();

    await expect(rows).toHaveCount(1);
    await expect(rows).toHaveAttribute('data-outcome', 'key_refused');
    await expect(page.getByTestId('journal-filter-chip-outcome')).toBeVisible();

    await rows.getByTestId('journal-list-link').click();

    const panel = page.getByTestId('journal-details-panel');
    await expect(panel).toBeVisible();
    await expect(panel.getByTestId('journal-entry-facts-account')).toHaveText('Магазин Б');
    await expect(panel.getByTestId('journal-entry-facts-status')).toHaveText('401');
    expect(await page.content(), 'the journal holds the key').not.toContain(refused.slice(0, -4));
    // On a phone the details are a bottom sheet, padded as its heading is.
    const computer = page.viewportSize()!;
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(panel.getByTestId('journal-entry-outcome'), "the request's outcome");
    await expectInsidePhoneGutter(panel.getByTestId('journal-entry-facts'), "the request's facts");
    await page.setViewportSize(computer);

    await page.getByTestId('journal-details-close').click();
    await page.getByTestId('journal-filter-reset').click();
    await expect(rows).toHaveCount(all);

    await openSection(page, 'journal-filter');
    await page.getByTestId('journal-filter-account').selectOption(String(first.id));
    await page.getByTestId('journal-filter-apply').click();

    await expect(rows).toHaveCount(1);
    await expect(rows).toContainText('Магазин А');
    await expect(rows).toContainText('GET /api/v3/warehouses');
    await expect(refusedRows).toHaveCount(0);
    await expect(page.getByTestId('journal-filter-chip-account')).toBeVisible();
    // Rule R3: the page asked only the wyo server.
    expect(requests.outside([baseURL!])).toEqual([]);
});
