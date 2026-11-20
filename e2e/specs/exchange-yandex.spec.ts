import type { BrowserContext, Page } from '@playwright/test';
import { expect, test } from '../fixtures/test.js';
import { readHandoverRows, runExchangeUntil, runExchangeUntilHandover } from '../helpers/external-api.js';
import { openScreen } from '../helpers/open-screen.js';
import { orderRowOf } from '../helpers/order-rows.js';
import { expectInsidePhoneGutter, PHONE } from '../helpers/phone.js';
import { openAllSections, openSection } from '../helpers/sections.js';
import { expectNoSidewaysScrollAtEveryWidth, sweepScreen } from '../helpers/sweep.js';
import {
    FAKE_FC_YANDEX,
    runExchangeUntilCourierAnswer,
    setUpYandexExchange,
    tomorrow,
    type IYandexExchangeSetUp
} from '../helpers/yandex-exchange.js';
import {
    fcCourierCode,
    fcHandover,
    fcHandoverStep,
    fcOrders,
    fcOrderStep,
    MARKING_CODE,
    setFcLabelSizes
} from '../support/fake-fulfilment.js';
import {
    FAKE_YANDEX_SHOP_EXPRESS,
    FAKE_YANDEX_SHOP_FBS,
    FAKE_YANDEX_WAREHOUSE_FBS,
    fakeYandexWrites,
    makeFakeYandexOrder,
    makeFakeYandexShipment,
    setFakeYandexCourierCode,
    setFakeYandexVerdict,
    updateFakeYandexShipment
} from '../support/fake-yandex.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';
import type { IWyoServer } from '../support/wyo-server.js';

/**
 * The exchange of a Яндекс Маркет order with the fake fulfilment centre, seen in the wyo interface (plan 8, section J): an order
 * made at the fake Яндекс Маркет is read and sent, then the fake fulfilment centre's control calls play its part (reserve, pack,
 * the hand-over's documents, the courier's code) and the fake Яндекс Маркет's control calls play Яндекс Маркет's (the verdict on the
 * codes, the day's shipment, the code the courier must name); the wyo server's jobs are run at once after each call
 * (`runExchangeUntil`, the test build's `POST /__test/exchange/run`, which reads Яндекс Маркет's orders too). What every state, parcel,
 * step and check says is proved by the unit tests of `ParcelList`, `HandoverSteps`, `HandoverDetails`, `ProblemLine`,
 * `CourierCodeChecks`, `WarehouseMatchTable` and `OrderExchangeDetails`; here they come from the real wyo server and both fakes.
 * Every test walks orders, so each runs on a wyo server of its own, and on keys of its own at the fakes.
 */
test.use({ serverKind: 'own' });

const SHIRT_CODE_B = '0104600000000024215mnopqrstuvwx\u001d93Qw3r';

/** The two lines of the walk's order: two shirts, each carrying a marking code, and a pair of socks that carries none. */
const WALK_ITEMS = [
    { offer_id: 'SHIRT', offer_name: 'Рубашка', count: 2, required_instance_types: ['CIS'] },
    { offer_id: 'SOCKS', offer_name: 'Носки', count: 1 }
];

/** A made-up order of the FBS shop, due tomorrow; `extra` adds fields. */
const fbsOrder = (id: number, extra: { fake?: boolean; items?: typeof WALK_ITEMS } = {}) => ({
    order_id: id,
    campaign_id: FAKE_YANDEX_SHOP_FBS,
    shipment_date: tomorrow(),
    shipment_time: '18:00',
    items: [{ offer_id: 'SOCKS', offer_name: 'Носки', count: 1 }],
    ...extra
});

/**
 * Reads the order `id`, sends it, plays the fulfilment centre reserving it and packing the one pair of socks, and runs the exchange
 * until the order is «готов к отгрузке». Returns the order's number at the fulfilment centre.
 */
async function walkToReady(
    context: BrowserContext,
    server: IWyoServer,
    setUp: IYandexExchangeSetUp,
    id: number
): Promise<string> {
    const number = String(id);
    await runExchangeUntil(context, server, number, 'sent');
    const fcId = (await fcOrders(setUp.key)).find((order) => order.external_id === number)!.id;
    await fcOrderStep(setUp.key, fcId, 'reserve');
    await runExchangeUntil(context, server, number, 'reserved');
    await fcOrderStep(setUp.key, fcId, 'pack', { units: [{ line: 1 }] });
    await runExchangeUntil(context, server, number, 'ready');
    return fcId;
}

