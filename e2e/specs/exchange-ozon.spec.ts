import { expect, test } from '../fixtures/test.js';
import {
    runExchangeUntil,
    runExchangeUntilHandover,
    setUpOzonExchange,
    ozonLine,
    OZON_PRODUCTS,
    OZON_WAREHOUSE
} from '../helpers/external-api.js';
import { openScreen } from '../helpers/open-screen.js';
import { orderRowOf } from '../helpers/order-rows.js';
import { expectInsidePhoneGutter, PHONE } from '../helpers/phone.js';
import { expectNoSidewaysScrollAtEveryWidth, sweepScreen } from '../helpers/sweep.js';
import {
    fcHandover,
    fcHandoverStep,
    fcOrders,
    fcOrderStep,
    MARKING_CODE,
    setFcProducts
} from '../support/fake-fulfilment.js';
import {
    fakeOzonCalls,
    hoursFromNow,
    makeFakeOzonPosting,
    planFakeOzonCodeCheck,
    type IFakeOzonCall
} from '../support/fake-ozon.js';
import { screenOf } from '../support/screens.js';
import { ruText } from '../support/texts.js';

/**
 * The exchange of an Ozon posting with the fake fulfilment centre, seen in the wyo interface (plan 6, section J): a posting made at
 * the fake Ozon is read and sent, then the fake fulfilment centre's control calls play its part (reserve, pack) and the wyo
 * server's jobs are run at once after each call (`runExchangeUntil`, the test build's `POST /__test/exchange/run`). Nothing is
 * written to Ozon when the goods are reserved; after packing the wyo server reports the packing with the fulfilment centre's
 * parcels, which splits the posting, and attaches each parcel's label. «Заказы» then shows the parcels and the numbers Ozon gave
 * their parts, and «Передачи» a hand-over of four steps. What every state, parcel and step says is proved by the unit tests of
 * `ParcelList`, `HandoverSteps`, `HandoverDetails`, `ProblemLine` and `OrderExchangeDetails`; here they come from the real wyo
 * server and both fakes. Every test walks orders, so each runs on a wyo server of its own, and on keys of its own at the fakes.
 */
test.use({ serverKind: 'own' });

/** The paths of Ozon's methods that change something; a call to one of them is a write the wyo server made. */
const OZON_WRITES: readonly string[] = [
    '/v6/fbs/posting/product/exemplar/set',
    '/v4/posting/fbs/ship',
    '/v3/posting/fbs/package-label/create',
    '/v1/carriage/create',
    '/v1/carriage/set-postings',
    '/v1/carriage/approve',
    '/v1/carriage/cancel'
];

/** The calls the fake Ozon answered with success to the method at `path`. */
const answered = (calls: IFakeOzonCall[], path: string): IFakeOzonCall[] =>
    calls.filter((call) => call.path.endsWith(path) && call.status === 200);

const POSTING = '32100001-0001-1';
/** The number the fake Ozon gives the second part of a posting packed in two parcels (the first part keeps the posting's own). */
const SECOND_PART = '32100001-0001-2';

