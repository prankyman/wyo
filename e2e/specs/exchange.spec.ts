import type { Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import {
    receiveNotificationsThroughApi,
    runExchangeUntil,
    setUpExchange,
    waitForExchange
} from '../helpers/external-api.js';
import { openScreen } from '../helpers/open-screen.js';
import { orderRowOf as rowOf } from '../helpers/order-rows.js';
import { openSection } from '../helpers/sections.js';
import { fcOrderStep, fcOrders, flushFc, MARKING_CODE, setFcProducts } from '../support/fake-fulfilment.js';
import { makeFakeOrder, setFakeVerdict } from '../support/fake-wildberries.js';
import { ruText } from '../support/texts.js';

/**
 * The exchange of an order with the fake fulfilment centre, seen in the wyo interface: an order made at the fake Wildberries is read
 * and sent, then the fake fulfilment centre's control calls play its part (reserve, pack) and the wyo server's jobs are run at once
 * after each call (`runExchangeUntil`, the test build's `POST /__test/exchange/run`). «Заказы» shows the column «Обмен» move, an
 * order the fulfilment centre holds shows its reason under the filter «Требуют внимания», and a code the marketplace refuses
 * shows as a problem in the order's details. What every state, reason and verdict says is proved by the unit tests of
 * `ExchangeStateBadge`, `HeldReasons`, `CodeVerdictList` and `ProblemLine`; here they come from the real wyo server. Every test
 * walks orders, so each runs on a wyo server of its own.
 */
test.use({ serverKind: 'own' });

/** The exchange state the list shows for the order, in the list's words. */
async function expectListedState(page: Page, number: string, state: string): Promise<void> {
    await openScreen(page, '/orders');
    await expect(rowOf(page, number).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText(`exchange.state.${state}`)
    );
}

test('an order walks from «передан» through «этикетка передана» to «коды приняты» in the list and in its details', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const { token, key } = await setUpExchange(page.context(), server);
    const number = String(
        await makeFakeOrder(token, { article: 'ART-WALK', skus: ['4600000000501'], required_meta: ['sgtin'] })
    );

    await runExchangeUntil(page.context(), server, number, 'sent');
    await expectListedState(page, number, 'sent');

    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(page.context(), server, number, 'label_sent');
    await expectListedState(page, number, 'label_sent');

    await fcOrderStep(key, fcOrder.id, 'pack', { units: [{ line: 1, marking_code: MARKING_CODE }] });
    await runExchangeUntil(page.context(), server, number, 'codes_accepted');
    await expectListedState(page, number, 'codes_accepted');

    await rowOf(page, number).getByTestId('orders-list-link').click();

    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.codes_accepted'));
    await expect(details.getByTestId('exchange-verdicts-row')).toHaveCount(1);
    await expect(details.getByTestId('exchange-verdicts-row')).toContainText(ruText('exchange.identifier.marking'));
    await expect(details.getByTestId('exchange-verdicts-verdict')).toHaveText(ruText('exchange.verdict.accepted'));
    await expect(details.getByTestId('exchange-problem')).toHaveCount(0);
    await expect(details.getByTestId('exchange-history-row').first()).toContainText(ruText('exchange.event.sent'));
    await expect(details.getByTestId('exchange-history-row').last()).toContainText(
        ruText('exchange.event.codes_accepted')
    );
    // Rule R4: the marking code is in no page the owner sees.
    expect(await page.content(), 'the page holds a marking code').not.toContain(MARKING_CODE.slice(0, 20));
    // Rule R3: the page asked only the wyo server; the fakes were asked by the wyo server and the test alone.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test('an order the fulfilment centre holds for an unknown product shows its reason under «Требуют внимания» and leaves it once released', async ({
    ownerPage: page,
    server
}) => {
    const { token, key } = await setUpExchange(page.context(), server);
    const barcode = '4600000000601';
    await setFcProducts(key, { unknown: [barcode] });
    const held = String(await makeFakeOrder(token, { article: 'ART-HELD', skus: [barcode] }));
    const fine = String(await makeFakeOrder(token, { article: 'ART-FINE', skus: ['4600000000602'] }));

    await runExchangeUntil(page.context(), server, held, 'held');
    await runExchangeUntil(page.context(), server, fine, 'sent');
    await openScreen(page, '/orders');
    await expect(page.getByTestId('orders-list-row')).toHaveCount(2);
    await openSection(page, 'orders-filter');
    await page.getByTestId('orders-filter-attention').check();
    await page.getByTestId('orders-filter-apply').click();

    await expect(page.getByTestId('orders-list-row')).toHaveCount(1);
    await expect(page.getByTestId('orders-filter-chip-attention')).toContainText(ruText('orders.chip.attention'));
    const row = rowOf(page, held);
    await expect(row.getByTestId('orders-list-exchange-state')).toHaveText(ruText('exchange.state.held'));

    await row.getByTestId('orders-list-link').click();

    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-attention')).toHaveText(ruText('exchange.attention.held'));
    await expect(details.getByTestId('exchange-held-reason')).toHaveText(
        ruText('exchange.held.unknown_product.barcode', { barcode })
    );

    // The owner makes the product known at the fulfilment centre; the order is taken in and leaves the filter.
    await setFcProducts(key, { known: [barcode] });
    await runExchangeUntil(page.context(), server, held, 'sent');
    await page.reload();

    await expect(page.getByTestId('orders-empty-filtered')).toBeVisible();
    await expect(page.getByTestId('orders-list-row')).toHaveCount(0);
});