/** Opens «Заказы» and the details of the order whose number is `number`. */
async function openOrderDetails(page: Page, number: string) {
    await openScreen(page, '/orders');
    await orderRowOf(page, number).getByTestId('orders-list-link').click();
    return page.getByTestId('order-details-panel');
}

test('the Яндекс Маркет warehouses are matched and each shows the label size the fulfilment centre names for it', async ({
    ownerPage: page,
    server
}) => {
    const setUp = await setUpYandexExchange(page.context(), server);
    await setFcLabelSizes(setUp.key, 'main', [{ marketplace: FAKE_FC_YANDEX, width_mm: 58, height_mm: 40 }]);

    await openScreen(page, '/external-api');
    await openSection(page, 'external-api-warehouses');

    const rows = page.getByTestId('warehouses-row');
    await expect(rows).toHaveCount(2);
    for (const index of [0, 1]) {
        await expect(page.getByTestId(`warehouses-${index}-unmatched`)).toHaveCount(0);
        await expect(page.getByTestId(`warehouses-${index}-label-size`)).toContainText(
            ruText('externalApi.warehouses.labelSize.named', { width: 58, height: 40 })
        );
    }
    // Only the Express warehouse carries the note that a courier waits for the code check.
    await expect(page.locator('[data-test$="-express"]')).toHaveCount(1);

    await setFcLabelSizes(setUp.key, 'main', []);
    await page.reload();
    await openSection(page, 'external-api-warehouses');
    await expect(page.getByTestId('warehouses-0-label-size')).toContainText(
        ruText('externalApi.warehouses.labelSize.default')
    );

    await sweepScreen(page, screenOf('/external-api'), { before: openAllSections });
});

