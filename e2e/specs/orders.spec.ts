import { expect, test } from '../fixtures/test.js';
import { addAccountThroughApi, readOrdersThroughApi } from '../helpers/accounts.js';
import { openScreen } from '../helpers/open-screen.js';
import { expectInsidePhoneGutter, PHONE } from '../helpers/phone.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { sweepScreen } from '../helpers/sweep.js';
import { addFakeFboOrder, makeFakeOrder, makeToken, setFakeWbStatus } from '../support/fake-wildberries.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';
import { restartWyoServer } from '../support/wyo-server.js';

/**
 * «Заказы» (`/orders`): orders made in the fake Wildberries reach the list through «Обновить», the button then waits 5 minutes, a
 * cancellation made in the fake shows after the next reading, the filters live in the address, and a row opens the order's
 * details beside the list. What each badge, cell and empty state says is proved by the unit tests of the components and the page;
 * here the orders go the whole way from the fake through the real wyo server to the page. «Обновить» is answered once in 5
 * minutes per wyo server, so every test runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

test('«Заказы» with orders fits the phone and stands level at every width, with its filter open too', async ({
    ownerPage: page,
    server
}) => {
    const token = makeToken();
    await addAccountThroughApi(page.context(), server, 'Магазин для замера', token);
    await makeFakeOrder(token, { article: 'ART-SWEEP-1', skus: ['4600000000101'] });
    await addFakeFboOrder(token, {
        srid: 'sweep-fbo-1',
        supplier_article: 'ART-SWEEP-2',
        barcode: '4600000000102',
        warehouse_name: 'Коледино'
    });
    await readOrdersThroughApi(page.context(), server, 2);

    await sweepScreen(page, screenOf('/orders'));
    await sweepScreen(page, screenOf('/orders'), { before: openAllSections });
});

test('with no account the list says so and leads to «Маркетплейсы»', async ({ ownerPage: page }) => {
    await openScreen(page, '/orders');

    await expect(page.getByTestId('orders-empty-no-accounts')).toContainText(ruText('orders.empty.noAccounts'));
    await page.getByTestId('orders-open-marketplaces').click();

    await expect(page.getByTestId('marketplaces-loaded')).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/marketplaces');
});

test('an order made in the fake appears after «Обновить», which then waits 5 minutes, and opens its details beside the list', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const token = makeToken();
    await addAccountThroughApi(page.context(), server, 'Магазин одежды', token);
    const number = String(await makeFakeOrder(token, { article: 'ART-E2E-1', skus: ['4600000000201'] }));
    await openScreen(page, '/orders');
    await expect(page.getByTestId('orders-empty')).toHaveText(ruText('orders.empty.none'));
    await expect(page.getByTestId('orders-refresh-reason')).toHaveCount(0);

    await page.getByTestId('orders-refresh').click();

    await expect(page.getByTestId('orders-refresh-answer-saved')).toHaveText(ruText('orders.refresh.started'));
    await expect(page.getByTestId('orders-refresh-reason')).toHaveText(ruText('orders.refresh.wait', { minutes: 5 }));
    // The page reads its list again by itself a few seconds after the press; the row is waited for, not the seconds.
    const row = page.getByTestId('orders-list-row');
    await expect(row).toHaveCount(1, { timeout: 15_000 });
    await expect(row).toHaveAttribute('data-state', 'new');
    await expect(row).toHaveAttribute('data-scheme', 'fbs');
    await expect(row.getByTestId('orders-list-link'), "the number is the row's link").toHaveText(number);

    await row.getByTestId('orders-list-link').click();

    const panel = page.getByTestId('order-details-panel');
    await expect(panel).toBeVisible();
    await expect(row, 'the list stays beside the details').toBeVisible();
    await expect(panel.getByTestId('order-state')).toHaveText(ruText('orders.state.new'));
    await expect(panel.getByTestId('order-facts-number')).toHaveText(number);
    await expect(panel.getByTestId('order-lines-1-article')).toHaveText('ART-E2E-1');
    await expect(panel.getByTestId('order-lines-1-barcodes')).toHaveText('4600000000201');
    expect(new URL(page.url()).searchParams.get('order')).toBe(await row.getAttribute('data-order-id'));

    await page.getByTestId('order-details-close').click();

    await expect(panel).toHaveCount(0);
    expect(new URL(page.url()).searchParams.get('order')).toBeNull();
    // Rule R3: the page asked only the wyo server; the fake was asked by the wyo server alone.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test('a cancellation made in the fake shows after the next reading', async ({ ownerPage: page, server }) => {
    const token = makeToken();
    await addAccountThroughApi(page.context(), server, 'Магазин обуви', token);
    const fakeOrder = await makeFakeOrder(token, { article: 'ART-E2E-2', skus: ['4600000000301'] });
    await readOrdersThroughApi(page.context(), server, 1);
    await setFakeWbStatus(token, fakeOrder, 'declined_by_client');
    // «Обновить» is answered once in 5 minutes; a wyo server started again on the same data allows the next press at once.
    await restartWyoServer(server);
    await openScreen(page, '/orders');
    const row = page.getByTestId('orders-list-row');
    await expect(row).toHaveAttribute('data-state', 'new');

    await page.getByTestId('orders-refresh').click();

    await expect(row).toHaveAttribute('data-state', 'cancelled', { timeout: 15_000 });
    await expect(row.getByTestId('orders-list-state')).toHaveText(ruText('orders.state.cancelled.buyer'));
    await row.getByTestId('orders-list-link').click();
    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('order-state')).toHaveText(ruText('orders.state.cancelled.buyer'));
    // The history holds one entry per change of the connector's state, in the list's words, with who cancelled.
    await expect(details.getByTestId('order-history-state')).toHaveCount(2);
    await expect(details.getByTestId('order-history-state').first()).toHaveText(ruText('orders.state.new'));
    await expect(details.getByTestId('order-history-state').last()).toHaveText(ruText('orders.state.cancelled.buyer'));

    // On a phone the details are a bottom sheet, padded as its heading is.
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(details.getByTestId('order-mark'), "the order's marketplace");
    await expectInsidePhoneGutter(details.getByTestId('order-facts'), "the order's facts");
});

test('the filters narrow the list, live in the address and come back when the address is opened again', async ({
    ownerPage: page,
    server
}) => {
    const token = makeToken();
    await addAccountThroughApi(page.context(), server, 'Магазин посуды', token);
    await makeFakeOrder(token, { article: 'ART-RED', skus: ['4600000000401'] });
    await makeFakeOrder(token, { article: 'ART-BLUE', skus: ['4600000000402'] });
    await addFakeFboOrder(token, {
        srid: 'filters-fbo-1',
        supplier_article: 'ART-GREEN',
        barcode: '4600000000403',
        warehouse_name: 'Коледино'
    });
    await readOrdersThroughApi(page.context(), server, 3);
    await openScreen(page, '/orders');
    const rows = page.getByTestId('orders-list-row');
    await expect(rows).toHaveCount(3);

    await openSection(page, 'orders-filter');
    await page.getByTestId('orders-filter-scheme').selectOption('fbo');
    await page.getByTestId('orders-filter-apply').click();

    await expect(rows).toHaveCount(1);
    await expect(rows).toHaveAttribute('data-scheme', 'fbo');
    await expect(page.getByTestId('orders-filter-chip-scheme')).toContainText(
        ruText('orders.chip.scheme', { value: ruText('orders.scheme.fbo') })
    );
    const filtered = page.url();

    await openScreen(page, '/orders');
    await expect(rows).toHaveCount(3);
    await page.goto(filtered);

    await expect(rows).toHaveCount(1);
    await expect(rows).toHaveAttribute('data-scheme', 'fbo');
    await expect(page.getByTestId('orders-filter-chip-scheme')).toBeVisible();

    await page.getByTestId('orders-filter-reset').click();

    await expect(rows).toHaveCount(3);
    await expect(page.getByTestId('orders-filter-chip-scheme')).toHaveCount(0);

    await openSection(page, 'orders-filter');
    await page.getByTestId('orders-filter-q').fill('ART-BLUE');
    await page.getByTestId('orders-filter-apply').click();

    await expect(rows).toHaveCount(1);
    await expect(rows).toContainText('ART-BLUE');
    await expect(page.getByTestId('orders-filter-chip-q')).toContainText(
        ruText('orders.chip.q', { value: 'ART-BLUE' })
    );

    await openSection(page, 'orders-filter');
    await page.getByTestId('orders-filter-q').fill('NO-SUCH-ARTICLE');
    await page.getByTestId('orders-filter-apply').click();

    await expect(page.getByTestId('orders-empty-filtered')).toContainText('NO-SUCH-ARTICLE');
    await expect(rows).toHaveCount(0);
});