test('a code the marketplace refuses shows as a problem in the order, with the marketplace’s own words', async ({
    ownerPage: page,
    server
}) => {
    const { token, key } = await setUpExchange(page.context(), server);
    const fakeOrder = await makeFakeOrder(token, {
        article: 'ART-REFUSED',
        skus: ['4600000000701'],
        required_meta: ['sgtin']
    });
    const number = String(fakeOrder);
    await runExchangeUntil(page.context(), server, number, 'sent');
    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(page.context(), server, number, 'label_sent');
    // The fulfilment centre announces the packing by a notification, so the wyo server reports the code to Wildberries and waits
    // for the verdict; Wildberries clears a verdict when a code is put, so the refusal is set after that and before it is asked.
    await receiveNotificationsThroughApi(page.context(), server, key);

    await fcOrderStep(key, fcOrder.id, 'pack', { units: [{ line: 1, marking_code: MARKING_CODE }] });
    await flushFc(key);
    await waitForExchange(page.context(), server, number, 'codes_checking');
    await setFakeVerdict(token, fakeOrder, 'sgtin', 'sgtinNotFound');
    await runExchangeUntil(page.context(), server, number, { problem: 'codes_refused' });

    await expectListedState(page, number, 'problem');
    await rowOf(page, number).getByTestId('orders-list-link').click();
    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.problem'));
    await expect(details.getByTestId('exchange-problem')).toContainText(ruText('problem.codes_refused'));
    await expect(details.getByTestId('exchange-problem-words')).toBeVisible();
    await expect(details.getByTestId('exchange-verdicts-verdict')).toHaveText(ruText('exchange.verdict.refused'));
    expect(await page.content(), 'the page holds a marking code').not.toContain(MARKING_CODE.slice(0, 20));
});

test('the request journal filtered by «Фулфилмент-центр» lists the requests sent to the fulfilment centre only', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const { token } = await setUpExchange(page.context(), server);
    const number = String(await makeFakeOrder(token, { article: 'ART-JOURNAL', skus: ['4600000000801'] }));
    await runExchangeUntil(page.context(), server, number, 'sent');
    await openScreen(page, '/journal');
    const rows = page.getByTestId('journal-list-row');
    const all = await rows.count();

    await openSection(page, 'journal-filter');
    await page.getByTestId('journal-filter-target').selectOption('fulfilment');
    await page.getByTestId('journal-filter-apply').click();

    await expect(page.getByTestId('journal-filter-chip-target')).toContainText(
        ruText('journal.chip.target', { value: ruText('journal.target.fulfilment') })
    );
    await expect(page.getByTestId('journal-list-target').first()).toHaveText(ruText('journal.target.fulfilment'));
    const ofFulfilment = await rows.count();
    expect(ofFulfilment, 'the fulfilment centre has requests of its own').toBeGreaterThan(0);
    expect(ofFulfilment, 'the filter leaves out the marketplace’s requests').toBeLessThan(all);
    await expect(rows.filter({ hasNot: page.getByTestId('journal-list-target') })).toHaveCount(0);
    expect(requests.outside([baseURL!])).toEqual([]);
});