test('an Ozon posting packed in two parcels shows both parcels in «Заказы» and goes through a hand-over of four steps in «Передачи»', async ({
    ownerPage: page,
    server,
    requests,
    baseURL
}) => {
    const context = page.context();
    const { ozon, key } = await setUpOzonExchange(context, server);
    await makeFakeOzonPosting(ozon.apiKey, {
        posting_number: POSTING,
        order_number: '32100001',
        warehouse_id: OZON_WAREHOUSE.id,
        shipment_date: hoursFromNow(30),
        products: [ozonLine(OZON_PRODUCTS[0], 1), ozonLine(OZON_PRODUCTS[1], 1)]
    });

    // The posting is sent to the fulfilment centre as an order with its two lines, and the fulfilment centre reserves the goods.
    await runExchangeUntil(context, server, POSTING, 'sent');
    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(context, server, POSTING, 'reserved');
    const writtenAtReservation = (await fakeOzonCalls(ozon.apiKey)).filter((call) =>
        OZON_WRITES.some((path) => call.path.endsWith(path))
    );
    expect(writtenAtReservation, 'nothing is written to Ozon when the goods are reserved').toEqual([]);

    // The fulfilment centre packs the first line into the first parcel and the second line into the second one.
    await fcOrderStep(key, fcOrder.id, 'pack', {
        units: [
            { line: 1, parcel: 1 },
            { line: 2, parcel: 2 }
        ]
    });
    await runExchangeUntil(context, server, POSTING, 'label_sent');
    const calls = await fakeOzonCalls(ozon.apiKey);
    expect(answered(calls, '/v4/posting/fbs/ship'), 'the packing is reported once, never again').toHaveLength(1);

    // «Заказы»: the order's details show both parcels, the number Ozon gave each part and the labels attached.
    await openScreen(page, '/orders');
    await expect(orderRowOf(page, POSTING).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.label_sent')
    );
    await orderRowOf(page, POSTING).getByTestId('orders-list-link').click();
    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.label_sent'));
    await expect(details.getByTestId('parcels-1-position')).toHaveText(ruText('parcels.position', { n: 1, total: 2 }));
    await expect(details.getByTestId('parcels-2-position')).toHaveText(ruText('parcels.position', { n: 2, total: 2 }));
    await expect(details.getByTestId('parcels-1-marketplaceNumber')).toHaveText(
        ruText('parcels.fact.marketplaceNumber.same', { number: POSTING })
    );
    await expect(details.getByTestId('parcels-2-marketplaceNumber')).toHaveText(SECOND_PART);
    await expect(details.getByTestId('parcels-1-label')).toContainText(ruText('parcels.label.sent'));
    await expect(details.getByTestId('parcels-2-label')).toContainText(ruText('parcels.label.sent'));
    const history = details.getByTestId('exchange-history-row');
    for (const code of ['packing_finished', 'packing_reported', 'packing_confirmed', 'labels_asked']) {
        await expect(history.filter({ hasText: ruText(`exchange.event.${code}`) }), code).toHaveCount(1);
    }
    await expect(details.getByTestId('exchange-problem')).toHaveCount(0);
    // The fulfilment centre holds the order packed in two parcels, each with its label attached.
    const packed = (await fcOrders(key))[0];
    expect(packed.status).toBe('packed');
    expect(packed.parcels.map((parcel) => parcel.label_attached)).toEqual([true, true]);
    // On a phone and at every wider width the order's details do not push the page sideways.
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(details.getByTestId('parcels'), "the order's parcels");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Заказы» with an Ozon order's details open");

    // «Передачи»: the fulfilment centre asks for the hand-over's documents and names two transport boxes, which Ozon is not told.
    const handoverAtFc = (await fcOrders(key))[0].handover_id;
    expect(handoverAtFc, 'the packed order stands in a hand-over at the fulfilment centre').not.toBeNull();
    await fcHandoverStep(key, handoverAtFc!, 'request-documents', { transport_boxes: 2 });
    await runExchangeUntilHandover(context, server, 'documents_sent');

    // The list of hand-overs is read and clicked at a computer's width; the phone's width is swept below.
    await page.setViewportSize({ width: 1280, height: 800 });
    await openScreen(page, '/handovers');
    const row = page.getByTestId('handovers-list-row');
    await expect(row).toHaveCount(1);
    await expect(row).toHaveAttribute('data-state', 'documents_sent');
    await row.getByTestId('handovers-list-link').click();
    const panel = page.getByTestId('handovers-details-panel');
    await expect(panel.getByTestId('handover-state')).toHaveText(ruText('handovers.state.documents_sent'));
    // Four steps, all done; the steps of boxes and of the shipping point are not drawn at all.
    for (const step of ['batch', 'move', 'close', 'documents']) {
        await expect(panel.getByTestId(`handover-steps-${step}`), step).toHaveAttribute('data-state', 'done');
    }
    for (const step of ['boxes', 'shipping']) {
        await expect(panel.getByTestId(`handover-steps-${step}`), step).toHaveCount(0);
    }
    await expect(panel.getByTestId('handover-steps-now')).toHaveText(ruText('handovers.now.done'));
    await expect(panel.getByTestId('handover-note')).toHaveText(ruText('handovers.note.noBoxes'));
    await expect(panel.getByTestId('handover-facts-boxes')).toHaveText('2');
    await expect(panel.getByTestId('handover-facts-orders')).toContainText(POSTING);
    await expect(panel.getByTestId('handover-facts-batches'), "Ozon's number of the shipment").toHaveText(/\S/);
    await expect(panel.getByTestId('handover-closed')).toHaveText(ruText('handovers.closed'));
    await expect(panel.getByTestId('shipping-point-box')).toHaveCount(0);
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(panel.getByTestId('handover-note'), "the hand-over's note");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Передачи» with an Ozon hand-over's details open");

    // The shipment was approved once, and the fulfilment centre holds the two documents of the hand-over.
    expect(
        answered(await fakeOzonCalls(ozon.apiKey), '/v1/carriage/approve'),
        'the shipment is approved once'
    ).toHaveLength(1);
    const atFc = await fcHandover(key, handoverAtFc!);
    expect(atFc.state).toBe('documents_ready');
    expect(atFc.documents.map((document) => document.kind).sort()).toEqual(['act', 'handover_code']);

    // The pages with an Ozon order and an Ozon hand-over in them fit the phone and stand level at every width.
    await sweepScreen(page, screenOf('/orders'));
    await sweepScreen(page, screenOf('/handovers'));
    // Rule R4: neither the marketplace's key nor a marking code is in a page the owner sees.
    expect(await page.content(), 'the page holds the Ozon key').not.toContain(ozon.apiKey.slice(0, -4));
    // Rule R3: the page asked only the wyo server; the fakes were asked by the wyo server and the test alone.
    expect(requests.outside([baseURL!])).toEqual([]);
});