test('a Яндекс Маркет order packed in two parcels shows both boxes in «Заказы» and goes through a hand-over of four steps in «Передачи»', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const context = page.context();
    const setUp = await setUpYandexExchange(context, server);
    const { yandexKey, key } = setUp;
    await setFcLabelSizes(key, 'main', [{ marketplace: FAKE_FC_YANDEX, width_mm: 58, height_mm: 40 }]);
    const id = 1000008001;
    const number = String(id);
    await makeFakeYandexOrder(yandexKey, fbsOrder(id, { items: WALK_ITEMS }));
    // The verdict on the codes stays «in progress» until the test lets it through, so «коды проверяются» can be looked at.
    await setFakeYandexVerdict(yandexKey, id, 'IN_PROGRESS');

    await runExchangeUntil(context, server, number, 'sent');
    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(context, server, number, 'reserved');

    // The fulfilment centre packs both shirts with their codes, one into each box, and the socks into the second box.
    await fcOrderStep(key, fcOrder.id, 'pack', {
        units: [
            { line: 1, marking_code: MARKING_CODE, parcel: 1 },
            { line: 1, marking_code: SHIRT_CODE_B, parcel: 2 },
            { line: 2, parcel: 2 }
        ]
    });
    await runExchangeUntil(context, server, number, 'codes_checking');
    await openScreen(page, '/orders');
    await expect(orderRowOf(page, number).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.codes_checking')
    );

    await setFakeYandexVerdict(yandexKey, id, 'OK');
    await runExchangeUntil(context, server, number, 'ready');

    // «Заказы»: both boxes with their labels, and the order «готов к отгрузке».
    await openScreen(page, '/orders');
    await expect(orderRowOf(page, number).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.ready')
    );
    await orderRowOf(page, number).getByTestId('orders-list-link').click();
    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.ready'));
    await expect(details.getByTestId('exchange-facts-readyAt')).toHaveText(/\S/);
    await expect(details.getByTestId('parcels-1-position')).toHaveText(ruText('parcels.position', { n: 1, total: 2 }));
    await expect(details.getByTestId('parcels-2-position')).toHaveText(ruText('parcels.position', { n: 2, total: 2 }));
    for (const parcel of [1, 2]) {
        await expect(details.getByTestId(`parcels-${parcel}-box`), `the box of parcel ${parcel}`).toHaveText(/\S/);
        await expect(details.getByTestId(`parcels-${parcel}-label`)).toContainText(ruText('parcels.label.sent'));
    }
    await expect(details.getByTestId('exchange-problem')).toHaveCount(0);
    const packed = (await fcOrders(key))[0];
    expect(packed.parcels.map((parcel) => parcel.label_attached)).toEqual([true, true]);
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(details.getByTestId('parcels'), "the order's boxes");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Заказы» with a Яндекс Маркет order's details open");

    // «Передачи»: the fulfilment centre asks for the documents; the hand-over waits until Яндекс Маркет has made the day's shipment.
    const fcHandoverId = packed.handover_id;
    expect(fcHandoverId, 'the packed order stands in a hand-over at the fulfilment centre').not.toBeNull();
    await fcHandoverStep(key, fcHandoverId!, 'request-documents');
    const waiting = await runExchangeUntilHandover(context, server, 'waiting');
    expect(await fakeYandexWrites(yandexKey), 'the shipment was confirmed before it existed').not.toContain(
        'POST /confirm'
    );

    await makeFakeYandexShipment(yandexKey, {
        warehouse_id: FAKE_YANDEX_WAREHOUSE_FBS,
        date: waiting.day,
        kind: 'WITHDRAW',
        order_ids: [id],
        cutoff_passed: true
    });
    await runExchangeUntilHandover(context, server, 'documents_sent');

    await page.setViewportSize({ width: 1280, height: 800 });
    await openScreen(page, '/handovers');
    const row = page.getByTestId('handovers-list-row');
    await expect(row).toHaveCount(1);
    await expect(row).toHaveAttribute('data-state', 'documents_sent');
    await row.getByTestId('handovers-list-link').click();
    const panel = page.getByTestId('handovers-details-panel');
    await expect(panel.getByTestId('handover-state')).toHaveText(ruText('handovers.state.documents_sent'));
    for (const step of ['batch', 'move', 'close', 'documents']) {
        await expect(panel.getByTestId(`handover-steps-${step}`), step).toHaveAttribute('data-state', 'done');
    }
    for (const step of ['boxes', 'shipping']) {
        await expect(panel.getByTestId(`handover-steps-${step}`), step).toHaveCount(0);
    }
    await expect(panel.getByTestId('handover-steps-now')).toHaveText(ruText('handovers.now.done'));
    await expect(panel.getByTestId('handover-facts-batches'), 'the number of the shipment').toHaveText(/\S/);
    await expect(panel.getByTestId('handover-facts-batchKind')).toHaveText(
        ruText('marketplace.yandex.batchKind.WITHDRAW')
    );
    await expect(panel.getByTestId('handover-facts-orders')).toContainText(number);
    await expect(panel.getByTestId('handover-closed')).toHaveText(ruText('handovers.closed'));
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(panel.getByTestId('handover-note'), "the hand-over's note");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Передачи» with a Яндекс Маркет hand-over's details open");

    // Яндекс Маркет was written to once for the boxes, once for the status and once for the confirmation of the shipment.
    expect(await fakeYandexWrites(yandexKey)).toEqual(['PUT /boxes', 'PUT /status', 'POST /confirm']);
    // The fulfilment centre holds the act of the hand-over.
    const atFc = await fcHandover(key, fcHandoverId!);
    expect(atFc.state).toBe('documents_ready');
    expect(atFc.documents.map((document) => document.kind)).toEqual(['act']);

    await sweepScreen(page, screenOf('/orders'));
    await sweepScreen(page, screenOf('/handovers'));
    // Rule R4: neither the key nor a marking code is in a page the owner sees.
    expect(await page.content(), 'the page holds the key').not.toContain(yandexKey.slice(0, -4));
    expect(await page.content(), 'the page holds a marking code').not.toContain(MARKING_CODE.slice(0, 20));
    // Rule R3: the page asked only the wyo server.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test("an Express order's courier code is refused with the tries left and then accepted, in its details, and the code is in no page", async ({
    ownerPage: page,
    server
}) => {
    const context = page.context();
    const setUp = await setUpYandexExchange(context, server);
    const { yandexKey, key } = setUp;
    const id = 1000008002;
    const number = String(id);
    await makeFakeYandexOrder(yandexKey, {
        order_id: id,
        campaign_id: FAKE_YANDEX_SHOP_EXPRESS,
        program_type: 'EXPRESS',
        items: [{ offer_id: 'SOCKS', offer_name: 'Носки', count: 1 }]
    });

    const fcId = await walkToReady(context, server, setUp, id);
    // An Express order enters no hand-over.
    expect(await readHandoverRows(context, server)).toEqual([]);

    const REFUSED = 'COURIER-0000';
    const RIGHT = 'COURIER-4711';
    await setFakeYandexCourierCode(yandexKey, { order_id: id, code: RIGHT, attempts_left: 3 });
    const refused = await fcCourierCode(key, fcId, REFUSED);
    const first = await runExchangeUntilCourierAnswer(context, server, key, fcId, refused, 'refused');
    expect(first.attempts_left).toBe(2);
    const accepted = await fcCourierCode(key, fcId, RIGHT);
    await runExchangeUntilCourierAnswer(context, server, key, fcId, accepted, 'accepted');

    const details = await openOrderDetails(page, number);
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.ready'));
    await expect(details.getByTestId('exchange-facts-handover')).toHaveText(ruText('exchange.handover.courier_code'));
    const checks = details.getByTestId('courier-checks');
    await expect(checks.getByTestId('courier-checks-row')).toHaveCount(2);
    await expect(checks.getByTestId('courier-checks-1-result')).toHaveText(ruText('exchange.courier.refused'));
    await expect(checks.getByTestId('courier-checks-1-attempts')).toHaveText(
        ruText('exchange.courier.attemptsLeft.few', { count: 2 })
    );
    await expect(checks.getByTestId('courier-checks-2-result')).toHaveText(ruText('exchange.courier.accepted'));
    await expect(details.getByTestId('exchange-problem')).toHaveCount(0);
    // Rule R4: the code the courier named is in no page and in no answer of the wyo server.
    const content = await page.content();
    expect(content, 'the page holds the refused code').not.toContain(REFUSED);
    expect(content, 'the page holds the right code').not.toContain(RIGHT);
    const orderAnswer = await context.request.get(`${server.origin}/api/v1/orders`);
    expect(await orderAnswer.text(), "the wyo server's answer holds a code").not.toMatch(/COURIER-/);
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(checks, "the order's code checks");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Заказы» with an Express order's code checks open");

    // The courier takes the order: it is handed over without a hand-over.
    await fcOrderStep(key, fcId, 'give-to-courier');
    await runExchangeUntil(context, server, number, 'handed_over');
    await openScreen(page, '/orders');
    await expect(orderRowOf(page, number).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.handed_over')
    );
    expect(await readHandoverRows(context, server)).toEqual([]);
    expect(
        (await fakeYandexWrites(yandexKey)).filter((write) => write === 'PUT /verifyEac'),
        'one code check at Яндекс Маркет for each code'
    ).toHaveLength(2);
});