test('a marking code Ozon refuses shows as a problem in the order with Ozon’s own words, and the code is in no page', async ({
    ownerPage: page,
    server
}) => {
    const context = page.context();
    const article = OZON_PRODUCTS[0];
    const { ozon, key } = await setUpOzonExchange(context, server, {
        products: [{ ...article, mark: 'needed' }, OZON_PRODUCTS[1]]
    });
    // The fulfilment centre's card of the product says each unit carries a code; it must be set before the order is sent.
    await setFcProducts(key, {
        catalog: [{ article: article.offer_id, barcode: article.barcodes![0], marking_required: true }]
    });
    const posting = '32200001-0001-1';
    await makeFakeOzonPosting(ozon.apiKey, {
        posting_number: posting,
        order_number: '32200001',
        warehouse_id: OZON_WAREHOUSE.id,
        shipment_date: hoursFromNow(30),
        products: [ozonLine(article, 1)]
    });
    await runExchangeUntil(context, server, posting, 'sent');
    const [fcOrder] = await fcOrders(key);
    await fcOrderStep(key, fcOrder.id, 'reserve');
    await runExchangeUntil(context, server, posting, 'reserved');

    // Ozon will answer the next set of codes with its own error codes.
    await planFakeOzonCodeCheck(ozon.apiKey, {
        posting_number: posting,
        error_sets: 1,
        error_codes: ['MARK_NOT_FOUND']
    });
    await fcOrderStep(key, fcOrder.id, 'pack', { units: [{ line: 1, marking_code: MARKING_CODE }] });
    await runExchangeUntil(context, server, posting, { problem: 'codes_refused' });

    await openScreen(page, '/orders');
    await expect(orderRowOf(page, posting).getByTestId('orders-list-exchange-state')).toHaveText(
        ruText('exchange.state.problem')
    );
    await orderRowOf(page, posting).getByTestId('orders-list-link').click();
    const details = page.getByTestId('order-details-panel');
    await expect(details.getByTestId('exchange-state')).toHaveText(ruText('exchange.state.problem'));
    await expect(details.getByTestId('exchange-problem')).toContainText(ruText('problem.codes_refused'));
    await expect(details.getByTestId('exchange-problem-words')).toContainText('MARK_NOT_FOUND');
    await expect(details.getByTestId('exchange-verdicts-verdict')).toHaveText(ruText('exchange.verdict.refused'));
    // The packing is not reported to Ozon while its codes are refused.
    const calls = await fakeOzonCalls(ozon.apiKey);
    expect(answered(calls, '/v4/posting/fbs/ship'), 'no packing report while the codes are refused').toHaveLength(0);
    // Rule R4: the marking code is in no page the owner sees.
    expect(await page.content(), 'the page holds a marking code').not.toContain(MARKING_CODE.slice(0, 20));
    await page.setViewportSize(PHONE);
    await expectInsidePhoneGutter(details.getByTestId('exchange-problem'), "the order's problem");
    await expectNoSidewaysScrollAtEveryWidth(page, "«Заказы» with an Ozon order's problem open");
});