test('a test order is marked in «Заказы», walks to «готов к отгрузке» and never appears in «Передачи»', async ({
    ownerPage: page,
    server
}) => {
    const context = page.context();
    const setUp = await setUpYandexExchange(context, server);
    const { yandexKey, key } = setUp;
    const testId = 1000008003;
    const realId = 1000008004;
    await makeFakeYandexOrder(yandexKey, fbsOrder(testId, { fake: true }));
    await makeFakeYandexOrder(yandexKey, fbsOrder(realId));

    await walkToReady(context, server, setUp, testId);
    const realFcId = await walkToReady(context, server, setUp, realId);

    await openScreen(page, '/orders');
    await expect(orderRowOf(page, String(testId)).getByTestId('orders-list-test')).toContainText(
        ruText('orders.test.label')
    );
    await expect(orderRowOf(page, String(realId)).getByTestId('orders-list-test')).toHaveCount(0);
    await expect(orderRowOf(page, String(testId)).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.ready')
    );
    await orderRowOf(page, String(testId)).getByTestId('orders-list-link').click();
    await expect(page.getByTestId('order-details-panel').getByTestId('order-test')).toContainText(
        ruText('orders.test.label')
    );

    // The fulfilment centre hands over only the real order; Яндекс Маркет's shipment holds both, and is confirmed all the same.
    const handoverAtFc = (await fcOrders(key)).find((order) => order.id === realFcId)!.handover_id;
    expect(handoverAtFc, 'the real order stands in a hand-over').not.toBeNull();
    expect(
        (await fcOrders(key)).find((order) => order.external_id === String(testId))!.handover_id,
        'the test order stands in no hand-over at the fulfilment centre'
    ).toBeNull();
    await fcHandoverStep(key, handoverAtFc!, 'request-documents');
    const waiting = await runExchangeUntilHandover(context, server, 'waiting');
    await makeFakeYandexShipment(yandexKey, {
        warehouse_id: FAKE_YANDEX_WAREHOUSE_FBS,
        date: waiting.day,
        kind: 'IMPORT',
        order_ids: [realId, testId],
        cutoff_passed: true
    });
    await runExchangeUntilHandover(context, server, 'documents_sent');

    await page.setViewportSize({ width: 1280, height: 800 });
    await openScreen(page, '/handovers');
    await expect(page.getByTestId('handovers-list-row')).toHaveCount(1);
    await page.getByTestId('handovers-list-link').click();
    const panel = page.getByTestId('handovers-details-panel');
    await expect(panel.getByTestId('handover-facts-orders')).toHaveText(String(realId));
    await expect(panel.getByTestId('handover-facts-batchKind')).toHaveText(
        ruText('marketplace.yandex.batchKind.IMPORT')
    );
    await expect(panel.getByTestId('handover-problem')).toHaveCount(0);
    expect(await page.getByTestId('handovers').textContent(), 'the test order is in «Передачи»').not.toContain(
        String(testId)
    );
    await page.setViewportSize(PHONE);
    await expectNoSidewaysScrollAtEveryWidth(page, "«Передачи» beside a test order's shipment");
    // «Заказы» with a test order marked among real ones fits the phone and stands level at every width.
    await sweepScreen(page, screenOf('/orders'));
});

test('a shipment holding an order the hand-over lacks is shown as a problem naming it, is never confirmed, and goes on once it is taken out', async ({
    ownerPage: page,
    server
}) => {
    const context = page.context();
    const setUp = await setUpYandexExchange(context, server);
    const { yandexKey, key } = setUp;
    const handedId = 1000008005;
    const strayId = 1000008006;
    await makeFakeYandexOrder(yandexKey, fbsOrder(handedId));
    await makeFakeYandexOrder(yandexKey, fbsOrder(strayId));
    const handedFcId = await walkToReady(context, server, setUp, handedId);

    const handoverAtFc = (await fcOrders(key)).find((order) => order.id === handedFcId)!.handover_id;
    await fcHandoverStep(key, handoverAtFc!, 'request-documents');
    const waiting = await runExchangeUntilHandover(context, server, 'waiting');
    // Яндекс Маркет's shipment also holds an order that the fulfilment centre does not hand over.
    const shipment = await makeFakeYandexShipment(yandexKey, {
        warehouse_id: FAKE_YANDEX_WAREHOUSE_FBS,
        date: waiting.day,
        kind: 'WITHDRAW',
        order_ids: [handedId, strayId],
        cutoff_passed: true
    });
    await runExchangeUntilHandover(context, server, 'problem');

    await page.setViewportSize({ width: 1280, height: 800 });
    await openScreen(page, '/handovers');
    await page.getByTestId('handovers-list-link').click();
    const panel = page.getByTestId('handovers-details-panel');
    await expect(panel.getByTestId('handover-state')).toHaveText(ruText('handovers.state.problem'));
    await expect(panel.getByTestId('handover-problem')).toContainText(
        ruText('problem.shipment_differs.differs', { extra: String(strayId), missing: '—' })
    );
    await expect(panel.getByTestId('handover-steps-now')).toHaveText(
        ruText('handovers.now.failed', { number: 2, total: 4, step: ruText('handovers.step.move').toLowerCase() })
    );
    await expect(panel.getByTestId('handover-retry')).toBeVisible();
    // The wyo server never confirms such a shipment and never moves an order out of it.
    expect(await fakeYandexWrites(yandexKey), 'the shipment was confirmed').not.toContain('POST /confirm');
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(panel.getByTestId('handover-problem'), "the hand-over's problem");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Передачи» with a Яндекс Маркет hand-over's problem open");
    await sweepScreen(page, screenOf('/handovers'));

    // The owner takes the stray order out of the shipment in the cabinet and presses «Повторить»: the hand-over goes on.
    await updateFakeYandexShipment(yandexKey, shipment, { order_ids: [handedId] });
    await page.setViewportSize({ width: 1280, height: 800 });
    await openScreen(page, '/handovers');
    await page.getByTestId('handovers-list-link').click();
    await panel.getByTestId('handover-retry').click();
    await runExchangeUntilHandover(context, server, 'documents_sent');
    await expect(page.getByTestId('handovers-list-row')).toHaveAttribute('data-state', 'documents_sent');
    expect((await fakeYandexWrites(yandexKey)).filter((write) => write === 'POST /confirm')).toHaveLength(1);
});
